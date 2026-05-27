// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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
	snapshot, err := s.snapshotWithTimeout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeIndentedJSON(w, snapshot.Logs)
}

func (s *testnetLabServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.snapshotWithTimeout(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if snapshot.Summary.CriticalAlerts > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "critical alerts active\n")
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

	var minHeight, maxHeight int32
	for _, node := range manifest.Nodes {
		nodeSnapshot := s.collectNode(ctx, node)
		if nodeSnapshot.Healthy && nodeSnapshot.Snapshot != nil {
			height := nodeSnapshot.Snapshot.Chain.Blocks
			if snapshot.Summary.HealthyNodes == 0 || height < minHeight {
				minHeight = height
			}
			if snapshot.Summary.HealthyNodes == 0 || height > maxHeight {
				maxHeight = height
			}
			snapshot.Summary.HealthyNodes++
		}
		snapshot.Nodes = append(snapshot.Nodes, nodeSnapshot)
	}
	if snapshot.Summary.HealthyNodes > 0 {
		snapshot.Summary.BestHeight = maxHeight
		snapshot.Summary.HeightSpread = maxHeight - minHeight
	}

	for _, wallet := range manifest.MinerWallets {
		snapshot.MinerWallets = append(snapshot.MinerWallets, s.collectWallet(ctx, wallet))
	}
	for _, wallet := range manifest.UserWallets {
		snapshot.UserWallets = append(snapshot.UserWallets, s.collectWallet(ctx, wallet))
	}
	for _, logSpec := range manifest.Logs {
		snapshot.Logs = append(snapshot.Logs, summarizeLabLog(logSpec, s.logTailLines))
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
	if blocks, ok := numberAsFloat64(info["blocks"]); ok {
		snapshot.Blocks = int32(blocks)
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
	params := []interface{}{map[string]interface{}{"limit": 100}}
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
    <form method="post" action="/action"><input type="hidden" name="action" value="refresh"><button type="submit">Refresh</button></form>
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
    <tr><th>Name</th><th>Role</th><th>Healthy</th><th>Height</th><th>Peers</th><th>ExpiryIndex</th><th>REAP Picked</th><th>Error</th></tr>
    {{range .Snapshot.Nodes}}
    <tr>
      <td>{{.Node.Name}}</td><td>{{.Node.Role}}</td><td>{{.Healthy}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.Chain.Blocks}}{{end}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.Peers.Count}}{{end}}</td>
      <td>{{if .Snapshot}}{{if .Snapshot.ExpiryIndex.Available}}{{if .Snapshot.ExpiryIndex.Disabled}}disabled{{else}}active tip {{.Snapshot.ExpiryIndex.TipHeight}}{{end}}{{else}}n/a{{end}}{{end}}</td>
      <td>{{if .Snapshot}}{{.Snapshot.ReapPlan.Picked}}{{end}}</td>
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
