// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

// ExpiryParams defines parameters for expiry calculation and indexing.
type ExpiryParams struct {
	// WindowBlocks is the expiry window in blocks.
	WindowBlocks uint64

	// ListBatchLimit is the maximum number of items returned in one RPC scan.
	ListBatchLimit int

	// StartScanHeight is the block height at which to start building the index.
	StartScanHeight int32

	// EnableAtHeight is the block height at which expiry enforcement begins.
	EnableAtHeight int32

	// ExpiryCommitmentEnableAtHeight is the height at which the expiry
	// commitment in coinbase becomes mandatory.
	ExpiryCommitmentEnableAtHeight int32
}

// GetExpiryParams returns expiry parameters for an OBTC network, or nil.
func GetExpiryParams(params *chaincfg.Params) *ExpiryParams {
	cfg := chaincfg.GetExpiryParams(params)
	if cfg == nil {
		return nil
	}

	return &ExpiryParams{
		WindowBlocks:                   cfg.WindowBlocks,
		ListBatchLimit:                 cfg.ListBatchLimit,
		StartScanHeight:                cfg.StartScanHeight,
		EnableAtHeight:                 cfg.EnableAtHeight,
		ExpiryCommitmentEnableAtHeight: cfg.ExpiryCommitmentEnableAtHeight,
	}
}

// CalculateExpiryKey returns the expiry key for a UTXO created at createHeight.
func (p *ExpiryParams) CalculateExpiryKey(createHeight int32) uint64 {
	return uint64(createHeight) + p.WindowBlocks
}

// IsExpiryEnabled reports whether expiry enforcement is enabled at height.
func (p *ExpiryParams) IsExpiryEnabled(height int32) bool {
	return height >= p.EnableAtHeight
}

// IsIndexingEnabled reports whether expiry indexing should be active at height.
func (p *ExpiryParams) IsIndexingEnabled(height int32) bool {
	return height >= p.StartScanHeight
}

// ValidateListParams validates parameters for the listexpiring RPC.
func (p *ExpiryParams) ValidateListParams(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be positive, got %d", limit)
	}
	if limit > p.ListBatchLimit {
		return fmt.Errorf("limit %d exceeds maximum %d", limit, p.ListBatchLimit)
	}
	return nil
}

// CalculateExpiryRange returns the [fromKey, toKey] scan range for a horizon.
func (p *ExpiryParams) CalculateExpiryRange(fromHeight uint64, horizonBlocks uint64) (fromKey, toKey uint64) {
	fromKey = fromHeight
	toKey = fromHeight + horizonBlocks

	if toKey < fromHeight {
		toKey = ^uint64(0)
	}

	return fromKey, toKey
}

// GetDefaultHorizon returns the default RPC scan horizon.
func (p *ExpiryParams) GetDefaultHorizon() uint64 {
	return 144
}
