// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

type fakeRPCCaller struct {
	results map[string]interface{}
	errs    map[string]error
}

func (f *fakeRPCCaller) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err, ok := f.errs[method]; ok {
		return err
	}

	raw, err := json.Marshal(f.results[method])
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}

func TestStatusCollectorSnapshot(t *testing.T) {
	tips := []*btcjson.GetChainTipsResult{
		{Height: 120, Hash: "active-hash", Status: "active"},
		{Height: 119, Hash: "fork-hash", Status: "valid-fork"},
	}
	fake := &fakeRPCCaller{
		results: map[string]interface{}{
			"getblockchaininfo": btcjson.GetBlockChainInfoResult{
				Chain:                "obtctestnet",
				Blocks:               120,
				Headers:              123,
				BestBlockHash:        "best",
				Difficulty:           1.5,
				MedianTime:           1000,
				InitialBlockDownload: false,
				VerificationProgress: 0.99,
			},
			"getblockheader": btcjson.GetBlockHeaderVerboseResult{
				Hash: "best",
				Time: 2000,
			},
			"getpeerinfo": []btcjson.GetPeerInfoResult{
				{Inbound: true},
				{Inbound: false, SyncNode: true},
				{Inbound: false},
			},
			"getmempoolinfo": btcjson.GetMempoolInfoResult{
				Size:  7,
				Bytes: 4096,
			},
			"getchaintips": tips,
			"getexpiryindexstats": btcjson.ExpiryIndexStatsResult{
				Disabled:        false,
				TipHeight:       120,
				TotalUTXOs:      42,
				TotalExpiryKeys: 9,
			},
			"getexpirycommitment": btcjson.GetExpiryCommitmentResult{
				Enabled:            true,
				Root:               "root",
				TipHeight:          120,
				TipHash:            "tip",
				EnableAtHeight:     100,
				Active:             true,
				ActiveAtNextHeight: true,
			},
			"getreapplan": btcjson.GetReapPlanResult{
				Enabled:     true,
				Active:      true,
				Height:      121,
				Picked:      3,
				TaxTotal:    1000,
				RefundTotal: 2000,
				EstWeight:   800,
				MarkerHash:  "marker",
			},
		},
	}

	collector := &statusCollector{rpc: fake, rpcServer: "127.0.0.1:19528"}
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}

	if snapshot.Chain.HeaderLag != 3 {
		t.Fatalf("expected header lag 3, got %d", snapshot.Chain.HeaderLag)
	}
	if snapshot.Chain.MedianTimeUnix != 1000 || snapshot.Chain.TipTimeUnix != 2000 {
		t.Fatalf("unexpected chain time summary: %+v", snapshot.Chain)
	}
	if snapshot.Peers.Count != 3 || snapshot.Peers.Inbound != 1 ||
		snapshot.Peers.Outbound != 2 || snapshot.Peers.SyncPeer != 1 {
		t.Fatalf("unexpected peer summary: %+v", snapshot.Peers)
	}
	if snapshot.Tips.ActiveHeight != 120 || snapshot.Tips.ForkCount != 1 {
		t.Fatalf("unexpected tips summary: %+v", snapshot.Tips)
	}
	if !snapshot.ExpiryIndex.Available || snapshot.ExpiryIndex.TotalUTXOs != 42 {
		t.Fatalf("unexpected expiry index status: %+v", snapshot.ExpiryIndex)
	}
	if !snapshot.ExpiryCommitment.Available || snapshot.ExpiryCommitment.Root != "root" {
		t.Fatalf("unexpected expiry commitment status: %+v", snapshot.ExpiryCommitment)
	}
	if !snapshot.ReapPlan.Available || snapshot.ReapPlan.Picked != 3 {
		t.Fatalf("unexpected reap plan status: %+v", snapshot.ReapPlan)
	}
}

func TestStatusCollectorOptionalWarnings(t *testing.T) {
	fake := &fakeRPCCaller{
		results: map[string]interface{}{
			"getblockchaininfo": btcjson.GetBlockChainInfoResult{Chain: "obtcmainnet", Blocks: 1, Headers: 1},
			"getpeerinfo":       []btcjson.GetPeerInfoResult{},
			"getmempoolinfo":    btcjson.GetMempoolInfoResult{},
			"getchaintips":      []*btcjson.GetChainTipsResult{{Height: 1, Hash: "h", Status: "active"}},
		},
		errs: map[string]error{
			"getexpiryindexstats": errors.New("disabled"),
			"getexpirycommitment": errors.New("disabled"),
			"getreapplan":         errors.New("disabled"),
		},
	}

	collector := &statusCollector{rpc: fake, rpcServer: "127.0.0.1:9528"}
	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapshot.Warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(snapshot.Warnings))
	}
	if snapshot.ExpiryIndex.Error == "" || snapshot.ReapPlan.Error == "" {
		t.Fatalf("expected optional RPC errors to be recorded")
	}
}

func TestStatusServerHandlers(t *testing.T) {
	fake := &fakeRPCCaller{
		results: map[string]interface{}{
			"getblockchaininfo":   btcjson.GetBlockChainInfoResult{Chain: "obtcmainnet", Blocks: 1, Headers: 1, BestBlockHash: "best"},
			"getpeerinfo":         []btcjson.GetPeerInfoResult{},
			"getmempoolinfo":      btcjson.GetMempoolInfoResult{},
			"getchaintips":        []*btcjson.GetChainTipsResult{{Height: 1, Hash: "best", Status: "active"}},
			"getexpiryindexstats": btcjson.ExpiryIndexStatsResult{Disabled: true},
			"getexpirycommitment": btcjson.GetExpiryCommitmentResult{Enabled: false},
			"getreapplan":         btcjson.GetReapPlanResult{Enabled: false},
		},
	}

	server := &statusServer{
		collector: &statusCollector{rpc: fake, rpcServer: "127.0.0.1:9528"},
		refresh:   5 * time.Second,
		timeout:   2 * time.Second,
	}

	jsonReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	jsonRec := httptest.NewRecorder()
	server.routes().ServeHTTP(jsonRec, jsonReq)
	if jsonRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", jsonRec.Code)
	}
	if !strings.Contains(jsonRec.Body.String(), "\"rpc_server\": \"127.0.0.1:9528\"") {
		t.Fatalf("unexpected json body: %s", jsonRec.Body.String())
	}

	htmlReq := httptest.NewRequest(http.MethodGet, "/", nil)
	htmlRec := httptest.NewRecorder()
	server.routes().ServeHTTP(htmlRec, htmlReq)
	if htmlRec.Code != http.StatusOK {
		t.Fatalf("expected html status 200, got %d", htmlRec.Code)
	}
	if !strings.Contains(htmlRec.Body.String(), "OBTC Status") {
		t.Fatalf("unexpected html body: %s", htmlRec.Body.String())
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	server.routes().ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected healthz 200, got %d", healthRec.Code)
	}
}
