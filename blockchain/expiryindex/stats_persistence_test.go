// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func newStatsTestIndex(t *testing.T) (database.DB, *ExpiryIndex, func()) {
	t.Helper()
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		teardown()
		t.Fatalf("create expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		teardown()
		t.Fatalf("initialize expiry index: %v", err)
	}
	return db, idx, teardown
}

func statsTestOutPoint(label string, index uint32) wire.OutPoint {
	return wire.OutPoint{
		Hash:  chainhash.DoubleHashH([]byte(label)),
		Index: index,
	}
}

func removePersistedStats(dbTx database.Tx) error {
	meta := dbTx.Metadata().Bucket(bktExpiryMeta)
	if err := meta.Delete(keyStatsTotalUTXOs); err != nil {
		return err
	}
	return meta.Delete(keyStatsTotalExpiryKeys)
}

func TestInitMigratesLegacyStatsAndPersistsAcrossRestart(t *testing.T) {
	db, _, teardown := newStatsTestIndex(t)
	defer teardown()

	opA := statsTestOutPoint("legacy-a", 0)
	opB := statsTestOutPoint("legacy-b", 1)
	opC := statsTestOutPoint("legacy-c", 2)
	if err := db.Update(func(dbTx database.Tx) error {
		if err := putTxOutMapping(dbTx, &opA, 100, 10); err != nil {
			return err
		}
		if err := putTxOutMapping(dbTx, &opB, 100, 20); err != nil {
			return err
		}
		if err := putTxOutMapping(dbTx, &opC, 101, 30); err != nil {
			return err
		}
		return removePersistedStats(dbTx)
	}); err != nil {
		t.Fatalf("create legacy index state: %v", err)
	}

	restarted, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("create restarted index: %v", err)
	}
	if err := restarted.Init(); err != nil {
		t.Fatalf("migrate legacy stats: %v", err)
	}
	stats, err := restarted.GetStats()
	if err != nil {
		t.Fatalf("read migrated stats: %v", err)
	}
	if stats.TotalUTXOs != 3 || stats.TotalExpiryKeys != 2 {
		t.Fatalf("unexpected migrated stats: %+v", stats)
	}
	if _, err := restarted.AuditStats(); err != nil {
		t.Fatalf("audit migrated stats: %v", err)
	}

	secondRestart, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("create second restarted index: %v", err)
	}
	stats, err = secondRestart.GetStats()
	if err != nil {
		t.Fatalf("read stats after second restart: %v", err)
	}
	if stats.TotalUTXOs != 3 || stats.TotalExpiryKeys != 2 {
		t.Fatalf("persisted stats changed after restart: %+v", stats)
	}
}

func TestInitRejectsInconsistentLegacyIndexWithoutPartialMigration(t *testing.T) {
	db, _, teardown := newStatsTestIndex(t)
	defer teardown()

	op := statsTestOutPoint("legacy-corrupt", 0)
	if err := db.Update(func(dbTx database.Tx) error {
		if err := putTxOutMapping(dbTx, &op, 200, 50); err != nil {
			return err
		}
		if err := dbTx.Metadata().Bucket(bktExpiry2Outpoints).Delete(
			encodeExpiryOutpointCompositeKey(200, &op)); err != nil {

			return err
		}
		return removePersistedStats(dbTx)
	}); err != nil {
		t.Fatalf("create inconsistent legacy state: %v", err)
	}

	restarted, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("create restarted index: %v", err)
	}
	err = restarted.Init()
	if err == nil || !strings.Contains(err.Error(), "inconsistent index entry counts") {
		t.Fatalf("expected legacy migration consistency error, got %v", err)
	}
	if err := db.View(func(dbTx database.Tx) error {
		_, initialized, err := dbGetIndexStats(dbTx)
		if err != nil {
			return err
		}
		if initialized {
			t.Fatal("failed migration must not persist partial counters")
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect failed migration: %v", err)
	}
}

func TestGetStatsRejectsMalformedOrPartialMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(database.Bucket) error
		want   string
	}{
		{
			name: "malformed total",
			mutate: func(meta database.Bucket) error {
				return meta.Put(keyStatsTotalUTXOs, []byte{1})
			},
			want: "invalid total UTXO count encoding",
		},
		{
			name: "partial pair",
			mutate: func(meta database.Bucket) error {
				return meta.Delete(keyStatsTotalExpiryKeys)
			},
			want: "incomplete persisted expiry index stats",
		},
		{
			name: "impossible counts",
			mutate: func(meta database.Bucket) error {
				if err := meta.Put(keyStatsTotalUTXOs, encodeStatsCounter(1)); err != nil {
					return err
				}
				return meta.Put(keyStatsTotalExpiryKeys, encodeStatsCounter(2))
			},
			want: "expiry keys 2 exceed UTXOs 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, idx, teardown := newStatsTestIndex(t)
			defer teardown()
			if err := db.Update(func(dbTx database.Tx) error {
				return test.mutate(dbTx.Metadata().Bucket(bktExpiryMeta))
			}); err != nil {
				t.Fatalf("mutate stats metadata: %v", err)
			}
			if _, err := idx.GetStats(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestMappingMutationRollsBackWhenStatsAreIncomplete(t *testing.T) {
	db, _, teardown := newStatsTestIndex(t)
	defer teardown()
	op := statsTestOutPoint("rollback-incomplete-stats", 0)

	if err := db.Update(func(dbTx database.Tx) error {
		return dbTx.Metadata().Bucket(bktExpiryMeta).Delete(keyStatsTotalExpiryKeys)
	}); err != nil {
		t.Fatalf("remove counter: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return putTxOutMapping(dbTx, &op, 300, 100)
	}); err == nil {
		t.Fatal("expected mapping update to reject incomplete counters")
	}
	if err := db.View(func(dbTx database.Tx) error {
		if got := dbTx.Metadata().Bucket(bktOutpoint2Expiry).Get(encodeOutPoint(&op)); got != nil {
			t.Fatalf("failed transaction left outpoint mapping %x", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect rolled back transaction: %v", err)
	}
}

func TestAuditStatsDetectsPersistedCounterDrift(t *testing.T) {
	db, idx, teardown := newStatsTestIndex(t)
	defer teardown()
	op := statsTestOutPoint("counter-drift", 0)
	if err := db.Update(func(dbTx database.Tx) error {
		return putTxOutMapping(dbTx, &op, 400, 100)
	}); err != nil {
		t.Fatalf("seed mapping: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return dbPutIndexStats(dbTx, persistedIndexStats{
			totalUTXOs:      9,
			totalExpiryKeys: 1,
		})
	}); err != nil {
		t.Fatalf("seed counter drift: %v", err)
	}

	fast, err := idx.GetStats()
	if err != nil {
		t.Fatalf("read persisted stats: %v", err)
	}
	if fast.TotalUTXOs != 9 {
		t.Fatalf("fast path unexpectedly scanned index: %+v", fast)
	}
	if _, err := idx.AuditStats(); err == nil || !strings.Contains(err.Error(), "stats mismatch") {
		t.Fatalf("expected audit mismatch, got %v", err)
	}
}

func TestPutTxOutMappingIdempotentAndReplacementStats(t *testing.T) {
	db, idx, teardown := newStatsTestIndex(t)
	defer teardown()
	op := statsTestOutPoint("replace-mapping", 0)

	if err := db.Update(func(dbTx database.Tx) error {
		if err := putTxOutMapping(dbTx, &op, 500, 10); err != nil {
			return err
		}
		return putTxOutMapping(dbTx, &op, 500, 10)
	}); err != nil {
		t.Fatalf("idempotent mapping write: %v", err)
	}
	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("read idempotent stats: %v", err)
	}
	if stats.TotalUTXOs != 1 || stats.TotalExpiryKeys != 1 {
		t.Fatalf("idempotent write changed counts: %+v", stats)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return putTxOutMapping(dbTx, &op, 501, 20)
	}); err != nil {
		t.Fatalf("replace mapping: %v", err)
	}
	stats, err = idx.GetStats()
	if err != nil {
		t.Fatalf("read replacement stats: %v", err)
	}
	if stats.TotalUTXOs != 1 || stats.TotalExpiryKeys != 1 {
		t.Fatalf("replacement changed total counts: %+v", stats)
	}
	if _, err := idx.AuditStats(); err != nil {
		t.Fatalf("audit replacement: %v", err)
	}
	rows, _, err := idx.ScanExpiringUTXOs(500, 501, 10, nil)
	if err != nil {
		t.Fatalf("scan replacement: %v", err)
	}
	if len(rows) != 1 || rows[0].ExpiryKey != 501 {
		t.Fatalf("replacement left stale expiry mapping: %+v", rows)
	}
}

func TestCreatePreservesExistingPersistedStats(t *testing.T) {
	db, idx, teardown := newStatsTestIndex(t)
	defer teardown()
	op := statsTestOutPoint("create-idempotent", 0)
	if err := db.Update(func(dbTx database.Tx) error {
		if err := putTxOutMapping(dbTx, &op, 600, 10); err != nil {
			return err
		}
		return idx.Create(dbTx)
	}); err != nil {
		t.Fatalf("repeat create: %v", err)
	}
	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("read stats after repeated create: %v", err)
	}
	if stats.TotalUTXOs != 1 || stats.TotalExpiryKeys != 1 {
		t.Fatalf("repeat create reset stats: %+v", stats)
	}
}
