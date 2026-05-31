// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvaluateLabAlertsDetectsCriticalConditions(t *testing.T) {
	manifest := &labManifest{
		NoBlockWarningSeconds:  1200,
		NoBlockCriticalSeconds: 1800,
	}
	snapshot := &testnetLabSnapshot{
		Summary: labSummary{HeightSpread: 4},
		Nodes: []labNodeSnapshot{
			{
				Node:    labNode{Name: "seed-1"},
				Healthy: true,
				Snapshot: &statusSnapshot{
					Chain: chainStatus{
						Blocks:        120,
						TipAgeSeconds: 1900,
					},
					ExpiryIndex: expiryIndexStatus{
						Available: false,
					},
					ExpiryCommitment: expiryCommitmentStatus{
						Available: false,
					},
				},
			},
		},
		UserWallets: []labWalletSnapshot{
			{Wallet: labWallet{Name: "w-passive"}, Healthy: false, Error: "connection refused"},
		},
		Logs: []labLogSummary{
			{Log: labLog{Name: "obtcd"}, CriticalCount: 1},
		},
	}

	alerts := evaluateLabAlerts(snapshot, manifest)
	if len(alerts) < 5 {
		t.Fatalf("expected multiple alerts, got %+v", alerts)
	}
	if alerts[0].Severity != "critical" {
		t.Fatalf("expected critical alerts to sort first, got %+v", alerts[0])
	}
	if !hasLabAlert(alerts, "node_stale_tip") ||
		!hasLabAlert(alerts, "expiryindex_unavailable") ||
		!hasLabAlert(alerts, "wallet_unavailable") ||
		!hasLabAlert(alerts, "log_critical_pattern") {

		t.Fatalf("missing expected alerts: %+v", alerts)
	}
}

func TestEvaluateLabAlertsDetectsWalletSyncLag(t *testing.T) {
	snapshot := &testnetLabSnapshot{
		Summary: labSummary{BestHeight: 120},
		UserWallets: []labWalletSnapshot{
			{
				Wallet:  labWallet{Name: "w-passive"},
				Healthy: true,
				Blocks:  118,
			},
		},
	}

	alerts := evaluateLabAlerts(snapshot, &labManifest{})
	if !hasLabAlert(alerts, "wallet_sync_lag") {
		t.Fatalf("expected wallet_sync_lag alert, got %+v", alerts)
	}
}

func TestSummarizeLabLogDetectsPatterns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	if err := os.WriteFile(path, []byte("ok\nWARN slow peer\nERROR rpc failed\npanic: boom\n[INF] GRPC: loopyWriter exiting with error: transport closed by client\n[ERR] RPCS: Websocket receive error from 127.0.0.1:60676: websocket: close 1006 unexpected EOF\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	summary := summarizeLabLog(labLog{Name: "service", Path: path}, 10)
	if summary.WarningCount != 1 || summary.ErrorCount != 1 || summary.CriticalCount != 1 {
		t.Fatalf("unexpected log summary: %+v", summary)
	}
}

func TestSummarizeLabLogForModeDefersMissingOnDemandLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-fetched.log")
	summary := summarizeLabLogForMode(labLog{Name: "remote", Path: path}, 10, true)
	if !summary.Deferred || summary.Error != "" {
		t.Fatalf("expected missing on-demand log to be deferred: %+v", summary)
	}

	summary = summarizeLabLogForMode(labLog{Name: "remote", Path: path}, 10, false)
	if summary.Deferred || summary.Error == "" {
		t.Fatalf("expected missing eager log to remain unavailable: %+v", summary)
	}
}

func TestReadLabManifestDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lab.json")
	if err := os.WriteFile(path, []byte(`{"nodes":[]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, warnings, err := readLabManifest(path)
	if err != nil {
		t.Fatalf("readLabManifest: %v", err)
	}
	if manifest.Network != "obtctestnet" {
		t.Fatalf("expected default network, got %q", manifest.Network)
	}
	if manifest.BlockIntervalSeconds != defaultLabBlockIntervalSeconds {
		t.Fatalf("expected default interval, got %d", manifest.BlockIntervalSeconds)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing network")
	}
}

func TestTestnetLabCollectSnapshotParallelizesRPCResources(t *testing.T) {
	var activeRequests int32
	var maxActiveRequests int32
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active := atomic.AddInt32(&activeRequests, 1)
		defer atomic.AddInt32(&activeRequests, -1)
		for {
			maxActive := atomic.LoadInt32(&maxActiveRequests)
			if active <= maxActive || atomic.CompareAndSwapInt32(&maxActiveRequests, maxActive, active) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)

		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		result := labTestRPCResult(t, req.Method)
		writeFakeRPCResponse(t, w, req.ID, result)
	}))
	defer rpc.Close()

	rpcServer := strings.TrimPrefix(rpc.URL, "http://")
	manifestPath := filepath.Join(t.TempDir(), "lab.json")
	manifest := `{
		"network":"obtctestnet",
		"nodes":[
			{"name":"seed-1","role":"seed","rpc_server":"` + rpcServer + `"},
			{"name":"seed-2","role":"seed","rpc_server":"` + rpcServer + `"}
		],
		"user_wallets":[
			{"name":"w-passive","behavior":"hold","rpc_server":"` + rpcServer + `"},
			{"name":"w-renewall","behavior":"renew","rpc_server":"` + rpcServer + `"}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	server, err := newTestnetLabServer(&config{
		LabManifest:     manifestPath,
		Refresh:         time.Second,
		RPCTimeout:      2 * time.Second,
		LabLogTailLines: 10,
		NetworkName:     "obtctestnet",
		ObtcTestNet:     true,
	})
	if err != nil {
		t.Fatalf("newTestnetLabServer: %v", err)
	}

	snapshot, err := server.collectSnapshot(context.Background())
	if err != nil {
		t.Fatalf("collectSnapshot: %v", err)
	}
	if snapshot.Summary.HealthyNodes != 2 {
		t.Fatalf("expected two healthy nodes, got %+v", snapshot.Summary)
	}
	if len(snapshot.UserWallets) != 2 || !snapshot.UserWallets[0].Healthy || !snapshot.UserWallets[1].Healthy {
		t.Fatalf("expected two healthy wallets, got %+v", snapshot.UserWallets)
	}
	if atomic.LoadInt32(&maxActiveRequests) < 2 {
		t.Fatalf("expected concurrent RPC collection, max active requests was %d", maxActiveRequests)
	}
}

func TestResolveLabActionValidatesScriptArgs(t *testing.T) {
	label, args, err := resolveLabAction("renewall-dry-run", map[string][]string{
		"wallet": {"w-renewall"},
	})
	if err != nil {
		t.Fatalf("resolveLabAction: %v", err)
	}
	if label != "renewall-dry-run" || len(args) != 2 || args[1] != "w-renewall" {
		t.Fatalf("unexpected action: label=%s args=%v", label, args)
	}

	if _, _, err := resolveLabAction("renewall-dry-run", map[string][]string{
		"wallet": {"bad;wallet"},
	}); err == nil {
		t.Fatal("expected unsafe wallet token to fail")
	}

	label, args, err = resolveLabAction("fetch-logs", map[string][]string{
		"lines": {"250"},
	})
	if err != nil {
		t.Fatalf("resolve fetch-logs: %v", err)
	}
	if label != "fetch-logs" || len(args) != 2 || args[1] != "250" {
		t.Fatalf("unexpected fetch-logs action: label=%s args=%v", label, args)
	}

	if _, _, err := resolveLabAction("fetch-logs", map[string][]string{
		"lines": {"bad"},
	}); err == nil {
		t.Fatal("expected invalid fetch-logs lines to fail")
	}
}

func TestTestnetLabDetailRoutes(t *testing.T) {
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		var result interface{}
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]interface{}{
				"chain":                "obtctestnet",
				"blocks":               120,
				"headers":              120,
				"bestblockhash":        "0000000000000000000000000000000000000000000000000000000000000120",
				"difficulty":           1,
				"mediantime":           int64(1700000000),
				"verificationprogress": 1,
				"initialblockdownload": false,
			}
		case "getpeerinfo":
			result = []interface{}{}
		case "getmempoolinfo":
			result = map[string]interface{}{"size": 0, "bytes": 0}
		case "getchaintips":
			result = []map[string]interface{}{{
				"height":    120,
				"hash":      "0000000000000000000000000000000000000000000000000000000000000120",
				"branchlen": 0,
				"status":    "active",
			}}
		case "getblockheader":
			result = map[string]interface{}{
				"hash":          "0000000000000000000000000000000000000000000000000000000000000120",
				"height":        120,
				"confirmations": 1,
				"time":          time.Now().Unix(),
			}
		case "getexpiryindexstats":
			result = map[string]interface{}{
				"disabled":          false,
				"tip_height":        120,
				"total_utxos":       42,
				"total_expiry_keys": 7,
				"network_params": map[string]interface{}{
					"window_blocks":     144,
					"list_batch_limit":  5000,
					"start_scan_height": 0,
					"enable_at_height":  100,
				},
			}
		case "getexpirycommitment":
			result = map[string]interface{}{
				"enabled":               true,
				"root":                  "abc123",
				"tip_height":            120,
				"tip_hash":              "0000000000000000000000000000000000000000000000000000000000000120",
				"enable_at_height":      100,
				"active":                true,
				"active_at_next_height": true,
			}
		case "getreapplan":
			result = map[string]interface{}{
				"height":       121,
				"enabled":      true,
				"active":       true,
				"picked":       1,
				"tax_total":    10,
				"refund_total": 90,
				"est_weight":   500,
				"marker_hash":  "marker",
			}
		case "listexpiring":
			result = map[string]interface{}{
				"start_height":  120,
				"end_height":    264,
				"total_results": 1,
				"expiring_utxos": []map[string]interface{}{{
					"txid":             "00000000000000000000000000000000000000000000000000000000000000aa",
					"vout":             0,
					"expiry_height":    121,
					"create_height":    1,
					"blocks_to_expiry": 1,
					"amount_sat":       100,
				}},
			}
		case "getblockhash":
			result = "0000000000000000000000000000000000000000000000000000000000000120"
		case "getblock":
			result = map[string]interface{}{
				"hash":          "0000000000000000000000000000000000000000000000000000000000000120",
				"height":        120,
				"confirmations": 1,
				"time":          time.Now().Unix(),
				"rawtx": []map[string]interface{}{{
					"txid":   "reap-tx",
					"weight": 700,
					"vin": []map[string]interface{}{{
						"txid": "expired-utxo",
						"vout": 0,
					}},
					"vout": []map[string]interface{}{
						{
							"value": 0.00000090,
							"n":     0,
							"scriptPubKey": map[string]interface{}{
								"hex":  "0d6e6f742d726561702d64617461",
								"type": "nonstandard",
							},
						},
						{
							"value": 0,
							"n":     1,
							"scriptPubKey": map[string]interface{}{
								"hex":  "6a0e524541503a3132303a313a616263",
								"type": "nulldata",
							},
						},
					},
				}},
			}
		default:
			t.Fatalf("unexpected rpc method %q", req.Method)
		}
		writeFakeRPCResponse(t, w, req.ID, result)
	}))
	defer rpc.Close()

	manifestPath := filepath.Join(t.TempDir(), "lab.json")
	rpcServer := strings.TrimPrefix(rpc.URL, "http://")
	manifest := `{"network":"obtctestnet","nodes":[{"name":"seed-1","role":"seed","rpc_server":"` + rpcServer + `"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	server, err := newTestnetLabServer(&config{
		LabManifest: manifestPath,
		Refresh:     time.Second,
		RPCTimeout:  5 * time.Second,
		NetworkName: "obtctestnet",
		ObtcTestNet: true,
	})
	if err != nil {
		t.Fatalf("newTestnetLabServer: %v", err)
	}
	handler := server.routes()
	for path, want := range map[string]string{
		"/expiryindex?limit=2": "ExpiryIndex Detail",
		"/reap?count=1":        "Current REAP Plan",
		"/commitment":          "Expiry Commitment Detail",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d body=%s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("%s: missing %q in body", path, want)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/expiryindex.json?limit=2", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expiryindex json expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var detail labExpiryIndexDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode expiry detail: %v", err)
	}
	if detail.Returned != 1 || detail.PreviewPicked != 1 {
		t.Fatalf("unexpected expiry detail: %+v", detail)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/reap.json?count=1", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reap json expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var reapDetail labReapDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &reapDetail); err != nil {
		t.Fatalf("decode reap detail: %v", err)
	}
	if len(reapDetail.History) != 1 || len(reapDetail.History[0].ReapTxs) != 1 {
		t.Fatalf("expected rawtx REAP history, got %+v", reapDetail.History)
	}
	if got := reapDetail.History[0].ReapTxs[0].MarkerPayload; got != "REAP:120:1:abc" {
		t.Fatalf("unexpected marker payload %q", got)
	}
}

func labTestRPCResult(t *testing.T, method string) interface{} {
	t.Helper()
	switch method {
	case "getblockchaininfo":
		return map[string]interface{}{
			"chain":                "obtctestnet",
			"blocks":               120,
			"headers":              120,
			"bestblockhash":        "0000000000000000000000000000000000000000000000000000000000000120",
			"difficulty":           1,
			"mediantime":           time.Now().Unix(),
			"verificationprogress": 1,
			"initialblockdownload": false,
		}
	case "getpeerinfo":
		return []interface{}{}
	case "getmempoolinfo":
		return map[string]interface{}{"size": 0, "bytes": 0}
	case "getchaintips":
		return []map[string]interface{}{{
			"height":    120,
			"hash":      "0000000000000000000000000000000000000000000000000000000000000120",
			"branchlen": 0,
			"status":    "active",
		}}
	case "getblockheader":
		return map[string]interface{}{
			"hash":          "0000000000000000000000000000000000000000000000000000000000000120",
			"height":        120,
			"confirmations": 1,
			"time":          time.Now().Unix(),
		}
	case "getexpiryindexstats":
		return map[string]interface{}{
			"disabled":          false,
			"tip_height":        120,
			"total_utxos":       42,
			"total_expiry_keys": 7,
		}
	case "getexpirycommitment":
		return map[string]interface{}{
			"enabled":               true,
			"root":                  "abc123",
			"tip_height":            120,
			"tip_hash":              "0000000000000000000000000000000000000000000000000000000000000120",
			"enable_at_height":      100,
			"active":                true,
			"active_at_next_height": true,
		}
	case "getreapplan":
		return map[string]interface{}{
			"height":      121,
			"enabled":     true,
			"active":      true,
			"picked":      0,
			"est_weight":  0,
			"marker_hash": "marker",
		}
	case "getinfo":
		return map[string]interface{}{"blocks": 120}
	case "getblockcount":
		return 120
	case "getbalance":
		return 1.25
	case "obtc.getexpiry":
		return map[string]interface{}{
			"tip_height":    120,
			"window_blocks": 144,
			"items":         []interface{}{},
		}
	default:
		t.Fatalf("unexpected rpc method %q", method)
	}
	return nil
}

func writeFakeRPCResponse(t *testing.T, w http.ResponseWriter, id json.RawMessage, result interface{}) {
	t.Helper()
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if len(id) == 0 {
		id = []byte("null")
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"1.0","result":`))
	_, _ = w.Write(resultJSON)
	_, _ = w.Write([]byte(`,"error":null,"id":`))
	_, _ = w.Write(id)
	_, _ = w.Write([]byte(`}`))
}

func hasLabAlert(alerts []labAlert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}
