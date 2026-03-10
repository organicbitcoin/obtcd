// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
)

// ---- listexpiring ----

// FutureListExpiringResult is a future promise to deliver the result of a
// ListExpiringAsync RPC invocation (or an applicable error).
type FutureListExpiringResult chan *Response

// Receive waits for the Response promised by the future and returns the list
// of expiring UTXOs.
func (r FutureListExpiringResult) Receive() (*btcjson.ListExpiringResult, error) {
	res, err := ReceiveFuture(r)
	if err != nil {
		return nil, err
	}

	var result btcjson.ListExpiringResult
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListExpiringAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See ListExpiring for the blocking version and more details.
//
// NOTE: This is an OBTC extension.
func (c *Client) ListExpiringAsync(startHeight, endHeight *int32, maxResults *int, startAfter *string, minAmountSat ...*int64) FutureListExpiringResult {
	cmd := btcjson.NewListExpiringCmd(startHeight, endHeight, maxResults, startAfter, minAmountSat...)
	return c.SendCmd(cmd)
}

// ListExpiring returns UTXOs that will expire within the specified height range.
// Optional minAmountSat filters out UTXOs below the given value in satoshis.
//
// NOTE: This is an OBTC extension.
func (c *Client) ListExpiring(startHeight, endHeight *int32, maxResults *int, startAfter *string, minAmountSat ...*int64) (*btcjson.ListExpiringResult, error) {
	return c.ListExpiringAsync(startHeight, endHeight, maxResults, startAfter, minAmountSat...).Receive()
}

// ---- getexpiryindexstats ----

// FutureGetExpiryIndexStatsResult is a future promise to deliver the result of
// a GetExpiryIndexStatsAsync RPC invocation (or an applicable error).
type FutureGetExpiryIndexStatsResult chan *Response

// Receive waits for the Response promised by the future and returns the expiry
// index statistics.
func (r FutureGetExpiryIndexStatsResult) Receive() (*btcjson.ExpiryIndexStatsResult, error) {
	res, err := ReceiveFuture(r)
	if err != nil {
		return nil, err
	}

	var result btcjson.ExpiryIndexStatsResult
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExpiryIndexStatsAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetExpiryIndexStats for the blocking version and more details.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetExpiryIndexStatsAsync() FutureGetExpiryIndexStatsResult {
	cmd := btcjson.NewGetExpiryIndexStatsCmd()
	return c.SendCmd(cmd)
}

// GetExpiryIndexStats returns statistics about the OBTC expiry index.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetExpiryIndexStats() (*btcjson.ExpiryIndexStatsResult, error) {
	return c.GetExpiryIndexStatsAsync().Receive()
}

// ---- getreapplan ----

// FutureGetReapPlanResult is a future promise to deliver the result of a
// GetReapPlanAsync RPC invocation (or an applicable error).
type FutureGetReapPlanResult chan *Response

// Receive waits for the Response promised by the future and returns the REAP
// dry-run plan.
func (r FutureGetReapPlanResult) Receive() (*btcjson.GetReapPlanResult, error) {
	res, err := ReceiveFuture(r)
	if err != nil {
		return nil, err
	}

	var result btcjson.GetReapPlanResult
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetReapPlanAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetReapPlan for the blocking version and more details.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetReapPlanAsync() FutureGetReapPlanResult {
	cmd := btcjson.NewGetReapPlanCmd()
	return c.SendCmd(cmd)
}

// GetReapPlan returns a dry-run REAP plan for the next block.  The plan is
// computed without broadcasting or committing any transaction.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetReapPlan() (*btcjson.GetReapPlanResult, error) {
	return c.GetReapPlanAsync().Receive()
}

// ---- getexpirycommitment ----

// FutureGetExpiryCommitmentResult is a future promise to deliver the result of
// a GetExpiryCommitmentAsync RPC invocation (or an applicable error).
type FutureGetExpiryCommitmentResult chan *Response

// Receive waits for the Response promised by the future and returns the expiry
// commitment snapshot.
func (r FutureGetExpiryCommitmentResult) Receive() (*btcjson.GetExpiryCommitmentResult, error) {
	res, err := ReceiveFuture(r)
	if err != nil {
		return nil, err
	}

	var result btcjson.GetExpiryCommitmentResult
	if err := json.Unmarshal(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExpiryCommitmentAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetExpiryCommitment for the blocking version and more details.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetExpiryCommitmentAsync() FutureGetExpiryCommitmentResult {
	cmd := btcjson.NewGetExpiryCommitmentCmd()
	return c.SendCmd(cmd)
}

// GetExpiryCommitment returns the current expiry accumulator snapshot and
// commitment activation metadata from the expiry index.
//
// NOTE: This is an OBTC extension.
func (c *Client) GetExpiryCommitment() (*btcjson.GetExpiryCommitmentResult, error) {
	return c.GetExpiryCommitmentAsync().Receive()
}
