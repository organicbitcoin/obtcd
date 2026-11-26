// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
)

const defaultClearBatchSize = 50_000

var expiryDataBuckets = [][]byte{
	bktOutpoint2Expiry,
	bktExpiry2Outpoints,
	bktReapStrictCandidates,
	bktOutpoint2ReapStrict,
}

func resetExpiryIndexMetadata(dbTx database.Tx) error {
	if err := dbPutAccumulatorState(dbTx, NewMuHash()); err != nil {
		return fmt.Errorf("failed to reset accumulator: %v", err)
	}
	var zero chainhash.Hash
	if err := dbPutAccumulatorTipHash(dbTx, &zero); err != nil {
		return fmt.Errorf("failed to reset accumulator tip hash: %v", err)
	}
	if err := dbPutTipHeightIndexed(dbTx, -1); err != nil {
		return fmt.Errorf("failed to reset indexed tip height: %v", err)
	}
	return nil
}

// clearExpiryIndexBuckets clears all expiry index data while preserving the
// bucket layout so startup can rebuild the index in-place.
func clearExpiryIndexBuckets(dbTx database.Tx) error {
	// Clear outpoint-to-expiry bucket.
	outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
	if outpointBucket != nil {
		cursor := outpointBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}

	// Clear expiry-to-outpoints bucket.
	expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
	if expiryBucket != nil {
		cursor := expiryBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}

	// Clear REAP strict candidate bucket.
	reapBucket := dbTx.Metadata().Bucket(bktReapStrictCandidates)
	if reapBucket != nil {
		cursor := reapBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}

	// Clear outpoint-to-REAP-strict bucket.
	reapOutpointBucket := dbTx.Metadata().Bucket(bktOutpoint2ReapStrict)
	if reapOutpointBucket != nil {
		cursor := reapOutpointBucket.Cursor()
		found := cursor.First()
		for found {
			if err := cursor.Delete(); err != nil {
				return err
			}
			found = cursor.Next()
		}
	}

	return resetExpiryIndexMetadata(dbTx)
}

// clearExpiryIndexBucketsBatched bounds transaction memory while clearing a
// large production index.  The tip is invalidated before deleting any entries
// so an interruption always causes the next startup to resume a clean rebuild.
func clearExpiryIndexBucketsBatched(db database.DB, interrupt <-chan struct{}) error {
	return clearExpiryIndexBucketsBatchedWithSize(db, interrupt, defaultClearBatchSize)
}

func clearExpiryIndexBucketsBatchedWithSize(db database.DB,
	interrupt <-chan struct{}, batchSize int) error {

	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if batchSize <= 0 {
		return fmt.Errorf("clear batch size must be positive")
	}

	if err := db.Update(resetExpiryIndexMetadata); err != nil {
		return err
	}
	if isInterruptRequested(interrupt) {
		return errRebuildInterrupted
	}

	startedAt := time.Now()
	var totalDeleted int64
	for {
		deleted := 0
		err := db.Update(func(dbTx database.Tx) error {
			meta := dbTx.Metadata()
			for _, bucketName := range expiryDataBuckets {
				bucket := meta.Bucket(bucketName)
				if bucket == nil {
					continue
				}
				cursor := bucket.Cursor()
				for found := cursor.First(); found && deleted < batchSize; found = cursor.Next() {
					if err := cursor.Delete(); err != nil {
						return err
					}
					deleted++
				}
				if deleted >= batchSize {
					break
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		totalDeleted += int64(deleted)
		if deleted == 0 {
			break
		}
		if totalDeleted%1_000_000 == 0 {
			elapsed := time.Since(startedAt)
			log.Infof("ExpiryIndex: Cleared %d stale entries (%.0f/s)",
				totalDeleted, float64(totalDeleted)/elapsed.Seconds())
		}
		if isInterruptRequested(interrupt) {
			return errRebuildInterrupted
		}
	}

	if totalDeleted > 0 {
		elapsed := time.Since(startedAt)
		log.Infof("ExpiryIndex: Cleared %d stale entries in bounded batches in %.2fs (%.0f/s)",
			totalDeleted, elapsed.Seconds(), float64(totalDeleted)/elapsed.Seconds())
	}
	return nil
}

func isInterruptRequested(interrupt <-chan struct{}) bool {
	if interrupt == nil {
		return false
	}
	select {
	case <-interrupt:
		return true
	default:
		return false
	}
}

// ReindexExpiryIndex explicitly resets the persisted expiry index state so the
// next startup rebuilds it from chain state.
func ReindexExpiryIndex(db database.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	log.Warnf("ExpiryIndex: explicit reindex requested, clearing persisted state")
	if err := db.Update(func(dbTx database.Tx) error {
		if err := createExpiryIndexBuckets(dbTx); err != nil {
			return fmt.Errorf("failed to ensure expiry buckets: %v", err)
		}
		if err := dbPutIndexVersion(dbTx, CurrentIndexVersion); err != nil {
			return fmt.Errorf("failed to reset expiry index version: %v", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := clearExpiryIndexBucketsBatched(db, nil); err != nil {
		return fmt.Errorf("failed to clear expiry index buckets: %v", err)
	}
	return nil
}
