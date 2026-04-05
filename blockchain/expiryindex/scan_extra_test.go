package expiryindex

import (
	"bytes"
	"reflect"
	"sort"
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

func TestScanExpiringUTXOsContract(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	hashWithLastByte := func(b byte) chainhash.Hash {
		var h chainhash.Hash
		h[chainhash.HashSize-1] = b
		return h
	}
	entries := []ExpiringUTXO{
		{ExpiryKey: 100, OutPoint: wire.OutPoint{Hash: hashWithLastByte(0x01), Index: 0}},
		{ExpiryKey: 100, OutPoint: wire.OutPoint{Hash: hashWithLastByte(0x01), Index: 2}},
		{ExpiryKey: 100, OutPoint: wire.OutPoint{Hash: hashWithLastByte(0x02), Index: 0}},
		{ExpiryKey: 101, OutPoint: wire.OutPoint{Hash: hashWithLastByte(0x03), Index: 0}},
		{ExpiryKey: 102, OutPoint: wire.OutPoint{Hash: hashWithLastByte(0x04), Index: 0}},
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ExpiryKey != entries[j].ExpiryKey {
			return entries[i].ExpiryKey < entries[j].ExpiryKey
		}
		return compareOutPoint(&entries[i].OutPoint, &entries[j].OutPoint) < 0
	})

	if err := db.Update(func(dbTx database.Tx) error {
		for i := range entries {
			if err := putTxOutMapping(dbTx, &entries[i].OutPoint, entries[i].ExpiryKey); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	assertScan := func(name string, fromKey, toKey uint64, maxResults int, startAfter *wire.OutPoint,
		want []ExpiringUTXO, wantHasMore bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			got, hasMore, err := idx.ScanExpiringUTXOs(fromKey, toKey, maxResults, startAfter)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			if hasMore != wantHasMore {
				t.Fatalf("hasMore mismatch: got %t want %t", hasMore, wantHasMore)
			}
			gotRows := make([]ExpiringUTXO, len(got))
			for i := range got {
				gotRows[i] = *got[i]
			}
			if !reflect.DeepEqual(gotRows, want) {
				t.Fatalf("scan mismatch: got %#v want %#v", gotRows, want)
			}
		})
	}

	missingBetween := wire.OutPoint{Hash: entries[0].OutPoint.Hash, Index: 1}
	afterLastInKey100 := wire.OutPoint{Hash: entries[2].OutPoint.Hash, Index: 1}

	assertScan("all entries", 100, 102, 10, nil, entries, false)
	assertScan("single key", 100, 100, 10, nil, entries[:3], false)
	assertScan("empty range when from exceeds to", 103, 102, 10, nil, []ExpiringUTXO{}, false)
	assertScan("max results one sets hasMore", 100, 102, 1, nil, entries[:1], true)
	assertScan("startAfter exact composite key", 100, 102, 10, &entries[0].OutPoint, entries[1:], false)
	assertScan("startAfter missing outpoint within key", 100, 102, 10, &missingBetween, entries[1:], false)
	assertScan("startAfter skips to next key when tail reached", 100, 102, 10, &afterLastInKey100, entries[3:], false)
}

func TestScanExpiringUTXOsRejectsCorruptCompositeKey(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return dbTx.Metadata().Bucket(bktExpiry2Outpoints).Put(
			append(encodeExpiryKey(77), []byte{0x01, 0x02, 0x03}...),
			emptyIndexValue,
		)
	}); err != nil {
		t.Fatalf("seed corrupt composite key: %v", err)
	}

	if _, _, err := idx.ScanExpiringUTXOs(77, 77, 10, nil); err == nil {
		t.Fatal("expected corrupt composite key scan to fail")
	}
}

func TestGetStatsRejectsCorruptCompositeKey(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		badKey := append(encodeExpiryKey(88), bytes.Repeat([]byte{0xaa}, orderedOutPointEncodedSize-1)...)
		return dbTx.Metadata().Bucket(bktExpiry2Outpoints).Put(badKey, emptyIndexValue)
	}); err != nil {
		t.Fatalf("seed corrupt composite key: %v", err)
	}

	if _, err := idx.GetStats(); err == nil {
		t.Fatal("expected corrupt composite key stats read to fail")
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
