// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/database"
)

// Database bucket names for expiry index
//
// The expiry index uses three buckets to maintain bidirectional mappings
// and metadata for efficient UTXO expiry tracking:
//
// 1. bktExpiryMeta: Stores index metadata (version, tip height, etc.)
// 2. bktOutpoint2Expiry: Maps OutPoint -> ExpiryKey for fast deletion
// 3. bktExpiry2Outpoints: Stores ordered composite keys for scanning
// 4. bktReapStrictCandidates: Stores REAP canonical keys for consensus prefix scans
// 5. bktOutpoint2ReapStrict: Maps OutPoint -> REAP canonical key for deletion
var (
	// bktExpiryMeta stores metadata about the index state
	// Keys: keyTipHeightIndexed, keyIndexVersion
	bktExpiryMeta = []byte("expiry-meta")

	// bktOutpoint2Expiry maps outpoint (36 bytes) -> expiry key (8 bytes)
	// This enables fast lookup when UTXOs are spent or renewed
	bktOutpoint2Expiry = []byte("outpoint-to-expiry")

	// bktExpiry2Outpoints stores one ordered composite-key entry per indexed
	// UTXO: expiry key (8 bytes) || ordered outpoint (36 bytes) -> empty.
	// This enables efficient scanning of UTXOs by expiry order.
	bktExpiry2Outpoints = []byte("expiry-to-outpoints")

	// bktReapStrictCandidates stores one ordered composite-key entry per
	// indexed UTXO: expiry key (8 bytes) || amount (8 bytes) || REAP-order
	// outpoint (36 bytes) -> empty.  This enables bounded consensus scans
	// of the canonical global REAP prefix.
	bktReapStrictCandidates = []byte("reap-strict-candidates")

	// bktOutpoint2ReapStrict maps outpoint (36 bytes) -> strict REAP
	// composite key.  This enables fast deletion from bktReapStrictCandidates.
	bktOutpoint2ReapStrict = []byte("outpoint-to-reap-strict")
)

// Metadata keys within bktExpiryMeta bucket
var (
	// keyTipHeightIndexed tracks the highest block height that has been indexed
	keyTipHeightIndexed = []byte("tip-height")

	// keyIndexVersion tracks the index format version for future upgrades
	keyIndexVersion = []byte("version")

	// keyAccumulatorState stores the MuHash accumulator state (768 bytes)
	// for the expiry commitment. Updated atomically with tip height.
	keyAccumulatorState = []byte("accumulator-state")

	// keyAccumulatorTipHash stores the block hash corresponding to the
	// accumulator/tip-height snapshot.
	keyAccumulatorTipHash = []byte("accumulator-tip-hash")
)

// Index configuration constants
const (
	// CurrentIndexVersion tracks the current index format version
	// Increment this when making breaking changes to the index format
	CurrentIndexVersion = 6

	// DefaultBatchSize is the default number of entries to process in a single
	// database transaction to balance memory usage and transaction overhead
	DefaultBatchSize = 1000
)

// dbPutIndexMeta stores metadata in the expiry index meta bucket
func dbPutIndexMeta(dbTx database.Tx, key, value []byte) error {
	metaBucket := dbTx.Metadata().Bucket(bktExpiryMeta)
	if metaBucket == nil {
		return fmt.Errorf("expiry meta bucket does not exist")
	}
	return metaBucket.Put(key, value)
}

// dbGetIndexMeta retrieves metadata from the expiry index meta bucket
func dbGetIndexMeta(dbTx database.Tx, key []byte) ([]byte, error) {
	metaBucket := dbTx.Metadata().Bucket(bktExpiryMeta)
	if metaBucket == nil {
		return nil, fmt.Errorf("expiry meta bucket does not exist")
	}
	return metaBucket.Get(key), nil
}

// dbPutTipHeightIndexed stores the tip height that has been indexed
func dbPutTipHeightIndexed(dbTx database.Tx, height int32) error {
	heightBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBytes, uint32(height))
	return dbPutIndexMeta(dbTx, keyTipHeightIndexed, heightBytes)
}

// dbGetTipHeightIndexed retrieves the tip height that has been indexed
func dbGetTipHeightIndexed(dbTx database.Tx) (int32, error) {
	heightBytes, err := dbGetIndexMeta(dbTx, keyTipHeightIndexed)
	if err != nil {
		return 0, err
	}
	if heightBytes == nil {
		return -1, nil // No height indexed yet
	}
	if len(heightBytes) != 4 {
		return 0, fmt.Errorf("invalid tip height encoding: got %d bytes", len(heightBytes))
	}
	height := binary.LittleEndian.Uint32(heightBytes)
	return int32(height), nil
}

// dbPutIndexVersion stores the current index version
func dbPutIndexVersion(dbTx database.Tx, version uint16) error {
	versionBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(versionBytes, version)
	return dbPutIndexMeta(dbTx, keyIndexVersion, versionBytes)
}

// dbGetIndexVersion retrieves the current index version
func dbGetIndexVersion(dbTx database.Tx) (uint16, error) {
	versionBytes, err := dbGetIndexMeta(dbTx, keyIndexVersion)
	if err != nil {
		return 0, err
	}
	if versionBytes == nil {
		return 0, nil // No version set yet (first time initialization)
	}
	if len(versionBytes) != 2 {
		return 0, fmt.Errorf("invalid index version encoding: got %d bytes", len(versionBytes))
	}
	version := binary.LittleEndian.Uint16(versionBytes)
	return version, nil
}

// createExpiryIndexBuckets creates all the buckets needed for the expiry index
func createExpiryIndexBuckets(dbTx database.Tx) error {
	meta := dbTx.Metadata()

	// Create meta bucket
	_, err := meta.CreateBucketIfNotExists(bktExpiryMeta)
	if err != nil {
		return fmt.Errorf("failed to create expiry meta bucket: %v", err)
	}

	// Create outpoint-to-expiry bucket
	_, err = meta.CreateBucketIfNotExists(bktOutpoint2Expiry)
	if err != nil {
		return fmt.Errorf("failed to create outpoint-to-expiry bucket: %v", err)
	}

	// Create expiry-to-outpoints bucket
	_, err = meta.CreateBucketIfNotExists(bktExpiry2Outpoints)
	if err != nil {
		return fmt.Errorf("failed to create expiry-to-outpoints bucket: %v", err)
	}

	// Create REAP strict candidate bucket.
	_, err = meta.CreateBucketIfNotExists(bktReapStrictCandidates)
	if err != nil {
		return fmt.Errorf("failed to create REAP strict candidate bucket: %v", err)
	}

	// Create outpoint-to-REAP-strict bucket.
	_, err = meta.CreateBucketIfNotExists(bktOutpoint2ReapStrict)
	if err != nil {
		return fmt.Errorf("failed to create outpoint-to-REAP-strict bucket: %v", err)
	}

	return nil
}

// dropExpiryIndexBuckets removes all expiry index buckets from the database
// This is used during index deletion or version upgrades
func dropExpiryIndexBuckets(dbTx database.Tx) error {
	meta := dbTx.Metadata()

	// Drop all buckets in reverse order of creation
	if err := meta.DeleteBucket(bktOutpoint2ReapStrict); err != nil {
		return fmt.Errorf("failed to drop outpoint-to-REAP-strict bucket: %v", err)
	}

	if err := meta.DeleteBucket(bktReapStrictCandidates); err != nil {
		return fmt.Errorf("failed to drop REAP strict candidate bucket: %v", err)
	}

	if err := meta.DeleteBucket(bktExpiry2Outpoints); err != nil {
		return fmt.Errorf("failed to drop expiry-to-outpoints bucket: %v", err)
	}

	if err := meta.DeleteBucket(bktOutpoint2Expiry); err != nil {
		return fmt.Errorf("failed to drop outpoint-to-expiry bucket: %v", err)
	}

	if err := meta.DeleteBucket(bktExpiryMeta); err != nil {
		return fmt.Errorf("failed to drop expiry meta bucket: %v", err)
	}

	return nil
}
