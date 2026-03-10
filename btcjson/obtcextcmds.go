// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// NOTE: This file is intended to house the RPC commands that are specific to
// OBTC (Organic Bitcoin) extensions.

// OBTC-only: JSON-RPC command definitions.

package btcjson

// ListExpiringCmd defines the listexpiring JSON-RPC command.
// This command is an OBTC-specific extension for querying UTXOs that are
// approaching expiration.
type ListExpiringCmd struct {
	// StartHeight is the block height to start scanning from.
	// Optional: defaults to current tip height.
	StartHeight *int32 `jsonrpcusage:"\"start_height\""`

	// EndHeight is the block height to stop scanning at.
	// Optional: defaults to startHeight + default horizon.
	EndHeight *int32 `jsonrpcusage:"\"end_height\""`

	// MaxResults is the maximum number of results to return.
	// Optional: defaults to network-specific limit.
	MaxResults *int `jsonrpcusage:"\"max_results\""`

	// StartAfter is an optional cursor in the form "txid:vout" to resume
	// pagination within the start height.
	StartAfter *string `jsonrpcusage:"\"start_after\""`

	// MinAmountSat is an optional minimum UTXO value filter in satoshis.
	// Only UTXOs with amount >= min_amount_sat are returned.
	MinAmountSat *int64 `jsonrpcusage:"\"min_amount_sat\""`
}

// NewListExpiringCmd returns a new instance which can be used to issue a
// `listexpiring` JSON-RPC command.
//
// The parameters which are pointers indicate they are optional. Passing nil
// for optional parameters will use the default value.
func NewListExpiringCmd(startHeight *int32, endHeight *int32, maxResults *int, startAfter *string, minAmountSat ...*int64) *ListExpiringCmd {
	cmd := &ListExpiringCmd{
		StartHeight: startHeight,
		EndHeight:   endHeight,
		MaxResults:  maxResults,
		StartAfter:  startAfter,
	}
	if len(minAmountSat) > 0 {
		cmd.MinAmountSat = minAmountSat[0]
	}
	return cmd
}

// GetExpiryIndexStatsCmd defines the getexpiryindexstats JSON-RPC command.
// This command returns statistics about the OBTC expiry index.
type GetExpiryIndexStatsCmd struct{}

// NewGetExpiryIndexStatsCmd returns a new instance which can be used to issue a
// `getexpiryindexstats` JSON-RPC command.
func NewGetExpiryIndexStatsCmd() *GetExpiryIndexStatsCmd {
	return &GetExpiryIndexStatsCmd{}
}

// GetReapPlanCmd defines the getreapplan JSON-RPC command.
// Returns a dry-run summary of the REAP transaction that would be included in
// the next block, without broadcasting or committing anything.
type GetReapPlanCmd struct{}

// NewGetReapPlanCmd returns a new instance which can be used to issue a
// `getreapplan` JSON-RPC command.
func NewGetReapPlanCmd() *GetReapPlanCmd {
	return &GetReapPlanCmd{}
}

// GetExpiryCommitmentCmd defines the getexpirycommitment JSON-RPC command.
// Returns the current expiry accumulator snapshot and activation metadata from
// the expiry index.
type GetExpiryCommitmentCmd struct{}

// NewGetExpiryCommitmentCmd returns a new instance which can be used to issue a
// `getexpirycommitment` JSON-RPC command.
func NewGetExpiryCommitmentCmd() *GetExpiryCommitmentCmd {
	return &GetExpiryCommitmentCmd{}
}

func init() {
	// No special handling for flags in any of the commands in this file so
	// far, so just register them all without any special flags.
	flags := UsageFlag(0)

	MustRegisterCmd("listexpiring", (*ListExpiringCmd)(nil), flags)
	MustRegisterCmd("getexpiryindexstats", (*GetExpiryIndexStatsCmd)(nil), flags)
	MustRegisterCmd("getreapplan", (*GetReapPlanCmd)(nil), flags)
	MustRegisterCmd("getexpirycommitment", (*GetExpiryCommitmentCmd)(nil), flags)
}
