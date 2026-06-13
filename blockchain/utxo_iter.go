// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

// UTXOSnapshotEntry is a read-only view of a single unspent transaction output
// as stored in the on-disk UTXO set.
type UTXOSnapshotEntry struct {
	OutPoint    wire.OutPoint
	BlockHeight int32
	Amount      int64
	IsCoinBase  bool
}

// DBBestSnapshot is the best chain state read directly from the block
// database. UTXOStateHash is the hash recorded when the on-disk UTXO set was
// last flushed, when present.
type DBBestSnapshot struct {
	Hash          chainhash.Hash
	Height        int32
	TotalTxns     uint64
	UTXOStateHash *chainhash.Hash
}

// ForEachUTXO iterates over every unspent transaction output in the database
// and calls fn with the outpoint and the block height at which it was created.
// Iteration stops early if fn returns a non-nil error.
//
// This method is intended for bulk operations such as rebuilding the expiry
// index from scratch. It reads directly from the on-disk UTXO set (not the
// in-memory cache) so the caller should ensure the cache has been flushed
// before calling.
//
// This function is safe for concurrent access (holds chainLock for reading).
func (b *BlockChain) ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error {
	return b.ForEachUTXOWithAmount(func(outpoint wire.OutPoint, height int32, amount int64) error {
		return fn(outpoint, height)
	})
}

// ForEachUTXOWithAmount iterates over every unspent transaction output in the
// database and calls fn with the outpoint, creation height, and amount.
func (b *BlockChain) ForEachUTXOWithAmount(fn func(outpoint wire.OutPoint,
	height int32, amount int64) error) error {

	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	return ForEachUTXOInDB(b.db, func(entry UTXOSnapshotEntry) error {
		return fn(entry.OutPoint, entry.BlockHeight, entry.Amount)
	})
}

// BestSnapshotFromDB returns the best chain state directly from the database.
func BestSnapshotFromDB(db database.DB) (*DBBestSnapshot, error) {
	var snapshot *DBBestSnapshot
	err := db.View(func(dbTx database.Tx) error {
		serializedData := dbTx.Metadata().Get(chainStateKeyName)
		if serializedData == nil {
			return database.Error{
				ErrorCode:   database.ErrCorruption,
				Description: "missing best chain state",
			}
		}
		state, err := deserializeBestChainState(serializedData)
		if err != nil {
			return err
		}
		snapshot = &DBBestSnapshot{
			Hash:      state.hash,
			Height:    int32(state.height),
			TotalTxns: state.totalTxns,
		}
		utxoStateHashBytes := dbFetchUtxoStateConsistency(dbTx)
		if len(utxoStateHashBytes) > 0 {
			if len(utxoStateHashBytes) != chainhash.HashSize {
				return database.Error{
					ErrorCode: database.ErrCorruption,
					Description: fmt.Sprintf("corrupt UTXO state consistency hash length %d",
						len(utxoStateHashBytes)),
				}
			}
			var utxoStateHash chainhash.Hash
			copy(utxoStateHash[:], utxoStateHashBytes)
			snapshot.UTXOStateHash = &utxoStateHash
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// HashByHeightFromDB returns the main-chain block hash at the provided height
// directly from the database height index.
func HashByHeightFromDB(db database.DB, height int32) (*chainhash.Hash, error) {
	var hash *chainhash.Hash
	err := db.View(func(dbTx database.Tx) error {
		var err error
		hash, err = dbFetchHashByHeight(dbTx, height)
		return err
	})
	if err != nil {
		return nil, err
	}
	return hash, nil
}

// HeaderByHeightFromDB returns the main-chain block header at the provided
// height directly from the database.
func HeaderByHeightFromDB(db database.DB, height int32) (*wire.BlockHeader, error) {
	var header *wire.BlockHeader
	err := db.View(func(dbTx database.Tx) error {
		var err error
		header, err = dbFetchHeaderByHeight(dbTx, height)
		return err
	})
	if err != nil {
		return nil, err
	}
	return header, nil
}

// ForEachUTXOInDB iterates over the on-disk UTXO set in database key order.
// It is intended for read-only offline tools that open a synced node database
// without constructing a BlockChain instance.
func ForEachUTXOInDB(db database.DB, fn func(entry UTXOSnapshotEntry) error) error {
	return db.View(func(dbTx database.Tx) error {
		utxoBucket := dbTx.Metadata().Bucket(utxoSetBucketName)
		if utxoBucket == nil {
			return nil // empty UTXO set
		}

		cursor := utxoBucket.Cursor()
		for ok := cursor.First(); ok; ok = cursor.Next() {
			key := cursor.Key()
			value := cursor.Value()

			// Parse outpoint from key: <hash(32)><VLQ-encoded index>.
			if len(key) < chainhash.HashSize+1 {
				continue
			}
			var hash chainhash.Hash
			copy(hash[:], key[:chainhash.HashSize])
			idx, _ := deserializeVLQ(key[chainhash.HashSize:])

			entry, err := deserializeUtxoEntry(value)
			if err != nil {
				continue // skip corrupt entries to preserve legacy iterator behavior.
			}

			op := wire.OutPoint{Hash: hash, Index: uint32(idx)}
			if err := fn(UTXOSnapshotEntry{
				OutPoint:    op,
				BlockHeight: entry.BlockHeight(),
				Amount:      entry.Amount(),
				IsCoinBase:  entry.IsCoinBase(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
