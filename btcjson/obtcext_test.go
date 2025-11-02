// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package btcjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListExpiringCmd tests the ListExpiringCmd JSON marshaling and unmarshaling
func TestListExpiringCmd(t *testing.T) {
	require := require.New(t)

	t.Run("marshal with all parameters", func(t *testing.T) {
		startHeight := int32(1000)
		endHeight := int32(2000)
		maxResults := 500

		cmd := NewListExpiringCmd(&startHeight, &endHeight, &maxResults)

		// Test that the command can be marshaled to JSON
		data, err := json.Marshal(cmd)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the command can be unmarshaled from JSON
		var unmarshaled ListExpiringCmd
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(startHeight, *unmarshaled.StartHeight)
		require.Equal(endHeight, *unmarshaled.EndHeight)
		require.Equal(maxResults, *unmarshaled.MaxResults)
	})

	t.Run("marshal with nil parameters", func(t *testing.T) {
		cmd := NewListExpiringCmd(nil, nil, nil)

		// Test that the command can be marshaled to JSON
		data, err := json.Marshal(cmd)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the command can be unmarshaled from JSON
		var unmarshaled ListExpiringCmd
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Nil(unmarshaled.StartHeight)
		require.Nil(unmarshaled.EndHeight)
		require.Nil(unmarshaled.MaxResults)
	})

	t.Run("marshal with partial parameters", func(t *testing.T) {
		startHeight := int32(1500)
		maxResults := 100

		cmd := NewListExpiringCmd(&startHeight, nil, &maxResults)

		// Test that the command can be marshaled to JSON
		data, err := json.Marshal(cmd)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the command can be unmarshaled from JSON
		var unmarshaled ListExpiringCmd
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(startHeight, *unmarshaled.StartHeight)
		require.Nil(unmarshaled.EndHeight)
		require.Equal(maxResults, *unmarshaled.MaxResults)
	})

	t.Run("json rpc command creation", func(t *testing.T) {
		startHeight := int32(500)
		cmd := NewListExpiringCmd(&startHeight, nil, nil)

		// Test that the command implements the expected interface
		require.NotNil(cmd)
		require.Equal(startHeight, *cmd.StartHeight)
		require.Nil(cmd.EndHeight)
		require.Nil(cmd.MaxResults)
	})
}

// TestGetExpiryIndexStatsCmd tests the GetExpiryIndexStatsCmd JSON marshaling and unmarshaling
func TestGetExpiryIndexStatsCmd(t *testing.T) {
	require := require.New(t)

	t.Run("marshal and unmarshal", func(t *testing.T) {
		cmd := NewGetExpiryIndexStatsCmd()

		// Test that the command can be marshaled to JSON
		data, err := json.Marshal(cmd)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the command can be unmarshaled from JSON
		var unmarshaled GetExpiryIndexStatsCmd
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
	})

	t.Run("command creation", func(t *testing.T) {
		cmd := NewGetExpiryIndexStatsCmd()
		require.NotNil(cmd)
	})
}

// TestExpiringUTXOResult tests the ExpiringUTXOResult JSON marshaling and unmarshaling
func TestExpiringUTXOResult(t *testing.T) {
	require := require.New(t)

	t.Run("marshal and unmarshal", func(t *testing.T) {
		result := ExpiringUTXOResult{
			TxID:           "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Vout:           1,
			ExpiryHeight:   1000,
			CreateHeight:   900,
			BlocksToExpiry: 100,
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ExpiringUTXOResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(result.TxID, unmarshaled.TxID)
		require.Equal(result.Vout, unmarshaled.Vout)
		require.Equal(result.ExpiryHeight, unmarshaled.ExpiryHeight)
		require.Equal(result.CreateHeight, unmarshaled.CreateHeight)
		require.Equal(result.BlocksToExpiry, unmarshaled.BlocksToExpiry)
	})

	t.Run("json tags", func(t *testing.T) {
		result := ExpiringUTXOResult{
			TxID:           "test-txid",
			Vout:           2,
			ExpiryHeight:   500,
			CreateHeight:   400,
			BlocksToExpiry: 50,
		}

		data, err := json.Marshal(result)
		require.NoError(err)

		// Verify that the JSON contains the expected field names
		jsonStr := string(data)
		require.Contains(jsonStr, "txid")
		require.Contains(jsonStr, "vout")
		require.Contains(jsonStr, "expiry_height")
		require.Contains(jsonStr, "create_height")
		require.Contains(jsonStr, "blocks_to_expiry")
	})
}

// TestListExpiringResult tests the ListExpiringResult JSON marshaling and unmarshaling
func TestListExpiringResult(t *testing.T) {
	require := require.New(t)

	t.Run("marshal and unmarshal with full data", func(t *testing.T) {
		nextHeight := int32(1500)
		result := ListExpiringResult{
			ExpiringUTXOs: []ExpiringUTXOResult{
				{
					TxID:           "abc123",
					Vout:           0,
					ExpiryHeight:   1000,
					CreateHeight:   900,
					BlocksToExpiry: 100,
				},
				{
					TxID:           "def456",
					Vout:           1,
					ExpiryHeight:   1100,
					CreateHeight:   1000,
					BlocksToExpiry: 200,
				},
			},
			StartHeight:  1000,
			EndHeight:    1200,
			TotalResults: 2,
			NextHeight:   &nextHeight,
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ListExpiringResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(len(result.ExpiringUTXOs), len(unmarshaled.ExpiringUTXOs))
		require.Equal(result.StartHeight, unmarshaled.StartHeight)
		require.Equal(result.EndHeight, unmarshaled.EndHeight)
		require.Equal(result.TotalResults, unmarshaled.TotalResults)
		require.Equal(*result.NextHeight, *unmarshaled.NextHeight)
	})

	t.Run("marshal and unmarshal without next height", func(t *testing.T) {
		result := ListExpiringResult{
			ExpiringUTXOs: []ExpiringUTXOResult{},
			StartHeight:   1000,
			EndHeight:     1200,
			TotalResults:  0,
			NextHeight:    nil,
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ListExpiringResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(result.StartHeight, unmarshaled.StartHeight)
		require.Equal(result.EndHeight, unmarshaled.EndHeight)
		require.Equal(result.TotalResults, unmarshaled.TotalResults)
		require.Nil(unmarshaled.NextHeight)
	})
}

// TestExpiryIndexStatsResult tests the ExpiryIndexStatsResult JSON marshaling and unmarshaling
func TestExpiryIndexStatsResult(t *testing.T) {
	require := require.New(t)

	t.Run("marshal and unmarshal with network params", func(t *testing.T) {
		result := ExpiryIndexStatsResult{
			Disabled:        false,
			TipHeight:       1000,
			TotalUTXOs:      500,
			TotalExpiryKeys: 100,
			NetworkParams: &ExpiryParamsResult{
				WindowBlocks:    100,
				ListBatchLimit:  1000,
				StartScanHeight: 0,
				EnableAtHeight:  0,
			},
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ExpiryIndexStatsResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(result.Disabled, unmarshaled.Disabled)
		require.Equal(result.TipHeight, unmarshaled.TipHeight)
		require.Equal(result.TotalUTXOs, unmarshaled.TotalUTXOs)
		require.Equal(result.TotalExpiryKeys, unmarshaled.TotalExpiryKeys)
		require.NotNil(unmarshaled.NetworkParams)
		require.Equal(result.NetworkParams.WindowBlocks, unmarshaled.NetworkParams.WindowBlocks)
	})

	t.Run("marshal and unmarshal disabled", func(t *testing.T) {
		result := ExpiryIndexStatsResult{
			Disabled: true,
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ExpiryIndexStatsResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.True(unmarshaled.Disabled)
	})
}

// TestExpiryParamsResult tests the ExpiryParamsResult JSON marshaling and unmarshaling
func TestExpiryParamsResult(t *testing.T) {
	require := require.New(t)

	t.Run("marshal and unmarshal", func(t *testing.T) {
		result := ExpiryParamsResult{
			WindowBlocks:    144,
			ListBatchLimit:  5000,
			StartScanHeight: 100,
			EnableAtHeight:  200,
		}

		// Test that the result can be marshaled to JSON
		data, err := json.Marshal(result)
		require.NoError(err)
		require.NotEmpty(data)

		// Test that the result can be unmarshaled from JSON
		var unmarshaled ExpiryParamsResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(result.WindowBlocks, unmarshaled.WindowBlocks)
		require.Equal(result.ListBatchLimit, unmarshaled.ListBatchLimit)
		require.Equal(result.StartScanHeight, unmarshaled.StartScanHeight)
		require.Equal(result.EnableAtHeight, unmarshaled.EnableAtHeight)
	})

	t.Run("json tags", func(t *testing.T) {
		result := ExpiryParamsResult{
			WindowBlocks:    144,
			ListBatchLimit:  5000,
			StartScanHeight: 100,
			EnableAtHeight:  200,
		}

		data, err := json.Marshal(result)
		require.NoError(err)

		// Verify that the JSON contains the expected field names
		jsonStr := string(data)
		require.Contains(jsonStr, "window_blocks")
		require.Contains(jsonStr, "list_batch_limit")
		require.Contains(jsonStr, "start_scan_height")
		require.Contains(jsonStr, "enable_at_height")
	})
}
