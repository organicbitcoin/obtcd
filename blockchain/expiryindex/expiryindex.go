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
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

const (
	// indexName is the human-readable name for the index.
	indexName = "expiry index"
)

// Ensure the ExpiryIndex type implements the indexers.Indexer interface.
var _ indexers.Indexer = (*ExpiryIndex)(nil)

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
}

// NewExpiryIndex returns a new instance of an expiry index.
//
// Returns an error if the network doesn't support OBTC expiry
// (i.e., it's a Bitcoin network rather than an OBTC network).
func NewExpiryIndex(db database.DB, params *chaincfg.Params) (*ExpiryIndex, error) {
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
	err := idx.db.View(func(dbTx database.Tx) error {
		// Check index version compatibility
		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return fmt.Errorf("failed to get index version: %v", err)
		}
		
		if version != 0 && version != CurrentIndexVersion {
			return fmt.Errorf("index version mismatch: got %d, expected %d", 
				version, CurrentIndexVersion)
		}
		
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
	
	// Set the current tip height
	idx.curTipHeight = indexTipHeight
	
	// Smart rebuild strategy: choose optimal method based on lag
	return idx.smartRebuild(indexTipHeight)
}

// ConnectBlock is invoked by the index manager when a new block has been
// connected to the main chain. This indexer adds entries for all the new
// UTXOs created by the block.
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
		return dbPutTipHeightIndexed(dbTx, blockHeight)
	}
	
	// Process all transactions in the block
	blockHash := block.Hash()
	for txIdx, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		
		// Skip the coinbase transaction inputs (they don't spend existing UTXOs)
		if !blockchain.IsCoinBaseTx(msgTx) {
			// Process spent UTXOs (remove from index)
			for _, txIn := range msgTx.TxIn {
				if err := idx.disconnectTxOut(dbTx, &txIn.PreviousOutPoint); err != nil {
					return fmt.Errorf("failed to disconnect txout %v: %v", 
						txIn.PreviousOutPoint, err)
				}
			}
		}
		
		// Process new UTXOs (add to index)
		for voutIdx, _ := range msgTx.TxOut {
			// Create outpoint for this output
			outpoint := &wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(voutIdx),
			}
			
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
// the UTXOs created by the block.
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
		return dbPutTipHeightIndexed(dbTx, blockHeight-1)
	}
	
	// Process all transactions in reverse order
	transactions := block.Transactions()
	
	for txIdx := len(transactions) - 1; txIdx >= 0; txIdx-- {
		tx := transactions[txIdx]
		msgTx := tx.MsgTx()
		
		// Remove new UTXOs that were created by this block
		for voutIdx := len(msgTx.TxOut) - 1; voutIdx >= 0; voutIdx-- {
			outpoint := &wire.OutPoint{
				Hash:  *tx.Hash(),
				Index: uint32(voutIdx),
			}
			
			if err := idx.disconnectTxOut(dbTx, outpoint); err != nil {
				return fmt.Errorf("failed to disconnect txout %v: %v", outpoint, err)
			}
		}
		
		// Restore spent UTXOs (re-add to index) 
		// Skip coinbase transactions as they don't spend existing UTXOs
		if !blockchain.IsCoinBaseTx(msgTx) {
			for vinIdx := len(msgTx.TxIn) - 1; vinIdx >= 0; vinIdx-- {
				txIn := msgTx.TxIn[vinIdx]
				
				// We need the original UTXO creation height to restore it
				// This information should be available in stxos
				if vinIdx < len(stxos) {
					stxo := stxos[vinIdx]
					if err := idx.connectTxOut(dbTx, &txIn.PreviousOutPoint, 
						stxo.Height); err != nil {
						return fmt.Errorf("failed to reconnect txout %v: %v", 
							txIn.PreviousOutPoint, err)
					}
				}
			}
		}
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

// ScanExpiringUTXOs scans for UTXOs expiring within the specified range.
// This method is used by RPC calls to find UTXOs approaching expiration.
func (idx *ExpiryIndex) ScanExpiringUTXOs(fromKey, toKey uint64, 
	maxResults int) ([]*ExpiringUTXO, error) {
	
	if idx.disabled {
		return nil, fmt.Errorf("expiry index is disabled")
	}
	
	var results []*ExpiringUTXO
	
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
			
			// Add each outpoint to results
			for _, outpoint := range outpoints {
				if len(results) >= maxResults {
					break
				}
				
				results = append(results, &ExpiringUTXO{
					OutPoint:  *outpoint,
					ExpiryKey: expiryKey,
				})
			}
			
			// Move to next key
			found = cursor.Next()
		}
		
		return nil
	})
	
	return results, err
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

// fastRebuildFromUTXO rebuilds the index from existing UTXO set
func (idx *ExpiryIndex) fastRebuildFromUTXO(chainTipHeight int32) error {
	startTime := time.Now()
	processed := 0
	
	log.Infof("ExpiryIndex: Starting fast rebuild from UTXO set")
	
	err := idx.db.Update(func(dbTx database.Tx) error {
		// Clear existing index data
		if err := idx.clearIndexBuckets(dbTx); err != nil {
			return fmt.Errorf("failed to clear index buckets: %v", err)
		}
		
		// Get the UTXO bucket from chainstate
		utxoBucket, err := idx.getUTXOBucket(dbTx)
		if err != nil {
			return fmt.Errorf("failed to access UTXO bucket: %v", err)
		}
		
		// Iterate through all UTXOs
		cursor := utxoBucket.Cursor()
		found := cursor.First()
		for found {
			k := cursor.Key()
			v := cursor.Value()
			// Parse UTXO entry
			outpoint, createHeight, err := idx.parseUTXOEntry(k, v)
			if err != nil {
				continue // Skip invalid entries
			}
			
			// Check if UTXO is within indexing scope
			if createHeight < idx.expiryParams.StartScanHeight {
				continue
			}
			
			// Add to ExpiryIndex
			err = idx.connectTxOut(dbTx, outpoint, createHeight)
			if err != nil {
				return fmt.Errorf("failed to add UTXO %v: %v", outpoint, err)
			}
			
			processed++
			if processed%50000 == 0 {
				elapsed := time.Since(startTime)
				rate := float64(processed) / elapsed.Seconds()
				log.Infof("ExpiryIndex: Processed %d UTXOs (%.0f/s)", processed, rate)
			}
			
			found = cursor.Next()
		}
		
		// Mark index as complete
		if err := dbPutTipHeightIndexed(dbTx, chainTipHeight); err != nil {
			return fmt.Errorf("failed to update tip height: %v", err)
		}
		
		// Update internal state
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
	// Clear outpoint-to-expiry bucket
	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	if outpointBucket != nil {
		cursor := outpointBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}
	
	// Clear expiry-to-outpoints bucket
	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket != nil {
		cursor := expiryBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}
	
	return nil
}

// getUTXOBucket returns the UTXO bucket from chainstate
func (idx *ExpiryIndex) getUTXOBucket(dbTx database.Tx) (database.Bucket, error) {
	// Note: This would need to access btcd's chainstate UTXO bucket
	// The exact implementation depends on btcd's internal structure
	// For now, return an error to indicate this needs blockchain instance access
	return nil, fmt.Errorf("UTXO bucket access not implemented - needs blockchain instance")
}

// parseUTXOEntry parses a UTXO entry from chainstate
func (idx *ExpiryIndex) parseUTXOEntry(key, value []byte) (*wire.OutPoint, int32, error) {
	// Note: This would need to parse btcd's UTXO entry format
	// The exact format depends on btcd's internal serialization
	// This is a placeholder that needs proper implementation
	return nil, 0, fmt.Errorf("UTXO entry parsing not implemented")
}

// getChainTipHeight gets the current blockchain tip height
func (idx *ExpiryIndex) getChainTipHeight() int32 {
	// Note: This would need access to the blockchain instance
	// For now, return the current index height as fallback
	return idx.curTipHeight
}

// getBlockByHeight retrieves a block by its height
func (idx *ExpiryIndex) getBlockByHeight(height int32) (*btcutil.Block, error) {
	// Note: This would need access to the blockchain instance
	// This is a placeholder that needs proper implementation
	return nil, fmt.Errorf("block retrieval not implemented - needs blockchain instance")
}

// getSpentTxOuts retrieves spent transaction outputs for a block
func (idx *ExpiryIndex) getSpentTxOuts(height int32) ([]blockchain.SpentTxOut, error) {
	// Note: This would need access to the spend journal
	// This is a placeholder that needs proper implementation
	return nil, fmt.Errorf("spent txouts retrieval not implemented - needs blockchain instance")
}