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

		cmd := NewListExpiringCmd(&startHeight, &endHeight, &maxResults, nil)

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
		cmd := NewListExpiringCmd(nil, nil, nil, nil)

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

		cmd := NewListExpiringCmd(&startHeight, nil, &maxResults, nil)

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
		cmd := NewListExpiringCmd(&startHeight, nil, nil, nil)

		// Test that the command implements the expected interface
		require.NotNil(cmd)
		require.Equal(startHeight, *cmd.StartHeight)
		require.Nil(cmd.EndHeight)
		require.Nil(cmd.MaxResults)
	})

	t.Run("marshal with min_amount_sat filter", func(t *testing.T) {
		startHeight := int32(1000)
		minAmt := int64(50000)
		cmd := NewListExpiringCmd(&startHeight, nil, nil, nil, &minAmt)

		data, err := json.Marshal(cmd)
		require.NoError(err)

		var unmarshaled ListExpiringCmd
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(err)
		require.Equal(startHeight, *unmarshaled.StartHeight)
		require.NotNil(unmarshaled.MinAmountSat)
		require.Equal(minAmt, *unmarshaled.MinAmountSat)
	})

	t.Run("marshal without min_amount_sat yields nil", func(t *testing.T) {
		cmd := NewListExpiringCmd(nil, nil, nil, nil)
		require.Nil(cmd.MinAmountSat)
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

// TestGetReapPlanCmd tests GetReapPlanCmd marshaling.
func TestGetReapPlanCmd(t *testing.T) {
	cmd := NewGetReapPlanCmd()
	require.NotNil(t, cmd)

	data, err := json.Marshal(cmd)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var unmarshaled GetReapPlanCmd
	require.NoError(t, json.Unmarshal(data, &unmarshaled))
}

// TestGetExpiryCommitmentCmd tests GetExpiryCommitmentCmd marshaling.
func TestGetExpiryCommitmentCmd(t *testing.T) {
	cmd := NewGetExpiryCommitmentCmd()
	require.NotNil(t, cmd)

	data, err := json.Marshal(cmd)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var unmarshaled GetExpiryCommitmentCmd
	require.NoError(t, json.Unmarshal(data, &unmarshaled))
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
			AmountSat:      100_000,
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
		require.Equal(result.AmountSat, unmarshaled.AmountSat)
	})

	t.Run("json tags", func(t *testing.T) {
		result := ExpiringUTXOResult{
			TxID:           "test-txid",
			Vout:           2,
			ExpiryHeight:   500,
			CreateHeight:   400,
			BlocksToExpiry: 50,
			AmountSat:      546,
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
		require.Contains(jsonStr, "amount_sat")
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
					AmountSat:      200_000,
				},
				{
					TxID:           "def456",
					Vout:           1,
					ExpiryHeight:   1100,
					CreateHeight:   1000,
					BlocksToExpiry: 200,
					AmountSat:      50_000,
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
		require.Equal(result.ExpiringUTXOs[0].AmountSat, unmarshaled.ExpiringUTXOs[0].AmountSat)
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

// TestGetReapPlanResult tests GetReapPlanResult marshaling.
func TestGetReapPlanResult(t *testing.T) {
	t.Run("active plan with inputs", func(t *testing.T) {
		result := GetReapPlanResult{
			Height:      145,
			Enabled:     true,
			Active:      true,
			Picked:      3,
			TaxTotal:    90000,
			RefundTotal: 210000,
			EstWeight:   1500,
			MarkerHash:  "deadbeef",
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var got GetReapPlanResult
		require.NoError(t, json.Unmarshal(data, &got))
		require.Equal(t, result.Height, got.Height)
		require.Equal(t, result.Enabled, got.Enabled)
		require.Equal(t, result.Active, got.Active)
		require.Equal(t, result.Picked, got.Picked)
		require.Equal(t, result.TaxTotal, got.TaxTotal)
		require.Equal(t, result.RefundTotal, got.RefundTotal)
		require.Equal(t, result.EstWeight, got.EstWeight)
		require.Equal(t, result.MarkerHash, got.MarkerHash)
	})

	t.Run("reason omitted when empty", func(t *testing.T) {
		result := GetReapPlanResult{
			Height:  145,
			Enabled: true,
			Active:  true,
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		require.NotContains(t, string(data), "reason")
	})

	t.Run("disabled result", func(t *testing.T) {
		result := GetReapPlanResult{
			Height:  1,
			Enabled: false,
			Active:  false,
			Reason:  "expiry index disabled",
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)

		var got GetReapPlanResult
		require.NoError(t, json.Unmarshal(data, &got))
		require.False(t, got.Enabled)
		require.False(t, got.Active)
		require.Equal(t, "expiry index disabled", got.Reason)
	})

	t.Run("json field names", func(t *testing.T) {
		result := GetReapPlanResult{
			Height:      200,
			Enabled:     true,
			Active:      true,
			Picked:      5,
			TaxTotal:    1000,
			RefundTotal: 2000,
			EstWeight:   3000,
			MarkerHash:  "aabbcc",
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		s := string(data)
		for _, field := range []string{"height", "enabled", "active", "picked", "tax_total", "refund_total", "est_weight", "marker_hash"} {
			require.Contains(t, s, field)
		}
	})
}

// TestGetExpiryCommitmentResult tests GetExpiryCommitmentResult marshaling.
func TestGetExpiryCommitmentResult(t *testing.T) {
	t.Run("enabled with root", func(t *testing.T) {
		result := GetExpiryCommitmentResult{
			Enabled:            true,
			Root:               "abcdef1234",
			TipHeight:          144,
			TipHash:            "000000000000000000",
			EnableAtHeight:     112,
			Active:             true,
			ActiveAtNextHeight: true,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var got GetExpiryCommitmentResult
		require.NoError(t, json.Unmarshal(data, &got))
		require.Equal(t, result.Enabled, got.Enabled)
		require.Equal(t, result.Root, got.Root)
		require.Equal(t, result.TipHeight, got.TipHeight)
		require.Equal(t, result.TipHash, got.TipHash)
		require.Equal(t, result.EnableAtHeight, got.EnableAtHeight)
		require.Equal(t, result.Active, got.Active)
		require.Equal(t, result.ActiveAtNextHeight, got.ActiveAtNextHeight)
	})

	t.Run("disabled omits root and tip_hash", func(t *testing.T) {
		result := GetExpiryCommitmentResult{
			Enabled: false,
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		s := string(data)
		require.NotContains(t, s, `"root"`)
		require.NotContains(t, s, `"tip_hash"`)
	})

	t.Run("json field names", func(t *testing.T) {
		result := GetExpiryCommitmentResult{
			Enabled:            true,
			Root:               "aa",
			TipHeight:          10,
			EnableAtHeight:     5,
			Active:             true,
			ActiveAtNextHeight: false,
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		s := string(data)
		for _, field := range []string{"enabled", "root", "tip_height", "enable_at_height", "active", "active_at_next_height"} {
			require.Contains(t, s, field)
		}
	})
}
