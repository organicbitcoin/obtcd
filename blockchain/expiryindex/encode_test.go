package expiryindex

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// TestEncodeDecodeOutPoint tests OutPoint encoding/decoding functionality
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
			// Test encoding
			encoded := encodeOutPoint(&test.outpoint)
			
			// Verify encoded length (32 bytes hash + 4 bytes index)
			expectedLen := 36
			if len(encoded) != expectedLen {
				t.Errorf("Encoded length: got %d, want %d", len(encoded), expectedLen)
			}

			// Test decoding
			decoded, err := decodeOutPoint(encoded)
			if err != nil {
				t.Errorf("Decode error: %v", err)
			}

			// Verify round-trip consistency
			if !decoded.Hash.IsEqual(&test.outpoint.Hash) {
				t.Errorf("Hash mismatch: got %v, want %v", decoded.Hash, test.outpoint.Hash)
			}
			if decoded.Index != test.outpoint.Index {
				t.Errorf("Index mismatch: got %d, want %d", decoded.Index, test.outpoint.Index)
			}
		})
	}
}

// TestEncodeDecodeExpiryKey tests ExpiryKey encoding/decoding functionality
func TestEncodeDecodeExpiryKey(t *testing.T) {
	tests := []struct {
		name      string
		expiryKey uint64
	}{
		{
			name:      "zero key",
			expiryKey: 0,
		},
		{
			name:      "small key",
			expiryKey: 12345,
		},
		{
			name:      "large key",
			expiryKey: 1<<32 + 67890,
		},
		{
			name:      "max key",
			expiryKey: 0xFFFFFFFFFFFFFFFF,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test encoding
			encoded := encodeExpiryKey(test.expiryKey)
			
			// Verify encoded length (8 bytes for uint64)
			expectedLen := 8
			if len(encoded) != expectedLen {
				t.Errorf("Encoded length: got %d, want %d", len(encoded), expectedLen)
			}

			// Test decoding
			decoded, err := decodeExpiryKey(encoded)
			if err != nil {
				t.Errorf("Decode error: %v", err)
			}

			// Verify round-trip consistency
			if decoded != test.expiryKey {
				t.Errorf("ExpiryKey mismatch: got %d, want %d", decoded, test.expiryKey)
			}
		})
	}
}

// TestExpiryKeyOrdering tests that ExpiryKey encoding produces correct ordering
func TestExpiryKeyOrdering(t *testing.T) {
	keys := []uint64{
		0,
		1,
		255,
		256,
		65535,
		65536,
		0xFFFFFFFF,
		0x100000000,
		0xFFFFFFFFFFFFFFFF,
	}

	// Encode all keys
	var encodedKeys [][]byte
	for _, key := range keys {
		encodedKeys = append(encodedKeys, encodeExpiryKey(key))
	}

	// Verify that encoded keys are in ascending order (big-endian)
	for i := 0; i < len(encodedKeys)-1; i++ {
		if bytes.Compare(encodedKeys[i], encodedKeys[i+1]) >= 0 {
			t.Errorf("ExpiryKey ordering violation at index %d: %d >= %d", i, keys[i], keys[i+1])
		}
	}
}

// TestEncodeDecodeOutPointList tests OutPointList encoding/decoding functionality
func TestEncodeDecodeOutPointList(t *testing.T) {
	tests := []struct {
		name      string
		outpoints []wire.OutPoint
	}{
		{
			name:      "empty list",
			outpoints: []wire.OutPoint{},
		},
		{
			name: "single outpoint",
			outpoints: []wire.OutPoint{
				{
					Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
					Index: 0,
				},
			},
		},
		{
			name: "multiple outpoints",
			outpoints: []wire.OutPoint{
				{
					Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
					Index: 0,
				},
				{
					Hash:  chainhash.Hash{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F, 0x40},
					Index: 1,
				},
				{
					Hash:  chainhash.Hash{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F, 0x60},
					Index: 2,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Convert to pointer slice
			var ptrs []*wire.OutPoint
			for i := range test.outpoints {
				ptrs = append(ptrs, &test.outpoints[i])
			}
			
			// Test encoding
			encoded := encodeOutPointList(ptrs)

			// Test decoding
			decoded, err := decodeOutPointList(encoded)
			if err != nil {
				t.Errorf("Decode error: %v", err)
			}

			// Verify length matches
			if len(decoded) != len(test.outpoints) {
				t.Errorf("Length mismatch: got %d, want %d", len(decoded), len(test.outpoints))
			}

			// Verify each outpoint matches
			for i, expected := range test.outpoints {
				if i >= len(decoded) {
					t.Errorf("Missing outpoint at index %d", i)
					continue
				}
				
				actual := decoded[i]
				if !actual.Hash.IsEqual(&expected.Hash) {
					t.Errorf("Hash mismatch at index %d: got %v, want %v", i, actual.Hash, expected.Hash)
				}
				if actual.Index != expected.Index {
					t.Errorf("Index mismatch at index %d: got %d, want %d", i, actual.Index, expected.Index)
				}
			}
		})
	}
}

// TestDecodeInvalidData tests error handling for invalid encoded data
func TestDecodeInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		fn   func([]byte) error
	}{
		{
			name: "outpoint too short",
			data: make([]byte, 35), // Should be 36 bytes
			fn: func(data []byte) error {
				_, err := decodeOutPoint(data)
				return err
			},
		},
		{
			name: "outpoint too long",
			data: make([]byte, 37), // Should be 36 bytes
			fn: func(data []byte) error {
				_, err := decodeOutPoint(data)
				return err
			},
		},
		{
			name: "expiry key too short",
			data: make([]byte, 7), // Should be 8 bytes
			fn: func(data []byte) error {
				_, err := decodeExpiryKey(data)
				return err
			},
		},
		{
			name: "expiry key too long",
			data: make([]byte, 9), // Should be 8 bytes
			fn: func(data []byte) error {
				_, err := decodeExpiryKey(data)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.fn(test.data)
			if err == nil {
				t.Errorf("Expected error for invalid data, got nil")
			}
		})
	}
}

// TestEncodingDeterminism tests that encoding is deterministic
func TestEncodingDeterminism(t *testing.T) {
	outpoint := wire.OutPoint{
		Hash:  chainhash.Hash{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20},
		Index: 12345,
	}

	// Encode the same outpoint multiple times
	encoded1 := encodeOutPoint(&outpoint)
	encoded2 := encodeOutPoint(&outpoint)
	encoded3 := encodeOutPoint(&outpoint)

	// Verify all encodings are identical
	if !bytes.Equal(encoded1, encoded2) {
		t.Errorf("Encoding not deterministic: first != second")
	}
	if !bytes.Equal(encoded2, encoded3) {
		t.Errorf("Encoding not deterministic: second != third")
	}
}