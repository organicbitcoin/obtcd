// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

// TestConnectTxOut tests adding UTXOs to the index
func TestConnectTxOut(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
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

	tests := []struct {
		name         string
		createHeight int32
		shouldIndex  bool
	}{
		{
			name:         "valid height after fork",
			createHeight: 150,
			shouldIndex:  true,
		},
		{
			name:         "valid height near genesis",
			createHeight: 1,
			shouldIndex:  true,
		},
		{
			name:         "negative height should skip",
			createHeight: -1,
			shouldIndex:  false,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hash, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			outpoint := &wire.OutPoint{
				Hash:  *hash,
				Index: uint32(i),
			}

			err = db.Update(func(dbTx database.Tx) error {
				return idx.connectTxOut(dbTx, outpoint, test.createHeight)
			})

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if test.shouldIndex {
				err = db.View(func(dbTx database.Tx) error {
					// Check outpoint-to-expiry mapping
					outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
					if outpointBucket == nil {
						t.Error("Outpoint bucket not found")
						return nil
					}

					encodedOutpoint := encodeOutPoint(outpoint)
					expiryBytes := outpointBucket.Get(encodedOutpoint)
					if expiryBytes == nil {
						t.Error("Outpoint not found in index")
						return nil
					}

					// Verify expiry calculation
					expectedExpiry := idx.expiryParams.CalculateExpiryKey(test.createHeight)
					actualExpiry, err := decodeExpiryKey(expiryBytes)
					if err != nil {
						t.Errorf("Failed to decode expiry: %v", err)
						return nil
					}

					if actualExpiry != expectedExpiry {
						t.Errorf("Expiry mismatch: got %d, want %d", actualExpiry, expectedExpiry)
					}

					// Check expiry-to-outpoints mapping
					expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
					if expiryBucket == nil {
						t.Error("Expiry bucket not found")
						return nil
					}

					encodedExpiry := encodeExpiryKey(expectedExpiry)
					outpointListBytes := expiryBucket.Get(encodedExpiry)
					if outpointListBytes == nil {
						t.Error("Expiry key not found in index")
						return nil
					}

					// Verify outpoint is in the list
					outpointList, err := decodeOutPointList(outpointListBytes)
					if err != nil {
						t.Errorf("Failed to decode outpoint list: %v", err)
						return nil
					}

					found := false
					for _, op := range outpointList {
						if op.Hash.IsEqual(&outpoint.Hash) && op.Index == outpoint.Index {
							found = true
							break
						}
					}

					if !found {
						t.Error("Outpoint not found in expiry list")
					}

					return nil
				})

				if err != nil {
					t.Errorf("Verification failed: %v", err)
				}
				return
			}

			err = db.View(func(dbTx database.Tx) error {
				outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
				if outpointBucket == nil {
					t.Error("Outpoint bucket not found")
					return nil
				}
				if got := outpointBucket.Get(encodeOutPoint(outpoint)); got != nil {
					t.Error("Outpoint should not have been indexed")
				}
				return nil
			})
			if err != nil {
				t.Errorf("Skip verification failed: %v", err)
			}
		})
	}
}

// TestDisconnectTxOut tests removing UTXOs from the index
func TestDisconnectTxOut(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
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

	// Create test outpoints
	hash1, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	hash2, _ := chainhash.NewHashFromStr("1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	outpoint1 := &wire.OutPoint{Hash: *hash1, Index: 0}
	outpoint2 := &wire.OutPoint{Hash: *hash2, Index: 1}

	// First, add some UTXOs
	err = db.Update(func(dbTx database.Tx) error {
		err := idx.connectTxOut(dbTx, outpoint1, 150)
		if err != nil {
			return err
		}
		return idx.connectTxOut(dbTx, outpoint2, 150)
	})
	if err != nil {
		t.Fatalf("Failed to add initial UTXOs: %v", err)
	}

	tests := []struct {
		name        string
		outpoint    *wire.OutPoint
		expectError bool
	}{
		{
			name:        "remove existing UTXO",
			outpoint:    outpoint1,
			expectError: false,
		},
		{
			name:        "remove non-existing UTXO",
			outpoint:    &wire.OutPoint{Hash: chainhash.Hash{}, Index: 999},
			expectError: false, // Should not error, just no-op
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Check if UTXO exists before removal
			var existedBefore bool
			db.View(func(dbTx database.Tx) error {
				outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
				if outpointBucket != nil {
					encodedOutpoint := encodeOutPoint(test.outpoint)
					existedBefore = outpointBucket.Get(encodedOutpoint) != nil
				}
				return nil
			})

			// Attempt removal
			err = db.Update(func(dbTx database.Tx) error {
				return idx.disconnectTxOut(dbTx, test.outpoint)
			})

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify removal (only check if it existed before)
			if existedBefore {
				err = db.View(func(dbTx database.Tx) error {
					// Check outpoint-to-expiry mapping is gone
					outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
					if outpointBucket == nil {
						return nil // Bucket might not exist if empty
					}

					encodedOutpoint := encodeOutPoint(test.outpoint)
					expiryBytes := outpointBucket.Get(encodedOutpoint)
					if expiryBytes != nil {
						t.Error("Outpoint still exists in index after removal")
					}

					return nil
				})

				if err != nil {
					t.Errorf("Verification failed: %v", err)
				}
			}
		})
	}
}

// TestConnectDisconnectTxOutRoundTrip tests adding and removing the same UTXO
func TestConnectDisconnectTxOutRoundTrip(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
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

	// Create test outpoint
	hash, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	outpoint := &wire.OutPoint{Hash: *hash, Index: 0}

	// Get initial stats
	initialStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get initial stats: %v", err)
	}

	// Add UTXO
	err = db.Update(func(dbTx database.Tx) error {
		return idx.connectTxOut(dbTx, outpoint, 150)
	})
	if err != nil {
		t.Fatalf("Failed to add UTXO: %v", err)
	}

	// Verify addition
	addedStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after addition: %v", err)
	}

	if addedStats.TotalUTXOs <= initialStats.TotalUTXOs {
		t.Errorf("UTXO count should have increased: initial=%d, after_add=%d",
			initialStats.TotalUTXOs, addedStats.TotalUTXOs)
	}

	// Remove UTXO
	err = db.Update(func(dbTx database.Tx) error {
		return idx.disconnectTxOut(dbTx, outpoint)
	})
	if err != nil {
		t.Fatalf("Failed to remove UTXO: %v", err)
	}

	// Verify removal
	finalStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get final stats: %v", err)
	}

	if finalStats.TotalUTXOs != initialStats.TotalUTXOs {
		t.Errorf("UTXO count should be back to initial: initial=%d, final=%d",
			initialStats.TotalUTXOs, finalStats.TotalUTXOs)
	}
}

// createUTXOTestDB creates a temporary database for UTXO operation testing
func createUTXOTestDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_utxo_test_")
	if err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(tmpDir, "utxo_test.db")

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
