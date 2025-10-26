// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

// TestCreateBuckets tests the creation of ExpiryIndex buckets
func TestCreateBuckets(t *testing.T) {
	// Create a temporary database
	db, teardown, err := createTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Test bucket creation
	err = db.Update(func(dbTx database.Tx) error {
		return createExpiryIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create buckets: %v", err)
	}

	// Verify buckets exist
	err = db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		
		// Check outpoint-to-expiry bucket
		outpointBucket := meta.Bucket(bktOutpoint2Expiry)
		if outpointBucket == nil {
			t.Error("outpoint-to-expiry bucket not created")
		}
		
		// Check expiry-to-outpoints bucket
		expiryBucket := meta.Bucket(bktExpiry2Outpoints)
		if expiryBucket == nil {
			t.Error("expiry-to-outpoints bucket not created")
		}
		
		// Check metadata bucket
		metaBucket := meta.Bucket(bktExpiryMeta)
		if metaBucket == nil {
			t.Error("metadata bucket not created")
		}
		
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to verify buckets: %v", err)
	}
}

// TestDropBuckets tests the deletion of ExpiryIndex buckets
func TestDropBuckets(t *testing.T) {
	// Create a temporary database
	db, teardown, err := createTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Create buckets first
	err = db.Update(func(dbTx database.Tx) error {
		return createExpiryIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create buckets: %v", err)
	}

	// Drop buckets
	err = db.Update(func(dbTx database.Tx) error {
		return dropExpiryIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to drop buckets: %v", err)
	}

	// Verify buckets are gone
	err = db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		
		// Check outpoint-to-expiry bucket
		outpointBucket := meta.Bucket(bktOutpoint2Expiry)
		if outpointBucket != nil {
			t.Error("outpoint-to-expiry bucket still exists after drop")
		}
		
		// Check expiry-to-outpoints bucket
		expiryBucket := meta.Bucket(bktExpiry2Outpoints)
		if expiryBucket != nil {
			t.Error("expiry-to-outpoints bucket still exists after drop")
		}
		
		// Check metadata bucket
		metaBucket := meta.Bucket(bktExpiryMeta)
		if metaBucket != nil {
			t.Error("metadata bucket still exists after drop")
		}
		
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to verify bucket deletion: %v", err)
	}
}

// TestMetadataOperations tests metadata read/write operations
func TestMetadataOperations(t *testing.T) {
	// Create a temporary database
	db, teardown, err := createTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Create buckets
	err = db.Update(func(dbTx database.Tx) error {
		return createExpiryIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create buckets: %v", err)
	}

	// Test tip height operations
	testHeights := []int32{0, 100, 12345, 999999}
	
	for _, height := range testHeights {
		t.Run("height", func(t *testing.T) {
			// Write tip height
			err = db.Update(func(dbTx database.Tx) error {
				return dbPutTipHeightIndexed(dbTx, height)
			})
			if err != nil {
				t.Fatalf("Failed to write tip height %d: %v", height, err)
			}
			
			// Read tip height
			var readHeight int32
			err = db.View(func(dbTx database.Tx) error {
				var err error
				readHeight, err = dbGetTipHeightIndexed(dbTx)
				return err
			})
			if err != nil {
				t.Fatalf("Failed to read tip height: %v", err)
			}
			
			// Verify
			if readHeight != height {
				t.Errorf("Height mismatch: got %d, want %d", readHeight, height)
			}
		})
	}
}

// TestEmptyMetadataRead tests reading metadata when no data exists
func TestEmptyMetadataRead(t *testing.T) {
	// Create a temporary database
	db, teardown, err := createTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Create buckets but don't write any metadata
	err = db.Update(func(dbTx database.Tx) error {
		return createExpiryIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create buckets: %v", err)
	}

	// Try to read tip height when none exists
	err = db.View(func(dbTx database.Tx) error {
		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			t.Errorf("Unexpected error when reading non-existent tip height: %v", err)
		}
		// Should return -1 when no data exists
		if height != -1 {
			t.Errorf("Expected height -1 for non-existent data, got %d", height)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Database view failed: %v", err)
	}
}

// TestBucketCreationIdempotent tests that creating buckets multiple times is safe
func TestBucketCreationIdempotent(t *testing.T) {
	// Create a temporary database
	db, teardown, err := createTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	// Create buckets multiple times
	for i := 0; i < 3; i++ {
		err = db.Update(func(dbTx database.Tx) error {
			return createExpiryIndexBuckets(dbTx)
		})
		if err != nil {
			t.Fatalf("Failed to create buckets on iteration %d: %v", i, err)
		}
	}

	// Verify buckets still exist and work
	err = db.Update(func(dbTx database.Tx) error {
		return dbPutTipHeightIndexed(dbTx, 12345)
	})
	if err != nil {
		t.Fatalf("Failed to write metadata after multiple bucket creations: %v", err)
	}
}

// createTestDB creates a temporary database for testing
func createTestDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_test_")
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