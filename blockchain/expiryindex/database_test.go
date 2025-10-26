// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

// TestDBPutGetIndexMeta tests metadata storage and retrieval
func TestDBPutGetIndexMeta(t *testing.T) {
	db, teardown, err := createDBTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Initialize the ExpiryIndex to create necessary buckets
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index buckets: %v", err)
	}

	tests := []struct {
		name        string
		key         []byte
		value       []byte
		expectError bool
	}{
		{
			name:        "store simple metadata",
			key:         []byte("test-key-1"),
			value:       []byte("test-value-1"),
			expectError: false,
		},
		{
			name:        "store empty value",
			key:         []byte("test-key-2"),
			value:       []byte{},
			expectError: false,
		},
		{
			name:        "store binary data",
			key:         []byte("test-key-3"),
			value:       []byte{0x01, 0x02, 0x03, 0xFF, 0x00},
			expectError: false,
		},
		{
			name:        "store large value",
			key:         []byte("test-key-4"),
			value:       make([]byte, 1024), // 1KB of zeros
			expectError: false,
		},
		{
			name:        "overwrite existing key",
			key:         []byte("test-key-1"), // Reuse first key
			value:       []byte("updated-value"),
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test putting metadata
			err = db.Update(func(dbTx database.Tx) error {
				return dbPutIndexMeta(dbTx, test.key, test.value)
			})

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to put metadata: %v", err)
				return
			}

			// Test getting metadata
			var retrievedValue []byte
			err = db.View(func(dbTx database.Tx) error {
				var err error
				retrievedValue, err = dbGetIndexMeta(dbTx, test.key)
				return err
			})

			if err != nil {
				t.Errorf("Failed to get metadata: %v", err)
				return
			}

			// Verify values match
			if len(retrievedValue) != len(test.value) {
				t.Errorf("Value length mismatch: got %d, want %d",
					len(retrievedValue), len(test.value))
				return
			}

			for i := 0; i < len(test.value); i++ {
				if retrievedValue[i] != test.value[i] {
					t.Errorf("Value mismatch at index %d: got %x, want %x",
						i, retrievedValue[i], test.value[i])
					break
				}
			}
		})
	}

	// Test getting non-existent key
	t.Run("get non-existent key", func(t *testing.T) {
		err = db.View(func(dbTx database.Tx) error {
			value, err := dbGetIndexMeta(dbTx, []byte("non-existent-key"))
			if err != nil {
				return err
			}
			if value != nil {
				t.Error("Expected nil value for non-existent key")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Unexpected error getting non-existent key: %v", err)
		}
	})
}

// TestDBPutGetIndexVersion tests version storage and retrieval
func TestDBPutGetIndexVersion(t *testing.T) {
	db, teardown, err := createDBTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Initialize the ExpiryIndex to create necessary buckets
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index buckets: %v", err)
	}

	tests := []struct {
		name        string
		version     uint16
		expectError bool
	}{
		{
			name:        "store version 0",
			version:     0,
			expectError: false,
		},
		{
			name:        "store version 1",
			version:     1,
			expectError: false,
		},
		{
			name:        "store large version",
			version:     0xFFFF,
			expectError: false,
		},
		{
			name:        "update version",
			version:     42,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test putting version
			err = db.Update(func(dbTx database.Tx) error {
				return dbPutIndexVersion(dbTx, test.version)
			})

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to put version: %v", err)
				return
			}

			// Test getting version
			var retrievedVersion uint16
			err = db.View(func(dbTx database.Tx) error {
				var err error
				retrievedVersion, err = dbGetIndexVersion(dbTx)
				return err
			})

			if err != nil {
				t.Errorf("Failed to get version: %v", err)
				return
			}

			// Verify versions match
			if retrievedVersion != test.version {
				t.Errorf("Version mismatch: got %d, want %d",
					retrievedVersion, test.version)
			}
		})
	}

	// Test getting version when not set
	t.Run("get version from fresh database", func(t *testing.T) {
		freshDB, freshTeardown, err := createDBTestDB()
		if err != nil {
			t.Fatalf("Failed to create fresh test database: %v", err)
		}
		defer freshTeardown()

		err = freshDB.View(func(dbTx database.Tx) error {
			_, err := dbGetIndexVersion(dbTx)
			// Should return error for non-existent version
			return err
		})

		if err == nil {
			t.Error("Expected error for non-existent version but got none")
		}
	})
}

// TestDBTipHeightIndexed tests tip height storage and retrieval
func TestDBTipHeightIndexed(t *testing.T) {
	db, teardown, err := createDBTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Initialize the ExpiryIndex to create necessary buckets
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index buckets: %v", err)
	}

	tests := []struct {
		name        string
		height      int32
		expectError bool
	}{
		{
			name:        "store height 0",
			height:      0,
			expectError: false,
		},
		{
			name:        "store negative height",
			height:      -1,
			expectError: false,
		},
		{
			name:        "store large positive height",
			height:      1000000,
			expectError: false,
		},
		{
			name:        "store maximum int32",
			height:      0x7FFFFFFF,
			expectError: false,
		},
		{
			name:        "store minimum int32",
			height:      -0x80000000,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test putting height
			err = db.Update(func(dbTx database.Tx) error {
				return dbPutTipHeightIndexed(dbTx, test.height)
			})

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Failed to put tip height: %v", err)
				return
			}

			// Test getting height
			var retrievedHeight int32
			err = db.View(func(dbTx database.Tx) error {
				var err error
				retrievedHeight, err = dbGetTipHeightIndexed(dbTx)
				return err
			})

			if err != nil {
				t.Errorf("Failed to get tip height: %v", err)
				return
			}

			// Verify heights match
			if retrievedHeight != test.height {
				t.Errorf("Height mismatch: got %d, want %d",
					retrievedHeight, test.height)
			}
		})
	}
}

// TestDBMetaBucketCreation tests metadata bucket creation and access
func TestDBMetaBucketCreation(t *testing.T) {
	db, teardown, err := createDBTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Initialize the ExpiryIndex to create necessary buckets
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Test accessing metadata before bucket creation
	t.Run("access before creation", func(t *testing.T) {
		err = db.View(func(dbTx database.Tx) error {
			_, err := dbGetIndexMeta(dbTx, []byte("test-key"))
			// Should handle gracefully (return nil or appropriate error)
			return err
		})

		// Error is acceptable here since bucket doesn't exist
		if err != nil {
			t.Logf("Expected error accessing non-existent bucket: %v", err)
		}
	})

	// Create the metadata bucket by putting something
	t.Run("implicit bucket creation", func(t *testing.T) {
		err = db.Update(func(dbTx database.Tx) error {
			return idx.Create(dbTx)
		})
		if err != nil {
			t.Errorf("Failed to create index buckets: %v", err)
			return
		}

		err = db.Update(func(dbTx database.Tx) error {
			return dbPutIndexMeta(dbTx, []byte("first-key"), []byte("first-value"))
		})

		if err != nil {
			t.Errorf("Failed to create metadata bucket: %v", err)
			return
		}

		// Verify we can now access metadata
		err = db.View(func(dbTx database.Tx) error {
			value, err := dbGetIndexMeta(dbTx, []byte("first-key"))
			if err != nil {
				return err
			}

			expectedValue := []byte("first-value")
			if len(value) != len(expectedValue) {
				t.Errorf("Value length mismatch: got %d, want %d",
					len(value), len(expectedValue))
				return nil
			}

			for i := 0; i < len(expectedValue); i++ {
				if value[i] != expectedValue[i] {
					t.Errorf("Value mismatch at index %d: got %x, want %x",
						i, value[i], expectedValue[i])
					break
				}
			}

			return nil
		})

		if err != nil {
			t.Errorf("Failed to verify bucket creation: %v", err)
		}
	})
}

// TestDBConcurrentAccess tests concurrent database access
func TestDBConcurrentAccess(t *testing.T) {
	db, teardown, err := createDBTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Initialize the ExpiryIndex to create necessary buckets
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index buckets: %v", err)
	}

	// Initialize some data
	err = db.Update(func(dbTx database.Tx) error {
		err := dbPutIndexVersion(dbTx, 1)
		if err != nil {
			return err
		}
		
		err = dbPutTipHeightIndexed(dbTx, 100)
		if err != nil {
			return err
		}

		return dbPutIndexMeta(dbTx, []byte("test-key"), []byte("test-value"))
	})
	if err != nil {
		t.Fatalf("Failed to initialize test data: %v", err)
	}

	// Test multiple concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		results := make(chan error, 3)

		// Start multiple read transactions
		go func() {
			err := db.View(func(dbTx database.Tx) error {
				version, err := dbGetIndexVersion(dbTx)
				if err != nil {
					return err
				}
				if version != 1 {
					t.Errorf("Version mismatch in concurrent read: got %d, want 1", version)
				}
				return nil
			})
			results <- err
		}()

		go func() {
			err := db.View(func(dbTx database.Tx) error {
				height, err := dbGetTipHeightIndexed(dbTx)
				if err != nil {
					return err
				}
				if height != 100 {
					t.Errorf("Height mismatch in concurrent read: got %d, want 100", height)
				}
				return nil
			})
			results <- err
		}()

		go func() {
			err := db.View(func(dbTx database.Tx) error {
				value, err := dbGetIndexMeta(dbTx, []byte("test-key"))
				if err != nil {
					return err
				}
				expectedValue := []byte("test-value")
				if len(value) != len(expectedValue) {
					t.Errorf("Value length mismatch in concurrent read: got %d, want %d",
						len(value), len(expectedValue))
				}
				return nil
			})
			results <- err
		}()

		// Wait for all reads to complete
		for i := 0; i < 3; i++ {
			if err := <-results; err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
		}
	})
}

// createDBTestDB creates a temporary database for DB function testing
func createDBTestDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_db_test_")
	if err != nil {
		return nil, nil, err
	}
	
	dbPath := filepath.Join(tmpDir, "db_test.db")
	
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