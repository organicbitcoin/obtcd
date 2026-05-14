// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

type reindexInterceptBucket struct {
	inner database.Bucket

	failCreateBucketIfNotExists map[string]error
	failPut                     map[string]error
	failPutOnAttempt            map[string]int
	putCounts                   map[string]int
}

func (b reindexInterceptBucket) CreateBucket(key []byte) (database.Bucket, error) {
	return b.inner.CreateBucket(key)
}

func (b reindexInterceptBucket) Bucket(key []byte) database.Bucket {
	child := b.inner.Bucket(key)
	if child == nil {
		return nil
	}
	if bytes.Equal(key, bktExpiryMeta) {
		return reindexInterceptBucket{
			inner:            child,
			failPut:          b.failPut,
			failPutOnAttempt: b.failPutOnAttempt,
			putCounts:        b.putCounts,
		}
	}
	return child
}

func (b reindexInterceptBucket) DeleteBucket(key []byte) error {
	return b.inner.DeleteBucket(key)
}

func (b reindexInterceptBucket) ForEach(fn func(k, v []byte) error) error {
	return b.inner.ForEach(fn)
}

func (b reindexInterceptBucket) ForEachBucket(fn func(k []byte) error) error {
	return b.inner.ForEachBucket(fn)
}

func (b reindexInterceptBucket) Cursor() database.Cursor {
	return b.inner.Cursor()
}

func (b reindexInterceptBucket) Writable() bool {
	return b.inner.Writable()
}

func (b reindexInterceptBucket) CreateBucketIfNotExists(key []byte) (database.Bucket, error) {
	if err, ok := b.failCreateBucketIfNotExists[string(key)]; ok {
		return nil, err
	}

	child, err := b.inner.CreateBucketIfNotExists(key)
	if err != nil || child == nil {
		return child, err
	}
	if bytes.Equal(key, bktExpiryMeta) {
		return reindexInterceptBucket{
			inner:            child,
			failPut:          b.failPut,
			failPutOnAttempt: b.failPutOnAttempt,
			putCounts:        b.putCounts,
		}, nil
	}
	return child, nil
}

func (b reindexInterceptBucket) Put(key, value []byte) error {
	if err, ok := b.failPut[string(key)]; ok {
		keyStr := string(key)
		b.putCounts[keyStr]++
		attempt := b.putCounts[keyStr]
		failAt := 1
		if configured := b.failPutOnAttempt[keyStr]; configured > 0 {
			failAt = configured
		}
		if attempt == failAt {
			return err
		}
	}
	return b.inner.Put(key, value)
}

func (b reindexInterceptBucket) Get(key []byte) []byte {
	return b.inner.Get(key)
}

func (b reindexInterceptBucket) Delete(key []byte) error {
	return b.inner.Delete(key)
}

type reindexInterceptTx struct {
	database.Tx
	meta database.Bucket
}

func (tx reindexInterceptTx) Metadata() database.Bucket {
	return tx.meta
}

type reindexInterceptDB struct {
	database.DB
	wrapMetadata func(database.Bucket) database.Bucket
}

func (db reindexInterceptDB) Update(fn func(database.Tx) error) error {
	return db.DB.Update(func(tx database.Tx) error {
		if db.wrapMetadata == nil {
			return fn(tx)
		}
		return fn(reindexInterceptTx{
			Tx:   tx,
			meta: db.wrapMetadata(tx.Metadata()),
		})
	})
}

func newReindexInterceptBucket(meta database.Bucket, failCreate map[string]error, failPut map[string]error, failPutOnAttempt map[string]int) database.Bucket {
	return reindexInterceptBucket{
		inner:                       meta,
		failCreateBucketIfNotExists: failCreate,
		failPut:                     failPut,
		failPutOnAttempt:            failPutOnAttempt,
		putCounts:                   make(map[string]int),
	}
}

func TestReindexExpiryIndexResetsState(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("unable to create test db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to create expiry index: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, 123); err != nil {
			return err
		}
		if err := dbPutIndexVersion(dbTx, CurrentIndexVersion); err != nil {
			return err
		}

		op := wire.OutPoint{
			Hash:  chainhash.DoubleHashH([]byte("outpoint")),
			Index: 0,
		}
		if err := idx.connectTxOut(dbTx, &op, 10); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unable to seed expiry index state: %v", err)
	}

	if err := ReindexExpiryIndex(db); err != nil {
		t.Fatalf("reindex failed: %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		tipHeight, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if tipHeight != -1 {
			t.Fatalf("expected reset tip height -1, got %d", tipHeight)
		}

		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return err
		}
		if version != CurrentIndexVersion {
			t.Fatalf("expected version %d, got %d", CurrentIndexVersion, version)
		}

		rows, hasMore, err := idx.ScanExpiringUTXOs(0, 1_000_000, 10, nil)
		if err != nil {
			return err
		}
		if hasMore {
			t.Fatal("expected cleared index to have no extra rows")
		}
		if len(rows) != 0 {
			t.Fatalf("expected cleared index to have no rows, got %d", len(rows))
		}

		snapshot, err := idx.GetAccumulatorSnapshot()
		if err != nil {
			return err
		}
		if snapshot.TipHeight != -1 {
			t.Fatalf("expected accumulator tip height -1, got %d", snapshot.TipHeight)
		}
		if snapshot.Root != NewMuHash().Digest() {
			t.Fatalf("expected identity accumulator root, got %x", snapshot.Root)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unable to inspect expiry index state: %v", err)
	}
}

func TestReindexExpiryIndexRejectsNilDB(t *testing.T) {
	if err := ReindexExpiryIndex(nil); err == nil {
		t.Fatal("expected nil db reindex to fail")
	}
}

func TestReindexExpiryIndexFailsOnClosedDB(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("unable to create test db: %v", err)
	}
	teardown()

	if err := ReindexExpiryIndex(db); err == nil {
		t.Fatal("expected closed db reindex to fail")
	}
}

func TestClearExpiryIndexBucketsPropagatesResetErrors(t *testing.T) {
	tests := []struct {
		name    string
		failKey []byte
		want    string
	}{
		{
			name:    "accumulator state reset",
			failKey: keyAccumulatorState,
			want:    "failed to reset accumulator: forced meta put failure",
		},
		{
			name:    "accumulator tip hash reset",
			failKey: keyAccumulatorTipHash,
			want:    "failed to reset accumulator tip hash: forced meta put failure",
		},
		{
			name:    "indexed tip height reset",
			failKey: keyTipHeightIndexed,
			want:    "failed to reset indexed tip height: forced meta put failure",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createCoreTestDB()
			if err != nil {
				t.Fatalf("unable to create test db: %v", err)
			}
			defer teardown()

			idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("unable to create expiry index: %v", err)
			}
			if err := db.Update(func(dbTx database.Tx) error {
				return idx.Create(dbTx)
			}); err != nil {
				t.Fatalf("unable to initialize expiry index: %v", err)
			}

			err = db.Update(func(dbTx database.Tx) error {
				return clearExpiryIndexBuckets(reindexInterceptTx{
					Tx: dbTx,
					meta: newReindexInterceptBucket(
						dbTx.Metadata(),
						nil,
						map[string]error{string(test.failKey): errors.New("forced meta put failure")},
						nil,
					),
				})
			})
			if err == nil {
				t.Fatal("expected clearExpiryIndexBuckets to fail")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestReindexExpiryIndexPropagatesWrappedUpdateErrors(t *testing.T) {
	tests := []struct {
		name       string
		failCreate map[string]error
		failPut    map[string]error
		failAt     map[string]int
		want       string
	}{
		{
			name: "ensure buckets failure",
			failCreate: map[string]error{
				string(bktOutpoint2Expiry): errors.New("forced bucket creation failure"),
			},
			want: "failed to ensure expiry buckets: failed to create outpoint-to-expiry bucket: forced bucket creation failure",
		},
		{
			name: "clear buckets failure",
			failPut: map[string]error{
				string(keyAccumulatorState): errors.New("forced meta put failure"),
			},
			want: "failed to clear expiry index buckets: failed to reset accumulator: forced meta put failure",
		},
		{
			name: "tip height reset failure",
			failPut: map[string]error{
				string(keyTipHeightIndexed): errors.New("forced meta put failure"),
			},
			failAt: map[string]int{
				string(keyTipHeightIndexed): 2,
			},
			want: "failed to reset expiry index tip height: forced meta put failure",
		},
		{
			name: "version reset failure",
			failPut: map[string]error{
				string(keyIndexVersion): errors.New("forced meta put failure"),
			},
			want: "failed to reset expiry index version: forced meta put failure",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createCoreTestDB()
			if err != nil {
				t.Fatalf("unable to create test db: %v", err)
			}
			defer teardown()

			wrappedDB := reindexInterceptDB{
				DB: db,
				wrapMetadata: func(meta database.Bucket) database.Bucket {
					return newReindexInterceptBucket(meta, test.failCreate, test.failPut, test.failAt)
				},
			}

			err = ReindexExpiryIndex(wrappedDB)
			if err == nil {
				t.Fatal("expected ReindexExpiryIndex to fail")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}
