package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestComputeEntryDataDeterministic(t *testing.T) {
	op := &wire.OutPoint{
		Hash:  chainhash.Hash{0x01, 0x02, 0x03},
		Index: 42,
	}
	d1 := computeEntryData(op, 12345)
	d2 := computeEntryData(op, 12345)
	if len(d1) != 44 {
		t.Fatalf("expected 44 bytes, got %d", len(d1))
	}
	for i := range d1 {
		if d1[i] != d2[i] {
			t.Fatalf("not deterministic at byte %d", i)
		}
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

func TestIsUnspendable(t *testing.T) {
	tests := []struct {
		name   string
		script []byte
		want   bool
	}{
		{"OP_RETURN", []byte{0x6a, 0x04, 0x01, 0x02, 0x03, 0x04}, true},
		{"empty", []byte{}, true},
		{"normal P2PKH", []byte{0x76, 0xa9, 0x14}, false},
		{"OP_RETURN only", []byte{0x6a}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnspendable(tt.script); got != tt.want {
				t.Fatalf("isUnspendable(%x) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}
