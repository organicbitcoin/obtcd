// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func sequentialHash(start byte) chainhash.Hash {
	var hash chainhash.Hash
	for i := range hash {
		hash[i] = start + byte(i)
	}
	return hash
}

func setupAccumulatorTestDB(t *testing.T) database.DB {
	t.Helper()

	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	t.Cleanup(teardown)

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

	return db
}

func TestComputeEntryDataKnownEncoding(t *testing.T) {
	op := &wire.OutPoint{
		Hash:  sequentialHash(0x00),
		Index: 0x11223344,
	}
	data := computeEntryData(op, 0x0102030405060708)

	const want = "" +
		"000102030405060708090a0b0c0d0e0f" +
		"101112131415161718191a1b1c1d1e1f" +
		"44332211" +
		"0102030405060708"
	if got := hex.EncodeToString(data); got != want {
		t.Fatalf("entry encoding mismatch: got %s, want %s", got, want)
	}
}

func TestComputeEntryDataDifferent(t *testing.T) {
	op1 := &wire.OutPoint{Hash: chainhash.Hash{0x01}, Index: 0}
	op2 := &wire.OutPoint{Hash: chainhash.Hash{0x02}, Index: 0}

	d1 := computeEntryData(op1, 100)
	d2 := computeEntryData(op2, 100)
	same := true
	for i := range d1 {
		if d1[i] != d2[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different outpoints should produce different entry data")
	}

	// Same outpoint, different expiry key.
	d3 := computeEntryData(op1, 100)
	d4 := computeEntryData(op1, 200)
	same = true
	for i := range d3 {
		if d3[i] != d4[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different expiry keys should produce different entry data")
	}
}

func TestMuHashAccumulatorAddRemoveWithEntryData(t *testing.T) {
	mh := NewMuHash()
	identity := mh.Digest()

	op := &wire.OutPoint{Hash: chainhash.Hash{0xAA}, Index: 7}
	data := computeEntryData(op, 362880)

	mh.Add(data)
	if mh.Digest() == identity {
		t.Fatal("digest should change after Add")
	}

	mh.Remove(data)
	if mh.Digest() != identity {
		t.Fatal("Add+Remove should return to identity")
	}
}

func TestMuHashAccumulatorDigestKnownVectors(t *testing.T) {
	tests := []struct {
		name    string
		entries []struct {
			start     byte
			index     uint32
			expiryKey uint64
		}
		want string
	}{
		{
			name: "single entry",
			entries: []struct {
				start     byte
				index     uint32
				expiryKey uint64
			}{
				{start: 0x00, index: 0x11223344, expiryKey: 0x0102030405060708},
			},
			want: "6cbd5ff033eb8d1677f90d0cac5a06a878b420635b3e229eeb1889bc6bb23e2f",
		},
		{
			name: "two entries",
			entries: []struct {
				start     byte
				index     uint32
				expiryKey uint64
			}{
				{start: 0x00, index: 0x11223344, expiryKey: 0x0102030405060708},
				{start: 0x20, index: 0xaabbccdd, expiryKey: 0x0f0e0d0c0b0a0908},
			},
			want: "bf6a4be097d930dbca2c3fdd9aad7a5223ab9e76f082d6947eef5f7be62ec659",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mh := NewMuHash()
			for _, entry := range test.entries {
				op := &wire.OutPoint{
					Hash:  sequentialHash(entry.start),
					Index: entry.index,
				}
				mh.Add(computeEntryData(op, entry.expiryKey))
			}

			digest := mh.Digest()
			if got := hex.EncodeToString(digest[:]); got != test.want {
				t.Fatalf("digest mismatch: got %s, want %s", got, test.want)
			}
		})
	}
}

func TestMuHashMultiEntryOrderIndependence(t *testing.T) {
	entries := []struct {
		hash  byte
		index uint32
		ek    uint64
	}{
		{0x01, 0, 100},
		{0x02, 1, 200},
		{0x03, 2, 300},
	}

	// Forward order.
	mh1 := NewMuHash()
	for _, e := range entries {
		op := &wire.OutPoint{Hash: chainhash.Hash{e.hash}, Index: e.index}
		mh1.Add(computeEntryData(op, e.ek))
	}

	// Reverse order.
	mh2 := NewMuHash()
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		op := &wire.OutPoint{Hash: chainhash.Hash{e.hash}, Index: e.index}
		mh2.Add(computeEntryData(op, e.ek))
	}

	if mh1.Digest() != mh2.Digest() {
		t.Fatal("digest should be order-independent")
	}
}

func TestDBGetAccumulatorStateDefaultsToIdentity(t *testing.T) {
	db := setupAccumulatorTestDB(t)

	err := db.View(func(dbTx database.Tx) error {
		mh, err := dbGetAccumulatorState(dbTx)
		if err != nil {
			return err
		}

		digest := mh.Digest()
		const want = "c85525462fdcf30a2c18d6f4b92923000974355c2477f59594d2c205a1d25add"
		if got := hex.EncodeToString(digest[:]); got != want {
			t.Fatalf("identity digest mismatch: got %s, want %s", got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dbGetAccumulatorState failed: %v", err)
	}
}

func TestDBPutGetAccumulatorStateKnownValue(t *testing.T) {
	db := setupAccumulatorTestDB(t)

	mh := NewMuHash()
	first := &wire.OutPoint{Hash: sequentialHash(0x00), Index: 0x11223344}
	second := &wire.OutPoint{Hash: sequentialHash(0x20), Index: 0xaabbccdd}
	mh.Add(computeEntryData(first, 0x0102030405060708))
	mh.Add(computeEntryData(second, 0x0f0e0d0c0b0a0908))

	err := db.Update(func(dbTx database.Tx) error {
		return dbPutAccumulatorState(dbTx, mh)
	})
	if err != nil {
		t.Fatalf("dbPutAccumulatorState failed: %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		raw, err := dbGetIndexMeta(dbTx, keyAccumulatorState)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, mh.Serialize()) {
			t.Fatal("stored accumulator state does not match serialized MuHash")
		}

		stored, err := dbGetAccumulatorState(dbTx)
		if err != nil {
			return err
		}

		digest := stored.Digest()
		const want = "bf6a4be097d930dbca2c3fdd9aad7a5223ab9e76f082d6947eef5f7be62ec659"
		if got := hex.EncodeToString(digest[:]); got != want {
			t.Fatalf("stored accumulator digest mismatch: got %s, want %s", got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dbGetAccumulatorState failed: %v", err)
	}
}

func TestDBPutGetAccumulatorTipHashKnownValue(t *testing.T) {
	db := setupAccumulatorTestDB(t)

	wantHash := sequentialHash(0x80)
	err := db.Update(func(dbTx database.Tx) error {
		return dbPutAccumulatorTipHash(dbTx, &wantHash)
	})
	if err != nil {
		t.Fatalf("dbPutAccumulatorTipHash failed: %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		raw, err := dbGetIndexMeta(dbTx, keyAccumulatorTipHash)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, wantHash[:]) {
			t.Fatal("stored tip hash bytes do not match expected hash")
		}

		gotHash, err := dbGetAccumulatorTipHash(dbTx)
		if err != nil {
			return err
		}
		if gotHash != wantHash {
			t.Fatalf("tip hash mismatch: got %x, want %x", gotHash, wantHash)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dbGetAccumulatorTipHash failed: %v", err)
	}
}

func TestDBPutAccumulatorTipHashNilStoresZeroHash(t *testing.T) {
	db := setupAccumulatorTestDB(t)

	err := db.Update(func(dbTx database.Tx) error {
		return dbPutAccumulatorTipHash(dbTx, nil)
	})
	if err != nil {
		t.Fatalf("dbPutAccumulatorTipHash(nil) failed: %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		raw, err := dbGetIndexMeta(dbTx, keyAccumulatorTipHash)
		if err != nil {
			return err
		}

		wantRaw := make([]byte, chainhash.HashSize)
		if !bytes.Equal(raw, wantRaw) {
			t.Fatal("nil tip hash should be stored as 32 zero bytes")
		}

		gotHash, err := dbGetAccumulatorTipHash(dbTx)
		if err != nil {
			return err
		}
		if gotHash != (chainhash.Hash{}) {
			t.Fatalf("zero tip hash mismatch: got %x, want %x", gotHash, chainhash.Hash{})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dbGetAccumulatorTipHash after nil put failed: %v", err)
	}
}

func TestIsUnspendableParity(t *testing.T) {
	tests := []struct {
		name   string
		script []byte
		want   bool
	}{
		{"OP_RETURN", []byte{0x6a, 0x04, 0x01, 0x02, 0x03, 0x04}, true},
		{"empty", []byte{}, false},
		{"normal P2PKH", append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...), false},
		{"OP_RETURN only", []byte{0x6a}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txscript.IsUnspendable(tt.script); got != tt.want {
				t.Fatalf("IsUnspendable(%x) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}
