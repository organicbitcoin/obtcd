// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TestNewExpiryIndex tests ExpiryIndex creation
func TestNewExpiryIndex(t *testing.T) {
	tests := []struct {
		name   string
		params *chaincfg.Params
		valid  bool
	}{
		{
			name:   "OBTC mainnet",
			params: &chaincfg.ObtcMainNetParams,
			valid:  true,
		},
		{
			name:   "OBTC testnet",
			params: &chaincfg.ObtcTestNetParams,
			valid:  true,
		},
		{
			name:   "OBTC regtest",
			params: &chaincfg.ObtcRegTestParams,
			valid:  true,
		},
		{
			name:   "Bitcoin mainnet (should fail)",
			params: &chaincfg.MainNetParams,
			valid:  false,
		},
		{
			name:   "nil params (should fail)",
			params: nil,
			valid:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createCoreTestDB()
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}
			defer teardown()

			idx, err := NewExpiryIndex(db, test.params)
			if test.valid {
				if err != nil {
					t.Errorf("Expected successful creation, got error: %v", err)
				}
				if idx == nil {
					t.Error("Expected valid index, got nil")
				}
			} else {
				if err == nil {
					t.Error("Expected error for invalid network, got nil")
				}
			}
		})
	}
}

// TestExpiryIndexInterface tests that ExpiryIndex implements required interfaces
func TestExpiryIndexInterface(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Test Key() method
	key := idx.Key()
	expectedKey := []byte("expiryindex")
	if string(key) != string(expectedKey) {
		t.Errorf("Key mismatch: got %s, want %s", string(key), string(expectedKey))
	}

	// Test Name() method
	name := idx.Name()
	expectedName := "expiry index"
	if name != expectedName {
		t.Errorf("Name mismatch: got %s, want %s", name, expectedName)
	}
}

// TestExpiryIndexCreate tests index creation
func TestExpiryIndexCreate(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Test Create method
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Verify buckets were created
	err = db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if meta.Bucket(bktOutpoint2Expiry) == nil {
			t.Error("outpoint-to-expiry bucket not created")
		}
		if meta.Bucket(bktExpiry2Outpoints) == nil {
			t.Error("expiry-to-outpoints bucket not created")
		}
		if meta.Bucket(bktExpiryMeta) == nil {
			t.Error("metadata bucket not created")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to verify buckets: %v", err)
	}
}

func TestExpiryIndexCreateDisabledNoOp(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	idx.disabled = true

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Disabled create should be a no-op, got %v", err)
	}

	if err := db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if meta.Bucket(bktExpiryMeta) != nil || meta.Bucket(bktOutpoint2Expiry) != nil || meta.Bucket(bktExpiry2Outpoints) != nil {
			t.Fatal("disabled create should not create expiry buckets")
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to inspect database: %v", err)
	}
}

func TestExpiryIndexCreateIsIdempotentAndStoresVersion(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Repeated create should be idempotent, got %v", err)
	}

	if err := db.View(func(dbTx database.Tx) error {
		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return err
		}
		if version != CurrentIndexVersion {
			t.Fatalf("expected index version %d, got %d", CurrentIndexVersion, version)
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to inspect version: %v", err)
	}
}

func TestExpiryIndexCreateFailsOnClosedTransaction(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	tx, err := db.Begin(true)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Failed to close transaction: %v", err)
	}
	if err := idx.Create(tx); err == nil {
		t.Fatal("expected create on closed transaction to fail")
	}
}

// TestExpiryIndexInit tests index initialization
func TestExpiryIndexInit(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create index first
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Test Init method
	err = idx.Init()
	if err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}
}

func TestExpiryIndexInitDisabledAndMissingExpiryParams(t *testing.T) {
	t.Run("disabled init is a no-op", func(t *testing.T) {
		db, teardown, err := createCoreTestDB()
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		defer teardown()

		idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
		if err != nil {
			t.Fatalf("Failed to create ExpiryIndex: %v", err)
		}
		idx.disabled = true
		idx.curTipHeight = 42

		if err := idx.Init(); err != nil {
			t.Fatalf("Disabled init should succeed, got %v", err)
		}
		if idx.curTipHeight != 42 {
			t.Fatalf("disabled init should not mutate tip height, got %d", idx.curTipHeight)
		}
	})

	t.Run("missing expiry params returns error", func(t *testing.T) {
		db, teardown, err := createCoreTestDB()
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		defer teardown()

		idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
		if err != nil {
			t.Fatalf("Failed to create ExpiryIndex: %v", err)
		}
		idx.expiryParams = nil

		if err := idx.Init(); err == nil {
			t.Fatal("expected init to fail without expiry params")
		}
	})
}

func TestExpiryIndexInitFailsWithoutBuckets(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := idx.Init(); err == nil {
		t.Fatal("expected init without buckets to fail")
	}
}

func TestExpiryIndexInitResetsOnVersionMismatch(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		if err := dbPutIndexVersion(dbTx, CurrentIndexVersion-1); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, 42); err != nil {
			return err
		}
		return dbTx.Metadata().Bucket(bktOutpoint2Expiry).Put([]byte("stale"), []byte("value"))
	}); err != nil {
		t.Fatalf("Failed to seed stale index data: %v", err)
	}

	if err := idx.Init(); err != nil {
		t.Fatalf("Init should reset stale index instead of failing: %v", err)
	}

	if idx.curTipHeight != -1 {
		t.Fatalf("expected reset tip height -1, got %d", idx.curTipHeight)
	}

	if err := db.View(func(dbTx database.Tx) error {
		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return err
		}
		if version != CurrentIndexVersion {
			t.Fatalf("version mismatch after reset: got %d want %d", version, CurrentIndexVersion)
		}

		tip, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if tip != -1 {
			t.Fatalf("expected cleared tip height -1, got %d", tip)
		}

		if got := dbTx.Metadata().Bucket(bktOutpoint2Expiry).Get([]byte("stale")); got != nil {
			t.Fatalf("stale index entry should have been cleared")
		}

		return nil
	}); err != nil {
		t.Fatalf("Failed to verify reset state: %v", err)
	}
}

func TestSetChainAccessorTriggersDeferredRebuild(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to init index: %v", err)
	}

	mock := &rebuildMockChain{
		bestHeight: -1,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}
	idx.SetChainAccessor(mock)

	if mock.forEachCalls != 1 {
		t.Fatalf("expected deferred rebuild to run exactly once, got %d", mock.forEachCalls)
	}
}

func TestExpiryIndexInitWithChainAccessorRunsImmediateRebuild(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	mock := &rebuildMockChain{
		bestHeight: -1,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}
	idx.SetChainAccessor(mock)
	if mock.forEachCalls != 0 {
		t.Fatalf("accessor injection before Init must not rebuild, got %d calls",
			mock.forEachCalls)
	}

	if err := idx.Init(); err != nil {
		t.Fatalf("Init with chain accessor should rebuild immediately, got %v", err)
	}
	if mock.forEachCalls != 1 {
		t.Fatalf("expected immediate rebuild during init, got %d fast rebuild calls", mock.forEachCalls)
	}
}

func TestExpiryIndexInitWithChainAccessorPropagatesRebuildError(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	idx.chain = &rebuildMockChain{
		bestHeight: 5,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}
	if err := idx.Init(); err == nil {
		t.Fatal("expected init to propagate deferred rebuild failure when chain accessor is already set")
	}
}

// TestExpiryCalculation tests UTXO expiry calculations
func TestExpiryCalculation(t *testing.T) {
	tests := []struct {
		name         string
		params       *chaincfg.Params
		createHeight int32
		expectedKey  uint64
	}{
		{
			name:         "OBTC regtest",
			params:       &chaincfg.ObtcRegTestParams,
			createHeight: 100,
			expectedKey:  244, // 100 + 144 blocks
		},
		{
			name:         "OBTC testnet",
			params:       &chaincfg.ObtcTestNetParams,
			createHeight: 1000,
			expectedKey:  1144, // 1000 + 144 blocks
		},
		{
			name:         "OBTC mainnet",
			params:       &chaincfg.ObtcMainNetParams,
			createHeight: 100000,
			expectedKey:  462880, // 100000 + 362880 blocks
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expiryParams := GetExpiryParams(test.params)
			if expiryParams == nil {
				t.Fatalf("Failed to get expiry params for %s", test.name)
			}

			expiryKey := expiryParams.CalculateExpiryKey(test.createHeight)
			if expiryKey != test.expectedKey {
				t.Errorf("Expiry key mismatch: got %d, want %d", expiryKey, test.expectedKey)
			}
		})
	}
}

// TestConnectBlockBasic tests basic ConnectBlock functionality
func TestConnectBlockBasic(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	// Create a test block with identity commitment (accumulator starts empty).
	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, 150, &identityRoot) // Height after fork point

	// Connect the block
	err = db.Update(func(dbTx database.Tx) error {
		spentTxOuts := []blockchain.SpentTxOut{} // No spent outputs for this test
		return idx.ConnectBlock(dbTx, block, spentTxOuts)
	})
	if err != nil {
		t.Fatalf("Failed to connect block: %v", err)
	}

	// Verify index was updated
	err = db.View(func(dbTx database.Tx) error {
		// Check that tip height was updated
		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if height != 150 {
			t.Errorf("Tip height not updated: got %d, want 150", height)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to verify index: %v", err)
	}
}

func TestConnectBlockSkipsGenesisCoinbase(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	genesis := createTestBlock(t, 0)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, genesis, nil)
	}); err != nil {
		t.Fatalf("Failed to connect genesis-height block: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats: %v", err)
	}
	if stats.TipHeight != 0 {
		t.Fatalf("tip height mismatch: got %d want 0", stats.TipHeight)
	}
	if stats.TotalUTXOs != 0 || stats.TotalExpiryKeys != 0 {
		t.Fatalf("genesis coinbase should not be indexed, got stats %+v", stats)
	}

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to read accumulator snapshot: %v", err)
	}
	if snapshot.Root != NewMuHash().Digest() {
		t.Fatalf("genesis coinbase should not change accumulator root: got %x", snapshot.Root)
	}

	candidates, err := idx.ReapPrefixCandidates(10_000, 10)
	if err != nil {
		t.Fatalf("Failed to read REAP candidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("genesis coinbase should not become a REAP candidate: %+v", candidates)
	}
}

func TestConnectBlockDisabledNoOp(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	idx.disabled = true

	block := createTestBlock(t, 150)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("disabled ConnectBlock should be a no-op, got %v", err)
	}

	if err := db.View(func(dbTx database.Tx) error {
		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if height != -1 {
			t.Fatalf("disabled ConnectBlock should not update tip height, got %d", height)
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to inspect disabled state: %v", err)
	}
}

func TestConnectBlockRejectsMissingCommitmentAfterActivation(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	block := createTestBlock(t, idx.expiryParams.ExpiryCommitmentEnableAtHeight)
	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	})
	if err == nil {
		t.Fatal("expected missing commitment to be rejected after activation")
	}
}

func TestConnectBlockRejectsDuplicateCommitmentAfterActivation(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, idx.expiryParams.ExpiryCommitmentEnableAtHeight, &identityRoot)
	block.MsgBlock().Transactions[0].TxOut = append(
		block.MsgBlock().Transactions[0].TxOut,
		&wire.TxOut{Value: 0, PkScript: BuildExpiryCommitmentScript(identityRoot)},
	)

	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	})
	if err == nil {
		t.Fatal("expected duplicate commitment to be rejected after activation")
	}
}

func TestConnectBlockFailsOnClosedTransaction(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tx, err := db.Begin(true)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Failed to close transaction: %v", err)
	}

	block := createTestBlockWithCommitment(t, 150, func() *[AccumulatorDigestSize]byte {
		root := NewMuHash().Digest()
		return &root
	}())
	if err := idx.ConnectBlock(tx, block, nil); err == nil {
		t.Fatal("expected ConnectBlock on a closed transaction to fail")
	}
}

func TestConnectBlockFailsOnReadOnlyTransaction(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, 150, &identityRoot)
	tx, err := db.Begin(false)
	if err != nil {
		t.Fatalf("Failed to begin read-only transaction: %v", err)
	}
	defer tx.Rollback()

	if err := idx.ConnectBlock(tx, block, nil); err == nil {
		t.Fatal("expected ConnectBlock on a read-only transaction to fail")
	}
}

func TestConnectBlockFailsWhenSpentIndexEntryIsInconsistent(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	indexedPrev := wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("broken-prev")), Index: 0}
	prevHash := chainhash.DoubleHashH([]byte("broken-tip"))
	mh := NewMuHash()
	mh.Add(computeEntryData(&indexedPrev, idx.expiryParams.CalculateExpiryKey(149)))
	if err := db.Update(func(dbTx database.Tx) error {
		if err := dbTx.Metadata().Bucket(bktOutpoint2Expiry).Put(
			encodeOutPoint(&indexedPrev), encodeExpiryKey(idx.expiryParams.CalculateExpiryKey(149)),
		); err != nil {
			return err
		}
		if err := dbPutAccumulatorState(dbTx, mh); err != nil {
			return err
		}
		if err := dbPutAccumulatorTipHash(dbTx, &prevHash); err != nil {
			return err
		}
		return dbPutTipHeightIndexed(dbTx, 149)
	}); err != nil {
		t.Fatalf("Failed to seed inconsistent state: %v", err)
	}
	idx.curTipHeight = 149

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}
	block := createTestMixedSpendBlockWithCommitment(
		t, 150, prevHash, &snapshot.Root, indexedPrev,
		wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("other-prev")), Index: 1},
	)
	stxos := []blockchain.SpentTxOut{
		{Height: 149, PkScript: []byte{0x51}, Amount: 1_0000_0000},
		{Height: 50, PkScript: []byte{0x51}, Amount: 1_0000_0000},
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, stxos)
	})
	if err == nil {
		t.Fatal("expected connect to fail on inconsistent spent entry state")
	}
}

func TestConnectBlockFailsWhenOutputBucketMissing(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return dbTx.Metadata().DeleteBucket(bktExpiry2Outpoints)
	}); err != nil {
		t.Fatalf("Failed to prepare broken buckets: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Init should still succeed with intact meta bucket, got %v", err)
	}

	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, 150, &identityRoot)
	err = db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	})
	if err == nil {
		t.Fatal("expected connect to fail when expiry bucket is missing")
	}
}

func TestGetAccumulatorSnapshotTracksTip(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, 150, &identityRoot)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("Failed to connect block: %v", err)
	}

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("GetAccumulatorSnapshot failed: %v", err)
	}
	if snapshot.TipHeight != block.Height() {
		t.Fatalf("tip height mismatch: got %d want %d", snapshot.TipHeight, block.Height())
	}
	if snapshot.TipHash != *block.Hash() {
		t.Fatalf("tip hash mismatch: got %v want %v", snapshot.TipHash, block.Hash())
	}
}

// TestConnectDisconnectBlock tests connecting and disconnecting blocks
func TestConnectDisconnectBlock(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	// Create a test block with identity commitment (accumulator starts empty).
	identityRoot := NewMuHash().Digest()
	block := createTestBlockWithCommitment(t, 150, &identityRoot)

	var originalStats *ExpiryIndexStats

	// Connect the block
	err = db.Update(func(dbTx database.Tx) error {
		spentTxOuts := []blockchain.SpentTxOut{}
		return idx.ConnectBlock(dbTx, block, spentTxOuts)
	})
	if err != nil {
		t.Fatalf("Failed to connect block: %v", err)
	}

	// Get stats after connecting
	connectedStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after connect: %v", err)
	}

	// Disconnect the block
	err = db.Update(func(dbTx database.Tx) error {
		spentTxOuts := []blockchain.SpentTxOut{}
		return idx.DisconnectBlock(dbTx, block, spentTxOuts)
	})
	if err != nil {
		t.Fatalf("Failed to disconnect block: %v", err)
	}

	// Get stats after disconnecting
	disconnectedStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after disconnect: %v", err)
	}

	// Tip height should be decremented
	if disconnectedStats.TipHeight != connectedStats.TipHeight-1 {
		t.Errorf("Tip height not decremented: got %d, want %d",
			disconnectedStats.TipHeight, connectedStats.TipHeight-1)
	}

	// UTXO count should be back to original (approximately)
	if originalStats != nil && disconnectedStats.TotalUTXOs != originalStats.TotalUTXOs {
		t.Errorf("UTXO count not restored: got %d, want %d",
			disconnectedStats.TotalUTXOs, originalStats.TotalUTXOs)
	}
}

func TestDisconnectBlockDisabledNoOp(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	idx.disabled = true

	block := createTestBlock(t, 150)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("disabled DisconnectBlock should be a no-op, got %v", err)
	}

	if err := db.View(func(dbTx database.Tx) error {
		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if height != -1 {
			t.Fatalf("disabled DisconnectBlock should not update tip height, got %d", height)
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to inspect disabled state: %v", err)
	}
}

func TestDisconnectBlockErrorsOnShortSpentJournal(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}
	idx.expiryParams.EnableAtHeight = 0
	idx.expiryParams.ExpiryCommitmentEnableAtHeight = 0

	block := createTestMixedSpendBlockWithCommitment(
		t,
		150,
		chainhash.DoubleHashH([]byte("prev-hash")),
		nil,
		wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("indexed")), Index: 0},
		wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("prestart")), Index: 1},
	)
	shortSpentJournal := []blockchain.SpentTxOut{{Height: 149}}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, shortSpentJournal)
	})
	if err == nil {
		t.Fatal("expected DisconnectBlock to fail with undersized spent journal")
	}
}

func TestDisconnectBlockFailsOnClosedTransaction(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tx, err := db.Begin(true)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Failed to close transaction: %v", err)
	}

	idx.expiryParams.EnableAtHeight = 0
	idx.expiryParams.ExpiryCommitmentEnableAtHeight = 0
	block := createTestBlock(t, 150)
	if err := idx.DisconnectBlock(tx, block, nil); err == nil {
		t.Fatal("expected DisconnectBlock on a closed transaction to fail")
	}
}

func TestDisconnectBlockFailsOnReadOnlyTransaction(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}
	idx.expiryParams.EnableAtHeight = 0
	idx.expiryParams.ExpiryCommitmentEnableAtHeight = 0

	block := createTestBlock(t, 150)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("Failed to seed connected block: %v", err)
	}

	tx, err := db.Begin(false)
	if err != nil {
		t.Fatalf("Failed to begin read-only transaction: %v", err)
	}
	defer tx.Rollback()

	if err := idx.DisconnectBlock(tx, block, nil); err == nil {
		t.Fatal("expected DisconnectBlock on a read-only transaction to fail")
	}
}

func TestDisconnectBlockFailsWhenCreatedOutputBucketIsMissing(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}
	idx.expiryParams.EnableAtHeight = 0
	idx.expiryParams.ExpiryCommitmentEnableAtHeight = 0

	block := createTestBlock(t, 150)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("Failed to connect block before breakage: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().DeleteBucket(bktOutpoint2Expiry)
	}); err != nil {
		t.Fatalf("Failed to drop outpoint bucket: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, nil)
	})
	if err == nil {
		t.Fatal("expected disconnect to fail when created output bucket is missing")
	}
}

func TestConnectDisconnectBlockBeforeStartScanHeightOnlyMovesTip(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	idx.expiryParams.StartScanHeight = 100
	preScanHeight := idx.expiryParams.StartScanHeight - 1
	block := createTestBlock(t, preScanHeight)
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("Failed to connect pre-scan block: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats after connect: %v", err)
	}
	if stats.TotalUTXOs != 0 || stats.TotalExpiryKeys != 0 {
		t.Fatalf("pre-scan connect should not index UTXOs, got stats %+v", stats)
	}
	if stats.TipHeight != preScanHeight {
		t.Fatalf("pre-scan connect tip mismatch: got %d want %d", stats.TipHeight, preScanHeight)
	}

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to read snapshot after connect: %v", err)
	}
	if snapshot.TipHash != *block.Hash() {
		t.Fatalf("pre-scan connect should update tip hash: got %v want %v", snapshot.TipHash, block.Hash())
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("Failed to disconnect pre-scan block: %v", err)
	}

	stats, err = idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats after disconnect: %v", err)
	}
	if stats.TotalUTXOs != 0 || stats.TotalExpiryKeys != 0 {
		t.Fatalf("pre-scan disconnect should keep index empty, got stats %+v", stats)
	}
	if stats.TipHeight != preScanHeight-1 {
		t.Fatalf("pre-scan disconnect tip mismatch: got %d want %d", stats.TipHeight, preScanHeight-1)
	}
}

func TestConnectDisconnectBlockRoundTripWithMixedInputs(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	idx.expiryParams.StartScanHeight = 100

	indexedPrev := wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("indexed-prev")), Index: 0}
	preStartPrev := wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("pre-start-prev")), Index: 1}
	prevHash := chainhash.DoubleHashH([]byte("tip-149"))
	indexedHeight := int32(149)
	preStartHeight := int32(50)

	initialMuHash := NewMuHash()
	initialMuHash.Add(computeEntryData(&indexedPrev, idx.expiryParams.CalculateExpiryKey(indexedHeight)))
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.connectTxOut(dbTx, &indexedPrev, indexedHeight); err != nil {
			return err
		}
		if err := dbPutAccumulatorState(dbTx, initialMuHash); err != nil {
			return err
		}
		if err := dbPutAccumulatorTipHash(dbTx, &prevHash); err != nil {
			return err
		}
		return dbPutTipHeightIndexed(dbTx, indexedHeight)
	}); err != nil {
		t.Fatalf("Failed to seed initial indexed state: %v", err)
	}
	idx.curTipHeight = indexedHeight

	beforeSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to get initial snapshot: %v", err)
	}
	block := createTestMixedSpendBlockWithCommitment(t, 150, prevHash, &beforeSnapshot.Root, indexedPrev, preStartPrev)
	stxos := []blockchain.SpentTxOut{
		{Amount: 1_0000_0000, PkScript: []byte{0x51}, Height: indexedHeight, IsCoinBase: false},
		{Amount: 1_0000_0000, PkScript: []byte{0x51}, Height: preStartHeight, IsCoinBase: false},
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, stxos)
	}); err != nil {
		t.Fatalf("Failed to connect mixed-input block: %v", err)
	}

	connectedStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after connect: %v", err)
	}
	if connectedStats.TotalUTXOs != 3 {
		t.Fatalf("expected 3 indexed outputs after connect, got %d", connectedStats.TotalUTXOs)
	}
	if connectedStats.TotalExpiryKeys != 1 {
		t.Fatalf("expected a single expiry key after connect, got %d", connectedStats.TotalExpiryKeys)
	}

	connectedRows, hasMore, err := idx.ScanExpiringUTXOs(0, ^uint64(0), 10, nil)
	if err != nil {
		t.Fatalf("Failed to scan after connect: %v", err)
	}
	if hasMore {
		t.Fatal("expected mixed-input block scan to fit on one page")
	}
	if len(connectedRows) != 3 {
		t.Fatalf("expected 3 scanned outputs after connect, got %d", len(connectedRows))
	}
	for _, row := range connectedRows {
		if row.ExpiryKey != idx.expiryParams.CalculateExpiryKey(150) {
			t.Fatalf("unexpected expiry key after connect: got %d want %d",
				row.ExpiryKey, idx.expiryParams.CalculateExpiryKey(150))
		}
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, stxos)
	}); err != nil {
		t.Fatalf("Failed to disconnect mixed-input block: %v", err)
	}

	afterSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to get snapshot after disconnect: %v", err)
	}
	if afterSnapshot != beforeSnapshot {
		t.Fatalf("disconnect should restore snapshot: got %+v want %+v", afterSnapshot, beforeSnapshot)
	}

	finalStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after disconnect: %v", err)
	}
	if finalStats.TotalUTXOs != 1 || finalStats.TotalExpiryKeys != 1 {
		t.Fatalf("disconnect should restore original indexed entry, got stats %+v", finalStats)
	}

	finalRows, hasMore, err := idx.ScanExpiringUTXOs(0, ^uint64(0), 10, nil)
	if err != nil {
		t.Fatalf("Failed to scan after disconnect: %v", err)
	}
	if hasMore {
		t.Fatal("expected restored scan to fit on one page")
	}
	if len(finalRows) != 1 || finalRows[0].OutPoint != indexedPrev {
		t.Fatalf("disconnect should restore only indexed previous outpoint, got %v", finalRows)
	}
}

// createTestBlock creates a test block with the given height
func createTestBlock(t *testing.T, height int32) *btcutil.Block {
	return createTestBlockWithCommitment(t, height, nil)
}

// createTestBlockWithCommitment creates a test block, optionally embedding
// an expiry commitment. If root is non-nil, a commitment output is added.
func createTestBlockWithCommitment(t *testing.T, height int32, root *[AccumulatorDigestSize]byte) *btcutil.Block {
	t.Helper()

	// Create a block header
	prevHash, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000000")
	merkleRoot, _ := chainhash.NewHashFromStr("1111111111111111111111111111111111111111111111111111111111111111")

	header := &wire.BlockHeader{
		Version:    1,
		PrevBlock:  *prevHash,
		MerkleRoot: *merkleRoot,
		Timestamp:  time.Now(),
		Bits:       0x207fffff,
		Nonce:      0,
	}

	// Create a coinbase transaction
	coinbaseTx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
				SignatureScript:  []byte{0x00, 0x00},
				Sequence:         0xffffffff,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    5000000000,   // 50 BTC
				PkScript: []byte{0x51}, // OP_1
			},
		},
		LockTime: 0,
	}

	// Add expiry commitment output if root is provided.
	if root != nil {
		coinbaseTx.TxOut = append(coinbaseTx.TxOut, &wire.TxOut{
			Value:    0,
			PkScript: BuildExpiryCommitmentScript(*root),
		})
	}

	// Create a block
	msgBlock := &wire.MsgBlock{
		Header:       *header,
		Transactions: []*wire.MsgTx{coinbaseTx},
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)

	return block
}

func createTestMixedSpendBlockWithCommitment(t *testing.T, height int32, prevHash chainhash.Hash,
	root *[AccumulatorDigestSize]byte, indexedPrevOut, preStartPrevOut wire.OutPoint) *btcutil.Block {
	t.Helper()

	header := &wire.BlockHeader{
		Version:    1,
		PrevBlock:  prevHash,
		MerkleRoot: chainhash.DoubleHashH([]byte("mixed-spend-block")),
		Timestamp:  time.Now(),
		Bits:       0x207fffff,
	}

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
		SignatureScript:  []byte{0x01, 0x00},
		Sequence:         0xffffffff,
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 50_0000_0000, PkScript: []byte{txscript.OP_TRUE}})
	if root != nil {
		coinbase.AddTxOut(&wire.TxOut{
			Value:    0,
			PkScript: BuildExpiryCommitmentScript(*root),
		})
	}

	spendIndexed := wire.NewMsgTx(1)
	spendIndexed.AddTxIn(&wire.TxIn{
		PreviousOutPoint: indexedPrevOut,
		SignatureScript:  []byte{txscript.OP_TRUE},
		Sequence:         0xffffffff,
	})
	spendIndexed.AddTxOut(&wire.TxOut{Value: 1_0000_0000, PkScript: []byte{txscript.OP_TRUE}})
	spendIndexed.AddTxOut(&wire.TxOut{Value: 0, PkScript: []byte{txscript.OP_RETURN, 0x01, 0x01}})

	spendPreStart := wire.NewMsgTx(1)
	spendPreStart.AddTxIn(&wire.TxIn{
		PreviousOutPoint: preStartPrevOut,
		SignatureScript:  []byte{txscript.OP_TRUE},
		Sequence:         0xffffffff,
	})
	spendPreStart.AddTxOut(&wire.TxOut{Value: 2_0000_0000, PkScript: []byte{txscript.OP_TRUE}})

	msgBlock := &wire.MsgBlock{
		Header:       *header,
		Transactions: []*wire.MsgTx{coinbase, spendIndexed, spendPreStart},
	}
	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)
	return block
}

// createCoreTestDB creates a temporary database for testing
func createCoreTestDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_core_test_")
	if err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database
	db, err := database.Create("ffldb", dbPath, wire.TestNet3)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, nil, err
	}

	teardown := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, teardown, nil
}
