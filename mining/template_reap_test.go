// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
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

func createMiningTestExpiryIndexWithOutputsAtHeight(t *testing.T, createBuckets bool,
	outputs int, blockHeight int32) (*expiryindex.ExpiryIndex, func()) {
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
	if createBuckets {
		if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("index create: %v", err)
		}
	}

	if createBuckets && outputs > 0 {
		msgBlock := wire.NewMsgBlock(&wire.BlockHeader{})
		cb := wire.NewMsgTx(1)
		cb.AddTxIn(&wire.TxIn{PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex)})
		for i := 0; i < outputs; i++ {
			cb.AddTxOut(&wire.TxOut{Value: int64(1000 + i), PkScript: []byte{txscript.OP_TRUE}})
		}
		// Add expiry commitment with identity root (accumulator is empty).
		identityRoot := expiryindex.NewMuHash().Digest()
		cb.AddTxOut(&wire.TxOut{
			Value:    0,
			PkScript: expiryindex.BuildExpiryCommitmentScript(identityRoot),
		})
		if err := msgBlock.AddTransaction(cb); err != nil {
			t.Fatalf("add tx: %v", err)
		}
		blk := btcutil.NewBlock(msgBlock)
		blk.SetHeight(blockHeight)
		if err := db.Update(func(dbTx database.Tx) error { return idx.ConnectBlock(dbTx, blk, nil) }); err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("connect block: %v", err)
		}
	}

	teardown := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
	return idx, teardown
}

func createMiningTestExpiryIndexWithOutputs(t *testing.T, createBuckets bool, outputs int) (*expiryindex.ExpiryIndex, func()) {
	return createMiningTestExpiryIndexWithOutputsAtHeight(t, createBuckets, outputs, 120)
}

func createMiningTestExpiryIndex(t *testing.T) (*expiryindex.ExpiryIndex, func()) {
	return createMiningTestExpiryIndexWithOutputs(t, true, 1)
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

func TestMaybeBuildREAPTxNoCandidatesPath(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputs(t, true, 0)
	defer teardown()
	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}

	tx, fee, err := g.maybeBuildREAPTx(500)
	if err != nil || tx != nil || fee != 0 {
		t.Fatalf("expected no-candidate early return, got tx=%v fee=%d err=%v", tx, fee, err)
	}
}

func TestMaybeBuildREAPTxCollectErrorPath(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputs(t, false, 0)
	defer teardown()
	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}

	tx, fee, err := g.maybeBuildREAPTx(500)
	if err == nil {
		t.Fatalf("expected collect error when index buckets are not created")
	}
	if tx != nil || fee != 0 {
		t.Fatalf("unexpected tx/fee on error path: tx=%v fee=%d", tx, fee)
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

func TestCollectExpiredOutpointsIncludesGenesisEraUTXO(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputsAtHeight(t, true, 1, 1)
	defer teardown()

	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	p.ScanBatch = 10

	ops, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("collectExpiredOutpoints failed: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected genesis-era utxo to be indexed and expirable, got %d candidates", len(ops))
	}
}

func TestCollectExpiredOutpointsErrorWhenBucketsMissing(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputs(t, false, 0)
	defer teardown()
	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	if _, err := g.collectExpiredOutpoints(500, p); err == nil {
		t.Fatalf("expected error when index buckets are missing")
	}
}

func TestCollectExpiredOutpointsRespectsMaxCandidates(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputs(t, true, 80)
	defer teardown()
	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	p.MaxInputs = 2 // maxCandidates = MaxInputs*20 => 40
	p.ScanBatch = 3

	ops, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}
	if len(ops) != 40 {
		t.Fatalf("expected maxCandidates limit 40, got %d", len(ops))
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

func TestCollectExpiredOutpointsPreservesExpiryIndexOrderWithinSingleExpiryKey(t *testing.T) {
	idx, teardown := createMiningTestExpiryIndexWithOutputsAtHeight(t, true, 4, 120)
	defer teardown()

	g := &BlkTmplGenerator{reapIndex: idx, chainParams: &chaincfg.ObtcRegTestParams}
	p := reap.DefaultREAPParams(reap.SortModeStrict)
	p.ScanBatch = 10

	ops, err := g.collectExpiredOutpoints(500, p)
	if err != nil {
		t.Fatalf("collectExpiredOutpoints failed: %v", err)
	}
	if len(ops) != 4 {
		t.Fatalf("expected 4 collected outpoints, got %d", len(ops))
	}
	for i := range ops {
		if ops[i].Index != uint32(i) {
			t.Fatalf("unexpected outpoint order at %d: got vout=%d want %d", i, ops[i].Index, i)
		}
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

func TestNormalTxWeightLimitReservePolicy(t *testing.T) {
	const blockMax = uint32(1_000_000)

	g := &BlkTmplGenerator{
		policy:      &Policy{BlockMaxWeight: blockMax},
		chainParams: &chaincfg.ObtcMainNetParams,
		reapIndex:   new(expiryindex.ExpiryIndex),
	}

	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	// Before enable height, do not reserve weight.
	if got := g.normalTxWeightLimit(ep.EnableAtHeight-1, true); got != blockMax {
		t.Fatalf("unexpected pre-enable weight limit: got %d want %d", got, blockMax)
	}

	// At/after enable height, reserve REAP budget from normal tx area.
	want := blockMax - uint32(ep.ReapMaxWeight)
	if got := g.normalTxWeightLimit(ep.EnableAtHeight, true); got != want {
		t.Fatalf("unexpected post-enable weight limit: got %d want %d", got, want)
	}
}

func TestNormalTxWeightLimitNoReserveWhenBudgetExceedsBlock(t *testing.T) {
	g := &BlkTmplGenerator{
		policy:      &Policy{BlockMaxWeight: 150_000},
		chainParams: &chaincfg.ObtcMainNetParams,
		reapIndex:   new(expiryindex.ExpiryIndex),
	}
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	if got := g.normalTxWeightLimit(ep.EnableAtHeight, true); got != 150_000 {
		t.Fatalf("reserve should be disabled when over block max, got %d", got)
	}
}

func TestNormalTxWeightLimitNoReserveWithoutREAP(t *testing.T) {
	const blockMax = uint32(1_000_000)
	g := &BlkTmplGenerator{
		policy:      &Policy{BlockMaxWeight: blockMax},
		chainParams: &chaincfg.ObtcMainNetParams,
	}
	if got := g.normalTxWeightLimit(1_000_000, false); got != blockMax {
		t.Fatalf("expected no reserve without planned reap, got %d", got)
	}
}

func TestNormalTxWeightLimitUsesPlannedREAPWeight(t *testing.T) {
	const blockMax = uint32(1_000_000)
	g := &BlkTmplGenerator{
		policy:      &Policy{BlockMaxWeight: blockMax},
		chainParams: &chaincfg.ObtcMainNetParams,
		reapIndex:   new(expiryindex.ExpiryIndex),
	}
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	tests := []struct {
		name          string
		plannedWeight uint32
		wantLimit     uint32
	}{
		{name: "actual_smaller_than_default_budget", plannedWeight: 125_000, wantLimit: 875_000},
		{name: "actual_larger_than_default_budget", plannedWeight: 250_000, wantLimit: 750_000},
		{name: "actual_equals_block_max", plannedWeight: blockMax, wantLimit: blockMax},
		{name: "actual_zero", plannedWeight: 0, wantLimit: blockMax},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := g.normalTxWeightLimit(ep.EnableAtHeight, true, tc.plannedWeight)
			if got != tc.wantLimit {
				t.Fatalf("normal tx weight limit mismatch: got %d want %d",
					got, tc.wantLimit)
			}
		})
	}
}

func TestReservedREAPWeightScenarios(t *testing.T) {
	mainEP := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if mainEP == nil {
		t.Fatalf("expected mainnet expiry params")
	}
	regEP := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if regEP == nil {
		t.Fatalf("expected regtest expiry params")
	}

	tests := []struct {
		name         string
		blockMax     uint32
		chainParams  *chaincfg.Params
		withIndex    bool
		nextHeight   int32
		wantReserved uint32
	}{
		{name: "nil_index", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: false, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
		{name: "nil_chain_params", blockMax: 1_000_000, chainParams: nil, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
		{name: "non_obtc_network", blockMax: 1_000_000, chainParams: &chaincfg.MainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
		{name: "before_enable_height", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight - 1, wantReserved: 0},
		{name: "enabled_obtc_mainnet", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: uint32(mainEP.ReapMaxWeight)},
		{name: "enabled_obtc_regtest", blockMax: 1_000_000, chainParams: &chaincfg.ObtcRegTestParams, withIndex: true, nextHeight: regEP.EnableAtHeight, wantReserved: uint32(regEP.ReapMaxWeight)},
		{name: "reserve_equals_block_max", blockMax: uint32(mainEP.ReapMaxWeight), chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
		{name: "reserve_exceeds_block_max", blockMax: uint32(mainEP.ReapMaxWeight - 1), chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
		{name: "zero_block_max", blockMax: 0, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, wantReserved: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &BlkTmplGenerator{
				policy:      &Policy{BlockMaxWeight: tc.blockMax},
				chainParams: tc.chainParams,
			}
			if tc.withIndex {
				g.reapIndex = new(expiryindex.ExpiryIndex)
			}

			if got := g.reservedREAPWeight(tc.nextHeight); got != tc.wantReserved {
				t.Fatalf("reserved weight mismatch: got %d want %d", got, tc.wantReserved)
			}
		})
	}
}

func TestNormalTxWeightLimitScenarioMatrix(t *testing.T) {
	mainEP := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if mainEP == nil {
		t.Fatalf("expected mainnet expiry params")
	}

	tests := []struct {
		name            string
		blockMax        uint32
		chainParams     *chaincfg.Params
		withIndex       bool
		nextHeight      int32
		reserveForREAP  bool
		wantNormalLimit uint32
	}{
		{name: "reserve_flag_off_even_if_reap_possible", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, reserveForREAP: false, wantNormalLimit: 1_000_000},
		{name: "reserve_flag_on_and_reap_possible", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, reserveForREAP: true, wantNormalLimit: 600_000},
		{name: "reserve_flag_on_but_pre_enable", blockMax: 1_000_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight - 1, reserveForREAP: true, wantNormalLimit: 1_000_000},
		{name: "reserve_flag_on_but_non_obtc", blockMax: 1_000_000, chainParams: &chaincfg.MainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, reserveForREAP: true, wantNormalLimit: 1_000_000},
		{name: "reserve_flag_on_but_block_too_small", blockMax: 150_000, chainParams: &chaincfg.ObtcMainNetParams, withIndex: true, nextHeight: mainEP.EnableAtHeight, reserveForREAP: true, wantNormalLimit: 150_000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &BlkTmplGenerator{
				policy:      &Policy{BlockMaxWeight: tc.blockMax},
				chainParams: tc.chainParams,
			}
			if tc.withIndex {
				g.reapIndex = new(expiryindex.ExpiryIndex)
			}
			if got := g.normalTxWeightLimit(tc.nextHeight, tc.reserveForREAP); got != tc.wantNormalLimit {
				t.Fatalf("normal tx weight limit mismatch: got %d want %d", got, tc.wantNormalLimit)
			}
		})
	}
}

func TestMergeUtxoEntriesIfMissing(t *testing.T) {
	mkTx := func(value int64) *btcutil.Tx {
		msg := wire.NewMsgTx(1)
		msg.AddTxOut(&wire.TxOut{Value: value, PkScript: []byte{txscript.OP_TRUE}})
		return btcutil.NewTx(msg)
	}

	dst := blockchain.NewUtxoViewpoint()
	src := blockchain.NewUtxoViewpoint()

	// Missing entry in dst should be copied from src.
	tx1 := mkTx(1000)
	op1 := wire.OutPoint{Hash: *tx1.Hash(), Index: 0}
	src.AddTxOut(tx1, 0, 100)
	mergeUtxoEntriesIfMissing(dst, src)
	if dst.LookupEntry(op1) == nil {
		t.Fatalf("expected missing entry to be copied")
	}

	// Existing (spent) entry in dst should not be overwritten by src.
	tx2 := mkTx(2000)
	op2 := wire.OutPoint{Hash: *tx2.Hash(), Index: 0}
	dst.AddTxOut(tx2, 0, 110)
	existing := dst.LookupEntry(op2)
	existing.Spend()
	src.AddTxOut(tx2, 0, 120)
	mergeUtxoEntriesIfMissing(dst, src)
	if !dst.LookupEntry(op2).IsSpent() {
		t.Fatalf("expected spent dst entry to remain spent")
	}
	if dst.LookupEntry(op2) != existing {
		t.Fatalf("expected existing dst entry pointer to be preserved")
	}
}

func TestMergeUtxoEntriesIfMissingReplacesNilPlaceholder(t *testing.T) {
	mkTx := func(value int64) *btcutil.Tx {
		msg := wire.NewMsgTx(1)
		msg.AddTxOut(&wire.TxOut{Value: value, PkScript: []byte{txscript.OP_TRUE}})
		return btcutil.NewTx(msg)
	}

	dst := blockchain.NewUtxoViewpoint()
	src := blockchain.NewUtxoViewpoint()

	tx := mkTx(3000)
	op := wire.OutPoint{Hash: *tx.Hash(), Index: 0}
	src.AddTxOut(tx, 0, 100)
	dst.Entries()[op] = nil

	mergeUtxoEntriesIfMissing(dst, src)
	if dst.LookupEntry(op) == nil {
		t.Fatalf("expected nil placeholder to be replaced")
	}
}
