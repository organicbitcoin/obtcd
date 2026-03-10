// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/indexers"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const (
	// indexName is the human-readable name for the index.
	indexName = "expiry index"
)

// ChainAccessor provides read-only access to blockchain state needed by the
// expiry index for rebuild and catch-up operations. This interface decouples
// the index from the concrete blockchain.BlockChain type, allowing the chain
// instance to be injected after construction (since indexers are created
// before the blockchain in btcd's initialization sequence).
type ChainAccessor interface {
	// BestHeight returns the current best chain tip height.
	BestHeight() int32

	// BlockByHeight returns the block at the given height on the main chain.
	BlockByHeight(height int32) (*btcutil.Block, error)

	// FetchSpendJournal returns the spent transaction outputs for the
	// given block. The returned slice is a flat list ordered by
	// transaction order (excluding coinbase) then input order.
	FetchSpendJournal(block *btcutil.Block) ([]blockchain.SpentTxOut, error)

	// ForEachUTXO iterates over all unspent outputs and calls fn for each.
	// If fn returns a non-nil error, iteration stops and that error is
	// returned. This is used for fast index rebuild from the UTXO set.
	ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error
}

// Ensure the ExpiryIndex type implements the indexers.Indexer interface.
var _ indexers.Indexer = (*ExpiryIndex)(nil)

// OBTC-only: ExpiryIndex implements UTXO expiry tracking.
// ExpiryIndex implements a UTXO expiry index that tracks when UTXOs will expire.
// It maintains bidirectional mappings to support both fast deletion when UTXOs
// are spent and efficient scanning for expired UTXOs.
//
// The index implements the blockchain.Indexer interface and integrates with
// the blockchain processing pipeline to automatically track UTXO lifecycles.
type ExpiryIndex struct {
	// db is the database instance for the index
	db database.DB

	// params contains the chain parameters
	params *chaincfg.Params

	// expiryParams contains the expiry-specific parameters
	expiryParams *ExpiryParams

	// curTipHeight tracks the current tip height that has been indexed
	curTipHeight int32

	// disabled indicates whether the index is disabled
	disabled bool

	// chain provides access to blockchain state for rebuild operations.
	// May be nil until SetChainAccessor is called.
	chain ChainAccessor
}

// SetChainAccessor injects the blockchain accessor after both the index and
// the blockchain have been constructed. Because indexers are created before
// the blockchain in btcd's initialization sequence, Init() runs with
// chain == nil and defers any rebuild/catch-up work. This method picks up
// that deferred work by running smartRebuild once the chain is available.
func (idx *ExpiryIndex) SetChainAccessor(chain ChainAccessor) {
	idx.chain = chain

	// Run deferred rebuild now that chain state is accessible.
	if idx.disabled {
		return
	}
	if err := idx.smartRebuild(idx.curTipHeight); err != nil {
		log.Errorf("ExpiryIndex: deferred smartRebuild failed: %v", err)
	}
}

// NewExpiryIndex returns a new instance of an expiry index.
//
// Returns an error if the network doesn't support OBTC expiry
// (i.e., it's a Bitcoin network rather than an OBTC network).
func NewExpiryIndex(db database.DB, params *chaincfg.Params) (*ExpiryIndex, error) {
	if params == nil {
		return nil, fmt.Errorf("chain params is nil")
	}

	// Get expiry parameters for this network
	expiryParams := GetExpiryParams(params)

	// Return error if this is not an OBTC network
	if expiryParams == nil {
		return nil, fmt.Errorf("expiry index not supported for network %s", params.Name)
	}

	disabled := false

	index := &ExpiryIndex{
		db:           db,
		params:       params,
		expiryParams: expiryParams,
		curTipHeight: -1, // Will be set during Init()
		disabled:     disabled,
	}

	return index, nil
}

// Key returns the database key to use for the index as a byte slice.
func (idx *ExpiryIndex) Key() []byte {
	return []byte("expiryindex")
}

// Name returns the human-readable name of the index.
func (idx *ExpiryIndex) Name() string {
	return indexName
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.
func (idx *ExpiryIndex) Create(dbTx database.Tx) error {
	// Do nothing if the index is disabled
	if idx.disabled {
		return nil
	}

	// Create the buckets for the expiry index
	if err := createExpiryIndexBuckets(dbTx); err != nil {
		return fmt.Errorf("failed to create expiry index buckets: %v", err)
	}

	// Initialize the index version
	if err := dbPutIndexVersion(dbTx, CurrentIndexVersion); err != nil {
		return fmt.Errorf("failed to store index version: %v", err)
	}

	return nil
}

// Init initializes the expiry index. This is part of the blockchain.Indexer
// interface.
func (idx *ExpiryIndex) Init() error {
	// Do nothing if the index is disabled
	if idx.disabled {
		return nil
	}

	// Validate that we have expiry parameters
	if idx.expiryParams == nil {
		return fmt.Errorf("expiry parameters not available for network")
	}

	// Open a read transaction to get the current tip height
	var indexTipHeight int32 = -1
	var storedVersion uint16
	err := idx.db.View(func(dbTx database.Tx) error {
		// Check index version compatibility
		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return fmt.Errorf("failed to get index version: %v", err)
		}
		storedVersion = version

		// Get the current tip height
		indexTipHeight, err = dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return fmt.Errorf("failed to get tip height: %v", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	if storedVersion != 0 && storedVersion != CurrentIndexVersion {
		log.Infof("ExpiryIndex: resetting index for version upgrade %d -> %d",
			storedVersion, CurrentIndexVersion)
		err = idx.db.Update(func(dbTx database.Tx) error {
			if err := idx.clearIndexBuckets(dbTx); err != nil {
				return err
			}
			if err := dbPutTipHeightIndexed(dbTx, -1); err != nil {
				return err
			}
			return dbPutIndexVersion(dbTx, CurrentIndexVersion)
		})
		if err != nil {
			return fmt.Errorf("failed to reset index for version upgrade: %v", err)
		}
		indexTipHeight = -1
	}

	// Set the current tip height
	idx.curTipHeight = indexTipHeight

	// If the chain accessor is available, run smart rebuild now.
	// Otherwise defer it to SetChainAccessor (the normal startup path,
	// since indexers are created before the blockchain instance).
	if idx.chain != nil {
		return idx.smartRebuild(indexTipHeight)
	}

	log.Infof("ExpiryIndex: Init complete (tip=%d), rebuild deferred until chain accessor is set", indexTipHeight)
	return nil
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain. This indexer adds entries for all the new
// UTXOs created by the block and maintains the MuHash accumulator.
//
// If the expiry commitment is active at this height, the pre-state root
// (i.e. Root_{n-1}) is validated against the coinbase commitment before
// processing any transactions.
func (idx *ExpiryIndex) ConnectBlock(dbTx database.Tx, block *btcutil.Block,
	stxos []blockchain.SpentTxOut) error {

	// Do nothing if the index is disabled
	if idx.disabled {
		return nil
	}

	// Check if indexing is enabled at this height
	blockHeight := block.Height()
	if !idx.expiryParams.IsIndexingEnabled(blockHeight) {
		// Update tip height but don't process the block
		if err := dbPutTipHeightIndexed(dbTx, blockHeight); err != nil {
			return err
		}
		return dbPutAccumulatorTipHash(dbTx, block.Hash())
	}

	// Load current accumulator state (represents Root_{n-1}).
	mh, err := dbGetAccumulatorState(dbTx)
	if err != nil {
		return fmt.Errorf("failed to load accumulator: %v", err)
	}

	// Validate the expiry commitment before processing (pre-state commitment).
	if err := idx.validateExpiryCommitment(block, mh.Digest()); err != nil {
		return err
	}

	// Process all transactions in the block.
	blockHash := block.Hash()
	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	for txIdx, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		// Skip the coinbase transaction inputs (they don't spend existing UTXOs)
		if !blockchain.IsCoinBaseTx(msgTx) {
			// Process spent UTXOs (remove from index)
			for _, txIn := range msgTx.TxIn {
				op := &txIn.PreviousOutPoint

				// Update accumulator before removing from index.
				encodedEK := outpointBucket.Get(encodeOutPoint(op))
				if encodedEK != nil {
					ek, _ := decodeExpiryKey(encodedEK)
					mh.Remove(computeEntryData(op, ek))
				}

				if err := idx.disconnectTxOut(dbTx, op); err != nil {
					return fmt.Errorf("failed to disconnect txout %v: %v",
						txIn.PreviousOutPoint, err)
				}
			}
		}

		// Process new UTXOs (add to index), skip provably unspendable outputs.
		for voutIdx, txOut := range msgTx.TxOut {
			if txscript.IsUnspendable(txOut.PkScript) {
				continue
			}

			outpoint := &wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(voutIdx),
			}

			expiryKey := idx.expiryParams.CalculateExpiryKey(blockHeight)
			mh.Add(computeEntryData(outpoint, expiryKey))

			if err := idx.connectTxOut(dbTx, outpoint, blockHeight); err != nil {
				return fmt.Errorf("failed to connect txout %v: %v", outpoint, err)
			}
		}

		// Log progress for large blocks
		if txIdx > 0 && txIdx%1000 == 0 {
			log.Debugf("Processed %d/%d transactions in block %v",
				txIdx, len(block.Transactions()), blockHash)
		}
	}

	// Store updated accumulator (now represents Root_n).
	if err := dbPutAccumulatorState(dbTx, mh); err != nil {
		return fmt.Errorf("failed to store accumulator: %v", err)
	}
	if err := dbPutAccumulatorTipHash(dbTx, block.Hash()); err != nil {
		return fmt.Errorf("failed to store accumulator tip hash: %v", err)
	}

	// Update the tip height
	if err := dbPutTipHeightIndexed(dbTx, blockHeight); err != nil {
		return fmt.Errorf("failed to update tip height: %v", err)
	}

	// Update our internal state
	idx.curTipHeight = blockHeight

	return nil
}

// DisconnectBlock is invoked by the index manager when a block has been
// disconnected from the main chain. This indexer removes entries for all
// the UTXOs created by the block and rolls back the MuHash accumulator.
func (idx *ExpiryIndex) DisconnectBlock(dbTx database.Tx, block *btcutil.Block,
	stxos []blockchain.SpentTxOut) error {

	// Do nothing if the index is disabled
	if idx.disabled {
		return nil
	}

	blockHeight := block.Height()

	// Check if indexing was enabled at this height
	if !idx.expiryParams.IsIndexingEnabled(blockHeight) {
		// Update tip height but don't process the block
		if err := dbPutTipHeightIndexed(dbTx, blockHeight-1); err != nil {
			return err
		}
		prevHash := &block.MsgBlock().Header.PrevBlock
		return dbPutAccumulatorTipHash(dbTx, prevHash)
	}

	// Load current accumulator state.
	mh, err := dbGetAccumulatorState(dbTx)
	if err != nil {
		return fmt.Errorf("failed to load accumulator: %v", err)
	}

	// Process all transactions in the block.
	//
	// The stxos slice is a flat list ordered by transaction order (excluding
	// the coinbase) then by input order within each transaction. We build a
	// per-transaction offset map in forward order so we can look up the
	// correct stxo for each input regardless of processing direction.
	transactions := block.Transactions()

	// Build stxo offset map: txIdx -> starting offset into stxos.
	stxoOffsets := make([]int, len(transactions))
	offset := 0
	for txIdx, tx := range transactions {
		stxoOffsets[txIdx] = offset
		if txIdx != 0 { // skip coinbase
			offset += len(tx.MsgTx().TxIn)
		}
	}

	for txIdx := len(transactions) - 1; txIdx >= 0; txIdx-- {
		tx := transactions[txIdx]
		msgTx := tx.MsgTx()

		// Remove new UTXOs that were created by this block (reverse of Add).
		for voutIdx := len(msgTx.TxOut) - 1; voutIdx >= 0; voutIdx-- {
			if txscript.IsUnspendable(msgTx.TxOut[voutIdx].PkScript) {
				continue
			}

			outpoint := &wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(voutIdx),
			}

			expiryKey := idx.expiryParams.CalculateExpiryKey(blockHeight)
			mh.Remove(computeEntryData(outpoint, expiryKey))

			if err := idx.disconnectTxOut(dbTx, outpoint); err != nil {
				return fmt.Errorf("failed to disconnect txout %v: %v", outpoint, err)
			}
		}

		// Restore spent UTXOs (re-add to index, reverse of Remove).
		// Skip coinbase (txIdx 0) as it doesn't spend existing UTXOs.
		if txIdx != 0 {
			baseOffset := stxoOffsets[txIdx]
			for vinIdx, txIn := range msgTx.TxIn {
				stxoIdx := baseOffset + vinIdx
				if stxoIdx >= len(stxos) {
					return fmt.Errorf("stxo index %d out of range (len=%d) for tx %d input %d",
						stxoIdx, len(stxos), txIdx, vinIdx)
				}
				stxo := stxos[stxoIdx]
				if stxo.Height < idx.expiryParams.StartScanHeight {
					continue
				}

				// Re-add to accumulator (reverse of the Remove done in ConnectBlock).
				op := &txIn.PreviousOutPoint
				expiryKey := idx.expiryParams.CalculateExpiryKey(stxo.Height)
				mh.Add(computeEntryData(op, expiryKey))

				if err := idx.connectTxOut(dbTx, op, stxo.Height); err != nil {
					return fmt.Errorf("failed to reconnect txout %v: %v",
						txIn.PreviousOutPoint, err)
				}
			}
		}
	}

	// Store rolled-back accumulator (now represents Root_{n-1}).
	if err := dbPutAccumulatorState(dbTx, mh); err != nil {
		return fmt.Errorf("failed to store accumulator: %v", err)
	}
	prevHash := &block.MsgBlock().Header.PrevBlock
	if err := dbPutAccumulatorTipHash(dbTx, prevHash); err != nil {
		return fmt.Errorf("failed to store accumulator tip hash: %v", err)
	}

	// Update the tip height
	if err := dbPutTipHeightIndexed(dbTx, blockHeight-1); err != nil {
		return fmt.Errorf("failed to update tip height: %v", err)
	}

	// Update our internal state
	idx.curTipHeight = blockHeight - 1

	return nil
}

// connectTxOut adds a new UTXO to the expiry index
func (idx *ExpiryIndex) connectTxOut(dbTx database.Tx, outpoint *wire.OutPoint,
	createHeight int32) error {
	if createHeight < idx.expiryParams.StartScanHeight {
		return nil
	}

	// Calculate the expiry key for this UTXO
	expiryKey := idx.expiryParams.CalculateExpiryKey(createHeight)

	// Encode the outpoint and expiry key
	encodedOutpoint := encodeOutPoint(outpoint)
	encodedExpiryKey := encodeExpiryKey(expiryKey)

	// Add the outpoint -> expiry mapping
	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	if outpointBucket == nil {
		return fmt.Errorf("outpoint-to-expiry bucket does not exist")
	}

	if err := outpointBucket.Put(encodedOutpoint, encodedExpiryKey); err != nil {
		return fmt.Errorf("failed to store outpoint mapping: %v", err)
	}

	// Add to the expiry -> outpoints mapping
	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket == nil {
		return fmt.Errorf("expiry-to-outpoints bucket does not exist")
	}

	// Get existing outpoint list for this expiry key
	existingEncoded := expiryBucket.Get(encodedExpiryKey)

	var newEncoded []byte
	var err error

	if existingEncoded == nil {
		// First outpoint for this expiry key
		newEncoded = encodeOutPointList([]*wire.OutPoint{outpoint})
	} else {
		// Append to existing list
		newEncoded, err = appendOutPointToList(existingEncoded, outpoint)
		if err != nil {
			return fmt.Errorf("failed to append outpoint to list: %v", err)
		}
	}

	// Store the updated list
	if err := expiryBucket.Put(encodedExpiryKey, newEncoded); err != nil {
		return fmt.Errorf("failed to store expiry mapping: %v", err)
	}

	return nil
}

// disconnectTxOut removes a UTXO from the expiry index
func (idx *ExpiryIndex) disconnectTxOut(dbTx database.Tx, outpoint *wire.OutPoint) error {
	// Encode the outpoint for lookup
	encodedOutpoint := encodeOutPoint(outpoint)

	// Get the expiry key for this outpoint
	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	if outpointBucket == nil {
		return fmt.Errorf("outpoint-to-expiry bucket does not exist")
	}

	encodedExpiryKey := outpointBucket.Get(encodedOutpoint)
	if encodedExpiryKey == nil {
		// Not found - this could be a UTXO that was created before indexing started
		return nil
	}

	// Remove the outpoint -> expiry mapping
	if err := outpointBucket.Delete(encodedOutpoint); err != nil {
		return fmt.Errorf("failed to delete outpoint mapping: %v", err)
	}

	// Update the expiry -> outpoints mapping
	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket == nil {
		return fmt.Errorf("expiry-to-outpoints bucket does not exist")
	}

	existingEncoded := expiryBucket.Get(encodedExpiryKey)
	if existingEncoded == nil {
		// This shouldn't happen if the index is consistent
		return fmt.Errorf("inconsistent index: expiry key %x not found", encodedExpiryKey)
	}

	// Remove the outpoint from the list
	newEncoded, err := removeOutPointFromList(existingEncoded, outpoint)
	if err != nil {
		return fmt.Errorf("failed to remove outpoint from list: %v", err)
	}

	if newEncoded == nil {
		// List is now empty, delete the entire key
		if err := expiryBucket.Delete(encodedExpiryKey); err != nil {
			return fmt.Errorf("failed to delete empty expiry mapping: %v", err)
		}
	} else {
		// Update the list
		if err := expiryBucket.Put(encodedExpiryKey, newEncoded); err != nil {
			return fmt.Errorf("failed to update expiry mapping: %v", err)
		}
	}

	return nil
}

// isCommitmentActive returns true if the expiry commitment is mandatory
// at the given block height.
func (idx *ExpiryIndex) isCommitmentActive(height int32) bool {
	h := idx.expiryParams.ExpiryCommitmentEnableAtHeight
	return h > 0 && height >= h
}

// validateExpiryCommitment checks the expiry commitment in the coinbase
// transaction against the expected pre-state root.
//
// Before the activation height, this is a no-op. After activation, blocks
// must contain exactly one valid expiry commitment matching the expected root.
func (idx *ExpiryIndex) validateExpiryCommitment(
	block *btcutil.Block, expectedRoot [AccumulatorDigestSize]byte) error {

	blockHeight := block.Height()
	if !idx.isCommitmentActive(blockHeight) {
		return nil
	}

	coinbaseTx := block.Transactions()[0]

	// Check for duplicate commitments.
	if CountExpiryCommitments(coinbaseTx) > 1 {
		return blockchain.RuleError{
			ErrorCode:   blockchain.ErrBadExpiryCommitmentDuplicate,
			Description: fmt.Sprintf("block %d has multiple expiry commitments", blockHeight),
		}
	}

	// Extract the commitment.
	version, root, found := ExtractExpiryCommitment(coinbaseTx)
	if !found {
		return blockchain.RuleError{
			ErrorCode:   blockchain.ErrBadExpiryCommitmentMissing,
			Description: fmt.Sprintf("block %d missing required expiry commitment", blockHeight),
		}
	}

	// Check version.
	if version != ExpiryCommitmentVersion {
		return blockchain.RuleError{
			ErrorCode: blockchain.ErrBadExpiryCommitmentFormat,
			Description: fmt.Sprintf("block %d has unsupported expiry commitment version %d",
				blockHeight, version),
		}
	}

	// Compare root.
	if root != expectedRoot {
		return blockchain.RuleError{
			ErrorCode: blockchain.ErrBadExpiryCommitmentMismatch,
			Description: fmt.Sprintf("block %d expiry commitment mismatch: "+
				"coinbase=%x, expected=%x", blockHeight, root, expectedRoot),
		}
	}

	return nil
}

// GetAccumulatorSnapshot returns the current accumulator root together with the
// indexed tip it corresponds to. Mining uses this to avoid building a block on
// one tip while embedding a root for another.
func (idx *ExpiryIndex) GetAccumulatorSnapshot() (AccumulatorSnapshot, error) {
	var snapshot AccumulatorSnapshot
	if idx.disabled {
		return snapshot, fmt.Errorf("expiry index is disabled")
	}
	err := idx.db.View(func(dbTx database.Tx) error {
		mh, err := dbGetAccumulatorState(dbTx)
		if err != nil {
			return err
		}
		snapshot.Root = mh.Digest()
		snapshot.TipHeight, err = dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		snapshot.TipHash, err = dbGetAccumulatorTipHash(dbTx)
		if err != nil {
			return err
		}
		return nil
	})
	return snapshot, err
}

// GetAccumulatorDigest returns the current MuHash digest (Root at current tip).
func (idx *ExpiryIndex) GetAccumulatorDigest() ([AccumulatorDigestSize]byte, error) {
	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		return [AccumulatorDigestSize]byte{}, err
	}
	return snapshot.Root, nil
}

// ScanExpiringUTXOs scans for UTXOs expiring within the specified range.
// This method is used by RPC calls to find UTXOs approaching expiration.
func (idx *ExpiryIndex) ScanExpiringUTXOs(fromKey, toKey uint64,
	maxResults int, startAfter *wire.OutPoint) ([]*ExpiringUTXO, bool, error) {

	if idx.disabled {
		return nil, false, fmt.Errorf("expiry index is disabled")
	}

	var results []*ExpiringUTXO
	var hasMore bool

	err := idx.db.View(func(dbTx database.Tx) error {
		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket == nil {
			return fmt.Errorf("expiry-to-outpoints bucket does not exist")
		}

		cursor := expiryBucket.Cursor()

		// Start from the fromKey
		encodedFromKey := encodeExpiryKey(fromKey)
		found := cursor.Seek(encodedFromKey)
		if !found {
			return nil
		}

		// Now we can safely get the key and value since found is true
	outer:
		for found && len(results) < maxResults {
			key := cursor.Key()
			value := cursor.Value()

			if key == nil {
				break
			}
			// Decode the expiry key
			expiryKey, err := decodeExpiryKey(key)
			if err != nil {
				return fmt.Errorf("failed to decode expiry key: %v", err)
			}

			// Stop if we've exceeded the range
			if expiryKey > toKey {
				break
			}

			// Decode the outpoint list
			outpoints, err := decodeOutPointList(value)
			if err != nil {
				return fmt.Errorf("failed to decode outpoint list: %v", err)
			}

			startIdx := 0
			if startAfter != nil && expiryKey == fromKey {
				startIdx = findOutPointStartIndex(outpoints, startAfter)
			}

			// Add each outpoint to results
			for i := startIdx; i < len(outpoints); i++ {
				outpoint := outpoints[i]
				if len(results) >= maxResults {
					// Check if there are more results in-range
					if i < len(outpoints) {
						hasMore = true
					} else if cursor.Next() {
						nextKey := cursor.Key()
						if nextKey != nil {
							nextExpiryKey, err := decodeExpiryKey(nextKey)
							if err == nil && nextExpiryKey <= toKey {
								hasMore = true
							}
						}
					}
					break outer
				}

				results = append(results, &ExpiringUTXO{
					OutPoint:  *outpoint,
					ExpiryKey: expiryKey,
				})

				if len(results) >= maxResults {
					// Determine if more results remain
					if i+1 < len(outpoints) {
						hasMore = true
					} else if cursor.Next() {
						nextKey := cursor.Key()
						if nextKey != nil {
							nextExpiryKey, err := decodeExpiryKey(nextKey)
							if err == nil && nextExpiryKey <= toKey {
								hasMore = true
							}
						}
					}
					break outer
				}
			}

			// Move to next key
			found = cursor.Next()
		}

		return nil
	})

	return results, hasMore, err
}

func compareOutPoint(a, b *wire.OutPoint) int {
	if a.Hash != b.Hash {
		if a.Hash.String() < b.Hash.String() {
			return -1
		}
		return 1
	}
	switch {
	case a.Index < b.Index:
		return -1
	case a.Index > b.Index:
		return 1
	default:
		return 0
	}
}

// findOutPointStartIndex returns the first index whose outpoint is greater than startAfter.
func findOutPointStartIndex(outpoints []*wire.OutPoint, startAfter *wire.OutPoint) int {
	if startAfter == nil || len(outpoints) == 0 {
		return 0
	}
	low := 0
	high := len(outpoints)
	for low < high {
		mid := (low + high) / 2
		if compareOutPoint(outpoints[mid], startAfter) <= 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low
}

// GetStats returns statistics about the expiry index
func (idx *ExpiryIndex) GetStats() (*ExpiryIndexStats, error) {
	if idx.disabled {
		return &ExpiryIndexStats{Disabled: true}, nil
	}

	var stats ExpiryIndexStats

	err := idx.db.View(func(dbTx database.Tx) error {
		// Get current tip height
		tipHeight, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		stats.TipHeight = tipHeight

		// Count entries in each bucket
		outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
		if outpointBucket != nil {
			cursor := outpointBucket.Cursor()
			found := cursor.First()
			for found {
				stats.TotalUTXOs++
				found = cursor.Next()
			}
		}

		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket != nil {
			cursor := expiryBucket.Cursor()
			found := cursor.First()
			for found {
				stats.TotalExpiryKeys++
				found = cursor.Next()
			}
		}

		return nil
	})

	return &stats, err
}

// ExpiringUTXO represents a UTXO that is scheduled to expire
type ExpiringUTXO struct {
	OutPoint  wire.OutPoint
	ExpiryKey uint64
}

// ExpiryIndexStats contains statistics about the expiry index
type ExpiryIndexStats struct {
	Disabled        bool
	TipHeight       int32
	TotalUTXOs      int
	TotalExpiryKeys int
}

// smartRebuild determines the optimal strategy for building/updating the index
func (idx *ExpiryIndex) smartRebuild(indexTipHeight int32) error {
	// Get the current blockchain tip height
	// Note: This should be provided by the blockchain instance
	// For now, we'll assume it's accessible via a method
	chainTipHeight := idx.getChainTipHeight()

	// Calculate the lag between index and chain
	lag := chainTipHeight - indexTipHeight

	// Choose rebuild strategy based on lag
	const fastRebuildThreshold = 1000 // blocks

	if indexTipHeight == -1 {
		// First time initialization - check if we can use fast rebuild
		log.Infof("ExpiryIndex: First initialization, checking for fast rebuild option")
		return idx.tryFastRebuildOrFallback(chainTipHeight)
	} else if lag > fastRebuildThreshold {
		// Significant lag - fast rebuild is more efficient
		log.Infof("ExpiryIndex: Significant lag detected (%d blocks), using fast rebuild", lag)
		return idx.tryFastRebuildOrFallback(chainTipHeight)
	} else if lag > 0 {
		// Small lag - incremental catch-up is more efficient
		log.Infof("ExpiryIndex: Small lag detected (%d blocks), using incremental catch-up", lag)
		return idx.incrementalCatchUp(indexTipHeight, chainTipHeight)
	}

	// Index is up to date
	log.Debugf("ExpiryIndex: Already up to date at height %d", indexTipHeight)
	return nil
}

// tryFastRebuildOrFallback attempts fast UTXO-based rebuild, falls back to incremental
func (idx *ExpiryIndex) tryFastRebuildOrFallback(chainTipHeight int32) error {
	// Try fast rebuild first
	err := idx.fastRebuildFromUTXO(chainTipHeight)
	if err == nil {
		log.Infof("ExpiryIndex: Fast rebuild completed successfully")
		return nil
	}

	// Log the fast rebuild failure and fall back
	log.Warnf("ExpiryIndex: Fast rebuild failed (%v), falling back to incremental", err)
	return idx.incrementalCatchUp(idx.curTipHeight, chainTipHeight)
}

// fastRebuildFromUTXO rebuilds the index by iterating the current UTXO set
// via the ChainAccessor.ForEachUTXO callback.
func (idx *ExpiryIndex) fastRebuildFromUTXO(chainTipHeight int32) error {
	if err := idx.requireChain(); err != nil {
		return err
	}

	startTime := time.Now()
	processed := 0

	log.Infof("ExpiryIndex: Starting fast rebuild from UTXO set")

	// Clear existing index data then repopulate from the UTXO set.
	// If repopulation fails partway through, we re-clear the index so that
	// the next startup triggers a clean full rebuild rather than leaving a
	// partially-populated index that looks valid.
	err := idx.db.Update(func(dbTx database.Tx) error {
		return idx.clearIndexBuckets(dbTx)
	})
	if err != nil {
		return fmt.Errorf("failed to clear index buckets: %v", err)
	}

	// Iterate all UTXOs and add qualifying entries, building the
	// MuHash accumulator in memory alongside the index.
	mh := NewMuHash()
	populateErr := idx.chain.ForEachUTXO(func(outpoint wire.OutPoint, createHeight int32) error {
		// Only index UTXOs created at or after the scan start height.
		if createHeight < idx.expiryParams.StartScanHeight {
			return nil
		}

		expiryKey := idx.expiryParams.CalculateExpiryKey(createHeight)
		mh.Add(computeEntryData(&outpoint, expiryKey))

		err := idx.db.Update(func(dbTx database.Tx) error {
			return idx.connectTxOut(dbTx, &outpoint, createHeight)
		})
		if err != nil {
			return fmt.Errorf("failed to add UTXO %v: %v", outpoint, err)
		}

		processed++
		if processed%50000 == 0 {
			elapsed := time.Since(startTime)
			rate := float64(processed) / elapsed.Seconds()
			log.Infof("ExpiryIndex: Processed %d UTXOs (%.0f/s)", processed, rate)
		}
		return nil
	})
	if populateErr != nil {
		// Re-clear so the index is in a clean empty state rather than
		// partially populated; the next start will retry from scratch.
		log.Warnf("ExpiryIndex: fast rebuild failed after %d UTXOs, re-clearing index: %v",
			processed, populateErr)
		_ = idx.db.Update(func(dbTx database.Tx) error {
			return idx.clearIndexBuckets(dbTx)
		})
		return populateErr
	}

	// Mark index as complete and store the accumulator.
	err = idx.db.Update(func(dbTx database.Tx) error {
		if err := dbPutAccumulatorState(dbTx, mh); err != nil {
			return fmt.Errorf("failed to store accumulator: %v", err)
		}
		if chainTipHeight >= 0 {
			block, err := idx.getBlockByHeight(chainTipHeight)
			if err != nil {
				return fmt.Errorf("failed to get block hash for height %d: %v", chainTipHeight, err)
			}
			if err := dbPutAccumulatorTipHash(dbTx, block.Hash()); err != nil {
				return fmt.Errorf("failed to store accumulator tip hash: %v", err)
			}
		} else {
			var zero chainhash.Hash
			if err := dbPutAccumulatorTipHash(dbTx, &zero); err != nil {
				return fmt.Errorf("failed to reset accumulator tip hash: %v", err)
			}
		}
		if err := dbPutTipHeightIndexed(dbTx, chainTipHeight); err != nil {
			return fmt.Errorf("failed to update tip height: %v", err)
		}
		idx.curTipHeight = chainTipHeight
		return nil
	})
	if err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	log.Infof("ExpiryIndex: Fast rebuild completed - %d UTXOs in %.2fs (%.0f/s)",
		processed, elapsed.Seconds(), float64(processed)/elapsed.Seconds())

	return nil
}

// incrementalCatchUp processes blocks incrementally to catch up the index
func (idx *ExpiryIndex) incrementalCatchUp(fromHeight, toHeight int32) error {
	if fromHeight >= toHeight {
		return nil
	}

	log.Infof("ExpiryIndex: Incremental catch-up from %d to %d", fromHeight, toHeight)
	startTime := time.Now()

	for height := fromHeight + 1; height <= toHeight; height++ {
		// Get block at height
		block, err := idx.getBlockByHeight(height)
		if err != nil {
			return fmt.Errorf("failed to get block at height %d: %v", height, err)
		}

		// Get spent transaction outputs for this block
		stxos, err := idx.getSpentTxOuts(height)
		if err != nil {
			return fmt.Errorf("failed to get spent txouts for height %d: %v", height, err)
		}

		// Update index with this block
		err = idx.db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, block, stxos)
		})
		if err != nil {
			return fmt.Errorf("failed to connect block %d: %v", height, err)
		}

		// Log progress periodically
		if height%1000 == 0 || height == toHeight {
			elapsed := time.Since(startTime)
			remaining := toHeight - height
			blocksProcessed := height - fromHeight
			rate := float64(blocksProcessed) / elapsed.Seconds()
			eta := time.Duration(float64(remaining)/rate) * time.Second

			log.Infof("ExpiryIndex: Progress %d/%d (%.1f blocks/s, ETA: %v)",
				height, toHeight, rate, eta)
		}
	}

	elapsed := time.Since(startTime)
	blocksProcessed := toHeight - fromHeight
	log.Infof("ExpiryIndex: Incremental catch-up completed - %d blocks in %.2fs (%.1f blocks/s)",
		blocksProcessed, elapsed.Seconds(), float64(blocksProcessed)/elapsed.Seconds())

	return nil
}

// Helper methods for UTXO access and blockchain interaction

// clearIndexBuckets clears all expiry index data
func (idx *ExpiryIndex) clearIndexBuckets(dbTx database.Tx) error {
	return clearExpiryIndexBuckets(dbTx)
}

// requireChain returns an error if the chain accessor has not been set.
func (idx *ExpiryIndex) requireChain() error {
	if idx.chain == nil {
		return fmt.Errorf("chain accessor not set - call SetChainAccessor first")
	}
	return nil
}

// getChainTipHeight gets the current blockchain tip height.
func (idx *ExpiryIndex) getChainTipHeight() int32 {
	if idx.chain == nil {
		return idx.curTipHeight
	}
	return idx.chain.BestHeight()
}

// getBlockByHeight retrieves a block by its height.
func (idx *ExpiryIndex) getBlockByHeight(height int32) (*btcutil.Block, error) {
	if err := idx.requireChain(); err != nil {
		return nil, err
	}
	return idx.chain.BlockByHeight(height)
}

// getSpentTxOuts retrieves spent transaction outputs for a block at the given height.
func (idx *ExpiryIndex) getSpentTxOuts(height int32) ([]blockchain.SpentTxOut, error) {
	if err := idx.requireChain(); err != nil {
		return nil, err
	}
	block, err := idx.chain.BlockByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to get block at height %d: %v", height, err)
	}
	return idx.chain.FetchSpendJournal(block)
}
