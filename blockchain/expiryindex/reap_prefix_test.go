// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func reapPrefixTestOutPoint(tag byte, index uint32) wire.OutPoint {
	var hash chainhash.Hash
	hash[0] = tag
	return wire.OutPoint{Hash: hash, Index: index}
}

func TestReapPrefixCandidatesStrictOrderAndLimit(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, 99); err != nil {
			return err
		}

		entries := []struct {
			op     wire.OutPoint
			expiry uint64
			amount int64
		}{
			{op: reapPrefixTestOutPoint(0x04, 0), expiry: 50, amount: 500},
			{op: reapPrefixTestOutPoint(0x03, 0), expiry: 50, amount: 100},
			{op: reapPrefixTestOutPoint(0x02, 0), expiry: 50, amount: 300},
			{op: reapPrefixTestOutPoint(0x01, 0), expiry: 50, amount: 300},
			{op: reapPrefixTestOutPoint(0x00, 0), expiry: 51, amount: 1},
		}
		for _, entry := range entries {
			if err := putTxOutMapping(dbTx, &entry.op, entry.expiry, entry.amount); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("populate index: %v", err)
	}

	tip, err := idx.ReapPrefixTipHeight()
	if err != nil {
		t.Fatalf("tip height: %v", err)
	}
	if tip != 99 {
		t.Fatalf("tip height: got %d want 99", tip)
	}

	got, err := idx.ReapPrefixCandidates(50, 10)
	if err != nil {
		t.Fatalf("prefix candidates: %v", err)
	}

	want := []wire.OutPoint{
		reapPrefixTestOutPoint(0x03, 0),
		reapPrefixTestOutPoint(0x01, 0),
		reapPrefixTestOutPoint(0x02, 0),
		reapPrefixTestOutPoint(0x04, 0),
	}
	if len(got) != len(want) {
		t.Fatalf("candidate count: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].OutPoint != want[i] {
			t.Fatalf("candidate %d: got %s want %s", i, got[i].OutPoint, want[i])
		}
	}

	limited, err := idx.ReapPrefixCandidates(50, 2)
	if err != nil {
		t.Fatalf("limited prefix candidates: %v", err)
	}
	if len(limited) != 2 || limited[0].OutPoint != want[0] ||
		limited[1].OutPoint != want[1] {
		t.Fatalf("limited prefix mismatch: %+v", limited)
	}

	rows, hasMore, err := idx.ScanExpiringUTXOs(50, 50, 10, nil)
	if err != nil {
		t.Fatalf("legacy scan: %v", err)
	}
	if hasMore || len(rows) != 4 {
		t.Fatalf("legacy scan count: got rows=%d hasMore=%t", len(rows), hasMore)
	}

	remove := reapPrefixTestOutPoint(0x01, 0)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.disconnectTxOut(dbTx, &remove)
	}); err != nil {
		t.Fatalf("disconnect txout: %v", err)
	}
	afterRemove, err := idx.ReapPrefixCandidates(50, 10)
	if err != nil {
		t.Fatalf("prefix after remove: %v", err)
	}
	for _, candidate := range afterRemove {
		if candidate.OutPoint == remove {
			t.Fatalf("removed outpoint still present in strict prefix")
		}
	}
}

func TestReapPrefixCandidatesRejectsCorruptStrictKey(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		bucket := dbTx.Metadata().Bucket(bktReapStrictCandidates)
		return bucket.Put([]byte{0x01, 0x02}, emptyIndexValue)
	}); err != nil {
		t.Fatalf("populate corrupt key: %v", err)
	}

	if _, err := idx.ReapPrefixCandidates(100, 1); err == nil {
		t.Fatalf("expected corrupt strict key error")
	}
}
