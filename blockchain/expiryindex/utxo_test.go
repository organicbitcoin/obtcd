// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
					if got := expiryBucket.Get(encodedExpiry); got != nil {
						t.Error("legacy expiry key entry should not exist in composite index")
						return nil
					}

					compositeKey := encodeExpiryOutpointCompositeKey(expectedExpiry, outpoint)
					if got := expiryBucket.Get(compositeKey); got == nil {
						t.Error("Outpoint not found in expiry composite index")
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

					expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
					if expiryBucket != nil {
						expectedExpiry := idx.expiryParams.CalculateExpiryKey(150)
						compositeKey := encodeExpiryOutpointCompositeKey(expectedExpiry, test.outpoint)
						if got := expiryBucket.Get(compositeKey); got != nil {
							t.Error("Outpoint still exists in expiry composite index after removal")
						}
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

func TestPutTxOutMappingErrorsWhenBucketsMissing(t *testing.T) {
	tests := []struct {
		name       string
		dropBucket []byte
		wantErr    string
	}{
		{
			name:       "missing outpoint bucket",
			dropBucket: bktOutpoint2Expiry,
			wantErr:    "outpoint-to-expiry bucket does not exist",
		},
		{
			name:       "missing expiry bucket",
			dropBucket: bktExpiry2Outpoints,
			wantErr:    "expiry-to-outpoints bucket does not exist",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createUTXOTestDB()
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
			if err := db.Update(func(dbTx database.Tx) error {
				return dbTx.Metadata().DeleteBucket(test.dropBucket)
			}); err != nil {
				t.Fatalf("Failed to drop bucket: %v", err)
			}

			outpoint := &wire.OutPoint{Hash: chainhash.DoubleHashH([]byte(test.name)), Index: 0}
			err = db.Update(func(dbTx database.Tx) error {
				return putTxOutMapping(dbTx, outpoint, 123)
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestDisconnectTxOutErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(database.Tx, *wire.OutPoint) error
		wantErr string
	}{
		{
			name: "missing outpoint bucket",
			prepare: func(dbTx database.Tx, outpoint *wire.OutPoint) error {
				return dbTx.Metadata().DeleteBucket(bktOutpoint2Expiry)
			},
			wantErr: "outpoint-to-expiry bucket does not exist",
		},
		{
			name: "invalid expiry key encoding",
			prepare: func(dbTx database.Tx, outpoint *wire.OutPoint) error {
				return dbTx.Metadata().Bucket(bktOutpoint2Expiry).Put(encodeOutPoint(outpoint), []byte{0x01})
			},
			wantErr: "failed to decode expiry key",
		},
		{
			name: "missing expiry bucket",
			prepare: func(dbTx database.Tx, outpoint *wire.OutPoint) error {
				if err := dbTx.Metadata().Bucket(bktOutpoint2Expiry).Put(
					encodeOutPoint(outpoint), encodeExpiryKey(200),
				); err != nil {
					return err
				}
				return dbTx.Metadata().DeleteBucket(bktExpiry2Outpoints)
			},
			wantErr: "expiry-to-outpoints bucket does not exist",
		},
		{
			name: "missing composite entry",
			prepare: func(dbTx database.Tx, outpoint *wire.OutPoint) error {
				return dbTx.Metadata().Bucket(bktOutpoint2Expiry).Put(
					encodeOutPoint(outpoint), encodeExpiryKey(200),
				)
			},
			wantErr: "inconsistent index",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createUTXOTestDB()
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

			outpoint := &wire.OutPoint{
				Hash:  chainhash.DoubleHashH([]byte(test.name)),
				Index: 3,
			}
			if err := db.Update(func(dbTx database.Tx) error {
				return test.prepare(dbTx, outpoint)
			}); err != nil {
				t.Fatalf("Failed to prepare corrupt state: %v", err)
			}

			err = db.Update(func(dbTx database.Tx) error {
				return idx.disconnectTxOut(dbTx, outpoint)
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
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

func TestGetStatsCountsDistinctExpiryKeys(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	hash1 := chainhash.DoubleHashH([]byte("stats-1"))
	hash2 := chainhash.DoubleHashH([]byte("stats-2"))
	hash3 := chainhash.DoubleHashH([]byte("stats-3"))
	outpoint1 := &wire.OutPoint{Hash: hash1, Index: 0}
	outpoint2 := &wire.OutPoint{Hash: hash2, Index: 0}
	outpoint3 := &wire.OutPoint{Hash: hash3, Index: 0}

	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.connectTxOut(dbTx, outpoint1, 10); err != nil {
			return err
		}
		if err := idx.connectTxOut(dbTx, outpoint2, 10); err != nil {
			return err
		}
		return idx.connectTxOut(dbTx, outpoint3, 11)
	}); err != nil {
		t.Fatalf("Failed to seed index: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalUTXOs != 3 {
		t.Fatalf("unexpected utxo count: got %d want 3", stats.TotalUTXOs)
	}
	if stats.TotalExpiryKeys != 2 {
		t.Fatalf("unexpected expiry key count: got %d want 2", stats.TotalExpiryKeys)
	}
}

func TestDisconnectTxOutRemovesLastEntryFromSharedExpiryKey(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	opA := &wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("shared-a")), Index: 0}
	opB := &wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("shared-b")), Index: 0}
	opC := &wire.OutPoint{Hash: chainhash.DoubleHashH([]byte("distinct-c")), Index: 0}

	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.connectTxOut(dbTx, opA, 10); err != nil {
			return err
		}
		if err := idx.connectTxOut(dbTx, opB, 10); err != nil {
			return err
		}
		return idx.connectTxOut(dbTx, opC, 11)
	}); err != nil {
		t.Fatalf("Failed to seed index: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read initial stats: %v", err)
	}
	if stats.TotalExpiryKeys != 2 {
		t.Fatalf("unexpected initial expiry key count: got %d want 2", stats.TotalExpiryKeys)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.disconnectTxOut(dbTx, opA)
	}); err != nil {
		t.Fatalf("Failed to disconnect first shared key entry: %v", err)
	}
	stats, err = idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats after first removal: %v", err)
	}
	if stats.TotalExpiryKeys != 2 {
		t.Fatalf("expected shared expiry key to remain after first removal, got %d", stats.TotalExpiryKeys)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.disconnectTxOut(dbTx, opB)
	}); err != nil {
		t.Fatalf("Failed to disconnect second shared key entry: %v", err)
	}
	stats, err = idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats after second removal: %v", err)
	}
	if stats.TotalExpiryKeys != 1 {
		t.Fatalf("expected expiry key count to decrement after last shared entry, got %d", stats.TotalExpiryKeys)
	}
}

func TestDisconnectTxOutSequentiallyDrainsSharedExpiryKeyWithoutGhostEntries(t *testing.T) {
	db, teardown, err := createUTXOTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	outpoints := []wire.OutPoint{
		{Hash: chainhash.DoubleHashH([]byte("drain-a")), Index: 0},
		{Hash: chainhash.DoubleHashH([]byte("drain-a")), Index: 2},
		{Hash: chainhash.DoubleHashH([]byte("drain-b")), Index: 0},
	}
	sort.Slice(outpoints, func(i, j int) bool {
		return compareOutPoint(&outpoints[i], &outpoints[j]) < 0
	})

	if err := db.Update(func(dbTx database.Tx) error {
		for i := range outpoints {
			if err := idx.connectTxOut(dbTx, &outpoints[i], 20); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to seed shared expiry key: %v", err)
	}

	expectedRemaining := append([]wire.OutPoint(nil), outpoints...)
	expiryKey := idx.expiryParams.CalculateExpiryKey(20)
	for i := range outpoints {
		op := outpoints[i]
		if err := db.Update(func(dbTx database.Tx) error {
			return idx.disconnectTxOut(dbTx, &op)
		}); err != nil {
			t.Fatalf("Failed to disconnect outpoint %v: %v", op, err)
		}

		expectedRemaining = expectedRemaining[1:]
		rows, hasMore, err := idx.ScanExpiringUTXOs(expiryKey, expiryKey, 10, nil)
		if err != nil {
			t.Fatalf("Failed to scan after removal %d: %v", i, err)
		}
		if hasMore {
			t.Fatalf("expected final scan page after removal %d", i)
		}
		if len(rows) != len(expectedRemaining) {
			t.Fatalf("remaining row count mismatch after removal %d: got %d want %d",
				i, len(rows), len(expectedRemaining))
		}
		for j := range expectedRemaining {
			if rows[j].OutPoint != expectedRemaining[j] {
				t.Fatalf("remaining outpoint mismatch at %d after removal %d: got %v want %v",
					j, i, rows[j].OutPoint, expectedRemaining[j])
			}
		}
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read final stats: %v", err)
	}
	if stats.TotalUTXOs != 0 || stats.TotalExpiryKeys != 0 {
		t.Fatalf("expected fully drained shared expiry key, got stats %+v", stats)
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
