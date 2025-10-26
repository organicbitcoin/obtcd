// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// TestAppendOutPointToList tests adding OutPoints to encoded lists
func TestAppendOutPointToList(t *testing.T) {
	// Create test outpoints
	hash1, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	hash2, _ := chainhash.NewHashFromStr("1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	
	outpoint1 := &wire.OutPoint{Hash: *hash1, Index: 0}
	outpoint2 := &wire.OutPoint{Hash: *hash2, Index: 1}

	// Test adding to empty list
	t.Run("add to empty list", func(t *testing.T) {
		initialData := encodeOutPointList([]*wire.OutPoint{})
		updatedData, err := appendOutPointToList(initialData, outpoint1)
		if err != nil {
			t.Errorf("Failed to append to empty list: %v", err)
			return
		}

		decodedPoints, err := decodeOutPointList(updatedData)
		if err != nil {
			t.Errorf("Failed to decode after append: %v", err)
			return
		}

		if len(decodedPoints) != 1 {
			t.Errorf("Expected 1 point, got %d", len(decodedPoints))
		}
	})

	// Test adding to existing list
	t.Run("add to existing list", func(t *testing.T) {
		initialData := encodeOutPointList([]*wire.OutPoint{outpoint1})
		updatedData, err := appendOutPointToList(initialData, outpoint2)
		if err != nil {
			t.Errorf("Failed to append to existing list: %v", err)
			return
		}

		decodedPoints, err := decodeOutPointList(updatedData)
		if err != nil {
			t.Errorf("Failed to decode after append: %v", err)
			return
		}

		if len(decodedPoints) != 2 {
			t.Errorf("Expected 2 points, got %d", len(decodedPoints))
		}
	})
}

// TestRemoveOutPointFromList tests removing OutPoints from encoded lists
func TestRemoveOutPointFromList(t *testing.T) {
	// Create test outpoints
	hash1, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	hash2, _ := chainhash.NewHashFromStr("1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	
	outpoint1 := &wire.OutPoint{Hash: *hash1, Index: 0}
	outpoint2 := &wire.OutPoint{Hash: *hash2, Index: 1}

	// Test removing from single item list
	t.Run("remove from single item", func(t *testing.T) {
		initialData := encodeOutPointList([]*wire.OutPoint{outpoint1})
		updatedData, err := removeOutPointFromList(initialData, outpoint1)
		if err != nil {
			t.Errorf("Failed to remove from list: %v", err)
			return
		}

		// When all items are removed, the list should be empty
		if len(updatedData) == 0 {
			// Empty data is expected when all points are removed
			t.Log("List is empty after removing all items - this is expected")
		} else {
			decodedPoints, err := decodeOutPointList(updatedData)
			if err != nil {
				t.Errorf("Failed to decode after remove: %v", err)
				return
			}

			if len(decodedPoints) != 0 {
				t.Errorf("Expected 0 points after remove, got %d", len(decodedPoints))
			}
		}
	})

	// Test removing from multi-item list
	t.Run("remove from multi-item list", func(t *testing.T) {
		initialData := encodeOutPointList([]*wire.OutPoint{outpoint1, outpoint2})
		updatedData, err := removeOutPointFromList(initialData, outpoint1)
		if err != nil {
			t.Errorf("Failed to remove from multi-item list: %v", err)
			return
		}

		decodedPoints, err := decodeOutPointList(updatedData)
		if err != nil {
			t.Errorf("Failed to decode after remove: %v", err)
			return
		}

		if len(decodedPoints) != 1 {
			t.Errorf("Expected 1 point after remove, got %d", len(decodedPoints))
		}
	})
}

// TestValidateOutPointListSize tests size validation
func TestValidateOutPointListSize(t *testing.T) {
	// Create test outpoints for validation
	hash1, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	outpoint1 := &wire.OutPoint{Hash: *hash1, Index: 0}

	// Test empty list
	t.Run("empty list", func(t *testing.T) {
		err := validateOutPointListSize([]*wire.OutPoint{})
		if err != nil {
			t.Errorf("Empty list should be valid: %v", err)
		}
	})

	// Test single outpoint
	t.Run("single outpoint", func(t *testing.T) {
		err := validateOutPointListSize([]*wire.OutPoint{outpoint1})
		if err != nil {
			t.Errorf("Single outpoint should be valid: %v", err)
		}
	})

	// Test multiple outpoints
	t.Run("multiple outpoints", func(t *testing.T) {
		err := validateOutPointListSize([]*wire.OutPoint{outpoint1, outpoint1})
		if err != nil {
			t.Errorf("Multiple outpoints should be valid: %v", err)
		}
	})
}