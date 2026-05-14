// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
)

// MuHash3072 implements a cryptographic multiset hash using modular
// arithmetic over Z/pZ where p = 2^3072 - 1103717.
//
// This is the same algorithm used by Bitcoin Core for UTXO set hashing.
// It supports incremental Add/Remove operations and produces a 32-byte
// digest via SHA256 of the normalized internal state.
//
// The implementation uses a numerator/denominator representation to defer
// expensive modular inverse computation until Digest() is called.

const (
	// muhashPrimeBits is the bit size of the MuHash prime.
	muhashPrimeBits = 3072

	// muhashByteLen is the byte length of a single MuHash element (3072/8).
	muhashByteLen = muhashPrimeBits / 8 // 384

	// muhashSerializedSize is the byte length of a serialized MuHash
	// (numerator + denominator).
	muhashSerializedSize = muhashByteLen * 2 // 768

	// muhashPrimeOffset is subtracted from 2^3072 to get the prime.
	muhashPrimeOffset = 1103717
)

// muhashPrime is the prime modulus: 2^3072 - 1103717.
var muhashPrime *big.Int

func init() {
	muhashPrime = new(big.Int).SetBit(new(big.Int), muhashPrimeBits, 1)
	muhashPrime.Sub(muhashPrime, big.NewInt(muhashPrimeOffset))
}

// MuHash is a cryptographic multiset hash accumulator.
type MuHash struct {
	numerator   *big.Int
	denominator *big.Int
}

// NewMuHash returns a MuHash initialized to the identity state.
func NewMuHash() *MuHash {
	return &MuHash{
		numerator:   big.NewInt(1),
		denominator: big.NewInt(1),
	}
}

// Add incorporates an element into the multiset.
func (m *MuHash) Add(data []byte) {
	n := hashToNum(data)
	m.numerator.Mul(m.numerator, n)
	m.numerator.Mod(m.numerator, muhashPrime)
}

// Remove removes an element from the multiset.
func (m *MuHash) Remove(data []byte) {
	n := hashToNum(data)
	m.denominator.Mul(m.denominator, n)
	m.denominator.Mod(m.denominator, muhashPrime)
}

// Digest computes the 32-byte hash of the current multiset state.
// It normalizes the state by computing numerator * denominator^(-1) mod p,
// then returns SHA256 of the result serialized as a 384-byte little-endian
// number.
func (m *MuHash) Digest() [32]byte {
	// Compute denominator^(-1) mod p.
	denInv := new(big.Int).ModInverse(m.denominator, muhashPrime)
	if denInv == nil {
		// Should never happen since denominator is always coprime with p.
		panic("muhash: denominator has no modular inverse")
	}

	// result = numerator * denominator^(-1) mod p
	result := new(big.Int).Mul(m.numerator, denInv)
	result.Mod(result, muhashPrime)

	// Serialize as little-endian 384 bytes.
	buf := bigIntToLE(result, muhashByteLen)
	return sha256.Sum256(buf)
}

// Serialize encodes the MuHash state to 768 bytes (numerator || denominator),
// each as a 384-byte little-endian number.
func (m *MuHash) Serialize() []byte {
	buf := make([]byte, muhashSerializedSize)
	copy(buf[:muhashByteLen], bigIntToLE(m.numerator, muhashByteLen))
	copy(buf[muhashByteLen:], bigIntToLE(m.denominator, muhashByteLen))
	return buf
}

// Deserialize restores the MuHash state from a 768-byte serialized form.
func (m *MuHash) Deserialize(data []byte) error {
	if len(data) != muhashSerializedSize {
		return fmt.Errorf("muhash: invalid data length %d, want %d",
			len(data), muhashSerializedSize)
	}
	m.numerator = leToBigInt(data[:muhashByteLen])
	m.denominator = leToBigInt(data[muhashByteLen:])
	return nil
}

// Clone returns a deep copy of the MuHash state.
func (m *MuHash) Clone() *MuHash {
	return &MuHash{
		numerator:   new(big.Int).Set(m.numerator),
		denominator: new(big.Int).Set(m.denominator),
	}
}

// hashToNum maps arbitrary data to a number in Z/pZ.
//
// Process:
//  1. SHA256(data) → 32-byte seed
//  2. Use seed as ChaCha20 key (nonce=0) to generate 384 bytes
//  3. Interpret as 3072-bit little-endian number
//  4. Reduce mod p
func hashToNum(data []byte) *big.Int {
	seed := sha256.Sum256(data)
	expanded := chacha20Expand(seed[:], muhashByteLen)
	n := leToBigInt(expanded)
	n.Mod(n, muhashPrime)

	// Ensure non-zero (zero has no inverse).
	if n.Sign() == 0 {
		n.SetInt64(1)
	}
	return n
}

// chacha20Expand uses the ChaCha20 quarter-round to expand a 32-byte key
// into outLen bytes of pseudorandom output (nonce = 0, counter starting at 0).
//
// This is a standalone implementation of ChaCha20 block function to avoid
// external dependencies. It matches the IETF RFC 8439 specification.
func chacha20Expand(key []byte, outLen int) []byte {
	if len(key) != 32 {
		panic("chacha20: key must be 32 bytes")
	}

	out := make([]byte, outLen)
	produced := 0

	// ChaCha20 "expand 32-byte k" constants.
	const0 := uint32(0x61707865)
	const1 := uint32(0x3320646e)
	const2 := uint32(0x79622d32)
	const3 := uint32(0x6b206574)

	// Key words (little-endian).
	k0 := binary.LittleEndian.Uint32(key[0:4])
	k1 := binary.LittleEndian.Uint32(key[4:8])
	k2 := binary.LittleEndian.Uint32(key[8:12])
	k3 := binary.LittleEndian.Uint32(key[12:16])
	k4 := binary.LittleEndian.Uint32(key[16:20])
	k5 := binary.LittleEndian.Uint32(key[20:24])
	k6 := binary.LittleEndian.Uint32(key[24:28])
	k7 := binary.LittleEndian.Uint32(key[28:32])

	for blockCounter := uint32(0); produced < outLen; blockCounter++ {
		// Initialize state.
		s0, s1, s2, s3 := const0, const1, const2, const3
		s4, s5, s6, s7 := k0, k1, k2, k3
		s8, s9, s10, s11 := k4, k5, k6, k7
		s12 := blockCounter
		s13 := uint32(0) // nonce word 0
		s14 := uint32(0) // nonce word 1
		s15 := uint32(0) // nonce word 2

		// 20 rounds (10 double rounds).
		for i := 0; i < 10; i++ {
			// Column rounds.
			s0, s4, s8, s12 = quarterRound(s0, s4, s8, s12)
			s1, s5, s9, s13 = quarterRound(s1, s5, s9, s13)
			s2, s6, s10, s14 = quarterRound(s2, s6, s10, s14)
			s3, s7, s11, s15 = quarterRound(s3, s7, s11, s15)
			// Diagonal rounds.
			s0, s5, s10, s15 = quarterRound(s0, s5, s10, s15)
			s1, s6, s11, s12 = quarterRound(s1, s6, s11, s12)
			s2, s7, s8, s13 = quarterRound(s2, s7, s8, s13)
			s3, s4, s9, s14 = quarterRound(s3, s4, s9, s14)
		}

		// Add original state.
		s0 += const0
		s1 += const1
		s2 += const2
		s3 += const3
		s4 += k0
		s5 += k1
		s6 += k2
		s7 += k3
		s8 += k4
		s9 += k5
		s10 += k6
		s11 += k7
		s12 += blockCounter
		// s13, s14, s15 += 0

		// Serialize block (64 bytes).
		var block [64]byte
		binary.LittleEndian.PutUint32(block[0:4], s0)
		binary.LittleEndian.PutUint32(block[4:8], s1)
		binary.LittleEndian.PutUint32(block[8:12], s2)
		binary.LittleEndian.PutUint32(block[12:16], s3)
		binary.LittleEndian.PutUint32(block[16:20], s4)
		binary.LittleEndian.PutUint32(block[20:24], s5)
		binary.LittleEndian.PutUint32(block[24:28], s6)
		binary.LittleEndian.PutUint32(block[28:32], s7)
		binary.LittleEndian.PutUint32(block[32:36], s8)
		binary.LittleEndian.PutUint32(block[36:40], s9)
		binary.LittleEndian.PutUint32(block[40:44], s10)
		binary.LittleEndian.PutUint32(block[44:48], s11)
		binary.LittleEndian.PutUint32(block[48:52], s12)
		binary.LittleEndian.PutUint32(block[52:56], s13)
		binary.LittleEndian.PutUint32(block[56:60], s14)
		binary.LittleEndian.PutUint32(block[60:64], s15)

		n := copy(out[produced:], block[:])
		produced += n
	}
	return out
}

// quarterRound performs the ChaCha20 quarter-round operation.
func quarterRound(a, b, c, d uint32) (uint32, uint32, uint32, uint32) {
	a += b
	d ^= a
	d = (d << 16) | (d >> 16)
	c += d
	b ^= c
	b = (b << 12) | (b >> 20)
	a += b
	d ^= a
	d = (d << 8) | (d >> 24)
	c += d
	b ^= c
	b = (b << 7) | (b >> 25)
	return a, b, c, d
}

// bigIntToLE serializes a big.Int as a little-endian byte slice of exactly
// size bytes.
func bigIntToLE(n *big.Int, size int) []byte {
	// big.Int.Bytes() returns big-endian.
	be := n.Bytes()
	le := make([]byte, size)
	for i, b := range be {
		le[len(be)-1-i] = b
	}
	return le
}

// leToBigInt interprets a little-endian byte slice as a big.Int.
func leToBigInt(data []byte) *big.Int {
	// Reverse to big-endian.
	be := make([]byte, len(data))
	for i, b := range data {
		be[len(data)-1-i] = b
	}
	return new(big.Int).SetBytes(be)
}
