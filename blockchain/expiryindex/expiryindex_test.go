// Copyright (c) 2024 The OBTC developers
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
			expectedKey:  2008, // 1000 + 1008 blocks
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
