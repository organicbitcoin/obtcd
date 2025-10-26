// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

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