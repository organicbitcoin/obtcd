// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"encoding/binary"
	"sort"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

type stagedExpiring struct {
	op        wire.OutPoint
	expiryKey uint64
}

func TestScanExpiringUTXOsStaircasePressure(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	expiry := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if expiry == nil {
		t.Fatalf("missing expiry params")
	}

	const total = 6000
	seeded := make([]stagedExpiring, 0, total)
	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}

		for i := 0; i < total; i++ {
			op := makeStaircaseOutPoint(uint32(i))
			createHeight := int32(100 + (i % 500))
			if err := idx.connectTxOut(dbTx, &op, createHeight); err != nil {
				return err
			}
			seeded = append(seeded, stagedExpiring{
				op:        op,
				expiryKey: expiry.CalculateExpiryKey(createHeight),
			})
		}

		return nil
	})
	if err != nil {
		t.Fatalf("seed index: %v", err)
	}

	tips := []uint64{260, 320, 420, 560, 760}
	pageSizes := []int{17, 73, 257, 1024}

	for _, tip := range tips {
		tip := tip
		t.Run("tip="+itoaU64(tip), func(t *testing.T) {
			expected := expectedScanOrder(seeded, tip)
			for _, pageSize := range pageSizes {
				actual, err := scanAllExpiringPages(idx, tip, pageSize)
				if err != nil {
					t.Fatalf("scan with page=%d failed: %v", pageSize, err)
				}
				if len(actual) != len(expected) {
					t.Fatalf("page=%d count mismatch: got %d, want %d", pageSize, len(actual), len(expected))
				}
				for i := range expected {
					if actual[i].ExpiryKey != expected[i].expiryKey || actual[i].OutPoint != expected[i].op {
						t.Fatalf("page=%d mismatch at %d: got (%d,%s) want (%d,%s)",
							pageSize,
							i,
							actual[i].ExpiryKey,
							actual[i].OutPoint.String(),
							expected[i].expiryKey,
							expected[i].op.String(),
						)
					}
				}
			}
		})
	}
}

func scanAllExpiringPages(idx *ExpiryIndex, toKey uint64, pageSize int) ([]*ExpiringUTXO, error) {
	if pageSize <= 0 {
		pageSize = 1
	}

	var (
		all        []*ExpiringUTXO
		fromKey    uint64
		startAfter *wire.OutPoint
	)

	for {
		rows, hasMore, err := idx.ScanExpiringUTXOs(fromKey, toKey, pageSize, startAfter)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return all, nil
		}

		for _, row := range rows {
			all = append(all, row)
		}

		if !hasMore {
			return all, nil
		}

		last := rows[len(rows)-1]
		fromKey = last.ExpiryKey
		op := last.OutPoint
		startAfter = &op
	}
}

func expectedScanOrder(seed []stagedExpiring, tip uint64) []stagedExpiring {
	out := make([]stagedExpiring, 0, len(seed))
	for _, s := range seed {
		if s.expiryKey <= tip {
			out = append(out, s)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]
		if a.expiryKey != b.expiryKey {
			return a.expiryKey < b.expiryKey
		}
		return compareOutPoint(&a.op, &b.op) < 0
	})

	return out
}

func makeStaircaseOutPoint(n uint32) wire.OutPoint {
	var h chainhash.Hash
	binary.LittleEndian.PutUint32(h[0:4], n+1)
	binary.LittleEndian.PutUint32(h[4:8], (n+1)*2654435761)
	binary.LittleEndian.PutUint32(h[8:12], (n+1)*2246822519)
	return wire.OutPoint{Hash: h, Index: n % 17}
}

func itoaU64(v uint64) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
