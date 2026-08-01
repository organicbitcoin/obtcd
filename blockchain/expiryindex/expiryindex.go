// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"bytes"
	"errors"
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

var errRebuildInterrupted = errors.New("expiry index rebuild interrupted")

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

type amountAwareChainAccessor interface {
	ForEachUTXOWithAmount(fn func(outpoint wire.OutPoint, height int32, amount int64) error) error
}

// Ensure the ExpiryIndex type implements the indexers.Indexer interface.
var _ indexers.Indexer = (*ExpiryIndex)(nil)
var _ indexers.TipSource = (*ExpiryIndex)(nil)
var _ indexers.InterruptibleIndexer = (*ExpiryIndex)(nil)

// ExpiryIndex tracks UTXOs by expiry height.
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

	// initialized prevents an accessor injected during blockchain startup
	// from rebuilding before Init has loaded the persisted index state.
	initialized bool

	// interrupt is closed when node shutdown is requested.
	interrupt <-chan struct{}
}

// SetInterrupt provides the node shutdown signal used by long-running rebuild
// and catch-up operations.
func (idx *ExpiryIndex) SetInterrupt(interrupt <-chan struct{}) {
	idx.interrupt = interrupt
}

func (idx *ExpiryIndex) interruptRequested() bool {
	if idx.interrupt == nil {
		return false
	}
	select {
	case <-idx.interrupt:
		return true
	default:
		return false
	}
}

// SetChainAccessor injects the blockchain accessor. When called before Init,
// Init performs the rebuild and propagates failures through node startup. When
// called after Init, this method picks up the deferred rebuild and logs errors
// for compatibility with callers that inject the accessor later.
func (idx *ExpiryIndex) SetChainAccessor(chain ChainAccessor) {
	idx.chain = chain

	// When injected before Init, Init itself performs the rebuild and returns
	// any error through the normal blockchain startup path.
	if idx.disabled || !idx.initialized {
		return
	}
	if err := idx.smartRebuild(idx.curTipHeight); err != nil {
		log.Errorf("ExpiryIndex: deferred smartRebuild failed: %v", err)
	}
}

// NewExpiryIndex returns a new expiry index instance.
func NewExpiryIndex(db database.DB, params *chaincfg.Params) (*ExpiryIndex, error) {
	if params == nil {
		return nil, fmt.Errorf("chain params is nil")
	}

	expiryParams := GetExpiryParams(params)

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

// IndexTip returns the authoritative tip represented by the expiry index.
// It allows the generic index manager to synchronize its own tip after a fast
// UTXO-set rebuild.
func (idx *ExpiryIndex) IndexTip() (*chainhash.Hash, int32, error) {
	var tipHash chainhash.Hash
	var tipHeight int32
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		tipHeight, err = dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		tipHash, err = dbGetAccumulatorTipHash(dbTx)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return &tipHash, tipHeight, nil
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
		err = clearExpiryIndexBucketsBatched(idx.db, idx.interrupt)
		if err != nil {
			return fmt.Errorf("failed to reset index for version upgrade: %v", err)
		}
		err = idx.db.Update(func(dbTx database.Tx) error {
			return dbPutIndexVersion(dbTx, CurrentIndexVersion)
		})
		if err != nil {
			return fmt.Errorf("failed to store upgraded index version: %v", err)
		}
		indexTipHeight = -1
	}

	// Set the current tip height
	idx.curTipHeight = indexTipHeight
	idx.initialized = true

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
	idx.devLogf("ExpiryIndex ConnectBlock start height=%d txs=%d stxos=%d",
		blockHeight, len(block.Transactions()), len(stxos))

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
	spentCount := 0
	addedCount := 0
	genesisCreatedCount := 0
	unspendableCount := 0
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
				spentCount++
			}
		}

		// Process new UTXOs (add to index), skip outputs that are never part of
		// the spendable UTXO set represented by validation.
		for voutIdx, txOut := range msgTx.TxOut {
			if !idx.shouldIndexCreateHeight(blockHeight) {
				genesisCreatedCount++
				continue
			}
			if txscript.IsUnspendable(txOut.PkScript) {
				unspendableCount++
				continue
			}

			outpoint := &wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(voutIdx),
			}

			expiryKey := idx.expiryParams.CalculateExpiryKey(blockHeight)
			mh.Add(computeEntryData(outpoint, expiryKey))

			if err := idx.connectTxOut(dbTx, outpoint, blockHeight, txOut.Value); err != nil {
				return fmt.Errorf("failed to connect txout %v: %v", outpoint, err)
			}
			addedCount++
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
	idx.devLogf("ExpiryIndex ConnectBlock done height=%d spent=%d added=%d skippedGenesisCreated=%d skippedUnspendable=%d root=%x",
		blockHeight, spentCount, addedCount, genesisCreatedCount, unspendableCount, mh.Digest())

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
			if !idx.shouldIndexCreateHeight(blockHeight) {
				continue
			}
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
				if !idx.shouldIndexCreateHeight(stxo.Height) {
					continue
				}

				// Re-add to accumulator (reverse of the Remove done in ConnectBlock).
				op := &txIn.PreviousOutPoint
				expiryKey := idx.expiryParams.CalculateExpiryKey(stxo.Height)
				mh.Add(computeEntryData(op, expiryKey))

				if err := idx.connectTxOut(dbTx, op, stxo.Height, stxo.Amount); err != nil {
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

func putTxOutMapping(dbTx database.Tx, outpoint *wire.OutPoint, expiryKey uint64,
	amounts ...int64) error {

	amount := int64(0)
	if len(amounts) > 0 {
		amount = amounts[0]
	}

	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	if outpointBucket == nil {
		return fmt.Errorf("outpoint-to-expiry bucket does not exist")
	}

	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket == nil {
		return fmt.Errorf("expiry-to-outpoints bucket does not exist")
	}

	reapBucket := dbTx.Metadata().Bucket(bktReapStrictCandidates)
	if reapBucket == nil {
		return fmt.Errorf("REAP strict candidate bucket does not exist")
	}

	reapOutpointBucket := dbTx.Metadata().Bucket(bktOutpoint2ReapStrict)
	if reapOutpointBucket == nil {
		return fmt.Errorf("outpoint-to-REAP-strict bucket does not exist")
	}

	if err := outpointBucket.Put(encodeOutPoint(outpoint), encodeExpiryKey(expiryKey)); err != nil {
		return fmt.Errorf("failed to store outpoint mapping: %v", err)
	}

	compositeKey := encodeExpiryOutpointCompositeKey(expiryKey, outpoint)
	if err := expiryBucket.Put(compositeKey, emptyIndexValue); err != nil {
		return fmt.Errorf("failed to store expiry mapping: %v", err)
	}

	reapKey, err := encodeReapStrictCompositeKey(expiryKey, amount, outpoint)
	if err != nil {
		return err
	}
	if err := reapBucket.Put(reapKey, emptyIndexValue); err != nil {
		return fmt.Errorf("failed to store REAP strict mapping: %v", err)
	}
	if err := reapOutpointBucket.Put(encodeOutPoint(outpoint), reapKey); err != nil {
		return fmt.Errorf("failed to store outpoint REAP strict mapping: %v", err)
	}

	return nil
}

// connectTxOut adds a new UTXO to the expiry index
func (idx *ExpiryIndex) connectTxOut(dbTx database.Tx, outpoint *wire.OutPoint,
	createHeight int32, amounts ...int64) error {
	if !idx.shouldIndexCreateHeight(createHeight) {
		return nil
	}

	// Calculate the expiry key for this UTXO
	expiryKey := idx.expiryParams.CalculateExpiryKey(createHeight)
	return putTxOutMapping(dbTx, outpoint, expiryKey, amounts...)
}

func (idx *ExpiryIndex) shouldIndexCreateHeight(createHeight int32) bool {
	// Bitcoin consensus treats the genesis coinbase as unspendable, so it must
	// not become an expiry or REAP candidate even when StartScanHeight is zero.
	return createHeight > 0 && createHeight >= idx.expiryParams.StartScanHeight
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

	expiryKey, err := decodeExpiryKey(encodedExpiryKey)
	if err != nil {
		return fmt.Errorf("failed to decode expiry key: %v", err)
	}

	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket == nil {
		return fmt.Errorf("expiry-to-outpoints bucket does not exist")
	}

	reapBucket := dbTx.Metadata().Bucket(bktReapStrictCandidates)
	if reapBucket == nil {
		return fmt.Errorf("REAP strict candidate bucket does not exist")
	}

	reapOutpointBucket := dbTx.Metadata().Bucket(bktOutpoint2ReapStrict)
	if reapOutpointBucket == nil {
		return fmt.Errorf("outpoint-to-REAP-strict bucket does not exist")
	}

	compositeKey := encodeExpiryOutpointCompositeKey(expiryKey, outpoint)
	if existingValue := expiryBucket.Get(compositeKey); existingValue == nil {
		return fmt.Errorf("inconsistent index: expiry composite key %x not found", compositeKey)
	}

	reapKey := reapOutpointBucket.Get(encodedOutpoint)
	if reapKey == nil {
		return fmt.Errorf("inconsistent index: REAP strict key for outpoint %s not found", outpoint)
	}
	reapKey = append([]byte(nil), reapKey...)
	if existingValue := reapBucket.Get(reapKey); existingValue == nil {
		return fmt.Errorf("inconsistent index: REAP strict composite key %x not found", reapKey)
	}

	if err := outpointBucket.Delete(encodedOutpoint); err != nil {
		return fmt.Errorf("failed to delete outpoint mapping: %v", err)
	}
	if err := expiryBucket.Delete(compositeKey); err != nil {
		return fmt.Errorf("failed to delete expiry mapping: %v", err)
	}
	if err := reapOutpointBucket.Delete(encodedOutpoint); err != nil {
		return fmt.Errorf("failed to delete outpoint REAP strict mapping: %v", err)
	}
	if err := reapBucket.Delete(reapKey); err != nil {
		return fmt.Errorf("failed to delete REAP strict mapping: %v", err)
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
	if err == nil {
		idx.devLogf("ExpiryIndex snapshot tipHeight=%d tipHash=%s root=%x",
			snapshot.TipHeight, snapshot.TipHash, snapshot.Root)
	}
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

// ReapPrefixTipHeight returns the indexed chain tip height represented by the
// REAP prefix source.
func (idx *ExpiryIndex) ReapPrefixTipHeight() (int32, error) {
	if idx.disabled {
		return -1, fmt.Errorf("expiry index is disabled")
	}

	var tipHeight int32
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		tipHeight, err = dbGetTipHeightIndexed(dbTx)
		return err
	})
	return tipHeight, err
}

// ReapPrefixCandidates returns the canonical prefix of globally expired live
// UTXOs at blockHeight.  The returned order matches REAP consensus ordering:
// expiry key, amount, transaction hash bytes, then output index.
func (idx *ExpiryIndex) ReapPrefixCandidates(blockHeight int32,
	limit int) ([]blockchain.ReapPrefixCandidate, error) {

	if idx.disabled {
		return nil, fmt.Errorf("expiry index is disabled")
	}
	if limit <= 0 {
		return nil, nil
	}
	if blockHeight < 0 {
		return nil, nil
	}

	var results []blockchain.ReapPrefixCandidate
	toKey := uint64(blockHeight)
	err := idx.db.View(func(dbTx database.Tx) error {
		reapBucket := dbTx.Metadata().Bucket(bktReapStrictCandidates)
		if reapBucket == nil {
			return fmt.Errorf("REAP strict candidate bucket does not exist")
		}

		cursor := reapBucket.Cursor()
		for found := cursor.First(); found; found = cursor.Next() {
			key := cursor.Key()
			if key == nil {
				break
			}

			expiryKey, amount, outpoint, err := decodeReapStrictCompositeKey(key)
			if err != nil {
				return fmt.Errorf("failed to decode REAP strict composite key: %v", err)
			}
			if expiryKey > toKey {
				break
			}

			results = append(results, blockchain.ReapPrefixCandidate{
				OutPoint:  *outpoint,
				ExpiryKey: expiryKey,
				Amount:    amount,
			})
			if len(results) >= limit {
				break
			}
		}

		return nil
	})

	if err == nil {
		idx.devLogf("ExpiryIndex REAP prefix height=%d limit=%d results=%d",
			blockHeight, limit, len(results))
	}
	return results, err
}

// ScanExpiringUTXOs scans for UTXOs expiring within the specified range.
// This method is used by RPC calls to find UTXOs approaching expiration.
func (idx *ExpiryIndex) ScanExpiringUTXOs(fromKey, toKey uint64,
	maxResults int, startAfter *wire.OutPoint) ([]*ExpiringUTXO, bool, error) {

	if idx.disabled {
		return nil, false, fmt.Errorf("expiry index is disabled")
	}
	if maxResults <= 0 {
		return nil, false, nil
	}

	var results []*ExpiringUTXO
	var hasMore bool

	err := idx.db.View(func(dbTx database.Tx) error {
		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket == nil {
			return fmt.Errorf("expiry-to-outpoints bucket does not exist")
		}

		cursor := expiryBucket.Cursor()

		var (
			found   bool
			seekKey []byte
		)
		if startAfter != nil {
			seekKey = encodeExpiryOutpointCompositeKey(fromKey, startAfter)
			found = cursor.Seek(seekKey)
			if found && bytes.Equal(cursor.Key(), seekKey) {
				found = cursor.Next()
			}
		} else {
			seekKey = expiryCompositePrefix(fromKey)
			found = cursor.Seek(seekKey)
		}

		for found {
			key := cursor.Key()
			if key == nil {
				break
			}

			expiryKey, outpoint, err := decodeExpiryOutpointCompositeKey(key)
			if err != nil {
				return fmt.Errorf("failed to decode expiry composite key: %v", err)
			}
			if expiryKey > toKey {
				break
			}
			if len(results) >= maxResults {
				hasMore = true
				break
			}

			results = append(results, &ExpiringUTXO{
				OutPoint:  *outpoint,
				ExpiryKey: expiryKey,
			})

			found = cursor.Next()
		}

		return nil
	})

	if err == nil {
		idx.devLogf("ExpiryIndex scan from=%d to=%d max=%d startAfter=%v results=%d hasMore=%t",
			fromKey, toKey, maxResults, startAfter, len(results), hasMore)
	}
	return results, hasMore, err
}

func compareOutPoint(a, b *wire.OutPoint) int {
	if a.Hash != b.Hash {
		for i := chainhash.HashSize - 1; i >= 0; i-- {
			switch {
			case a.Hash[i] < b.Hash[i]:
				return -1
			case a.Hash[i] > b.Hash[i]:
				return 1
			}
		}
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
			var (
				lastExpiry uint64
				haveExpiry bool
			)
			for found {
				key := cursor.Key()
				if len(key) != expiryOutpointCompositeKeySize {
					return fmt.Errorf("invalid expiry composite key length: got %d, expected %d",
						len(key), expiryOutpointCompositeKeySize)
				}
				expiryKey, err := decodeExpiryKey(key[:expiryKeyEncodedSize])
				if err != nil {
					return fmt.Errorf("failed to decode expiry key prefix: %v", err)
				}
				if !haveExpiry || expiryKey != lastExpiry {
					stats.TotalExpiryKeys++
					lastExpiry = expiryKey
					haveExpiry = true
				}
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
	if errors.Is(err, errRebuildInterrupted) {
		return err
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
	if idx.interruptRequested() {
		return errRebuildInterrupted
	}

	startTime := time.Now()
	processed := 0

	log.Infof("ExpiryIndex: Starting fast rebuild from UTXO set")

	resetToEmpty := func(logPrefix string, cause error) {
		log.Warnf("%s: %v", logPrefix, cause)
		var clearErr error
		if errors.Is(cause, errRebuildInterrupted) {
			clearErr = idx.db.Update(resetExpiryIndexMetadata)
		} else {
			clearErr = clearExpiryIndexBucketsBatched(idx.db, idx.interrupt)
		}
		if clearErr != nil {
			log.Errorf("ExpiryIndex: failed to reset index after fast rebuild failure: %v", clearErr)
		}
		idx.curTipHeight = -1
	}

	// Clear existing index data then repopulate from the UTXO set.
	// If repopulation fails partway through, we re-clear the index so that
	// the next startup triggers a clean full rebuild rather than leaving a
	// partially-populated index that looks valid.
	err := clearExpiryIndexBucketsBatched(idx.db, idx.interrupt)
	if err != nil {
		return fmt.Errorf("failed to clear index buckets: %w", err)
	}
	idx.curTipHeight = -1

	// Iterate all UTXOs and add qualifying entries, building the
	// MuHash accumulator in memory alongside the index.
	mh := NewMuHash()
	type rebuildBatchEntry struct {
		outpoint  wire.OutPoint
		expiryKey uint64
		amount    int64
	}
	batch := make([]rebuildBatchEntry, 0, DefaultBatchSize)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		if idx.interruptRequested() {
			return errRebuildInterrupted
		}

		if err := idx.db.Update(func(dbTx database.Tx) error {
			for i := range batch {
				entry := batch[i]
				if err := putTxOutMapping(dbTx, &entry.outpoint, entry.expiryKey,
					entry.amount); err != nil {
					return fmt.Errorf("failed to add UTXO %v: %v", entry.outpoint, err)
				}
			}
			return nil
		}); err != nil {
			return err
		}

		batch = batch[:0]
		return nil
	}

	processUTXO := func(outpoint wire.OutPoint, createHeight int32, amount int64) error {
		if idx.interruptRequested() {
			return errRebuildInterrupted
		}
		// Only index UTXOs represented by validation's spendable UTXO set.
		if !idx.shouldIndexCreateHeight(createHeight) {
			return nil
		}

		expiryKey := idx.expiryParams.CalculateExpiryKey(createHeight)
		mh.Add(computeEntryData(&outpoint, expiryKey))
		batch = append(batch, rebuildBatchEntry{
			outpoint:  outpoint,
			expiryKey: expiryKey,
			amount:    amount,
		})
		if len(batch) >= DefaultBatchSize {
			if err := flushBatch(); err != nil {
				return err
			}
		}

		processed++
		if processed%50000 == 0 {
			elapsed := time.Since(startTime)
			rate := float64(processed) / elapsed.Seconds()
			log.Infof("ExpiryIndex: Processed %d UTXOs (%.0f/s)", processed, rate)
		}
		return nil
	}

	var populateErr error
	if amountAware, ok := idx.chain.(amountAwareChainAccessor); ok {
		populateErr = amountAware.ForEachUTXOWithAmount(processUTXO)
	} else {
		populateErr = idx.chain.ForEachUTXO(func(outpoint wire.OutPoint, createHeight int32) error {
			return processUTXO(outpoint, createHeight, 0)
		})
	}
	if populateErr == nil {
		populateErr = flushBatch()
	}
	if populateErr != nil {
		// Re-clear so the index is in a clean empty state rather than
		// partially populated; the next start will retry from scratch.
		resetToEmpty(
			fmt.Sprintf("ExpiryIndex: fast rebuild failed after %d UTXOs, re-clearing index", processed),
			populateErr,
		)
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
		resetToEmpty(
			fmt.Sprintf("ExpiryIndex: fast rebuild finalization failed after %d UTXOs, re-clearing index", processed),
			err,
		)
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
	if idx.interruptRequested() {
		return errRebuildInterrupted
	}

	log.Infof("ExpiryIndex: Incremental catch-up from %d to %d", fromHeight, toHeight)
	startTime := time.Now()

	for height := fromHeight + 1; height <= toHeight; height++ {
		if idx.interruptRequested() {
			return errRebuildInterrupted
		}
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
