// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btclog"
)

func TestChainAccessorNotSetErrors(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	// Without SetChainAccessor, these should return "chain accessor not set".
	if _, err := idx.getBlockByHeight(10); err == nil || !strings.Contains(err.Error(), "chain accessor not set") {
		t.Fatalf("expected chain accessor error from getBlockByHeight, got %v", err)
	}
	if _, err := idx.getSpentTxOuts(10); err == nil || !strings.Contains(err.Error(), "chain accessor not set") {
		t.Fatalf("expected chain accessor error from getSpentTxOuts, got %v", err)
	}
}

func TestGetChainTipHeightAndUseLogger(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	// Without chain accessor, falls back to curTipHeight.
	idx.curTipHeight = 321
	if got := idx.getChainTipHeight(); got != 321 {
		t.Fatalf("tip height mismatch: got %d", got)
	}

	UseLogger(btclog.Disabled)
	DisableLog()
}

func TestSetChainAccessorDisabledSkipsDeferredRebuild(t *testing.T) {
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

	mock := &rebuildMockChain{
		bestHeight: -1,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}
	idx.SetChainAccessor(mock)
	if mock.forEachCalls != 0 {
		t.Fatalf("disabled SetChainAccessor should not rebuild, got %d calls", mock.forEachCalls)
	}
}

func TestGetAccumulatorSnapshotDisabled(t *testing.T) {
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

	if _, err := idx.GetAccumulatorSnapshot(); err == nil {
		t.Fatal("expected disabled snapshot to fail")
	}
	if _, err := idx.GetAccumulatorDigest(); err == nil {
		t.Fatal("expected disabled digest to fail")
	}
}

func TestSetChainAccessorSwallowsDeferredRebuildError(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	mock := &rebuildMockChain{
		bestHeight: 5,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}
	idx.SetChainAccessor(mock)
	if idx.chain != mock {
		t.Fatal("SetChainAccessor should retain the provided accessor even on rebuild failure")
	}
	if len(mock.blockRequests) == 0 && mock.forEachCalls == 0 {
		t.Fatal("expected SetChainAccessor to attempt deferred rebuild work")
	}
}
