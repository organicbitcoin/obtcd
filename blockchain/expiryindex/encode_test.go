// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestEncodeDecodeOutPoint(t *testing.T) {
	tests := []struct {
		name     string
		outpoint wire.OutPoint
	}{
		{
			name: "zero hash, zero index",
			outpoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0,
			},
		},
		{
			name: "max hash, max index",
			outpoint: wire.OutPoint{
				Hash:  chainhash.Hash{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
				Index: 0xFFFFFFFF,
			},
		},
		{
			name: "typical outpoint",
			outpoint: wire.OutPoint{
				Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
				Index: 12345,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := encodeOutPoint(&test.outpoint)
			if len(encoded) != outPointEncodedSize {
				t.Fatalf("encoded length: got %d, want %d", len(encoded), outPointEncodedSize)
			}

			decoded, err := decodeOutPoint(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if !decoded.Hash.IsEqual(&test.outpoint.Hash) {
				t.Fatalf("hash mismatch: got %v, want %v", decoded.Hash, test.outpoint.Hash)
			}
			if decoded.Index != test.outpoint.Index {
				t.Fatalf("index mismatch: got %d, want %d", decoded.Index, test.outpoint.Index)
			}
		})
	}
}

func TestEncodeDecodeOrderedOutPoint(t *testing.T) {
	outpoint := wire.OutPoint{
		Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
		Index: 12345,
	}

	encoded := encodeOrderedOutPoint(&outpoint)
	if len(encoded) != orderedOutPointEncodedSize {
		t.Fatalf("encoded length: got %d, want %d", len(encoded), orderedOutPointEncodedSize)
	}

	decoded, err := decodeOrderedOutPoint(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !decoded.Hash.IsEqual(&outpoint.Hash) {
		t.Fatalf("hash mismatch: got %v, want %v", decoded.Hash, outpoint.Hash)
	}
	if decoded.Index != outpoint.Index {
		t.Fatalf("index mismatch: got %d, want %d", decoded.Index, outpoint.Index)
	}
}

func TestEncodeDecodeExpiryKey(t *testing.T) {
	tests := []struct {
		name      string
		expiryKey uint64
	}{
		{name: "zero key", expiryKey: 0},
		{name: "small key", expiryKey: 12345},
		{name: "large key", expiryKey: 1<<32 + 67890},
		{name: "max key", expiryKey: 0xFFFFFFFFFFFFFFFF},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := encodeExpiryKey(test.expiryKey)
			if len(encoded) != expiryKeyEncodedSize {
				t.Fatalf("encoded length: got %d, want %d", len(encoded), expiryKeyEncodedSize)
			}

			decoded, err := decodeExpiryKey(encoded)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if decoded != test.expiryKey {
				t.Fatalf("expiry key mismatch: got %d, want %d", decoded, test.expiryKey)
			}
		})
	}
}

func TestExpiryKeyOrdering(t *testing.T) {
	keys := []uint64{0, 1, 255, 256, 65535, 65536, 0xFFFFFFFF, 0x100000000, 0xFFFFFFFFFFFFFFFF}
	var encodedKeys [][]byte
	for _, key := range keys {
		encodedKeys = append(encodedKeys, encodeExpiryKey(key))
	}

	for i := 0; i < len(encodedKeys)-1; i++ {
		if bytes.Compare(encodedKeys[i], encodedKeys[i+1]) >= 0 {
			t.Fatalf("expiry key ordering violation at index %d: %d >= %d", i, keys[i], keys[i+1])
		}
	}
}

func TestEncodeDecodeExpiryOutpointCompositeKey(t *testing.T) {
	expiryKey := uint64(123456)
	outpoint := wire.OutPoint{
		Hash:  chainhash.DoubleHashH([]byte("composite")),
		Index: 7,
	}

	encoded := encodeExpiryOutpointCompositeKey(expiryKey, &outpoint)
	if len(encoded) != expiryOutpointCompositeKeySize {
		t.Fatalf("encoded length: got %d, want %d", len(encoded), expiryOutpointCompositeKeySize)
	}

	gotExpiryKey, gotOutPoint, err := decodeExpiryOutpointCompositeKey(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if gotExpiryKey != expiryKey {
		t.Fatalf("expiry key mismatch: got %d, want %d", gotExpiryKey, expiryKey)
	}
	if *gotOutPoint != outpoint {
		t.Fatalf("outpoint mismatch: got %v, want %v", *gotOutPoint, outpoint)
	}
}

func TestOrderedEncodingMatchesCompareOutPoint(t *testing.T) {
	outpoints := []wire.OutPoint{
		{Hash: chainhash.DoubleHashH([]byte("b")), Index: 0},
		{Hash: chainhash.DoubleHashH([]byte("a")), Index: 2},
		{Hash: chainhash.DoubleHashH([]byte("a")), Index: 3},
		{Hash: chainhash.DoubleHashH([]byte("c")), Index: 1},
	}

	for i := 0; i < len(outpoints); i++ {
		for j := 0; j < len(outpoints); j++ {
			gotCmp := bytes.Compare(
				encodeOrderedOutPoint(&outpoints[i]),
				encodeOrderedOutPoint(&outpoints[j]),
			)
			wantCmp := compareOutPoint(&outpoints[i], &outpoints[j])
			switch {
			case gotCmp < 0 && wantCmp >= 0:
				t.Fatalf("ordering mismatch: expected %v < %v", outpoints[i], outpoints[j])
			case gotCmp > 0 && wantCmp <= 0:
				t.Fatalf("ordering mismatch: expected %v > %v", outpoints[i], outpoints[j])
			case gotCmp == 0 && wantCmp != 0:
				t.Fatalf("ordering mismatch: expected %v == %v", outpoints[i], outpoints[j])
			}
		}
	}
}

func TestExpiryCompositePrefix(t *testing.T) {
	expiryKey := uint64(98765)
	outpoint := wire.OutPoint{
		Hash:  chainhash.DoubleHashH([]byte("prefix")),
		Index: 9,
	}

	prefix := expiryCompositePrefix(expiryKey)
	composite := encodeExpiryOutpointCompositeKey(expiryKey, &outpoint)
	if !bytes.HasPrefix(composite, prefix) {
		t.Fatalf("composite key %x should have prefix %x", composite, prefix)
	}
}

func TestExpiryCompositePrefixDoesNotMatchAdjacentKeys(t *testing.T) {
	expiryKey := uint64(321)
	nextExpiryKey := expiryKey + 1
	outpoint := wire.OutPoint{
		Hash:  chainhash.DoubleHashH([]byte("adjacent")),
		Index: 5,
	}

	prefix := expiryCompositePrefix(expiryKey)
	current := encodeExpiryOutpointCompositeKey(expiryKey, &outpoint)
	next := encodeExpiryOutpointCompositeKey(nextExpiryKey, &outpoint)

	if !bytes.HasPrefix(current, prefix) {
		t.Fatalf("expected current composite key to match prefix %x", prefix)
	}
	if bytes.HasPrefix(next, prefix) {
		t.Fatalf("unexpected adjacent composite key %x to match prefix %x", next, prefix)
	}
}

func TestDecodeInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		fn   func([]byte) error
	}{
		{
			name: "outpoint too short",
			data: make([]byte, outPointEncodedSize-1),
			fn: func(data []byte) error {
				_, err := decodeOutPoint(data)
				return err
			},
		},
		{
			name: "ordered outpoint too short",
			data: make([]byte, orderedOutPointEncodedSize-1),
			fn: func(data []byte) error {
				_, err := decodeOrderedOutPoint(data)
				return err
			},
		},
		{
			name: "expiry key too short",
			data: make([]byte, expiryKeyEncodedSize-1),
			fn: func(data []byte) error {
				_, err := decodeExpiryKey(data)
				return err
			},
		},
		{
			name: "composite key too short",
			data: make([]byte, expiryOutpointCompositeKeySize-1),
			fn: func(data []byte) error {
				_, _, err := decodeExpiryOutpointCompositeKey(data)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(test.data); err == nil {
				t.Fatalf("expected error for invalid data, got nil")
			}
		})
	}
}

func TestDecodeExpiryOutpointCompositeKeyRejectsInvalidLengths(t *testing.T) {
	for _, size := range []int{0, 1, 7, 8, 9, expiryOutpointCompositeKeySize - 1, expiryOutpointCompositeKeySize + 1, 100} {
		data := make([]byte, size)
		if _, _, err := decodeExpiryOutpointCompositeKey(data); err == nil {
			t.Fatalf("expected error for composite key size %d", size)
		}
	}
}

func TestOrderedEncodingMatchesCompareOutPointEdgeCases(t *testing.T) {
	baseHash := chainhash.DoubleHashH([]byte("same-hash"))
	otherHash := chainhash.DoubleHashH([]byte("other-hash"))
	maxHash := chainhash.Hash{}
	for i := range maxHash {
		maxHash[i] = 0xff
	}

	tests := []struct {
		name string
		a    wire.OutPoint
		b    wire.OutPoint
		want int
	}{
		{
			name: "same hash lower index sorts first",
			a:    wire.OutPoint{Hash: baseHash, Index: 0},
			b:    wire.OutPoint{Hash: baseHash, Index: 1},
			want: -1,
		},
		{
			name: "same hash higher index sorts last",
			a:    wire.OutPoint{Hash: baseHash, Index: 2},
			b:    wire.OutPoint{Hash: baseHash, Index: 1},
			want: 1,
		},
		{
			name: "different hashes respect compareOutPoint ordering",
			a:    wire.OutPoint{Hash: otherHash, Index: 0},
			b:    wire.OutPoint{Hash: baseHash, Index: 0},
			want: compareOutPoint(&wire.OutPoint{Hash: otherHash, Index: 0}, &wire.OutPoint{Hash: baseHash, Index: 0}),
		},
		{
			name: "zero hash sorts before max hash",
			a:    wire.OutPoint{Hash: chainhash.Hash{}, Index: 0},
			b:    wire.OutPoint{Hash: maxHash, Index: 0},
			want: -1,
		},
	}

	for _, test := range tests {
		got := bytes.Compare(
			encodeOrderedOutPoint(&test.a),
			encodeOrderedOutPoint(&test.b),
		)
		switch {
		case test.want < 0 && got >= 0:
			t.Fatalf("%s: expected a < b, got %d", test.name, got)
		case test.want > 0 && got <= 0:
			t.Fatalf("%s: expected a > b, got %d", test.name, got)
		case test.want == 0 && got != 0:
			t.Fatalf("%s: expected equality, got %d", test.name, got)
		}
	}
}

func TestEncodingDeterminism(t *testing.T) {
	outpoint := wire.OutPoint{
		Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
		Index: 12345,
	}
	expiryKey := uint64(123456)

	if !bytes.Equal(encodeOutPoint(&outpoint), encodeOutPoint(&outpoint)) {
		t.Fatalf("raw outpoint encoding is not deterministic")
	}
	if !bytes.Equal(encodeOrderedOutPoint(&outpoint), encodeOrderedOutPoint(&outpoint)) {
		t.Fatalf("ordered outpoint encoding is not deterministic")
	}
	if !bytes.Equal(
		encodeExpiryOutpointCompositeKey(expiryKey, &outpoint),
		encodeExpiryOutpointCompositeKey(expiryKey, &outpoint),
	) {
		t.Fatalf("composite key encoding is not deterministic")
	}
}
