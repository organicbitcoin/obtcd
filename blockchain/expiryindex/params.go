// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

// ExpiryParams defines parameters for UTXO expiry calculation and indexing
//
// OBTC uses height-based expiry for deterministic and consensus-friendly behavior.
// All UTXOs expire after a fixed number of blocks from their creation height.
type ExpiryParams struct {
	// WindowBlocks is the expiry window in blocks
	// For mainnet: ~1 year worth of blocks (144 * 365 = 52,560)
	// For testnet: shorter periods for faster testing
	WindowBlocks uint64

	// ListBatchLimit is the maximum number of items returned in one RPC scan
	// This prevents excessive memory usage and long-running queries
	ListBatchLimit int

	// StartScanHeight is the block height at which to start building the index
	// This should typically be set to the OBTC fork height
	StartScanHeight int32

	// EnableAtHeight is the block height at which expiry enforcement begins
	// This allows the index to be built before enforcement starts (Week 3+)
	EnableAtHeight int32

	// ExpiryCommitmentEnableAtHeight is the height at which the expiry
	// commitment in coinbase becomes mandatory.
	ExpiryCommitmentEnableAtHeight int32
}

// GetExpiryParams returns expiry parameters for the given network.
// Returns nil if the network is not an OBTC network.
func GetExpiryParams(params *chaincfg.Params) *ExpiryParams {
	// Get the expiry params from chaincfg (which handles the network detection)
	cfg := chaincfg.GetExpiryParams(params)
	if cfg == nil {
		return nil
	}

	// Convert chaincfg.ExpiryParams to expiryindex.ExpiryParams
	return &ExpiryParams{
		WindowBlocks:                   cfg.WindowBlocks,
		ListBatchLimit:                 cfg.ListBatchLimit,
		StartScanHeight:                cfg.StartScanHeight,
		EnableAtHeight:                 cfg.EnableAtHeight,
		ExpiryCommitmentEnableAtHeight: cfg.ExpiryCommitmentEnableAtHeight,
	}
}

// CalculateExpiryKey calculates the expiry key for a UTXO created at the given height.
//
// The expiry key is the block height at which the UTXO will expire:
// expiryKey = createHeight + WindowBlocks
//
// This produces monotonically increasing keys that sort naturally by expiry order,
// enabling efficient database range scans.
func (p *ExpiryParams) CalculateExpiryKey(createHeight int32) uint64 {
	return uint64(createHeight) + p.WindowBlocks
}

// IsExpiryEnabled returns true if expiry enforcement is enabled at the given height
func (p *ExpiryParams) IsExpiryEnabled(height int32) bool {
	return height >= p.EnableAtHeight
}

// IsIndexingEnabled returns true if expiry indexing should be active at the given height
func (p *ExpiryParams) IsIndexingEnabled(height int32) bool {
	return height >= p.StartScanHeight
}

// ValidateListParams validates parameters for the listExpiring RPC call
func (p *ExpiryParams) ValidateListParams(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be positive, got %d", limit)
	}
	if limit > p.ListBatchLimit {
		return fmt.Errorf("limit %d exceeds maximum %d", limit, p.ListBatchLimit)
	}
	return nil
}

// CalculateExpiryRange calculates the range [fromKey, toKey] for scanning
// UTXOs that expire within the given horizon from the starting point.
//
// This is used by the RPC layer to convert user-friendly parameters
// (like "show me UTXOs expiring in the next 1000 blocks") into the
// low-level database scan range.
func (p *ExpiryParams) CalculateExpiryRange(fromHeight uint64, horizonBlocks uint64) (fromKey, toKey uint64) {
	fromKey = fromHeight
	toKey = fromHeight + horizonBlocks

	// Ensure toKey doesn't overflow
	if toKey < fromHeight {
		toKey = ^uint64(0) // Max uint64
	}

	return fromKey, toKey
}

// GetDefaultHorizon returns a sensible default horizon for RPC queries
// based on the network configuration (~1 day worth of blocks)
func (p *ExpiryParams) GetDefaultHorizon() uint64 {
	return 144 // ~1 day worth of blocks at 10min intervals
}
