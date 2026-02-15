package mining

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func createMiningTestExpiryIndex(t *testing.T) (*expiryindex.ExpiryIndex, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "mining_reap_test_")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "mining_reap.db")
	db, err := database.Create("ffldb", dbPath, wire.TestNet3)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create db: %v", err)
	}

	idx, err := expiryindex.NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("new index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("index create: %v", err)
	}

	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{})
	cb := wire.NewMsgTx(1)
	cb.AddTxIn(&wire.TxIn{PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex)})
	cb.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
	if err := msgBlock.AddTransaction(cb); err != nil {
		t.Fatalf("add tx: %v", err)
	}
	blk := btcutil.NewBlock(msgBlock)
	blk.SetHeight(120)
	if err := db.Update(func(dbTx database.Tx) error { return idx.ConnectBlock(dbTx, blk, nil) }); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("connect block: %v", err)
	}

	teardown := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
	return idx, teardown
}

func TestMaybeBuildREAPTxDirectEarlyReturns(t *testing.T) {
	g := &BlkTmplGenerator{chainParams: &chaincfg.MainNetParams}
	tx, fee, err := g.maybeBuildREAPTx(200)
	if err != nil || tx != nil || fee != 0 {
		t.Fatalf("expected clean early return for non-OBTC network, got tx=%v fee=%d err=%v", tx, fee, err)
	}

	g = &BlkTmplGenerator{chainParams: &chaincfg.ObtcRegTestParams}
	tx, fee, err = g.maybeBuildREAPTx(200)
	if err != nil || tx != nil || fee != 0 {
		t.Fatalf("expected clean early return with nil index, got tx=%v fee=%d err=%v", tx, fee, err)
	}
}

func TestCollectExpiredOutpointsDirect(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndex(t)
	defer teardown()

	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	p.ScanBatch = 10
	ops, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("collectExpiredOutpoints failed: %v", err)
	}
	if len(ops) == 0 {
		t.Fatalf("expected at least one expired outpoint")
	}
}

func TestSetREAPIndexDirect(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndex(t)
	defer teardown()
	g := &BlkTmplGenerator{}
	g.SetREAPIndex(idx)
	if g.reapIndex != idx {
		t.Fatalf("SetREAPIndex did not wire index")
	}
}

func TestMaybeBuildREAPTxBeforeEnableHeight(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndex(t)
	defer teardown()

	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}
	tx, fee, err := g.maybeBuildREAPTx(ep.EnableAtHeight - 1)
	if err != nil || tx != nil || fee != 0 {
		t.Fatalf("expected no REAP tx before enable height, got tx=%v fee=%d err=%v", tx, fee, err)
	}
}

func TestCollectExpiredOutpointsIdempotentAndConcurrent(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndex(t)
	defer teardown()
	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	p.ScanBatch = 1

	first, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("first collect failed: %v", err)
	}
	second, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("second collect failed: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("idempotent length mismatch %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("idempotent content mismatch at %d", i)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := g.collectExpiredOutpoints(500, p)
			if err != nil {
				errCh <- err
				return
			}
			if len(got) != len(first) {
				errCh <- fmt.Errorf("len mismatch got=%d want=%d", len(got), len(first))
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent collect failed: %v", err)
	}
}
