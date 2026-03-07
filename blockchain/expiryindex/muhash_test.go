package expiryindex

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"testing"
)

func TestMuHashIdentityDigest(t *testing.T) {
	mh := NewMuHash()
	d := mh.Digest()
	// Identity = 1, serialized as LE 384 bytes (0x01 followed by zeros).
	var one [muhashByteLen]byte
	one[0] = 1
	expected := sha256.Sum256(one[:])
	if d != expected {
		t.Fatalf("identity digest mismatch: got %x, want %x", d, expected)
	}
}

func TestMuHashAddRemoveInverse(t *testing.T) {
	mh := NewMuHash()
	identity := mh.Digest()

	elem := []byte("test element")
	mh.Add(elem)

	afterAdd := mh.Digest()
	if afterAdd == identity {
		t.Fatal("digest should change after Add")
	}

	mh.Remove(elem)
	afterRemove := mh.Digest()
	if afterRemove != identity {
		t.Fatalf("Add then Remove should return to identity: got %x, want %x",
			afterRemove, identity)
	}
}

func TestMuHashOrderIndependent(t *testing.T) {
	a := []byte("alpha")
	b := []byte("bravo")
	c := []byte("charlie")

	mh1 := NewMuHash()
	mh1.Add(a)
	mh1.Add(b)
	mh1.Add(c)

	mh2 := NewMuHash()
	mh2.Add(c)
	mh2.Add(a)
	mh2.Add(b)

	if mh1.Digest() != mh2.Digest() {
		t.Fatal("digest should be order-independent")
	}
}

func TestMuHashMultipleAddRemove(t *testing.T) {
	mh := NewMuHash()
	identity := mh.Digest()

	elems := [][]byte{
		{0x01, 0x02},
		{0x03, 0x04},
		{0x05, 0x06},
	}
	for _, e := range elems {
		mh.Add(e)
	}
	// Remove in reverse order.
	for i := len(elems) - 1; i >= 0; i-- {
		mh.Remove(elems[i])
	}
	if mh.Digest() != identity {
		t.Fatal("adding and removing all elements should return to identity")
	}
}

func TestMuHashSerializeDeserialize(t *testing.T) {
	mh := NewMuHash()
	mh.Add([]byte("foo"))
	mh.Add([]byte("bar"))
	mh.Remove([]byte("baz"))

	data := mh.Serialize()
	if len(data) != muhashSerializedSize {
		t.Fatalf("serialized size: got %d, want %d", len(data), muhashSerializedSize)
	}

	mh2 := NewMuHash()
	if err := mh2.Deserialize(data); err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	if mh.Digest() != mh2.Digest() {
		t.Fatal("digest mismatch after serialize/deserialize round-trip")
	}
}

func TestMuHashDeserializeInvalidLength(t *testing.T) {
	mh := NewMuHash()
	if err := mh.Deserialize([]byte{0x01, 0x02}); err == nil {
		t.Fatal("expected error for invalid length")
	}
}

func TestMuHashClone(t *testing.T) {
	mh := NewMuHash()
	mh.Add([]byte("data"))

	clone := mh.Clone()
	if mh.Digest() != clone.Digest() {
		t.Fatal("clone digest should match original")
	}

	// Mutate original, clone should be unaffected.
	mh.Add([]byte("more"))
	if mh.Digest() == clone.Digest() {
		t.Fatal("clone should be independent of original")
	}
}

func TestMuHashDeterministic(t *testing.T) {
	mh1 := NewMuHash()
	mh1.Add([]byte("deterministic"))

	mh2 := NewMuHash()
	mh2.Add([]byte("deterministic"))

	if mh1.Digest() != mh2.Digest() {
		t.Fatal("same input should produce same digest")
	}
}

func TestMuHashDifferentElements(t *testing.T) {
	mh1 := NewMuHash()
	mh1.Add([]byte("alpha"))

	mh2 := NewMuHash()
	mh2.Add([]byte("beta"))

	if mh1.Digest() == mh2.Digest() {
		t.Fatal("different elements should produce different digests")
	}
}

func TestChacha20ExpandLength(t *testing.T) {
	key := sha256.Sum256([]byte("test key"))

	for _, size := range []int{32, 64, 128, 384, 512} {
		out := chacha20Expand(key[:], size)
		if len(out) != size {
			t.Fatalf("chacha20Expand(%d): got len %d", size, len(out))
		}
	}
}

func TestChacha20ExpandDeterministic(t *testing.T) {
	key := sha256.Sum256([]byte("deterministic"))
	out1 := chacha20Expand(key[:], 384)
	out2 := chacha20Expand(key[:], 384)
	if !bytes.Equal(out1, out2) {
		t.Fatal("chacha20Expand should be deterministic")
	}
}

func TestBigIntLERoundTrip(t *testing.T) {
	cases := []int64{0, 1, 255, 256, 65535, 1<<32 - 1}
	for _, v := range cases {
		n := muhashPrime // use a large number too
		le := bigIntToLE(n, muhashByteLen)
		got := leToBigInt(le)
		if got.Cmp(n) != 0 {
			t.Fatalf("LE round-trip failed for prime")
		}

		small := new(big.Int).SetInt64(v)
		le2 := bigIntToLE(small, 8)
		got2 := leToBigInt(le2)
		if got2.Int64() != v {
			t.Fatalf("LE round-trip failed for %d: got %d", v, got2.Int64())
		}
	}
}
