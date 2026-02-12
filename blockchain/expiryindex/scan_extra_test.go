package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func TestScanExpiringUTXOsPaginationAndStartAfter(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		// heights chosen so expiry keys are 154, 155, 156 on regtest(window=144)
		op1 := makeOutPoint(1, 0)
		op2 := makeOutPoint(2, 0)
		op3 := makeOutPoint(3, 0)
		op4 := makeOutPoint(4, 0)
		if err := idx.connectTxOut(dbTx, &op1, 10); err != nil {
			return err
		}
		if err := idx.connectTxOut(dbTx, &op2, 11); err != nil {
			return err
		}
		if err := idx.connectTxOut(dbTx, &op3, 12); err != nil {
			return err
		}
		if err := idx.connectTxOut(dbTx, &op4, 12); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed index: %v", err)
	}

	fromKey := uint64(154)
	toKey := uint64(156)

	page1, hasMore, err := idx.ScanExpiringUTXOs(fromKey, toKey, 2, nil)
	if err != nil {
		t.Fatalf("scan page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 results on page1, got %d", len(page1))
	}
	if !hasMore {
		t.Fatalf("expected hasMore=true on page1")
	}

	lastItem := page1[len(page1)-1]
	page2, hasMore2, err := idx.ScanExpiringUTXOs(lastItem.ExpiryKey, toKey, 10, &lastItem.OutPoint)
	if err != nil {
		t.Fatalf("scan page2: %v", err)
	}
	if len(page2) == 0 {
		t.Fatalf("expected page2 to contain remaining results")
	}
	if hasMore2 {
		t.Fatalf("expected hasMore=false on final page")
	}

	// No duplicates across pages.
	seen := map[wire.OutPoint]struct{}{}
	for _, it := range page1 {
		seen[it.OutPoint] = struct{}{}
	}
	for _, it := range page2 {
		if _, ok := seen[it.OutPoint]; ok {
			t.Fatalf("duplicate outpoint across pages: %v", it.OutPoint)
		}
	}
}

func TestScanExpiringUTXOsErrors(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	idx.disabled = true
	if _, _, err := idx.ScanExpiringUTXOs(0, 10, 10, nil); err == nil {
		t.Fatalf("expected disabled index error")
	}
	idx.disabled = false

	// Buckets not created yet.
	if _, _, err := idx.ScanExpiringUTXOs(0, 10, 10, nil); err == nil {
		t.Fatalf("expected missing bucket error")
	}
}

func TestCompareOutPointAndStartIndex(t *testing.T) {
	a := makeOutPoint(1, 0)
	b := makeOutPoint(1, 1)
	c := makeOutPoint(2, 0)

	if compareOutPoint(&a, &a) != 0 {
		t.Fatalf("same outpoint should compare equal")
	}
	if compareOutPoint(&a, &b) >= 0 {
		t.Fatalf("expected a < b when hash equal and index smaller")
	}
	if compareOutPoint(&c, &a) <= 0 {
		t.Fatalf("expected c > a for different hashes")
	}

	list := []*wire.OutPoint{&a, &b, &c}
	idx := findOutPointStartIndex(list, &a)
	if idx != 1 {
		t.Fatalf("expected start index 1 after a, got %d", idx)
	}
	idx = findOutPointStartIndex(list, &c)
	if idx != 3 {
		t.Fatalf("expected start index end-of-list, got %d", idx)
	}
	idx = findOutPointStartIndex(nil, &a)
	if idx != 0 {
		t.Fatalf("expected zero start index for empty list")
	}
}

func makeOutPoint(seed byte, index uint32) wire.OutPoint {
	var h chainhash.Hash
	h[0] = seed
	return wire.OutPoint{Hash: h, Index: index}
}
