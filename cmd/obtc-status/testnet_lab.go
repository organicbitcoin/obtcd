// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/mining/reap"
)

const (
	defaultLabBlockIntervalSeconds = 600
	defaultLabNoBlockWarning       = 20 * time.Minute
	defaultLabNoBlockCritical      = 30 * time.Minute
)

type labManifest struct {
	GeneratedAt            time.Time   `json:"generated_at,omitempty"`
	Network                string      `json:"network"`
	BlockIntervalSeconds   int         `json:"block_interval_seconds,omitempty"`
	NoBlockWarningSeconds  int         `json:"no_block_warning_seconds,omitempty"`
	NoBlockCriticalSeconds int         `json:"no_block_critical_seconds,omitempty"`
	Nodes                  []labNode   `json:"nodes"`
	MinerWallets           []labWallet `json:"miner_wallets"`
	UserWallets            []labWallet `json:"user_wallets"`
	Logs                   []labLog    `json:"logs"`
}

type labNode struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	RPCServer  string `json:"rpc_server"`
	RPCUser    string `json:"rpc_user,omitempty"`
	RPCPass    string `json:"rpc_pass,omitempty"`
	RPCUserEnv string `json:"rpc_user_env,omitempty"`
	RPCPassEnv string `json:"rpc_pass_env,omitempty"`
	UseTLS     bool   `json:"use_tls,omitempty"`
	LogPath    string `json:"log_path,omitempty"`
}

type labWallet struct {
	Name           string `json:"name"`
	Behavior       string `json:"behavior"`
	RPCServer      string `json:"rpc_server"`
	RPCUser        string `json:"rpc_user,omitempty"`
	RPCPass        string `json:"rpc_pass,omitempty"`
	RPCUserEnv     string `json:"rpc_user_env,omitempty"`
	RPCPassEnv     string `json:"rpc_pass_env,omitempty"`
	AgentRPCServer string `json:"agent_rpc_server,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
}

type labLog struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Path string `json:"path"`
}

type labNodeSnapshot struct {
	Node     labNode         `json:"node"`
	Healthy  bool            `json:"healthy"`
	Snapshot *statusSnapshot `json:"snapshot,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type labWalletSnapshot struct {
	Wallet          labWallet `json:"wallet"`
	Healthy         bool      `json:"healthy"`
	Error           string    `json:"error,omitempty"`
	Balance         float64   `json:"balance,omitempty"`
	Blocks          int32     `json:"blocks,omitempty"`
	ExpiryTipHeight int32     `json:"expiry_tip_height,omitempty"`
	ExpiryWindow    uint64    `json:"expiry_window,omitempty"`
	ExpiryItems     int       `json:"expiry_items"`
	ExpiringItems   int       `json:"expiring_items"`
	ExpiredItems    int       `json:"expired_items"`
	Warnings        []string  `json:"warnings,omitempty"`
}

type labLogSummary struct {
	Log           labLog `json:"log"`
	Deferred      bool   `json:"deferred,omitempty"`
	ScannedLines  int    `json:"scanned_lines"`
	WarningCount  int    `json:"warning_count"`
	ErrorCount    int    `json:"error_count"`
	CriticalCount int    `json:"critical_count"`
	LastWarning   string `json:"last_warning,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Error         string `json:"error,omitempty"`
}

type labSummary struct {
	ConfiguredNodes int   `json:"configured_nodes"`
	HealthyNodes    int   `json:"healthy_nodes"`
	BestHeight      int32 `json:"best_height"`
	HeightSpread    int32 `json:"height_spread"`
	MinerWallets    int   `json:"miner_wallets"`
	UserWallets     int   `json:"user_wallets"`
	CriticalAlerts  int   `json:"critical_alerts"`
	WarningAlerts   int   `json:"warning_alerts"`
}

type labAlert struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
}

type labExpiryIndexDetail struct {
	Node                   labNode                        `json:"node"`
	Chain                  chainStatus                    `json:"chain"`
	Stats                  btcjson.ExpiryIndexStatsResult `json:"stats"`
	ReapPlan               reapPlanStatus                 `json:"reap_plan"`
	StartHeight            int32                          `json:"start_height"`
	EndHeight              int32                          `json:"end_height"`
	Limit                  int                            `json:"limit"`
	Returned               int                            `json:"returned"`
	TotalResults           int                            `json:"total_results"`
	NextCursor             string                         `json:"next_cursor,omitempty"`
	PreviewPicked          int                            `json:"preview_picked"`
	PreviewMaxInputs       int                            `json:"preview_max_inputs"`
	PreviewWeightBudget    int64                          `json:"preview_weight_budget"`
	ScanOrderDescription   string                         `json:"scan_order_description"`
	StrictOrderDescription string                         `json:"strict_order_description"`
	Rows                   []*labExpiryIndexRow           `json:"rows"`
	StrictRows             []*labExpiryIndexRow           `json:"strict_rows"`
	NetworkParams          *btcjson.ExpiryParamsResult    `json:"network_params,omitempty"`
}

type labExpiryIndexRow struct {
	ScanRank        int    `json:"scan_rank"`
	StrictRank      int    `json:"strict_rank"`
	Picked          bool   `json:"picked"`
	EligibleNow     bool   `json:"eligible_now"`
	OutPoint        string `json:"outpoint"`
	TxID            string `json:"txid"`
	Vout            uint32 `json:"vout"`
	AmountSat       int64  `json:"amount_sat"`
	ExpiryHeight    uint64 `json:"expiry_height"`
	CreateHeight    uint64 `json:"create_height"`
	BlocksToExpiry  int64  `json:"blocks_to_expiry"`
	SelectorHashHex string `json:"selector_hash_hex"`
}

type labReapDetail struct {
	Node           labNode        `json:"node"`
	Chain          chainStatus    `json:"chain"`
	Plan           reapPlanStatus `json:"plan"`
	HistoryCount   int            `json:"history_count"`
	HistoryScanned int            `json:"history_scanned"`
	History        []labReapBlock `json:"history"`
}

type labReapBlock struct {
	Height  int64       `json:"height"`
	Hash    string      `json:"hash"`
	Time    int64       `json:"time"`
	ReapTxs []labReapTx `json:"reap_txs"`
}

type labReapTx struct {
	TxID                 string `json:"txid"`
	Inputs               int    `json:"inputs"`
	Outputs              int    `json:"outputs"`
	Weight               int32  `json:"weight"`
	MarkerPayload        string `json:"marker_payload"`
	RefundOutputTotalSat int64  `json:"refund_output_total_sat"`
}

type labCommitmentDetail struct {
	Node                 labNode                `json:"node"`
	Chain                chainStatus            `json:"chain"`
	ExpiryIndex          expiryIndexStatus      `json:"expiry_index"`
	ExpiryCommitment     expiryCommitmentStatus `json:"expiry_commitment"`
	ReapPlan             reapPlanStatus         `json:"reap_plan"`
	NextHeight           int32                  `json:"next_height"`
	ExpiryIndexLag       int32                  `json:"expiry_index_lag"`
	CommitmentLag        int32                  `json:"commitment_lag"`
	CommitmentTipMatches bool                   `json:"commitment_tip_matches"`
}

type testnetLabSnapshot struct {
	GeneratedAt  time.Time           `json:"generated_at"`
	Network      string              `json:"network"`
	Summary      labSummary          `json:"summary"`
	Nodes        []labNodeSnapshot   `json:"nodes"`
	MinerWallets []labWalletSnapshot `json:"miner_wallets"`
	UserWallets  []labWalletSnapshot `json:"user_wallets"`
	Logs         []labLogSummary     `json:"logs"`
	Alerts       []labAlert          `json:"alerts,omitempty"`
	ManifestPath string              `json:"manifest_path"`
	LastAction   *devnetActionResult `json:"last_action,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

type testnetLabServer struct {
	cfg             *config
	manifestPath    string
	scriptPath      string
	refresh         time.Duration
	timeout         time.Duration
	actionTimeout   time.Duration
	logTailLines    int
	lastActionMutex sync.Mutex
	lastAction      *devnetActionResult
}

func newTestnetLabServer(cfg *config) (*testnetLabServer, error) {
	if _, err := os.Stat(cfg.LabManifest); err != nil {
		return nil, fmt.Errorf("lab manifest unavailable: %w", err)
	}
	return &testnetLabServer{
		cfg:           cfg,
		manifestPath:  cfg.LabManifest,
		scriptPath:    cfg.LabScript,
		refresh:       cfg.Refresh,
		timeout:       cfg.RPCTimeout,
		actionTimeout: cfg.LabActionTimeout,
		logTailLines:  cfg.LabLogTailLines,
	}, nil
}

func (s *testnetLabServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHTML)
	mux.HandleFunc("/status", s.handleJSON)
	mux.HandleFunc("/alerts", s.handleAlerts)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/expiryindex", s.handleLabExpiryIndexHTML)
	mux.HandleFunc("/expiryindex.json", s.handleLabExpiryIndexJSON)
	mux.HandleFunc("/reap", s.handleLabReapHTML)
	mux.HandleFunc("/reap.json", s.handleLabReapJSON)
	mux.HandleFunc("/commitment", s.handleLabCommitmentHTML)
	mux.HandleFunc("/commitment.json", s.handleLabCommitmentJSON)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/action", s.handleAction)
	return mux
}

func (s *testnetLabServer) handleHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	snapshot, err := s.snapshotWithTimeout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	view := struct {
		RefreshSeconds int
		Snapshot       *testnetLabSnapshot
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Snapshot:       snapshot,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := testnetLabTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *testnetLabServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.snapshotWithTimeout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, snapshot)
}

func (s *testnetLabServer) handleAlerts(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.snapshotWithTimeout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, snapshot.Alerts)
}

func (s *testnetLabServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.collectLogSummaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, logs)
}

func (s *testnetLabServer) handleLabExpiryIndexHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/expiryindex" {
		http.NotFound(w, r)
		return
	}
	manifest, warnings, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	view := struct {
		RefreshSeconds int
		Nodes          []labNode
		SelectedNode   string
		StartValue     string
		EndValue       string
		LimitValue     string
		Result         *labExpiryIndexDetail
		Warnings       []string
		Error          string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node")),
		StartValue:     strings.TrimSpace(r.URL.Query().Get("start")),
		EndValue:       strings.TrimSpace(r.URL.Query().Get("end")),
		LimitValue:     strings.TrimSpace(r.URL.Query().Get("limit")),
		Warnings:       warnings,
	}

	if view.SelectedNode == "" {
		view.Error = "no lab nodes configured"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
		defer cancel()
		node, err := findLabNode(manifest.Nodes, view.SelectedNode)
		if err != nil {
			view.Error = err.Error()
		} else if result, err := s.loadLabExpiryIndexDetail(ctx, node, view.StartValue, view.EndValue, view.LimitValue); err != nil {
			view.Error = err.Error()
		} else {
			view.Result = result
			if view.StartValue == "" {
				view.StartValue = strconv.FormatInt(int64(result.StartHeight), 10)
			}
			if view.EndValue == "" {
				view.EndValue = strconv.FormatInt(int64(result.EndHeight), 10)
			}
			if view.LimitValue == "" {
				view.LimitValue = strconv.Itoa(result.Limit)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := testnetLabExpiryIndexTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *testnetLabServer) handleLabExpiryIndexJSON(w http.ResponseWriter, r *http.Request) {
	manifest, _, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	selectedNode := selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node"))
	node, err := findLabNode(manifest.Nodes, selectedNode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
	defer cancel()
	result, err := s.loadLabExpiryIndexDetail(
		ctx, node,
		strings.TrimSpace(r.URL.Query().Get("start")),
		strings.TrimSpace(r.URL.Query().Get("end")),
		strings.TrimSpace(r.URL.Query().Get("limit")),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, result)
}

func (s *testnetLabServer) handleLabReapHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/reap" {
		http.NotFound(w, r)
		return
	}
	manifest, warnings, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	countValue := strings.TrimSpace(r.URL.Query().Get("count"))
	if countValue == "" {
		countValue = "144"
	}
	view := struct {
		RefreshSeconds int
		Nodes          []labNode
		SelectedNode   string
		CountValue     string
		Result         *labReapDetail
		Warnings       []string
		Error          string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node")),
		CountValue:     countValue,
		Warnings:       warnings,
	}

	if view.SelectedNode == "" {
		view.Error = "no lab nodes configured"
	} else {
		count, countErr := parseReapHistoryCount(countValue)
		if countErr != nil {
			view.Error = countErr.Error()
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
			defer cancel()
			node, err := findLabNode(manifest.Nodes, view.SelectedNode)
			if err != nil {
				view.Error = err.Error()
			} else if result, err := s.loadLabReapDetail(ctx, node, count); err != nil {
				view.Error = err.Error()
			} else {
				view.Result = result
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := testnetLabReapTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *testnetLabServer) handleLabReapJSON(w http.ResponseWriter, r *http.Request) {
	manifest, _, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	selectedNode := selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node"))
	node, err := findLabNode(manifest.Nodes, selectedNode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	countValue := strings.TrimSpace(r.URL.Query().Get("count"))
	if countValue == "" {
		countValue = "144"
	}
	count, err := parseReapHistoryCount(countValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
	defer cancel()
	result, err := s.loadLabReapDetail(ctx, node, count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, result)
}

func (s *testnetLabServer) handleLabCommitmentHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/commitment" {
		http.NotFound(w, r)
		return
	}
	manifest, warnings, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	view := struct {
		RefreshSeconds int
		Nodes          []labNode
		SelectedNode   string
		Result         *labCommitmentDetail
		Warnings       []string
		Error          string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node")),
		Warnings:       warnings,
	}

	if view.SelectedNode == "" {
		view.Error = "no lab nodes configured"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
		defer cancel()
		node, err := findLabNode(manifest.Nodes, view.SelectedNode)
		if err != nil {
			view.Error = err.Error()
		} else if result, err := s.loadLabCommitmentDetail(ctx, node); err != nil {
			view.Error = err.Error()
		} else {
			view.Result = result
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := testnetLabCommitmentTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *testnetLabServer) handleLabCommitmentJSON(w http.ResponseWriter, r *http.Request) {
	manifest, _, err := readLabManifest(s.manifestPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	selectedNode := selectedLabNodeName(manifest.Nodes, r.URL.Query().Get("node"))
	node, err := findLabNode(manifest.Nodes, selectedNode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
	defer cancel()
	result, err := s.loadLabCommitmentDetail(ctx, node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, result)
}

func (s *testnetLabServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if _, _, err := readLabManifest(s.manifestPath); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func (s *testnetLabServer) handleAction(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "testnet lab actions are only allowed from loopback clients\n", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed\n", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	action := strings.TrimSpace(r.Form.Get("action"))
	label, args, err := resolveLabAction(action, r.Form)
	var result devnetActionResult
	if err != nil {
		result = devnetActionResult{
			At:     time.Now().UTC(),
			Action: action,
			Error:  err.Error(),
		}
	} else if label == "refresh" {
		result = devnetActionResult{
			At:      time.Now().UTC(),
			Action:  label,
			Success: true,
			Output:  "snapshot refreshed",
		}
	} else {
		result = s.runLabScriptAction(r.Context(), label, args)
	}
	s.setLastAction(&result)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *testnetLabServer) runLabScriptAction(ctx context.Context, label string, args []string) devnetActionResult {
	result := devnetActionResult{
		At:     time.Now().UTC(),
		Action: label,
		Args:   append([]string(nil), args...),
	}
	if _, err := os.Stat(s.scriptPath); err != nil {
		result.Error = fmt.Sprintf("lab script unavailable: %v", err)
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, s.actionTimeout)
	defer cancel()

	cmdArgs := append([]string{s.scriptPath}, args...)
	cmd := exec.CommandContext(ctx, "bash", cmdArgs...)
	cmd.Env = append(os.Environ(),
		"OBTC_LAB_MANIFEST="+s.manifestPath,
		"OBTC_NETWORK="+s.cfg.NetworkName,
	)
	output, err := cmd.CombinedOutput()
	result.Output = trimActionOutput(string(output))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Success = true
	return result
}

func resolveLabAction(action string, form map[string][]string) (string, []string, error) {
	switch action {
	case "refresh":
		return "refresh", nil, nil
	case "health-check":
		return "health-check", []string{"health-check"}, nil
	case "capture-evidence":
		return "capture-evidence", []string{"capture-evidence"}, nil
	case "fetch-logs":
		lines := strings.TrimSpace(firstFormValue(form, "lines"))
		if lines == "" {
			lines = "200"
		}
		if !isSafeLabNumber(lines) {
			return "", nil, fmt.Errorf("invalid log line count")
		}
		return "fetch-logs", []string{"fetch-logs", lines}, nil
	case "renewall-dry-run":
		wallet := strings.TrimSpace(firstFormValue(form, "wallet"))
		if !isSafeLabToken(wallet) {
			return "", nil, fmt.Errorf("invalid wallet")
		}
		return "renewall-dry-run", []string{"renewall-dry-run", wallet}, nil
	case "traffic-sim":
		mode := strings.TrimSpace(firstFormValue(form, "mode"))
		count := strings.TrimSpace(firstFormValue(form, "count"))
		if !isSafeLabToken(mode) || !isSafeLabNumber(count) {
			return "", nil, fmt.Errorf("invalid traffic-sim parameters")
		}
		return "traffic-sim", []string{"traffic-sim", mode, count}, nil
	default:
		return "", nil, fmt.Errorf("unsupported lab action")
	}
}

func isSafeLabToken(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func isSafeLabNumber(s string) bool {
	if s == "" || len(s) > 9 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *testnetLabServer) snapshotWithTimeout(ctx context.Context) (*testnetLabSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.collectSnapshot(ctx)
}

func (s *testnetLabServer) collectSnapshot(ctx context.Context) (*testnetLabSnapshot, error) {
	manifest, warnings, err := readLabManifest(s.manifestPath)
	if err != nil {
		return nil, err
	}

	snapshot := &testnetLabSnapshot{
		GeneratedAt:  time.Now().UTC(),
		Network:      manifest.Network,
		ManifestPath: s.manifestPath,
		Warnings:     warnings,
		LastAction:   s.getLastAction(),
		Summary: labSummary{
			ConfiguredNodes: len(manifest.Nodes),
			MinerWallets:    len(manifest.MinerWallets),
			UserWallets:     len(manifest.UserWallets),
		},
	}

	snapshot.Nodes = s.collectNodes(ctx, manifest.Nodes)
	snapshot.Summary.HealthyNodes, snapshot.Summary.BestHeight,
		snapshot.Summary.HeightSpread = labNodeHeightSummary(snapshot.Nodes)
	snapshot.MinerWallets = s.collectWallets(ctx, manifest.MinerWallets)
	snapshot.UserWallets = s.collectWallets(ctx, manifest.UserWallets)
	for _, logSpec := range manifest.Logs {
		snapshot.Logs = append(snapshot.Logs, summarizeLabLogForMode(
			logSpec, s.logTailLines, s.deferMissingLogs()))
	}

	snapshot.Alerts = evaluateLabAlerts(snapshot, manifest)
	for _, alert := range snapshot.Alerts {
		switch alert.Severity {
		case "critical":
			snapshot.Summary.CriticalAlerts++
		case "warning":
			snapshot.Summary.WarningAlerts++
		}
	}
	sortLabSnapshot(snapshot)
	return snapshot, nil
}

func (s *testnetLabServer) collectNodes(ctx context.Context, nodes []labNode) []labNodeSnapshot {
	snapshots := make([]labNodeSnapshot, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func(i int, node labNode) {
			defer wg.Done()
			nodeCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			snapshots[i] = s.collectNode(nodeCtx, node)
		}(i, node)
	}
	wg.Wait()
	return snapshots
}

func labNodeHeightSummary(nodes []labNodeSnapshot) (healthy int, bestHeight, heightSpread int32) {
	var minHeight int32
	for _, node := range nodes {
		if !node.Healthy || node.Snapshot == nil {
			continue
		}
		height := node.Snapshot.Chain.Blocks
		if healthy == 0 || height < minHeight {
			minHeight = height
		}
		if healthy == 0 || height > bestHeight {
			bestHeight = height
		}
		healthy++
	}
	if healthy > 0 {
		heightSpread = bestHeight - minHeight
	}
	return healthy, bestHeight, heightSpread
}

func (s *testnetLabServer) collectWallets(ctx context.Context, wallets []labWallet) []labWalletSnapshot {
	snapshots := make([]labWalletSnapshot, len(wallets))
	var wg sync.WaitGroup
	for i, wallet := range wallets {
		wg.Add(1)
		go func(i int, wallet labWallet) {
			defer wg.Done()
			walletCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			snapshots[i] = s.collectWallet(walletCtx, wallet)
		}(i, wallet)
	}
	wg.Wait()
	return snapshots
}

func (s *testnetLabServer) collectLogSummaries() ([]labLogSummary, error) {
	manifest, _, err := readLabManifest(s.manifestPath)
	if err != nil {
		return nil, err
	}
	logs := make([]labLogSummary, 0, len(manifest.Logs))
	for _, logSpec := range manifest.Logs {
		logs = append(logs, summarizeLabLogForMode(
			logSpec, s.logTailLines, s.deferMissingLogs()))
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Log.Name < logs[j].Log.Name
	})
	return logs, nil
}

func (s *testnetLabServer) collectNode(ctx context.Context, node labNode) labNodeSnapshot {
	user := resolveSecret(node.RPCUser, node.RPCUserEnv)
	pass := resolveSecret(node.RPCPass, node.RPCPassEnv)
	caller := newRawJSONRPCCaller(node.RPCServer, user, pass, s.timeout, node.UseTLS)
	collector := &statusCollector{rpc: caller, rpcServer: node.RPCServer}
	status, err := collector.Snapshot(ctx)
	if err != nil {
		return labNodeSnapshot{Node: node, Error: err.Error()}
	}
	return labNodeSnapshot{Node: node, Healthy: true, Snapshot: status}
}

func (s *testnetLabServer) collectWallet(ctx context.Context, wallet labWallet) labWalletSnapshot {
	user := resolveSecret(wallet.RPCUser, wallet.RPCUserEnv)
	pass := resolveSecret(wallet.RPCPass, wallet.RPCPassEnv)
	caller := newRawJSONRPCCaller(wallet.RPCServer, user, pass, s.timeout, false)
	snapshot := labWalletSnapshot{Wallet: wallet}

	var info map[string]interface{}
	if err := caller.Call(ctx, "getinfo", nil, &info); err != nil {
		snapshot.Error = err.Error()
		return snapshot
	}
	snapshot.Healthy = true
	var blocks int32
	if err := caller.Call(ctx, "getblockcount", nil, &blocks); err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "getblockcount failed: "+err.Error())
	} else {
		snapshot.Blocks = blocks
	}

	var balance float64
	if err := caller.Call(ctx, "getbalance", nil, &balance); err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "getbalance failed: "+err.Error())
	} else {
		snapshot.Balance = balance
	}

	var expiry struct {
		TipHeight    int32  `json:"tip_height"`
		WindowBlocks uint64 `json:"window_blocks"`
		Items        []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	params := []interface{}{100}
	if err := caller.Call(ctx, "obtc.getexpiry", params, &expiry); err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "obtc.getexpiry failed: "+err.Error())
	} else {
		snapshot.ExpiryTipHeight = expiry.TipHeight
		snapshot.ExpiryWindow = expiry.WindowBlocks
		snapshot.ExpiryItems = len(expiry.Items)
		for _, item := range expiry.Items {
			switch strings.ToLower(item.Status) {
			case "expiring":
				snapshot.ExpiringItems++
			case "expired":
				snapshot.ExpiredItems++
			}
		}
	}

	sort.Strings(snapshot.Warnings)
	return snapshot
}

func (s *testnetLabServer) loadLabExpiryIndexDetail(ctx context.Context, node labNode, startRaw, endRaw, limitRaw string) (*labExpiryIndexDetail, error) {
	caller := s.labNodeCaller(node)
	status, err := (&statusCollector{rpc: caller, rpcServer: node.RPCServer}).Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	var stats btcjson.ExpiryIndexStatsResult
	if err := caller.Call(ctx, "getexpiryindexstats", nil, &stats); err != nil {
		return nil, err
	}

	defaultStart := status.Chain.Blocks
	defaultEnd := defaultStart
	if stats.NetworkParams != nil && stats.NetworkParams.WindowBlocks > 0 {
		window := stats.NetworkParams.WindowBlocks
		if window > uint64(^uint32(0)>>1) {
			window = uint64(^uint32(0) >> 1)
		}
		defaultEnd += int32(window)
	}

	startHeight, err := parseExpiryHeight(startRaw, defaultStart)
	if err != nil {
		return nil, err
	}
	endHeight, err := parseExpiryHeight(endRaw, defaultEnd)
	if err != nil {
		return nil, err
	}
	if endHeight < startHeight {
		return nil, fmt.Errorf("end height must be greater than or equal to start height")
	}
	limit, err := parseExpiryLimit(limitRaw)
	if err != nil {
		return nil, err
	}

	var result btcjson.ListExpiringResult
	if err := caller.Call(ctx, "listexpiring", []interface{}{
		startHeight, endHeight, limit,
	}, &result); err != nil {
		return nil, err
	}

	rows := make([]*labExpiryIndexRow, 0, len(result.ExpiringUTXOs))
	for idx, utxo := range result.ExpiringUTXOs {
		outpoint := fmt.Sprintf("%s:%d", utxo.TxID, utxo.Vout)
		selectorHash := utxo.TxID
		if op, err := parseResultOutPoint(utxo.TxID, utxo.Vout); err == nil {
			outpoint = formatOutPoint(op)
			selectorHash = hex.EncodeToString(op.Hash[:])
		}
		rows = append(rows, &labExpiryIndexRow{
			ScanRank:        idx + 1,
			OutPoint:        outpoint,
			TxID:            utxo.TxID,
			Vout:            utxo.Vout,
			AmountSat:       utxo.AmountSat,
			ExpiryHeight:    utxo.ExpiryHeight,
			CreateHeight:    utxo.CreateHeight,
			BlocksToExpiry:  utxo.BlocksToExpiry,
			SelectorHashHex: selectorHash,
		})
	}

	strictRows := append([]*labExpiryIndexRow(nil), rows...)
	sort.Slice(strictRows, func(i, j int) bool {
		a := strictRows[i]
		b := strictRows[j]
		if a.ExpiryHeight != b.ExpiryHeight {
			return a.ExpiryHeight < b.ExpiryHeight
		}
		if a.AmountSat != b.AmountSat {
			return a.AmountSat < b.AmountSat
		}
		if a.SelectorHashHex != b.SelectorHashHex {
			return a.SelectorHashHex < b.SelectorHashHex
		}
		return a.Vout < b.Vout
	})

	params, err := diagnosticsNetworkParams(s.cfg)
	if err != nil {
		return nil, err
	}
	reapParams := reap.DefaultREAPParamsForNet(params, reap.SortModeStrict)
	nextHeight := int64(status.Chain.Blocks) + 1
	picked := 0
	for idx, row := range strictRows {
		row.StrictRank = idx + 1
		row.EligibleNow = int64(row.ExpiryHeight) <= nextHeight
		if !row.EligibleNow {
			continue
		}
		nextWeight := reap.EstimateBlueprintWeight(picked + 1)
		if picked < reapParams.MaxInputs && (reapParams.WeightBudget <= 0 || nextWeight <= reapParams.WeightBudget) {
			row.Picked = true
			picked++
		}
	}

	detail := &labExpiryIndexDetail{
		Node:                   node,
		Chain:                  status.Chain,
		Stats:                  stats,
		ReapPlan:               status.ReapPlan,
		StartHeight:            result.StartHeight,
		EndHeight:              result.EndHeight,
		Limit:                  limit,
		Returned:               len(rows),
		TotalResults:           result.TotalResults,
		PreviewPicked:          picked,
		PreviewMaxInputs:       reapParams.MaxInputs,
		PreviewWeightBudget:    reapParams.WeightBudget,
		ScanOrderDescription:   "expiry_height -> canonical txid string -> vout",
		StrictOrderDescription: "expiry_height -> amount_sat -> raw hash bytes -> vout",
		Rows:                   rows,
		StrictRows:             strictRows,
		NetworkParams:          stats.NetworkParams,
	}
	if result.NextHeight != nil && result.NextOutpoint != nil {
		detail.NextCursor = fmt.Sprintf("next_height=%d, start_after=%s", *result.NextHeight, *result.NextOutpoint)
	}
	return detail, nil
}

func (s *testnetLabServer) loadLabReapDetail(ctx context.Context, node labNode, count int) (*labReapDetail, error) {
	caller := s.labNodeCaller(node)
	status, err := (&statusCollector{rpc: caller, rpcServer: node.RPCServer}).Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	bestHeight := int64(status.Chain.Blocks)
	startHeight := bestHeight - int64(count) + 1
	if startHeight < 0 {
		startHeight = 0
	}

	detail := &labReapDetail{
		Node:           node,
		Chain:          status.Chain,
		Plan:           status.ReapPlan,
		HistoryCount:   count,
		HistoryScanned: int(bestHeight - startHeight + 1),
		History:        []labReapBlock{},
	}
	for height := bestHeight; height >= startHeight; height-- {
		var hash string
		if err := caller.Call(ctx, "getblockhash", []interface{}{height}, &hash); err != nil {
			return nil, err
		}
		var block btcjson.GetBlockVerboseTxResult
		if err := caller.Call(ctx, "getblock", []interface{}{hash, 2}, &block); err != nil {
			return nil, err
		}
		reapTxs := labReapTxsFromBlock(labBlockRawTxs(block))
		if len(reapTxs) == 0 {
			continue
		}
		detail.History = append(detail.History, labReapBlock{
			Height:  block.Height,
			Hash:    block.Hash,
			Time:    block.Time,
			ReapTxs: reapTxs,
		})
	}
	return detail, nil
}

func labBlockRawTxs(block btcjson.GetBlockVerboseTxResult) []btcjson.TxRawResult {
	if len(block.RawTx) > 0 {
		return block.RawTx
	}
	return block.Tx
}

func (s *testnetLabServer) loadLabCommitmentDetail(ctx context.Context, node labNode) (*labCommitmentDetail, error) {
	caller := s.labNodeCaller(node)
	status, err := (&statusCollector{rpc: caller, rpcServer: node.RPCServer}).Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	detail := &labCommitmentDetail{
		Node:             node,
		Chain:            status.Chain,
		ExpiryIndex:      status.ExpiryIndex,
		ExpiryCommitment: status.ExpiryCommitment,
		ReapPlan:         status.ReapPlan,
		NextHeight:       status.Chain.Blocks + 1,
		CommitmentTipMatches: status.ExpiryCommitment.TipHash != "" &&
			status.ExpiryCommitment.TipHash == status.Chain.BestBlockHash,
	}
	if status.ExpiryIndex.Available {
		detail.ExpiryIndexLag = status.Chain.Blocks - status.ExpiryIndex.TipHeight
	}
	if status.ExpiryCommitment.Available {
		detail.CommitmentLag = status.Chain.Blocks - status.ExpiryCommitment.TipHeight
	}
	return detail, nil
}

func labReapTxsFromBlock(txs []btcjson.TxRawResult) []labReapTx {
	var results []labReapTx
	for _, tx := range txs {
		payload, markerIndex, ok := labReapMarkerPayload(tx)
		if !ok || !strings.HasPrefix(payload, "REAP:") {
			continue
		}
		refundTotal := int64(0)
		for idx, out := range tx.Vout {
			if idx == markerIndex {
				continue
			}
			refundTotal += btcToSat(out.Value)
		}
		inputs := 0
		for _, in := range tx.Vin {
			if !in.IsCoinBase() {
				inputs++
			}
		}
		results = append(results, labReapTx{
			TxID:                 tx.Txid,
			Inputs:               inputs,
			Outputs:              len(tx.Vout),
			Weight:               tx.Weight,
			MarkerPayload:        payload,
			RefundOutputTotalSat: refundTotal,
		})
	}
	return results
}

func labReapMarkerPayload(tx btcjson.TxRawResult) (string, int, bool) {
	for idx, out := range tx.Vout {
		rawScript, err := hex.DecodeString(out.ScriptPubKey.Hex)
		if err != nil {
			continue
		}
		payload, ok := reap.ExtractMarkerPayload(rawScript)
		if ok {
			return payload, idx, true
		}
	}
	return "", -1, false
}

func btcToSat(value float64) int64 {
	return int64(math.Round(value * 1e8))
}

func (s *testnetLabServer) labNodeCaller(node labNode) *jsonRPCCaller {
	user := resolveSecret(node.RPCUser, node.RPCUserEnv)
	pass := resolveSecret(node.RPCPass, node.RPCPassEnv)
	return newRawJSONRPCCaller(node.RPCServer, user, pass, s.timeout, node.UseTLS)
}

func selectedLabNodeName(nodes []labNode, raw string) string {
	selected := strings.TrimSpace(raw)
	if selected != "" {
		return selected
	}
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0].Name
}

func findLabNode(nodes []labNode, name string) (labNode, error) {
	if strings.TrimSpace(name) == "" {
		return labNode{}, fmt.Errorf("node is required")
	}
	for _, node := range nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return labNode{}, fmt.Errorf("node %q not found", name)
}

func readLabManifest(path string) (*labManifest, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var manifest labManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, err
	}
	var warnings []string
	if manifest.Network == "" {
		manifest.Network = "obtctestnet"
		warnings = append(warnings, "manifest network missing; defaulting to obtctestnet")
	}
	if manifest.BlockIntervalSeconds <= 0 {
		manifest.BlockIntervalSeconds = defaultLabBlockIntervalSeconds
	}
	if manifest.NoBlockWarningSeconds <= 0 {
		manifest.NoBlockWarningSeconds = int(defaultLabNoBlockWarning.Seconds())
	}
	if manifest.NoBlockCriticalSeconds <= 0 {
		manifest.NoBlockCriticalSeconds = int(defaultLabNoBlockCritical.Seconds())
	}
	return &manifest, warnings, nil
}

func evaluateLabAlerts(snapshot *testnetLabSnapshot, manifest *labManifest) []labAlert {
	var alerts []labAlert
	warnNoBlock := int64(manifest.NoBlockWarningSeconds)
	critNoBlock := int64(manifest.NoBlockCriticalSeconds)
	if warnNoBlock <= 0 {
		warnNoBlock = int64(defaultLabNoBlockWarning.Seconds())
	}
	if critNoBlock <= 0 {
		critNoBlock = int64(defaultLabNoBlockCritical.Seconds())
	}

	for _, node := range snapshot.Nodes {
		name := node.Node.Name
		if name == "" {
			name = node.Node.RPCServer
		}
		if !node.Healthy || node.Snapshot == nil {
			alerts = append(alerts, criticalAlert("node_unavailable", name, node.Error))
			continue
		}
		status := node.Snapshot
		if status.Chain.TipAgeSeconds > critNoBlock {
			alerts = append(alerts, criticalAlert("node_stale_tip", name,
				fmt.Sprintf("tip age %ds exceeds %ds", status.Chain.TipAgeSeconds, critNoBlock)))
		} else if status.Chain.TipAgeSeconds > warnNoBlock {
			alerts = append(alerts, warningAlert("node_slow_blocks", name,
				fmt.Sprintf("tip age %ds exceeds %ds", status.Chain.TipAgeSeconds, warnNoBlock)))
		}
		if status.Chain.Blocks >= 100 {
			if !status.ExpiryIndex.Available || status.ExpiryIndex.Disabled {
				alerts = append(alerts, criticalAlert("expiryindex_unavailable", name,
					"expiryindex is unavailable or disabled after activation"))
			} else {
				lag := status.Chain.Blocks - status.ExpiryIndex.TipHeight
				if lag > 3 {
					alerts = append(alerts, criticalAlert("expiryindex_lag", name,
						fmt.Sprintf("expiryindex lags chain tip by %d blocks", lag)))
				} else if lag > 1 {
					alerts = append(alerts, warningAlert("expiryindex_lag", name,
						fmt.Sprintf("expiryindex lags chain tip by %d blocks", lag)))
				}
			}
			if !status.ExpiryCommitment.Available || !status.ExpiryCommitment.Active {
				alerts = append(alerts, criticalAlert("expiry_commitment_inactive", name,
					"expiry commitment is unavailable or inactive after activation"))
			}
		}
		if status.ReapPlan.Available && status.ReapPlan.Active && status.ReapPlan.Picked > 0 {
			alerts = append(alerts, warningAlert("reap_candidates_pending", name,
				fmt.Sprintf("REAP plan picked %d candidates for height %d",
					status.ReapPlan.Picked, status.ReapPlan.Height)))
		}
	}

	if snapshot.Summary.HeightSpread > 3 {
		alerts = append(alerts, criticalAlert("height_spread", "nodes",
			fmt.Sprintf("node height spread is %d blocks", snapshot.Summary.HeightSpread)))
	} else if snapshot.Summary.HeightSpread > 1 {
		alerts = append(alerts, warningAlert("height_spread", "nodes",
			fmt.Sprintf("node height spread is %d blocks", snapshot.Summary.HeightSpread)))
	}

	for _, wallet := range append(append([]labWalletSnapshot{}, snapshot.MinerWallets...), snapshot.UserWallets...) {
		name := wallet.Wallet.Name
		if name == "" {
			name = wallet.Wallet.RPCServer
		}
		if !wallet.Healthy {
			alerts = append(alerts, criticalAlert("wallet_unavailable", name, wallet.Error))
			continue
		}
		if lag := snapshot.Summary.BestHeight - wallet.Blocks; lag > 0 {
			alerts = append(alerts, warningAlert("wallet_sync_lag", name,
				fmt.Sprintf("wallet processed height lags node tip by %d blocks", lag)))
		}
		for _, warning := range wallet.Warnings {
			alerts = append(alerts, warningAlert("wallet_warning", name, warning))
		}
	}

	for _, logSummary := range snapshot.Logs {
		name := logSummary.Log.Name
		if name == "" {
			name = logSummary.Log.Path
		}
		if logSummary.Error != "" {
			alerts = append(alerts, warningAlert("log_unavailable", name, logSummary.Error))
			continue
		}
		if logSummary.CriticalCount > 0 {
			alerts = append(alerts, criticalAlert("log_critical_pattern", name,
				fmt.Sprintf("%d critical log lines detected", logSummary.CriticalCount)))
		} else if logSummary.ErrorCount > 0 {
			alerts = append(alerts, warningAlert("log_error_pattern", name,
				fmt.Sprintf("%d error log lines detected", logSummary.ErrorCount)))
		}
	}

	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity == "critical"
		}
		if alerts[i].Subject != alerts[j].Subject {
			return alerts[i].Subject < alerts[j].Subject
		}
		return alerts[i].Code < alerts[j].Code
	})
	return alerts
}

func criticalAlert(code, subject, message string) labAlert {
	return labAlert{Severity: "critical", Code: code, Subject: subject, Message: message}
}

func warningAlert(code, subject, message string) labAlert {
	return labAlert{Severity: "warning", Code: code, Subject: subject, Message: message}
}

func summarizeLabLog(spec labLog, maxLines int) labLogSummary {
	summary := labLogSummary{Log: spec}
	lines, err := readRecentLogLines(spec.Path, maxLines)
	if err != nil {
		summary.Error = err.Error()
		return summary
	}
	summary.ScannedLines = len(lines)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if isBenignLabLogLine(lower) {
			continue
		}
		switch {
		case strings.Contains(lower, "panic") ||
			strings.Contains(lower, "fatal") ||
			strings.Contains(lower, "corrupt"):
			summary.CriticalCount++
			summary.LastError = line
		case strings.Contains(lower, "error") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "auth failure"):
			summary.ErrorCount++
			summary.LastError = line
		case strings.Contains(lower, "warn"):
			summary.WarningCount++
			summary.LastWarning = line
		}
	}
	return summary
}

func summarizeLabLogForMode(spec labLog, maxLines int, deferMissing bool) labLogSummary {
	summary := summarizeLabLog(spec, maxLines)
	if !deferMissing || summary.Error == "" {
		return summary
	}
	if _, err := os.Stat(spec.Path); errors.Is(err, os.ErrNotExist) {
		summary.Error = ""
		summary.Deferred = true
	}
	return summary
}

func (s *testnetLabServer) deferMissingLogs() bool {
	return strings.TrimSpace(os.Getenv("OBTC_LAB_LOG_FETCH_SPEC")) != ""
}

func isBenignLabLogLine(lower string) bool {
	return strings.Contains(lower, "transport closed by client") ||
		(strings.Contains(lower, "websocket receive error") &&
			strings.Contains(lower, "unexpected eof"))
}

func readRecentLogLines(path string, maxLines int) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("empty log path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return tailFileLines(path, maxLines)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    filepath.Join(path, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	var lines []string
	for _, file := range files {
		fileLines, err := tailFileLines(file.path, maxLines-len(lines))
		if err != nil {
			continue
		}
		lines = append(lines, fileLines...)
		if len(lines) >= maxLines {
			break
		}
	}
	return lines, nil
}

func tailFileLines(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > maxLines {
			copy(lines, lines[len(lines)-maxLines:])
			lines = lines[:maxLines]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func resolveSecret(value, envName string) string {
	if envName == "" {
		return value
	}
	if envValue := os.Getenv(envName); envValue != "" {
		return envValue
	}
	return value
}

func newRawJSONRPCCaller(endpoint, user, pass string, timeout time.Duration, useTLS bool) *jsonRPCCaller {
	return &jsonRPCCaller{
		endpoint: endpoint,
		user:     user,
		pass:     pass,
		useTLS:   useTLS,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: timeout}).DialContext,
			},
		},
	}
}

func numberAsFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func writeIndentedJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func sortLabSnapshot(snapshot *testnetLabSnapshot) {
	sort.Slice(snapshot.Nodes, func(i, j int) bool {
		return snapshot.Nodes[i].Node.Name < snapshot.Nodes[j].Node.Name
	})
	sort.Slice(snapshot.MinerWallets, func(i, j int) bool {
		return snapshot.MinerWallets[i].Wallet.Name < snapshot.MinerWallets[j].Wallet.Name
	})
	sort.Slice(snapshot.UserWallets, func(i, j int) bool {
		return snapshot.UserWallets[i].Wallet.Name < snapshot.UserWallets[j].Wallet.Name
	})
	sort.Slice(snapshot.Logs, func(i, j int) bool {
		return snapshot.Logs[i].Log.Name < snapshot.Logs[j].Log.Name
	})
	sort.Strings(snapshot.Warnings)
}

func (s *testnetLabServer) setLastAction(result *devnetActionResult) {
	s.lastActionMutex.Lock()
	defer s.lastActionMutex.Unlock()
	s.lastAction = result
}

func (s *testnetLabServer) getLastAction() *devnetActionResult {
	s.lastActionMutex.Lock()
	defer s.lastActionMutex.Unlock()
	if s.lastAction == nil {
		return nil
	}
	cp := *s.lastAction
	cp.Args = append([]string(nil), s.lastAction.Args...)
	return &cp
}

var testnetLabTemplate = template.Must(template.New("testnet-lab").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Testnet Lab</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1f2937; }
    h1, h2 { margin-bottom: 0.4rem; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 1.5rem; }
    th, td { border: 1px solid #d1d5db; padding: 0.5rem; text-align: left; vertical-align: top; }
    th { background: #f3f4f6; }
    code { background: #f3f4f6; padding: 0.1rem 0.3rem; }
    .critical { color: #b91c1c; font-weight: 700; }
    .warning { color: #b45309; }
    .ok { color: #047857; }
    .toolbar { display: flex; gap: 0.5rem; margin: 1rem 0; }
  </style>
</head>
<body>
  <h1>OBTC Testnet Lab</h1>
  <p>Generated at <code>{{.Snapshot.GeneratedAt}}</code> from <code>{{.Snapshot.ManifestPath}}</code>.</p>
  <div class="toolbar">
    <a href="/status">JSON</a>
    <a href="/alerts">Alerts</a>
    <a href="/logs">Logs</a>
    <a href="/expiryindex">ExpiryIndex</a>
    <a href="/reap">REAP</a>
    <a href="/commitment">Commitment</a>
    <form method="post" action="/action"><input type="hidden" name="action" value="refresh"><button type="submit">Refresh</button></form>
    <form method="post" action="/action"><input type="hidden" name="action" value="fetch-logs"><input type="hidden" name="lines" value="200"><button type="submit">Fetch Logs</button></form>
  </div>

  <h2>Summary</h2>
  <table>
    <tr><th>Network</th><td>{{.Snapshot.Network}}</td></tr>
    <tr><th>Healthy Nodes</th><td>{{.Snapshot.Summary.HealthyNodes}} / {{.Snapshot.Summary.ConfiguredNodes}}</td></tr>
    <tr><th>Best Height</th><td>{{.Snapshot.Summary.BestHeight}}</td></tr>
    <tr><th>Height Spread</th><td>{{.Snapshot.Summary.HeightSpread}}</td></tr>
    <tr><th>Miner Wallets</th><td>{{.Snapshot.Summary.MinerWallets}}</td></tr>
    <tr><th>User Wallets</th><td>{{.Snapshot.Summary.UserWallets}}</td></tr>
    <tr><th>Critical Alerts</th><td class="{{if gt .Snapshot.Summary.CriticalAlerts 0}}critical{{else}}ok{{end}}">{{.Snapshot.Summary.CriticalAlerts}}</td></tr>
    <tr><th>Warning Alerts</th><td class="{{if gt .Snapshot.Summary.WarningAlerts 0}}warning{{else}}ok{{end}}">{{.Snapshot.Summary.WarningAlerts}}</td></tr>
  </table>

  <h2>Alerts</h2>
  {{if .Snapshot.Alerts}}
  <table>
    <tr><th>Severity</th><th>Code</th><th>Subject</th><th>Message</th></tr>
    {{range .Snapshot.Alerts}}
    <tr><td class="{{.Severity}}">{{.Severity}}</td><td>{{.Code}}</td><td>{{.Subject}}</td><td>{{.Message}}</td></tr>
    {{end}}
  </table>
  {{else}}<p class="ok">No active alerts.</p>{{end}}

  <h2>Nodes</h2>
  <table>
    <tr><th>Name</th><th>Role</th><th>Healthy</th><th>Height</th><th>Peers</th><th>ExpiryIndex</th><th>Commitment</th><th>REAP Picked</th><th>Details</th><th>Error</th></tr>
    {{range .Snapshot.Nodes}}
    <tr>
      <td>{{.Node.Name}}</td><td>{{.Node.Role}}</td><td>{{.Healthy}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.Chain.Blocks}}{{end}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.Peers.Count}}{{end}}</td>
      <td>{{if .Snapshot}}{{if .Snapshot.ExpiryIndex.Available}}{{if .Snapshot.ExpiryIndex.Disabled}}disabled{{else}}active tip {{.Snapshot.ExpiryIndex.TipHeight}}{{end}}{{else}}n/a{{end}}{{end}}</td>
      <td>{{if .Snapshot}}{{if .Snapshot.ExpiryCommitment.Available}}{{if .Snapshot.ExpiryCommitment.Active}}active{{else}}inactive{{end}} tip {{.Snapshot.ExpiryCommitment.TipHeight}}{{else}}n/a{{end}}{{end}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.ReapPlan.Picked}}{{end}}</td>
      <td><a href="/expiryindex?node={{.Node.Name}}">expiry</a> <a href="/reap?node={{.Node.Name}}">reap</a> <a href="/commitment?node={{.Node.Name}}">commitment</a></td>
      <td>{{.Error}}</td>
    </tr>
    {{end}}
  </table>

  <h2>Miner Wallets</h2>
  {{template "wallet-table" .Snapshot.MinerWallets}}

  <h2>User Wallets</h2>
  {{template "wallet-table" .Snapshot.UserWallets}}

  <h2>Log Summary</h2>
  <table>
    <tr><th>Name</th><th>Role</th><th>Lines</th><th>Warnings</th><th>Errors</th><th>Critical</th><th>Last Error</th><th>Error</th></tr>
    {{range .Snapshot.Logs}}
    <tr>
      <td>{{.Log.Name}}</td><td>{{.Log.Role}}</td><td>{{.ScannedLines}}</td>
      <td>{{.WarningCount}}</td><td>{{.ErrorCount}}</td><td>{{.CriticalCount}}</td>
      <td>{{.LastError}}</td><td>{{.Error}}</td>
    </tr>
    {{end}}
  </table>
</body>
</html>
{{define "wallet-table"}}
<table>
  <tr><th>Name</th><th>Behavior</th><th>Healthy</th><th>Blocks</th><th>Balance</th><th>Expiry Items</th><th>Expiring</th><th>Expired</th><th>Error / Warnings</th></tr>
  {{range .}}
  <tr>
    <td>{{.Wallet.Name}}</td><td>{{.Wallet.Behavior}}</td><td>{{.Healthy}}</td><td>{{.Blocks}}</td>
    <td>{{printf "%.8f" .Balance}}</td><td>{{.ExpiryItems}}</td><td>{{.ExpiringItems}}</td><td>{{.ExpiredItems}}</td>
    <td>{{.Error}}{{range .Warnings}}<div class="warning">{{.}}</div>{{end}}</td>
  </tr>
{{end}}
</table>
{{end}}`))

var testnetLabExpiryIndexTemplate = template.Must(template.New("testnet-lab-expiryindex").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Testnet Lab ExpiryIndex</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1f2937; }
    h1, h2 { margin-bottom: 0.4rem; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 1.5rem; }
    th, td { border: 1px solid #d1d5db; padding: 0.45rem; text-align: left; vertical-align: top; }
    th { background: #f3f4f6; }
    code { background: #f3f4f6; padding: 0.1rem 0.3rem; word-break: break-all; }
    input, select, button { padding: 0.35rem; }
    .critical { color: #b91c1c; font-weight: 700; }
    .warning { color: #b45309; }
    .ok { color: #047857; }
    .toolbar, form { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin: 1rem 0; }
  </style>
</head>
<body>
  <h1>ExpiryIndex Detail</h1>
  <div class="toolbar">
    <a href="/">Overview</a>
    <a href="/expiryindex.json?node={{.SelectedNode}}&start={{.StartValue}}&end={{.EndValue}}&limit={{.LimitValue}}">JSON</a>
    <a href="/reap?node={{.SelectedNode}}">REAP</a>
    <a href="/commitment?node={{.SelectedNode}}">Commitment</a>
  </div>
  <form method="get" action="/expiryindex">
    <label>Node <select name="node">{{range .Nodes}}<option value="{{.Name}}" {{if eq .Name $.SelectedNode}}selected{{end}}>{{.Name}}</option>{{end}}</select></label>
    <label>Start <input name="start" value="{{.StartValue}}" size="8"></label>
    <label>End <input name="end" value="{{.EndValue}}" size="8"></label>
    <label>Limit <input name="limit" value="{{.LimitValue}}" size="6"></label>
    <button type="submit">Refresh</button>
  </form>
  {{range .Warnings}}<p class="warning">{{.}}</p>{{end}}
  {{if .Error}}<p class="critical">{{.Error}}</p>{{end}}
  {{with .Result}}
  <h2>Index State</h2>
  <table>
    <tr><th>Node</th><td>{{.Node.Name}}</td><th>Height</th><td>{{.Chain.Blocks}}</td></tr>
    <tr><th>Disabled</th><td>{{.Stats.Disabled}}</td><th>Tip Height</th><td>{{.Stats.TipHeight}}</td></tr>
    <tr><th>Total UTXOs</th><td>{{.Stats.TotalUTXOs}}</td><th>Total Expiry Keys</th><td>{{.Stats.TotalExpiryKeys}}</td></tr>
    {{with .NetworkParams}}<tr><th>Window Blocks</th><td>{{.WindowBlocks}}</td><th>Batch Limit</th><td>{{.ListBatchLimit}}</td></tr>
    <tr><th>Start Scan Height</th><td>{{.StartScanHeight}}</td><th>Enable At Height</th><td>{{.EnableAtHeight}}</td></tr>{{end}}
    <tr><th>Range</th><td>{{.StartHeight}} - {{.EndHeight}}</td><th>Returned / Total</th><td>{{.Returned}} / {{.TotalResults}}</td></tr>
    <tr><th>Next Cursor</th><td colspan="3">{{.NextCursor}}</td></tr>
  </table>

  <h2>REAP Strict Preview</h2>
  <table>
    <tr><th>Next Block</th><td>{{.ReapPlan.Height}}</td><th>Plan Picked</th><td>{{.ReapPlan.Picked}}</td></tr>
    <tr><th>Preview Picked</th><td>{{.PreviewPicked}}</td><th>Max Inputs</th><td>{{.PreviewMaxInputs}}</td></tr>
    <tr><th>Weight Budget</th><td>{{.PreviewWeightBudget}}</td><th>Strict Order</th><td>{{.StrictOrderDescription}}</td></tr>
  </table>

  <h2>ExpiryIndex / RPC Order</h2>
  {{template "expiry-table" .Rows}}
  <h2>REAP Strict Order</h2>
  {{template "expiry-table" .StrictRows}}
  {{end}}
</body>
</html>
{{define "expiry-table"}}
<table>
  <tr><th>Scan</th><th>Strict</th><th>Picked</th><th>Eligible Now</th><th>Outpoint</th><th>Amount Sat</th><th>Create</th><th>Expiry</th><th>Blocks To Expiry</th></tr>
  {{range .}}
  <tr>
    <td>{{.ScanRank}}</td><td>{{.StrictRank}}</td><td>{{.Picked}}</td><td>{{.EligibleNow}}</td>
    <td><code>{{.OutPoint}}</code></td><td>{{.AmountSat}}</td><td>{{.CreateHeight}}</td><td>{{.ExpiryHeight}}</td><td>{{.BlocksToExpiry}}</td>
  </tr>
  {{end}}
</table>
{{end}}`))

var testnetLabReapTemplate = template.Must(template.New("testnet-lab-reap").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Testnet Lab REAP</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1f2937; }
    h1, h2 { margin-bottom: 0.4rem; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 1.5rem; }
    th, td { border: 1px solid #d1d5db; padding: 0.45rem; text-align: left; vertical-align: top; }
    th { background: #f3f4f6; }
    code { background: #f3f4f6; padding: 0.1rem 0.3rem; word-break: break-all; }
    input, select, button { padding: 0.35rem; }
    .critical { color: #b91c1c; font-weight: 700; }
    .warning { color: #b45309; }
    .ok { color: #047857; }
    .toolbar, form { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin: 1rem 0; }
  </style>
</head>
<body>
  <h1>REAP Detail</h1>
  <div class="toolbar">
    <a href="/">Overview</a>
    <a href="/reap.json?node={{.SelectedNode}}&count={{.CountValue}}">JSON</a>
    <a href="/expiryindex?node={{.SelectedNode}}">ExpiryIndex</a>
    <a href="/commitment?node={{.SelectedNode}}">Commitment</a>
  </div>
  <form method="get" action="/reap">
    <label>Node <select name="node">{{range .Nodes}}<option value="{{.Name}}" {{if eq .Name $.SelectedNode}}selected{{end}}>{{.Name}}</option>{{end}}</select></label>
    <label>Recent Blocks <input name="count" value="{{.CountValue}}" size="6"></label>
    <button type="submit">Refresh</button>
  </form>
  {{range .Warnings}}<p class="warning">{{.}}</p>{{end}}
  {{if .Error}}<p class="critical">{{.Error}}</p>{{end}}
  {{with .Result}}
  <h2>Current REAP Plan</h2>
  <table>
    <tr><th>Node</th><td>{{.Node.Name}}</td><th>Chain Height</th><td>{{.Chain.Blocks}}</td></tr>
    <tr><th>Next Height</th><td>{{.Plan.Height}}</td><th>Active</th><td>{{.Plan.Active}}</td></tr>
    <tr><th>Enabled</th><td>{{.Plan.Enabled}}</td><th>Picked</th><td>{{.Plan.Picked}}</td></tr>
    <tr><th>Tax Total</th><td>{{.Plan.TaxTotal}}</td><th>Refund Total</th><td>{{.Plan.RefundTotal}}</td></tr>
    <tr><th>Estimated Weight</th><td>{{.Plan.EstWeight}}</td><th>Marker Hash</th><td><code>{{.Plan.MarkerHash}}</code></td></tr>
    <tr><th>Reason</th><td colspan="3">{{.Plan.Reason}}</td></tr>
  </table>

  <h2>Recent REAP Blocks</h2>
  <p>Scanned {{.HistoryScanned}} recent blocks.</p>
  {{if .History}}
    {{range .History}}
    <h3>Height {{.Height}}</h3>
    <table>
      <tr><th>Hash</th><td colspan="5"><code>{{.Hash}}</code></td></tr>
      <tr><th>TxID</th><th>Inputs</th><th>Outputs</th><th>Weight</th><th>Refund Outputs Sat</th><th>Marker Payload</th></tr>
      {{range .ReapTxs}}
      <tr><td><code>{{.TxID}}</code></td><td>{{.Inputs}}</td><td>{{.Outputs}}</td><td>{{.Weight}}</td><td>{{.RefundOutputTotalSat}}</td><td><code>{{.MarkerPayload}}</code></td></tr>
      {{end}}
    </table>
    {{end}}
  {{else}}<p>No REAP transactions found in the scanned range.</p>{{end}}
  {{end}}
</body>
</html>`))

var testnetLabCommitmentTemplate = template.Must(template.New("testnet-lab-commitment").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Testnet Lab Commitment</title>
  <style>
    body { font-family: sans-serif; margin: 2rem; color: #1f2937; }
    h1, h2 { margin-bottom: 0.4rem; }
    table { border-collapse: collapse; width: 100%; margin-bottom: 1.5rem; }
    th, td { border: 1px solid #d1d5db; padding: 0.45rem; text-align: left; vertical-align: top; }
    th { background: #f3f4f6; }
    code { background: #f3f4f6; padding: 0.1rem 0.3rem; word-break: break-all; }
    select, button { padding: 0.35rem; }
    .critical { color: #b91c1c; font-weight: 700; }
    .warning { color: #b45309; }
    .ok { color: #047857; }
    .toolbar, form { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin: 1rem 0; }
  </style>
</head>
<body>
  <h1>Expiry Commitment Detail</h1>
  <div class="toolbar">
    <a href="/">Overview</a>
    <a href="/commitment.json?node={{.SelectedNode}}">JSON</a>
    <a href="/expiryindex?node={{.SelectedNode}}">ExpiryIndex</a>
    <a href="/reap?node={{.SelectedNode}}">REAP</a>
  </div>
  <form method="get" action="/commitment">
    <label>Node <select name="node">{{range .Nodes}}<option value="{{.Name}}" {{if eq .Name $.SelectedNode}}selected{{end}}>{{.Name}}</option>{{end}}</select></label>
    <button type="submit">Refresh</button>
  </form>
  {{range .Warnings}}<p class="warning">{{.}}</p>{{end}}
  {{if .Error}}<p class="critical">{{.Error}}</p>{{end}}
  {{with .Result}}
  <h2>Commitment State</h2>
  <table>
    <tr><th>Node</th><td>{{.Node.Name}}</td><th>Chain Height</th><td>{{.Chain.Blocks}}</td></tr>
    <tr><th>Best Block</th><td colspan="3"><code>{{.Chain.BestBlockHash}}</code></td></tr>
    <tr><th>Enabled</th><td>{{.ExpiryCommitment.Enabled}}</td><th>Available</th><td>{{.ExpiryCommitment.Available}}</td></tr>
    <tr><th>Active</th><td>{{.ExpiryCommitment.Active}}</td><th>Active At Next Height</th><td>{{.ExpiryCommitment.ActiveAtNextHeight}}</td></tr>
    <tr><th>Tip Height</th><td>{{.ExpiryCommitment.TipHeight}}</td><th>Commitment Lag</th><td>{{.CommitmentLag}}</td></tr>
    <tr><th>Enable At Height</th><td>{{.ExpiryCommitment.EnableAtHeight}}</td><th>Next Height</th><td>{{.NextHeight}}</td></tr>
    <tr><th>Tip Hash Matches Chain</th><td>{{.CommitmentTipMatches}}</td><th>Tip Hash</th><td><code>{{.ExpiryCommitment.TipHash}}</code></td></tr>
    <tr><th>Root</th><td colspan="3"><code>{{.ExpiryCommitment.Root}}</code></td></tr>
    <tr><th>Error</th><td colspan="3">{{.ExpiryCommitment.Error}}</td></tr>
  </table>

  <h2>Related State</h2>
  <table>
    <tr><th>ExpiryIndex Available</th><td>{{.ExpiryIndex.Available}}</td><th>Disabled</th><td>{{.ExpiryIndex.Disabled}}</td></tr>
    <tr><th>ExpiryIndex Tip</th><td>{{.ExpiryIndex.TipHeight}}</td><th>ExpiryIndex Lag</th><td>{{.ExpiryIndexLag}}</td></tr>
    <tr><th>Total UTXOs</th><td>{{.ExpiryIndex.TotalUTXOs}}</td><th>Total Expiry Keys</th><td>{{.ExpiryIndex.TotalExpiryKeys}}</td></tr>
    <tr><th>REAP Active</th><td>{{.ReapPlan.Active}}</td><th>REAP Picked</th><td>{{.ReapPlan.Picked}}</td></tr>
  </table>
  {{end}}
</body>
</html>`))
