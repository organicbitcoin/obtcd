// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// Key and value encoding functions for the expiry index.
//
// This file provides deterministic encoding and decoding functions for:
// 1. OutPoint serialization (36 bytes: 32-byte hash + 4-byte index)
// 2. Ordered OutPoint serialization for lexicographic scans
// 3. ExpiryKey encoding (8 bytes big-endian for natural sorting)
// 4. Composite key encoding: ExpiryKey || OrderedOutPoint

const (
	outPointEncodedSize            = chainhash.HashSize + 4
	expiryKeyEncodedSize           = 8
	orderedOutPointEncodedSize     = chainhash.HashSize + 4
	expiryOutpointCompositeKeySize = expiryKeyEncodedSize + orderedOutPointEncodedSize
)

var emptyIndexValue = []byte{}

// encodeOutPoint serializes an outpoint as 32 raw hash bytes followed by the
// vout index encoded as little-endian.
func encodeOutPoint(op *wire.OutPoint) []byte {
	encoded := make([]byte, outPointEncodedSize)
	copy(encoded[:chainhash.HashSize], op.Hash[:])
	binary.LittleEndian.PutUint32(encoded[chainhash.HashSize:], op.Index)
	return encoded
}

// decodeOutPoint reconstructs an OutPoint from its 36-byte encoding.
func decodeOutPoint(encoded []byte) (*wire.OutPoint, error) {
	if len(encoded) != outPointEncodedSize {
		return nil, fmt.Errorf("invalid outpoint encoding length: got %d, expected %d",
			len(encoded), outPointEncodedSize)
	}

	var hash chainhash.Hash
	copy(hash[:], encoded[:chainhash.HashSize])

	return &wire.OutPoint{
		Hash:  hash,
		Index: binary.LittleEndian.Uint32(encoded[chainhash.HashSize:]),
	}, nil
}

// encodeOrderedOutPoint serializes an outpoint so lexicographic byte order
// matches compareOutPoint / chainhash.Hash.String() ordering.
func encodeOrderedOutPoint(op *wire.OutPoint) []byte {
	encoded := make([]byte, orderedOutPointEncodedSize)
	for i := 0; i < chainhash.HashSize; i++ {
		encoded[i] = op.Hash[chainhash.HashSize-1-i]
	}
	binary.BigEndian.PutUint32(encoded[chainhash.HashSize:], op.Index)
	return encoded
}

// decodeOrderedOutPoint reconstructs an OutPoint from its ordered encoding.
func decodeOrderedOutPoint(encoded []byte) (*wire.OutPoint, error) {
	if len(encoded) != orderedOutPointEncodedSize {
		return nil, fmt.Errorf("invalid ordered outpoint encoding length: got %d, expected %d",
			len(encoded), orderedOutPointEncodedSize)
	}

	var hash chainhash.Hash
	for i := 0; i < chainhash.HashSize; i++ {
		hash[chainhash.HashSize-1-i] = encoded[i]
	}

	return &wire.OutPoint{
		Hash:  hash,
		Index: binary.BigEndian.Uint32(encoded[chainhash.HashSize:]),
	}, nil
}

// encodeExpiryKey encodes the expiry key as big-endian uint64 so keys sort in
// natural expiry order.
func encodeExpiryKey(expiry uint64) []byte {
	key := make([]byte, expiryKeyEncodedSize)
	binary.BigEndian.PutUint64(key, expiry)
	return key
}

// decodeExpiryKey extracts the expiry value from its 8-byte encoding.
func decodeExpiryKey(encoded []byte) (uint64, error) {
	if len(encoded) != expiryKeyEncodedSize {
		return 0, fmt.Errorf("invalid expiry key length: got %d, expected %d",
			len(encoded), expiryKeyEncodedSize)
	}
	return binary.BigEndian.Uint64(encoded), nil
}

// encodeExpiryOutpointCompositeKey encodes the scan key used by
// bktExpiry2Outpoints: expiry key prefix followed by the ordered outpoint.
func encodeExpiryOutpointCompositeKey(expiryKey uint64, outpoint *wire.OutPoint) []byte {
	encoded := make([]byte, expiryOutpointCompositeKeySize)
	binary.BigEndian.PutUint64(encoded[:expiryKeyEncodedSize], expiryKey)
	copy(encoded[expiryKeyEncodedSize:], encodeOrderedOutPoint(outpoint))
	return encoded
}

// decodeExpiryOutpointCompositeKey reconstructs the expiry key and outpoint
// from a composite scan key.
func decodeExpiryOutpointCompositeKey(encoded []byte) (uint64, *wire.OutPoint, error) {
	if len(encoded) != expiryOutpointCompositeKeySize {
		return 0, nil, fmt.Errorf("invalid expiry composite key length: got %d, expected %d",
			len(encoded), expiryOutpointCompositeKeySize)
	}

	expiryKey, err := decodeExpiryKey(encoded[:expiryKeyEncodedSize])
	if err != nil {
		return 0, nil, err
	}

	outpoint, err := decodeOrderedOutPoint(encoded[expiryKeyEncodedSize:])
	if err != nil {
		return 0, nil, err
	}

	return expiryKey, outpoint, nil
}

// expiryCompositePrefix returns the prefix used to seek all entries for the
// provided expiry key.
func expiryCompositePrefix(expiryKey uint64) []byte {
	return encodeExpiryKey(expiryKey)
}
