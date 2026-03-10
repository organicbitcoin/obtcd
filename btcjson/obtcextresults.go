// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// OBTC-only: JSON-RPC result types for OBTC extensions.

package btcjson

// ExpiringUTXOResult represents a UTXO that is scheduled to expire.
type ExpiringUTXOResult struct {
	// TxID is the transaction ID of the UTXO
	TxID string `json:"txid"`

	// Vout is the output index in the transaction
	Vout uint32 `json:"vout"`

	// ExpiryHeight is the block height at which this UTXO will expire
	ExpiryHeight uint64 `json:"expiry_height"`

	// CreateHeight is the block height at which this UTXO was created
	CreateHeight uint64 `json:"create_height"`

	// BlocksToExpiry is the number of blocks until expiry
	BlocksToExpiry int64 `json:"blocks_to_expiry"`

	// AmountSat is the value of the UTXO in satoshis.
	// Zero if the UTXO could not be found in the UTXO set.
	AmountSat int64 `json:"amount_sat"`
}

// ListExpiringResult models the data returned from the listexpiring command.
type ListExpiringResult struct {
	// ExpiringUTXOs is the list of UTXOs approaching expiration
	ExpiringUTXOs []ExpiringUTXOResult `json:"expiring_utxos"`

	// StartHeight is the height at which the scan started
	StartHeight int32 `json:"start_height"`

	// EndHeight is the height at which the scan ended
	EndHeight int32 `json:"end_height"`

	// TotalResults is the total number of results found
	TotalResults int `json:"total_results"`

	// NextHeight can be used for pagination in subsequent requests
	NextHeight *int32 `json:"next_height,omitempty"`

	// NextOutpoint can be used with next_height to resume pagination.
	// Format: "txid:vout"
	NextOutpoint *string `json:"next_outpoint,omitempty"`
}

// ExpiryIndexStatsResult models the data returned from the getexpiryindexstats command.
type ExpiryIndexStatsResult struct {
	// Disabled indicates whether the expiry index is disabled
	Disabled bool `json:"disabled"`

	// TipHeight is the current tip height indexed
	TipHeight int32 `json:"tip_height"`

	// TotalUTXOs is the total number of UTXOs tracked in the index
	TotalUTXOs int `json:"total_utxos"`

	// TotalExpiryKeys is the total number of unique expiry heights
	TotalExpiryKeys int `json:"total_expiry_keys"`

	// NetworkParams contains network-specific expiry parameters
	NetworkParams *ExpiryParamsResult `json:"network_params,omitempty"`
}

// ExpiryParamsResult contains network-specific expiry parameters.
type ExpiryParamsResult struct {
	// WindowBlocks is the expiry window in blocks
	WindowBlocks uint64 `json:"window_blocks"`

	// ListBatchLimit is the maximum number of items returned in one RPC scan
	ListBatchLimit int `json:"list_batch_limit"`

	// StartScanHeight is the block height at which to start building the index
	StartScanHeight int32 `json:"start_scan_height"`

	// EnableAtHeight is the block height at which expiry enforcement begins
	EnableAtHeight int32 `json:"enable_at_height"`
}

// GetReapPlanResult models the data returned from the getreapplan command.
// It contains a dry-run summary of the REAP transaction for the next block.
type GetReapPlanResult struct {
	// Height is the next block height the plan is computed for.
	Height int32 `json:"height"`

	// Enabled is true when an expiry index is available and OBTC params are set.
	Enabled bool `json:"enabled"`

	// Active is true when REAP enforcement is active at the next block height.
	Active bool `json:"active"`

	// Reason is a human-readable explanation when Enabled or Active is false.
	Reason string `json:"reason,omitempty"`

	// Picked is the number of expiring UTXOs selected as REAP inputs.
	Picked int `json:"picked"`

	// TaxTotal is the total miner reward (tax) in satoshis.
	TaxTotal int64 `json:"tax_total"`

	// RefundTotal is the total refund to output addresses in satoshis.
	RefundTotal int64 `json:"refund_total"`

	// EstWeight is the estimated transaction weight units of the REAP tx.
	EstWeight int64 `json:"est_weight"`

	// MarkerHash is the hex-encoded SHA-256 commitment digest over the ordered
	// REAP inputs; empty when Picked == 0.
	MarkerHash string `json:"marker_hash,omitempty"`
}

// GetExpiryCommitmentResult models the data returned from the
// getexpirycommitment command.
type GetExpiryCommitmentResult struct {
	// Enabled is true when an expiry index is available.
	Enabled bool `json:"enabled"`

	// Root is the hex-encoded MuHash accumulator digest at the indexed tip.
	// Empty when the index is disabled.
	Root string `json:"root,omitempty"`

	// TipHeight is the block height the accumulator root corresponds to.
	TipHeight int32 `json:"tip_height"`

	// TipHash is the hex-encoded block hash the accumulator root corresponds to.
	TipHash string `json:"tip_hash,omitempty"`

	// EnableAtHeight is the block height at which expiry commitments become
	// mandatory in block coinbase transactions.
	EnableAtHeight int32 `json:"enable_at_height"`

	// Active is true when expiry commitments are required at TipHeight.
	Active bool `json:"active"`

	// ActiveAtNextHeight is true when expiry commitments will be required at
	// TipHeight+1 (the next block to be mined).
	ActiveAtNextHeight bool `json:"active_at_next_height"`
}
