// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

type rpcCaller interface {
	Call(ctx context.Context, method string, params []interface{}, result interface{}) error
}

type jsonRPCCaller struct {
	endpoint   string
	user       string
	pass       string
	httpClient *http.Client
	useTLS     bool
}

func newJSONRPCCaller(cfg *config) (*jsonRPCCaller, error) {
	return newJSONRPCCallerForEndpoint(cfg, cfg.RPCServer)
}

func newJSONRPCCallerForEndpoint(cfg *config, endpoint string) (*jsonRPCCaller, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: cfg.RPCTimeout}).DialContext,
	}

	if !cfg.NoTLS {
		tlsCfg := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
		if !cfg.TLSSkipVerify && cfg.RPCCert != "" {
			pem, err := os.ReadFile(cfg.RPCCert)
			if err != nil {
				return nil, fmt.Errorf("unable to read RPC cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("unable to parse RPC cert PEM")
			}
			tlsCfg.RootCAs = pool
		}
		transport.TLSClientConfig = tlsCfg
	}

	return &jsonRPCCaller{
		endpoint: endpoint,
		user:     cfg.RPCUser,
		pass:     cfg.RPCPassword,
		useTLS:   !cfg.NoTLS,
		httpClient: &http.Client{
			Timeout:   cfg.RPCTimeout,
			Transport: transport,
		},
	}, nil
}

func (c *jsonRPCCaller) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	req, err := btcjson.NewRequest(btcjson.RpcVersion1, 1, method, params)
	if err != nil {
		return err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	scheme := "http"
	if c.useTLS {
		scheme = "https"
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, scheme+"://"+c.endpoint, bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(c.user, c.pass)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(respBody) == 0 {
			return fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("%s", respBody)
	}

	var rpcResp btcjson.Response
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if result == nil {
		return nil
	}

	return json.Unmarshal(rpcResp.Result, result)
}

type chainStatus struct {
	Name                 string  `json:"name"`
	Blocks               int32   `json:"blocks"`
	Headers              int32   `json:"headers"`
	HeaderLag            int32   `json:"header_lag"`
	BestBlockHash        string  `json:"best_block_hash"`
	Difficulty           float64 `json:"difficulty"`
	MedianTimeUnix       int64   `json:"median_time_unix"`
	TipTimeUnix          int64   `json:"tip_time_unix"`
	TipAgeSeconds        int64   `json:"tip_age_seconds"`
	InitialBlockDownload bool    `json:"initial_block_download"`
	VerificationProgress float64 `json:"verification_progress"`
}

type peerStatus struct {
	Count    int `json:"count"`
	Inbound  int `json:"inbound"`
	Outbound int `json:"outbound"`
	SyncPeer int `json:"sync_peer_count"`
}

type mempoolStatus struct {
	Transactions int64 `json:"transactions"`
	Bytes        int64 `json:"bytes"`
}

type tipStatus struct {
	ActiveHeight int32  `json:"active_height"`
	ActiveHash   string `json:"active_hash"`
	ForkCount    int    `json:"fork_count"`
}

type expiryIndexStatus struct {
	Available       bool   `json:"available"`
	Disabled        bool   `json:"disabled"`
	TipHeight       int32  `json:"tip_height"`
	TotalUTXOs      int    `json:"total_utxos"`
	TotalExpiryKeys int    `json:"total_expiry_keys"`
	Error           string `json:"error,omitempty"`
}

type expiryCommitmentStatus struct {
	Available          bool   `json:"available"`
	Enabled            bool   `json:"enabled"`
	Root               string `json:"root,omitempty"`
	TipHeight          int32  `json:"tip_height"`
	TipHash            string `json:"tip_hash,omitempty"`
	EnableAtHeight     int32  `json:"enable_at_height"`
	Active             bool   `json:"active"`
	ActiveAtNextHeight bool   `json:"active_at_next_height"`
	Error              string `json:"error,omitempty"`
}

type reapPlanStatus struct {
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
	Active      bool   `json:"active"`
	Height      int32  `json:"height"`
	Picked      int    `json:"picked"`
	TaxTotal    int64  `json:"tax_total"`
	RefundTotal int64  `json:"refund_total"`
	EstWeight   int64  `json:"est_weight"`
	MarkerHash  string `json:"marker_hash,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Error       string `json:"error,omitempty"`
}

type statusSnapshot struct {
	GeneratedAt      time.Time              `json:"generated_at"`
	RPCServer        string                 `json:"rpc_server"`
	Chain            chainStatus            `json:"chain"`
	Peers            peerStatus             `json:"peers"`
	Mempool          mempoolStatus          `json:"mempool"`
	Tips             tipStatus              `json:"tips"`
	ExpiryIndex      expiryIndexStatus      `json:"expiry_index"`
	ExpiryCommitment expiryCommitmentStatus `json:"expiry_commitment"`
	ReapPlan         reapPlanStatus         `json:"reap_plan"`
	Warnings         []string               `json:"warnings,omitempty"`
}

type statusCollector struct {
	rpc       rpcCaller
	rpcServer string
}

func (c *statusCollector) Snapshot(ctx context.Context) (*statusSnapshot, error) {
	var (
		chainInfo   btcjson.GetBlockChainInfoResult
		peerInfo    []btcjson.GetPeerInfoResult
		mempoolInfo btcjson.GetMempoolInfoResult
		chainTips   []*btcjson.GetChainTipsResult
	)

	if err := c.rpc.Call(ctx, "getblockchaininfo", nil, &chainInfo); err != nil {
		return nil, err
	}
	if err := c.rpc.Call(ctx, "getpeerinfo", nil, &peerInfo); err != nil {
		return nil, err
	}
	if err := c.rpc.Call(ctx, "getmempoolinfo", nil, &mempoolInfo); err != nil {
		return nil, err
	}
	if err := c.rpc.Call(ctx, "getchaintips", nil, &chainTips); err != nil {
		return nil, err
	}

	tipTimeUnix := chainInfo.MedianTime
	if chainInfo.BestBlockHash != "" {
		var header btcjson.GetBlockHeaderVerboseResult
		if err := c.rpc.Call(ctx, "getblockheader", []interface{}{
			chainInfo.BestBlockHash, true,
		}, &header); err == nil && header.Time > 0 {
			tipTimeUnix = header.Time
		}
	}

	snapshot := &statusSnapshot{
		GeneratedAt: time.Now().UTC(),
		RPCServer:   c.rpcServer,
		Chain: chainStatus{
			Name:                 chainInfo.Chain,
			Blocks:               chainInfo.Blocks,
			Headers:              chainInfo.Headers,
			HeaderLag:            chainInfo.Headers - chainInfo.Blocks,
			BestBlockHash:        chainInfo.BestBlockHash,
			Difficulty:           chainInfo.Difficulty,
			MedianTimeUnix:       chainInfo.MedianTime,
			TipTimeUnix:          tipTimeUnix,
			TipAgeSeconds:        maxInt64(0, time.Now().UTC().Unix()-tipTimeUnix),
			InitialBlockDownload: chainInfo.InitialBlockDownload,
			VerificationProgress: chainInfo.VerificationProgress,
		},
		Mempool: mempoolStatus{
			Transactions: mempoolInfo.Size,
			Bytes:        mempoolInfo.Bytes,
		},
		ExpiryIndex: expiryIndexStatus{
			Available: false,
		},
		ExpiryCommitment: expiryCommitmentStatus{
			Available: false,
		},
		ReapPlan: reapPlanStatus{
			Available: false,
		},
	}

	for _, peer := range peerInfo {
		snapshot.Peers.Count++
		if peer.Inbound {
			snapshot.Peers.Inbound++
		} else {
			snapshot.Peers.Outbound++
		}
		if peer.SyncNode {
			snapshot.Peers.SyncPeer++
		}
	}

	for _, tip := range chainTips {
		switch tip.Status {
		case "active":
			snapshot.Tips.ActiveHeight = tip.Height
			snapshot.Tips.ActiveHash = tip.Hash
		default:
			snapshot.Tips.ForkCount++
		}
	}

	c.fillOptionalSections(ctx, snapshot)
	sort.Strings(snapshot.Warnings)
	return snapshot, nil
}

func (c *statusCollector) fillOptionalSections(ctx context.Context, snapshot *statusSnapshot) {
	var expiryStats btcjson.ExpiryIndexStatsResult
	if err := c.rpc.Call(ctx, "getexpiryindexstats", nil, &expiryStats); err != nil {
		snapshot.ExpiryIndex.Error = err.Error()
		snapshot.Warnings = append(snapshot.Warnings, "getexpiryindexstats failed")
	} else {
		snapshot.ExpiryIndex.Available = true
		snapshot.ExpiryIndex.Disabled = expiryStats.Disabled
		snapshot.ExpiryIndex.TipHeight = expiryStats.TipHeight
		snapshot.ExpiryIndex.TotalUTXOs = expiryStats.TotalUTXOs
		snapshot.ExpiryIndex.TotalExpiryKeys = expiryStats.TotalExpiryKeys
	}

	var commitment btcjson.GetExpiryCommitmentResult
	if err := c.rpc.Call(ctx, "getexpirycommitment", nil, &commitment); err != nil {
		snapshot.ExpiryCommitment.Error = err.Error()
		snapshot.Warnings = append(snapshot.Warnings, "getexpirycommitment failed")
	} else {
		snapshot.ExpiryCommitment.Available = true
		snapshot.ExpiryCommitment.Enabled = commitment.Enabled
		snapshot.ExpiryCommitment.Root = commitment.Root
		snapshot.ExpiryCommitment.TipHeight = commitment.TipHeight
		snapshot.ExpiryCommitment.TipHash = commitment.TipHash
		snapshot.ExpiryCommitment.EnableAtHeight = commitment.EnableAtHeight
		snapshot.ExpiryCommitment.Active = commitment.Active
		snapshot.ExpiryCommitment.ActiveAtNextHeight = commitment.ActiveAtNextHeight
	}

	var reapPlan btcjson.GetReapPlanResult
	if err := c.rpc.Call(ctx, "getreapplan", nil, &reapPlan); err != nil {
		snapshot.ReapPlan.Error = err.Error()
		snapshot.Warnings = append(snapshot.Warnings, "getreapplan failed")
	} else {
		snapshot.ReapPlan.Available = true
		snapshot.ReapPlan.Enabled = reapPlan.Enabled
		snapshot.ReapPlan.Active = reapPlan.Active
		snapshot.ReapPlan.Height = reapPlan.Height
		snapshot.ReapPlan.Picked = reapPlan.Picked
		snapshot.ReapPlan.TaxTotal = reapPlan.TaxTotal
		snapshot.ReapPlan.RefundTotal = reapPlan.RefundTotal
		snapshot.ReapPlan.EstWeight = reapPlan.EstWeight
		snapshot.ReapPlan.MarkerHash = reapPlan.MarkerHash
		snapshot.ReapPlan.Reason = reapPlan.Reason
	}
}

type statusServer struct {
	collector *statusCollector
	refresh   time.Duration
	timeout   time.Duration
}

func (s *statusServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHTML)
	mux.HandleFunc("/status", s.handleJSON)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

func (s *statusServer) handleHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	snapshot, err := s.collector.Snapshot(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	view := struct {
		RefreshSeconds int
		Snapshot       *statusSnapshot
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Snapshot:       snapshot,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := statusTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *statusServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	snapshot, err := s.collector.Snapshot(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snapshot)
}

func (s *statusServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	if _, err := s.collector.Snapshot(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Status</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1f2937; }
    h1, h2 { margin-bottom: 0.4rem; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 1.5rem; }
    th, td { border: 1px solid #d1d5db; padding: 0.5rem; text-align: left; }
    th { background: #f3f4f6; }
    code { background: #f3f4f6; padding: 0.1rem 0.3rem; }
    .warn { color: #b45309; }
  </style>
</head>
<body>
  <h1>OBTC Status</h1>
  <p>Generated at <code>{{.Snapshot.GeneratedAt}}</code> from <code>{{.Snapshot.RPCServer}}</code>.</p>

  <h2>Chain</h2>
  <table>
    <tr><th>Network</th><td>{{.Snapshot.Chain.Name}}</td></tr>
    <tr><th>Blocks</th><td>{{.Snapshot.Chain.Blocks}}</td></tr>
    <tr><th>Headers</th><td>{{.Snapshot.Chain.Headers}}</td></tr>
    <tr><th>Header Lag</th><td>{{.Snapshot.Chain.HeaderLag}}</td></tr>
    <tr><th>Difficulty</th><td>{{printf "%.4f" .Snapshot.Chain.Difficulty}}</td></tr>
    <tr><th>IBD</th><td>{{.Snapshot.Chain.InitialBlockDownload}}</td></tr>
    <tr><th>Best Block</th><td><code>{{.Snapshot.Chain.BestBlockHash}}</code></td></tr>
  </table>

  <h2>Network</h2>
  <table>
    <tr><th>Peers</th><td>{{.Snapshot.Peers.Count}}</td></tr>
    <tr><th>Inbound</th><td>{{.Snapshot.Peers.Inbound}}</td></tr>
    <tr><th>Outbound</th><td>{{.Snapshot.Peers.Outbound}}</td></tr>
    <tr><th>Sync Peers</th><td>{{.Snapshot.Peers.SyncPeer}}</td></tr>
    <tr><th>Fork Tips</th><td>{{.Snapshot.Tips.ForkCount}}</td></tr>
    <tr><th>Active Tip</th><td>{{.Snapshot.Tips.ActiveHeight}} / <code>{{.Snapshot.Tips.ActiveHash}}</code></td></tr>
  </table>

  <h2>Mempool</h2>
  <table>
    <tr><th>Transactions</th><td>{{.Snapshot.Mempool.Transactions}}</td></tr>
    <tr><th>Bytes</th><td>{{.Snapshot.Mempool.Bytes}}</td></tr>
  </table>

  <h2>Expiry Index</h2>
  <table>
    <tr><th>Available</th><td>{{.Snapshot.ExpiryIndex.Available}}</td></tr>
    <tr><th>Disabled</th><td>{{.Snapshot.ExpiryIndex.Disabled}}</td></tr>
    <tr><th>Tip Height</th><td>{{.Snapshot.ExpiryIndex.TipHeight}}</td></tr>
    <tr><th>Total UTXOs</th><td>{{.Snapshot.ExpiryIndex.TotalUTXOs}}</td></tr>
    <tr><th>Total Expiry Keys</th><td>{{.Snapshot.ExpiryIndex.TotalExpiryKeys}}</td></tr>
    <tr><th>Error</th><td>{{.Snapshot.ExpiryIndex.Error}}</td></tr>
  </table>

  <h2>Expiry Commitment</h2>
  <table>
    <tr><th>Available</th><td>{{.Snapshot.ExpiryCommitment.Available}}</td></tr>
    <tr><th>Enabled</th><td>{{.Snapshot.ExpiryCommitment.Enabled}}</td></tr>
    <tr><th>Tip Height</th><td>{{.Snapshot.ExpiryCommitment.TipHeight}}</td></tr>
    <tr><th>Active</th><td>{{.Snapshot.ExpiryCommitment.Active}}</td></tr>
    <tr><th>Active At Next Height</th><td>{{.Snapshot.ExpiryCommitment.ActiveAtNextHeight}}</td></tr>
    <tr><th>Root</th><td><code>{{.Snapshot.ExpiryCommitment.Root}}</code></td></tr>
    <tr><th>Error</th><td>{{.Snapshot.ExpiryCommitment.Error}}</td></tr>
  </table>

  <h2>REAP Plan</h2>
  <table>
    <tr><th>Available</th><td>{{.Snapshot.ReapPlan.Available}}</td></tr>
    <tr><th>Enabled</th><td>{{.Snapshot.ReapPlan.Enabled}}</td></tr>
    <tr><th>Active</th><td>{{.Snapshot.ReapPlan.Active}}</td></tr>
    <tr><th>Next Height</th><td>{{.Snapshot.ReapPlan.Height}}</td></tr>
    <tr><th>Picked</th><td>{{.Snapshot.ReapPlan.Picked}}</td></tr>
    <tr><th>Tax Total</th><td>{{.Snapshot.ReapPlan.TaxTotal}}</td></tr>
    <tr><th>Refund Total</th><td>{{.Snapshot.ReapPlan.RefundTotal}}</td></tr>
    <tr><th>Estimated Weight</th><td>{{.Snapshot.ReapPlan.EstWeight}}</td></tr>
    <tr><th>Marker Hash</th><td><code>{{.Snapshot.ReapPlan.MarkerHash}}</code></td></tr>
    <tr><th>Reason</th><td>{{.Snapshot.ReapPlan.Reason}}</td></tr>
    <tr><th>Error</th><td>{{.Snapshot.ReapPlan.Error}}</td></tr>
  </table>

  {{if .Snapshot.Warnings}}
  <h2>Warnings</h2>
  <ul>
    {{range .Snapshot.Warnings}}<li class="warn">{{.}}</li>{{end}}
  </ul>
  {{end}}
</body>
</html>`))
