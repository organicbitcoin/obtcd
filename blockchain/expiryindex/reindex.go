// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
)

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

	// Reset accumulator to identity state.
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

// ReindexExpiryIndex explicitly resets the persisted expiry index state so the
// next startup rebuilds it from chain state.
func ReindexExpiryIndex(db database.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	log.Warnf("ExpiryIndex: explicit reindex requested, clearing persisted state")
	return db.Update(func(dbTx database.Tx) error {
		if err := createExpiryIndexBuckets(dbTx); err != nil {
			return fmt.Errorf("failed to ensure expiry buckets: %v", err)
		}
		if err := clearExpiryIndexBuckets(dbTx); err != nil {
			return fmt.Errorf("failed to clear expiry index buckets: %v", err)
		}
		if err := dbPutTipHeightIndexed(dbTx, -1); err != nil {
			return fmt.Errorf("failed to reset expiry index tip height: %v", err)
		}
		if err := dbPutIndexVersion(dbTx, CurrentIndexVersion); err != nil {
			return fmt.Errorf("failed to reset expiry index version: %v", err)
		}

		return nil
	})
}
