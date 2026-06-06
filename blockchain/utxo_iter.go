// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

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

	return b.db.View(func(dbTx database.Tx) error {
		utxoBucket := dbTx.Metadata().Bucket(utxoSetBucketName)
		if utxoBucket == nil {
			return nil // empty UTXO set
		}

		cursor := utxoBucket.Cursor()
		for ok := cursor.First(); ok; ok = cursor.Next() {
			key := cursor.Key()
			value := cursor.Value()

			// Parse outpoint from key: <hash(32)><VLQ-encoded index>
			if len(key) < chainhash.HashSize+1 {
				continue
			}
			var hash chainhash.Hash
			copy(hash[:], key[:chainhash.HashSize])
			idx, _ := deserializeVLQ(key[chainhash.HashSize:])

			// Parse height from serialized UTXO entry.
			entry, err := deserializeUtxoEntry(value)
			if err != nil {
				continue // skip corrupt entries
			}

			op := wire.OutPoint{Hash: hash, Index: uint32(idx)}
			if err := fn(op, entry.BlockHeight(), entry.Amount()); err != nil {
				return err
			}
		}
		return nil
	})
}
