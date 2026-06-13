// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
)

func TestDBSnapshotHelpers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "blocks_ffldb")
	db, err := database.Create(testDbType, dbPath, blockDataNet)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	bestHash := chainhash.Hash{0x01, 0x02, 0x03}
	utxoHash := chainhash.Hash{0x04, 0x05, 0x06}
	const height int32 = 123
	const totalTxns uint64 = 456

	err = db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if _, err := meta.CreateBucket(heightIndexBucketName); err != nil {
			return err
		}
		var heightKey [4]byte
		byteOrder.PutUint32(heightKey[:], uint32(height))
		if err := meta.Bucket(heightIndexBucketName).Put(heightKey[:], bestHash[:]); err != nil {
			return err
		}
		if err := meta.Put(chainStateKeyName, serializeBestChainState(bestChainState{
			hash:      bestHash,
			height:    uint32(height),
			totalTxns: totalTxns,
			workSum:   big.NewInt(789),
		})); err != nil {
			return err
		}
		return dbPutUtxoStateConsistency(dbTx, &utxoHash)
	})
	if err != nil {
		t.Fatalf("seed db: %v", err)
	}

	snapshot, err := BestSnapshotFromDB(db)
	if err != nil {
		t.Fatalf("best snapshot: %v", err)
	}
	if snapshot.Height != height || snapshot.TotalTxns != totalTxns || !snapshot.Hash.IsEqual(&bestHash) {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.UTXOStateHash == nil || !snapshot.UTXOStateHash.IsEqual(&utxoHash) {
		t.Fatalf("unexpected utxo state hash: %+v", snapshot.UTXOStateHash)
	}

	hash, err := HashByHeightFromDB(db, height)
	if err != nil {
		t.Fatalf("hash by height: %v", err)
	}
	if !hash.IsEqual(&bestHash) {
		t.Fatalf("hash by height got %s want %s", hash, bestHash)
	}
}
