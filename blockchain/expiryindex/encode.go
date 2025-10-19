// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// Key and value encoding functions for the expiry index
//
// This file provides deterministic encoding and decoding functions for:
// 1. OutPoint serialization (36 bytes: 32-byte hash + 4-byte index)
// 2. ExpiryKey encoding (8 bytes big-endian for natural sorting)
// 3. OutPointList compression (variable length with deterministic ordering)
//
// All encodings are designed to be:
// - Deterministic: Same input always produces same output
// - Sortable: Keys sort in logical order (expiry time ascending)
// - Compact: Minimal space usage for efficient storage
// - Cross-platform: Fixed endianness for consistency

// OutPoint encoding format:
// [0:32]  - Transaction hash (32 bytes, preserved as-is from chainhash.Hash)
// [32:36] - Output index (4 bytes, little-endian uint32)
//
// Total: 36 bytes per OutPoint
func encodeOutPoint(op *wire.OutPoint) []byte {
	encoded := make([]byte, 36)
	copy(encoded[0:32], op.Hash[:])
	binary.LittleEndian.PutUint32(encoded[32:36], op.Index)
	return encoded
}

// decodeOutPoint reconstructs an OutPoint from its 36-byte encoding
func decodeOutPoint(encoded []byte) (*wire.OutPoint, error) {
	if len(encoded) != 36 {
		return nil, fmt.Errorf("invalid outpoint encoding length: got %d, expected 36", len(encoded))
	}
	
	var hash chainhash.Hash
	copy(hash[:], encoded[0:32])
	index := binary.LittleEndian.Uint32(encoded[32:36])
	
	return &wire.OutPoint{Hash: hash, Index: index}, nil
}

// ExpiryKey encoding format:
// 8 bytes, big-endian uint64
//
// Big-endian encoding ensures that keys sort in natural expiry order:
// - For height mode: earlier heights < later heights
// - For time mode: earlier timestamps < later timestamps
//
// This natural sorting is critical for efficient range scans.
func encodeExpiryKey(expiry uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, expiry)
	return key
}

// decodeExpiryKey extracts the expiry value from its 8-byte encoding
func decodeExpiryKey(encoded []byte) (uint64, error) {
	if len(encoded) != 8 {
		return 0, fmt.Errorf("invalid expiry key length: got %d, expected 8", len(encoded))
	}
	return binary.BigEndian.Uint64(encoded), nil
}

// OutPointList encoding format:
// [0:4]    - Count of outpoints (4 bytes, little-endian uint32)
// [4:40]   - First outpoint (36 bytes)
// [40:76]  - Second outpoint (36 bytes)
// ...      - Additional outpoints (36 bytes each)
//
// Outpoints are sorted deterministically by (txid, vout) to ensure
// consistent encoding across different implementations and platforms.
//
// Total length: 4 + (count * 36) bytes
func encodeOutPointList(outpoints []*wire.OutPoint) []byte {
	// Sort for deterministic encoding
	sortedOPs := make([]*wire.OutPoint, len(outpoints))
	copy(sortedOPs, outpoints)
	
	sort.Slice(sortedOPs, func(i, j int) bool {
		// Primary sort: transaction hash (lexicographic)
		hashCmp := sortedOPs[i].Hash.String() < sortedOPs[j].Hash.String()
		if sortedOPs[i].Hash != sortedOPs[j].Hash {
			return hashCmp
		}
		// Secondary sort: output index (numeric)
		return sortedOPs[i].Index < sortedOPs[j].Index
	})
	
	// Encode: count (4 bytes) + outpoints (36 bytes each)
	encoded := make([]byte, 4+len(sortedOPs)*36)
	binary.LittleEndian.PutUint32(encoded[0:4], uint32(len(sortedOPs)))
	
	for i, op := range sortedOPs {
		offset := 4 + i*36
		copy(encoded[offset:offset+36], encodeOutPoint(op))
	}
	
	return encoded
}

// decodeOutPointList reconstructs a list of OutPoints from encoded data
func decodeOutPointList(encoded []byte) ([]*wire.OutPoint, error) {
	if len(encoded) < 4 {
		return nil, fmt.Errorf("invalid outpoint list encoding: too short (got %d bytes)", len(encoded))
	}
	
	count := binary.LittleEndian.Uint32(encoded[0:4])
	expectedLen := 4 + int(count)*36
	if len(encoded) != expectedLen {
		return nil, fmt.Errorf("invalid outpoint list length: got %d, expected %d", 
			len(encoded), expectedLen)
	}
	
	outpoints := make([]*wire.OutPoint, count)
	for i := uint32(0); i < count; i++ {
		offset := 4 + int(i)*36
		op, err := decodeOutPoint(encoded[offset : offset+36])
		if err != nil {
			return nil, fmt.Errorf("failed to decode outpoint %d: %v", i, err)
		}
		outpoints[i] = op
	}
	
	return outpoints, nil
}

// appendOutPointToList adds an outpoint to an existing encoded list
// This is an optimization to avoid full decode/encode cycles for single additions
func appendOutPointToList(existingEncoded []byte, newOutPoint *wire.OutPoint) ([]byte, error) {
	// Decode existing list
	existingOutPoints, err := decodeOutPointList(existingEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode existing list: %v", err)
	}
	
	// Check if already exists (duplicate prevention)
	for _, existing := range existingOutPoints {
		if existing.Hash == newOutPoint.Hash && existing.Index == newOutPoint.Index {
			// Already exists, return unchanged
			return existingEncoded, nil
		}
	}
	
	// Add new outpoint and re-encode
	allOutPoints := append(existingOutPoints, newOutPoint)
	return encodeOutPointList(allOutPoints), nil
}

// removeOutPointFromList removes an outpoint from an existing encoded list
// Returns the updated list, or nil if the list becomes empty
func removeOutPointFromList(existingEncoded []byte, targetOutPoint *wire.OutPoint) ([]byte, error) {
	// Decode existing list
	existingOutPoints, err := decodeOutPointList(existingEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode existing list: %v", err)
	}
	
	// Filter out the target outpoint
	var filteredOutPoints []*wire.OutPoint
	for _, existing := range existingOutPoints {
		if existing.Hash != targetOutPoint.Hash || existing.Index != targetOutPoint.Index {
			filteredOutPoints = append(filteredOutPoints, existing)
		}
	}
	
	// If list is now empty, return nil (caller should delete the key)
	if len(filteredOutPoints) == 0 {
		return nil, nil
	}
	
	// Re-encode the filtered list
	return encodeOutPointList(filteredOutPoints), nil
}

// validateOutPointListSize checks if a list exceeds the maximum size limit
func validateOutPointListSize(outpoints []*wire.OutPoint) error {
	if len(outpoints) > MaxOutpointsPerKey {
		return fmt.Errorf("outpoint list too large: %d > %d (max)", 
			len(outpoints), MaxOutpointsPerKey)
	}
	return nil
}