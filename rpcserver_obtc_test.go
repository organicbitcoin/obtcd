// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// ---------------------------------------------------------------------------
// handleListExpiring / handleGetExpiryIndexStats
// ---------------------------------------------------------------------------

// createTestExpiryIndex creates a real ExpiryIndex backed by a temporary DB.
func createTestExpiryIndex(t *testing.T) (*expiryindex.ExpiryIndex, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.Create("ffldb", dbPath, wire.ObtcRegNet)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	idx, err := expiryindex.NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		db.Close()
		t.Fatalf("create expiry index: %v", err)
	}
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		db.Close()
		t.Fatalf("init expiry index: %v", err)
	}
	if err := idx.Init(); err != nil {
		db.Close()
		t.Fatalf("init expiry index: %v", err)
	}
	teardown := func() {
		db.Close()
	}
	return idx, teardown
}

func createTestExpiryRPCState(t *testing.T) (*blockchain.BlockChain, *expiryindex.ExpiryIndex, func()) {
	t.Helper()

	blockchain.DisableLog()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.Create("ffldb", dbPath, wire.ObtcRegNet)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	chain, err := blockchain.New(&blockchain.Config{
		DB:               db,
		UtxoCacheMaxSize: 1 << 20,
		ChainParams:      &chaincfg.ObtcRegTestParams,
		TimeSource:       blockchain.NewMedianTime(),
		SigCache:         txscript.NewSigCache(1000),
		HashCache:        txscript.NewHashCache(1000),
	})
	if err != nil {
		db.Close()
		t.Fatalf("create chain: %v", err)
	}

	idx, err := expiryindex.NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		db.Close()
		t.Fatalf("create expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		db.Close()
		t.Fatalf("init expiry index: %v", err)
	}
	if err := idx.Init(); err != nil {
		db.Close()
		t.Fatalf("init expiry index: %v", err)
	}

	return chain, idx, func() {
		db.Close()
	}
}

func TestHandleListExpiringDisabled(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{
		// ExpiryIndex is nil → disabled
	}}
	cmd := btcjson.NewListExpiringCmd(nil, nil, nil, nil)
	closeChan := make(chan struct{})

	result, err := handleListExpiring(s, cmd, closeChan)
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
	if err == nil {
		t.Fatalf("expected RPC error for disabled expiry index")
	}
	rpcErr, ok := err.(*btcjson.RPCError)
	if !ok {
		t.Fatalf("expected *btcjson.RPCError, got %T", err)
	}
	if !strings.Contains(rpcErr.Message, "disabled") {
		t.Fatalf("expected disabled message, got %q", rpcErr.Message)
	}
}

func TestHandleListExpiringInvalidParams(t *testing.T) {
	chain, idx, teardown := createTestExpiryRPCState(t)
	defer teardown()

	s := &rpcServer{cfg: rpcserverConfig{
		Chain:       chain,
		ChainParams: &chaincfg.ObtcRegTestParams,
		ExpiryIndex: idx,
	}}
	badCursor := "bad_cursor"
	cmd := btcjson.NewListExpiringCmd(nil, nil, nil, &badCursor)
	closeChan := make(chan struct{})

	_, err := handleListExpiring(s, cmd, closeChan)
	if err == nil {
		t.Fatalf("expected error")
	}
	rpcErr, ok := err.(*btcjson.RPCError)
	if !ok {
		t.Fatalf("expected *btcjson.RPCError, got %T", err)
	}
	if rpcErr.Code != btcjson.ErrRPCInvalidParameter {
		t.Fatalf("expected invalid parameter error, got code %d", rpcErr.Code)
	}
	if !strings.Contains(rpcErr.Message, "Invalid start_after cursor") {
		t.Fatalf("expected invalid cursor message, got %q", rpcErr.Message)
	}
}

func TestHandleGetExpiryIndexStatsDisabled(t *testing.T) {
	s := &rpcServer{cfg: rpcserverConfig{
		// ExpiryIndex is nil → returns Disabled: true
	}}
	cmd := btcjson.NewGetExpiryIndexStatsCmd()
	closeChan := make(chan struct{})

	result, err := handleGetExpiryIndexStats(s, cmd, closeChan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	statsResult, ok := result.(*btcjson.ExpiryIndexStatsResult)
	if !ok {
		t.Fatalf("expected *ExpiryIndexStatsResult, got %T", result)
	}
	if !statsResult.Disabled {
		t.Fatalf("expected Disabled=true")
	}
}

func TestHandleGetExpiryIndexStatsEnabled(t *testing.T) {
	idx, teardown := createTestExpiryIndex(t)
	defer teardown()

	s := &rpcServer{cfg: rpcserverConfig{
		ExpiryIndex: idx,
		ChainParams: &chaincfg.ObtcRegTestParams,
	}}
	cmd := btcjson.NewGetExpiryIndexStatsCmd()
	closeChan := make(chan struct{})

	result, err := handleGetExpiryIndexStats(s, cmd, closeChan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	statsResult, ok := result.(*btcjson.ExpiryIndexStatsResult)
	if !ok {
		t.Fatalf("expected *ExpiryIndexStatsResult, got %T", result)
	}
	if statsResult.Disabled {
		t.Fatalf("expected Disabled=false with real ExpiryIndex")
	}
	if statsResult.NetworkParams == nil {
		t.Fatalf("expected NetworkParams to be set for OBTC params")
	}
	if statsResult.NetworkParams.WindowBlocks == 0 {
		t.Fatalf("expected non-zero WindowBlocks")
	}
}

func TestHandleGetExpiryIndexStatsNoChainParams(t *testing.T) {
	idx, teardown := createTestExpiryIndex(t)
	defer teardown()

	s := &rpcServer{cfg: rpcserverConfig{
		ExpiryIndex: idx,
		// ChainParams is nil → no NetworkParams in result
	}}
	cmd := btcjson.NewGetExpiryIndexStatsCmd()
	closeChan := make(chan struct{})

	result, err := handleGetExpiryIndexStats(s, cmd, closeChan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	statsResult := result.(*btcjson.ExpiryIndexStatsResult)
	if statsResult.NetworkParams != nil {
		t.Fatalf("expected nil NetworkParams with nil ChainParams")
	}
}

// ---------------------------------------------------------------------------
// parseOutPointCursor
// ---------------------------------------------------------------------------

func TestParseOutPointCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  string
		wantErr bool
		errMsg  string
		vout    uint32
	}{
		{
			name:   "valid cursor",
			cursor: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:0",
			vout:   0,
		},
		{
			name:   "valid cursor vout 42",
			cursor: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:42",
			vout:   42,
		},
		{
			name:    "missing colon",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "too many colons",
			cursor:  "abc:def:0",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "empty string",
			cursor:  "",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "invalid txid",
			cursor:  "not_a_valid_hex_hash:0",
			wantErr: true,
			errMsg:  "invalid txid",
		},
		{
			name:    "invalid vout not a number",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:abc",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:    "negative vout",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:-1",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:    "vout overflow uint32",
			cursor:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890:4294967296",
			wantErr: true,
			errMsg:  "invalid vout",
		},
		{
			name:   "short txid accepted by chainhash",
			cursor: "abcdef:0",
			vout:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := parseOutPointCursor(tc.cursor)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tc.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.Index != tc.vout {
				t.Fatalf("expected vout %d, got %d", tc.vout, op.Index)
			}
		})
	}
}
