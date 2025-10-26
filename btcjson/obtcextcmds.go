// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// NOTE: This file is intended to house the RPC commands that are specific to
// OBTC (Organic Bitcoin) extensions.

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
}

// NewListExpiringCmd returns a new instance which can be used to issue a 
// `listexpiring` JSON-RPC command.
//
// The parameters which are pointers indicate they are optional. Passing nil
// for optional parameters will use the default value.
func NewListExpiringCmd(startHeight *int32, endHeight *int32, maxResults *int) *ListExpiringCmd {
	return &ListExpiringCmd{
		StartHeight: startHeight,
		EndHeight:   endHeight,
		MaxResults:  maxResults,
	}
}

// GetExpiryIndexStatsCmd defines the getexpiryindexstats JSON-RPC command.
// This command returns statistics about the OBTC expiry index.
type GetExpiryIndexStatsCmd struct{}

// NewGetExpiryIndexStatsCmd returns a new instance which can be used to issue a
// `getexpiryindexstats` JSON-RPC command.
func NewGetExpiryIndexStatsCmd() *GetExpiryIndexStatsCmd {
	return &GetExpiryIndexStatsCmd{}
}

func init() {
	// No special handling for flags in any of the commands in this file so
	// far, so just register them all without any special flags.
	flags := UsageFlag(0)

	MustRegisterCmd("listexpiring", (*ListExpiringCmd)(nil), flags)
	MustRegisterCmd("getexpiryindexstats", (*GetExpiryIndexStatsCmd)(nil), flags)
}