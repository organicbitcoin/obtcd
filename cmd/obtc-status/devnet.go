// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
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
	"unicode"
	"unicode/utf8"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type devnetNode struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	RPCServer string `json:"rpc_server"`
	P2PServer string `json:"p2p_server"`
	DataDir   string `json:"data_dir"`
}

type devnetManifest struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Network     string       `json:"network"`
	NodeCount   int          `json:"node_count"`
	DataDir     string       `json:"data_dir"`
	Nodes       []devnetNode `json:"nodes"`
}

type devnetNodeSnapshot struct {
	Node     devnetNode      `json:"node"`
	Healthy  bool            `json:"healthy"`
	Snapshot *statusSnapshot `json:"snapshot,omitempty"`
	Error    string          `json:"error,omitempty"`
}

type devnetSummary struct {
	ConfiguredNodes int   `json:"configured_nodes"`
	HealthyNodes    int   `json:"healthy_nodes"`
	BestHeight      int32 `json:"best_height"`
	HeightSpread    int32 `json:"height_spread"`
	TotalMempoolTxs int64 `json:"total_mempool_txs"`
	Synced          bool  `json:"synced"`
}

type devnetActionResult struct {
	At      time.Time `json:"at"`
	Action  string    `json:"action"`
	Args    []string  `json:"args"`
	Success bool      `json:"success"`
	Output  string    `json:"output,omitempty"`
	Error   string    `json:"error,omitempty"`
}

type devnetDashboardSnapshot struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Network     string               `json:"network"`
	NodeCount   int                  `json:"node_count"`
	DataDir     string               `json:"data_dir"`
	Nodes       []devnetNodeSnapshot `json:"nodes"`
	Summary     devnetSummary        `json:"summary"`
	LastAction  *devnetActionResult  `json:"last_action,omitempty"`
	Warnings    []string             `json:"warnings,omitempty"`
}

type nodeSnapshotSource interface {
	Snapshot(ctx context.Context, node devnetNode) (*statusSnapshot, error)
}

type rpcNodeSnapshotSource struct {
	cfg *config
}

func (s *rpcNodeSnapshotSource) Snapshot(ctx context.Context, node devnetNode) (*statusSnapshot, error) {
	caller, err := newJSONRPCCallerForEndpoint(s.cfg, node.RPCServer)
	if err != nil {
		return nil, err
	}

	collector := &statusCollector{
		rpc:       caller,
		rpcServer: node.RPCServer,
	}
	return collector.Snapshot(ctx)
}

type devnetActionSpec struct {
	Label string
	Args  []string
}

type devnetActionRunner interface {
	Run(ctx context.Context, action string, spec devnetActionSpec) devnetActionResult
}

type scriptActionRunner struct {
	cfg        *config
	scriptPath string
	workdir    string
}

func (r *scriptActionRunner) Run(ctx context.Context, action string, spec devnetActionSpec) devnetActionResult {
	args := append([]string{r.scriptPath}, spec.Args...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = r.workdir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DEVNET_NODE_COUNT=%d", r.cfg.DevnetNodes),
		fmt.Sprintf("DEVNET_NETWORK=%s", r.cfg.NetworkName),
	)

	output, err := cmd.CombinedOutput()
	result := devnetActionResult{
		At:     time.Now().UTC(),
		Action: action,
		Args:   append([]string(nil), spec.Args...),
		Output: trimActionOutput(string(output)),
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	return result
}

type devnetActionButton struct {
	Key   string
	Label string
}

type devnetSelectOption struct {
	Value       string
	Label       string
	Description string
}

type devnetSpamFormDefaults struct {
	Target       string
	Mode         string
	Count        int
	Value        int64
	FeeRate      int64
	Prepare      int
	PrepareValue int64
	PaceMS       int
	ValueMin     int64
	ValueMax     int64
	RandSeed     int64
}

type devnetMineFormDefaults struct {
	Blocks int
}

type devnetBlockResult struct {
	BlockHash  string
	PrettyJSON string
}

type devnetBlockListItem struct {
	Height     int64
	Hash       string
	PrettyJSON string
}

type devnetBlockViewMode string

const (
	devnetBlockViewModeRaw devnetBlockViewMode = "raw"
	devnetBlockViewModeRPC devnetBlockViewMode = "rpc"
)

type devnetBlockViewModeOption struct {
	Value string
	Label string
}

type devnetBlockFetcher interface {
	FetchBlock(ctx context.Context, node devnetNode, height, hash string, mode devnetBlockViewMode) (*devnetBlockResult, error)
	FetchRecentBlocks(ctx context.Context, node devnetNode, limit int, mode devnetBlockViewMode) ([]devnetBlockListItem, error)
}

type rpcDevnetBlockFetcher struct {
	cfg *config
}

func (f *rpcDevnetBlockFetcher) FetchBlock(ctx context.Context, node devnetNode, height, hash string, mode devnetBlockViewMode) (*devnetBlockResult, error) {
	caller, err := newJSONRPCCallerForEndpoint(f.cfg, node.RPCServer)
	if err != nil {
		return nil, err
	}

	blockHash, err := resolveBlockHash(ctx, caller, height, hash)
	if err != nil {
		return nil, err
	}
	payload, err := fetchPrettyBlockJSON(ctx, caller, blockHash, mode)
	if err != nil {
		return nil, err
	}

	return &devnetBlockResult{
		BlockHash:  blockHash,
		PrettyJSON: payload,
	}, nil
}

func (f *rpcDevnetBlockFetcher) FetchRecentBlocks(ctx context.Context, node devnetNode, limit int, mode devnetBlockViewMode) ([]devnetBlockListItem, error) {
	caller, err := newJSONRPCCallerForEndpoint(f.cfg, node.RPCServer)
	if err != nil {
		return nil, err
	}

	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	var chainInfo struct {
		Blocks int64 `json:"blocks"`
	}
	if err := caller.Call(ctx, "getblockchaininfo", nil, &chainInfo); err != nil {
		return nil, err
	}

	startHeight := chainInfo.Blocks - int64(limit) + 1
	if startHeight < 0 {
		startHeight = 0
	}

	blocks := make([]devnetBlockListItem, 0, chainInfo.Blocks-startHeight+1)
	for height := chainInfo.Blocks; height >= startHeight; height-- {
		var blockHash string
		if err := caller.Call(ctx, "getblockhash", []interface{}{height}, &blockHash); err != nil {
			return nil, err
		}
		payload, err := fetchPrettyBlockJSON(ctx, caller, blockHash, mode)
		if err != nil {
			return nil, err
		}

		blocks = append(blocks, devnetBlockListItem{
			Height:     height,
			Hash:       blockHash,
			PrettyJSON: payload,
		})
	}

	return blocks, nil
}

func resolveBlockHash(ctx context.Context, caller *jsonRPCCaller, height, hash string) (string, error) {
	blockHash := strings.TrimSpace(hash)
	if blockHash != "" {
		return blockHash, nil
	}

	if strings.TrimSpace(height) != "" {
		h, err := strconv.ParseInt(strings.TrimSpace(height), 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid block height: %w", err)
		}
		if err := caller.Call(ctx, "getblockhash", []interface{}{h}, &blockHash); err != nil {
			return "", err
		}
		return blockHash, nil
	}

	if err := caller.Call(ctx, "getbestblockhash", nil, &blockHash); err != nil {
		return "", err
	}
	return blockHash, nil
}

func fetchPrettyBlockJSON(ctx context.Context, caller *jsonRPCCaller, blockHash string, mode devnetBlockViewMode) (string, error) {
	switch normalizeDevnetBlockViewMode(mode) {
	case devnetBlockViewModeRPC:
		var block interface{}
		if err := caller.Call(ctx, "getblock", []interface{}{blockHash, 2}, &block); err != nil {
			return "", err
		}

		payload, err := json.MarshalIndent(block, "", "  ")
		if err != nil {
			return "", err
		}
		return string(payload), nil

	case devnetBlockViewModeRaw:
		var rawHex string
		if err := caller.Call(ctx, "getblock", []interface{}{blockHash, 0}, &rawHex); err != nil {
			return "", err
		}
		return renderRawBlockPrettyJSON(rawHex)

	default:
		return "", fmt.Errorf("unsupported block view mode %q", mode)
	}
}

type rawBlockJSON struct {
	BlockHash      string         `json:"block_hash"`
	SerializedSize int            `json:"serialized_size"`
	StrippedSize   int            `json:"stripped_size"`
	Header         rawBlockHeader `json:"header"`
	Transactions   []rawTxJSON    `json:"transactions"`
}

type rawBlockHeader struct {
	Version           int32  `json:"version"`
	PreviousBlockHash string `json:"previous_block_hash"`
	MerkleRoot        string `json:"merkle_root"`
	TimestampUnix     int64  `json:"timestamp_unix"`
	Bits              uint32 `json:"bits"`
	BitsHex           string `json:"bits_hex"`
	Nonce             uint32 `json:"nonce"`
}

type rawTxJSON struct {
	TxID           string         `json:"txid"`
	WitnessTxID    string         `json:"wtxid,omitempty"`
	Version        int32          `json:"version"`
	LockTime       uint32         `json:"lock_time"`
	SerializedSize int            `json:"serialized_size"`
	StrippedSize   int            `json:"stripped_size"`
	Inputs         []rawTxInJSON  `json:"vin"`
	Outputs        []rawTxOutJSON `json:"vout"`
}

type rawTxInJSON struct {
	PreviousOutPoint   rawOutPointJSON        `json:"previous_out_point"`
	SignatureScriptHex string                 `json:"signature_script_hex"`
	SignaturePayloads  []rawScriptPayloadJSON `json:"signature_script_payloads,omitempty"`
	SignatureScriptASM string                 `json:"signature_script_asm,omitempty"`
	Witness            []string               `json:"witness,omitempty"`
	Sequence           uint32                 `json:"sequence"`
}

type rawOutPointJSON struct {
	Hash  string `json:"hash"`
	Index uint32 `json:"index"`
}

type rawTxOutJSON struct {
	Value            int64                  `json:"value"`
	PkScriptHex      string                 `json:"pk_script_hex"`
	PkScriptPayloads []rawScriptPayloadJSON `json:"pk_script_payloads,omitempty"`
	PkScriptASM      string                 `json:"pk_script_asm,omitempty"`
}

type rawScriptPayloadJSON struct {
	Index     int    `json:"index"`
	Kind      string `json:"kind"`
	Length    int    `json:"length"`
	Hex       string `json:"hex"`
	Text      string `json:"text,omitempty"`
	ScriptASM string `json:"script_asm,omitempty"`
}

func renderRawBlockPrettyJSON(rawHex string) (string, error) {
	rawBytes, err := hex.DecodeString(strings.TrimSpace(rawHex))
	if err != nil {
		return "", fmt.Errorf("decode raw block hex: %w", err)
	}

	var msgBlock wire.MsgBlock
	if err := msgBlock.Deserialize(bytes.NewReader(rawBytes)); err != nil {
		return "", fmt.Errorf("deserialize raw block: %w", err)
	}

	blockJSON := makeRawBlockJSON(&msgBlock)
	payload, err := json.MarshalIndent(blockJSON, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func makeRawBlockJSON(block *wire.MsgBlock) rawBlockJSON {
	transactions := make([]rawTxJSON, 0, len(block.Transactions))
	for _, tx := range block.Transactions {
		transactions = append(transactions, makeRawTxJSON(tx))
	}

	return rawBlockJSON{
		BlockHash:      block.BlockHash().String(),
		SerializedSize: block.SerializeSize(),
		StrippedSize:   block.SerializeSizeStripped(),
		Header: rawBlockHeader{
			Version:           block.Header.Version,
			PreviousBlockHash: block.Header.PrevBlock.String(),
			MerkleRoot:        block.Header.MerkleRoot.String(),
			TimestampUnix:     block.Header.Timestamp.Unix(),
			Bits:              block.Header.Bits,
			BitsHex:           fmt.Sprintf("%08x", block.Header.Bits),
			Nonce:             block.Header.Nonce,
		},
		Transactions: transactions,
	}
}

func makeRawTxJSON(tx *wire.MsgTx) rawTxJSON {
	inputs := make([]rawTxInJSON, 0, len(tx.TxIn))
	for _, txIn := range tx.TxIn {
		inputs = append(inputs, rawTxInJSON{
			PreviousOutPoint: rawOutPointJSON{
				Hash:  txIn.PreviousOutPoint.Hash.String(),
				Index: txIn.PreviousOutPoint.Index,
			},
			SignatureScriptHex: hex.EncodeToString(txIn.SignatureScript),
			SignaturePayloads:  makeRawScriptPayloadJSON(txIn.SignatureScript),
			SignatureScriptASM: disassembleScript(txIn.SignatureScript),
			Witness:            makeRawWitnessJSON(txIn.Witness),
			Sequence:           txIn.Sequence,
		})
	}

	outputs := make([]rawTxOutJSON, 0, len(tx.TxOut))
	for _, txOut := range tx.TxOut {
		outputs = append(outputs, rawTxOutJSON{
			Value:            txOut.Value,
			PkScriptHex:      hex.EncodeToString(txOut.PkScript),
			PkScriptPayloads: makeRawScriptPayloadJSON(txOut.PkScript),
			PkScriptASM:      disassembleScript(txOut.PkScript),
		})
	}

	witnessTxID := tx.WitnessHash().String()
	txID := tx.TxHash().String()
	if witnessTxID == txID {
		witnessTxID = ""
	}

	return rawTxJSON{
		TxID:           txID,
		WitnessTxID:    witnessTxID,
		Version:        tx.Version,
		LockTime:       tx.LockTime,
		SerializedSize: tx.SerializeSize(),
		StrippedSize:   tx.SerializeSizeStripped(),
		Inputs:         inputs,
		Outputs:        outputs,
	}
}

func makeRawWitnessJSON(witness wire.TxWitness) []string {
	if len(witness) == 0 {
		return nil
	}

	items := make([]string, 0, len(witness))
	for _, item := range witness {
		items = append(items, hex.EncodeToString(item))
	}
	return items
}

func makeRawScriptPayloadJSON(script []byte) []rawScriptPayloadJSON {
	payloads, err := txscript.PushedData(script)
	if err != nil || len(payloads) == 0 {
		return nil
	}

	items := make([]rawScriptPayloadJSON, 0, len(payloads))
	for idx, payload := range payloads {
		text := printableUTF8String(payload)
		kind := classifyScriptPayload(payload, text)
		item := rawScriptPayloadJSON{
			Index:  idx,
			Kind:   kind,
			Length: len(payload),
			Hex:    hex.EncodeToString(payload),
			Text:   text,
		}
		if kind == "script" {
			item.ScriptASM = disassembleScript(payload)
		}
		items = append(items, item)
	}
	return items
}

func classifyScriptPayload(payload []byte, text string) string {
	switch {
	case len(payload) == 0:
		return "empty"
	case text != "":
		return "text"
	case looksLikePubKey(payload):
		return "pubkey"
	case looksLikeSignature(payload):
		return "signature"
	case looksLikeScriptPayload(payload):
		return "script"
	default:
		return "data"
	}
}

func printableUTF8String(payload []byte) string {
	if len(payload) == 0 || !utf8.Valid(payload) {
		return ""
	}

	s := string(payload)
	for _, r := range s {
		if !unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\t' {
			return ""
		}
	}
	return s
}

func looksLikePubKey(payload []byte) bool {
	if len(payload) == 33 && (payload[0] == 0x02 || payload[0] == 0x03) {
		return true
	}
	return len(payload) == 65 && payload[0] == 0x04
}

func looksLikeSignature(payload []byte) bool {
	return len(payload) >= 9 && payload[0] == 0x30
}

func looksLikeScriptPayload(payload []byte) bool {
	disasm, err := txscript.DisasmString(payload)
	if err != nil || strings.TrimSpace(disasm) == "" {
		return false
	}
	return strings.Contains(disasm, "OP_") ||
		strings.Contains(disasm, "CHECK") ||
		strings.Contains(disasm, "HASH") ||
		strings.Contains(disasm, "EQUAL") ||
		strings.Contains(disasm, "VERIFY") ||
		strings.Contains(disasm, "RETURN")
}

func disassembleScript(script []byte) string {
	if len(script) == 0 {
		return ""
	}

	disasm, err := txscript.DisasmString(script)
	if err != nil && strings.TrimSpace(disasm) == "" {
		return fmt.Sprintf("[disasm error: %v]", err)
	}
	return disasm
}

func normalizeDevnetBlockViewMode(mode devnetBlockViewMode) devnetBlockViewMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(devnetBlockViewModeRaw):
		return devnetBlockViewModeRaw
	case string(devnetBlockViewModeRPC):
		return devnetBlockViewModeRPC
	default:
		return devnetBlockViewModeRaw
	}
}

func devnetBlockViewModeOptions() []devnetBlockViewModeOption {
	return []devnetBlockViewModeOption{
		{Value: string(devnetBlockViewModeRaw), Label: "Raw"},
		{Value: string(devnetBlockViewModeRPC), Label: "RPC"},
	}
}

type devnetBlockView struct {
	Node       devnetNode
	QueryLabel string
	Result     *devnetBlockResult
	Error      string
}

type devnetBlocksView struct {
	Node   devnetNode
	Count  int
	Blocks []devnetBlockListItem
	Error  string
}

type devnetServer struct {
	cfg           *config
	manifestPath  string
	refresh       time.Duration
	timeout       time.Duration
	actionTimeout time.Duration
	source        nodeSnapshotSource
	runner        devnetActionRunner
	blockFetcher  devnetBlockFetcher
	diagnostics   devnetDiagnosticsSource

	mu         sync.RWMutex
	lastAction *devnetActionResult
}

func newDevnetServer(cfg *config) (*devnetServer, error) {
	scriptPath, err := filepath.Abs(cfg.DevnetScript)
	if err != nil {
		return nil, fmt.Errorf("resolve devnet script path: %w", err)
	}
	manifestPath, err := filepath.Abs(cfg.DevnetManifest)
	if err != nil {
		return nil, fmt.Errorf("resolve devnet manifest path: %w", err)
	}

	workdir := filepath.Dir(filepath.Dir(scriptPath))

	return &devnetServer{
		cfg:           cfg,
		manifestPath:  manifestPath,
		refresh:       cfg.Refresh,
		timeout:       cfg.RPCTimeout,
		actionTimeout: cfg.DevnetActionTimeout,
		source:        &rpcNodeSnapshotSource{cfg: cfg},
		blockFetcher:  &rpcDevnetBlockFetcher{cfg: cfg},
		diagnostics:   newRPCDevnetDiagnosticsSource(cfg),
		runner: &scriptActionRunner{
			cfg:        cfg,
			scriptPath: scriptPath,
			workdir:    workdir,
		},
	}, nil
}

func (s *devnetServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHTML)
	mux.HandleFunc("/block", s.handleBlockHTML)
	mux.HandleFunc("/blocks", s.handleBlocksHTML)
	mux.HandleFunc("/reap", s.handleReapHTML)
	mux.HandleFunc("/expiryindex", s.handleExpiryIndexHTML)
	mux.HandleFunc("/status", s.handleJSON)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/action", s.handleAction)
	return mux
}

func (s *devnetServer) handleHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	snapshot := s.collectSnapshot(ctx)
	view := struct {
		RefreshSeconds int
		Snapshot       *devnetDashboardSnapshot
		Actions        []devnetActionButton
		TrafficModes   []devnetSelectOption
		TrafficTargets []devnetSelectOption
		MineDefaults   devnetMineFormDefaults
		SpamDefaults   devnetSpamFormDefaults
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Snapshot:       snapshot,
		Actions:        devnetActionButtons(),
		TrafficModes:   devnetTrafficModeOptions(),
		TrafficTargets: devnetTrafficTargetOptions(),
		MineDefaults:   defaultDevnetMineFormDefaults(),
		SpamDefaults:   defaultDevnetSpamFormDefaults(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := devnetTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *devnetServer) handleBlockHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/block" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	manifest, warnings := s.loadManifest()
	selectedNode := strings.TrimSpace(r.URL.Query().Get("node"))
	height := strings.TrimSpace(r.URL.Query().Get("height"))
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	mode := normalizeDevnetBlockViewMode(devnetBlockViewMode(r.URL.Query().Get("view")))

	if selectedNode == "" && len(manifest.Nodes) > 0 {
		selectedNode = manifest.Nodes[0].Name
	}

	view := struct {
		RefreshSeconds int
		Nodes          []devnetNode
		SelectedNode   string
		Height         string
		Hash           string
		ViewMode       string
		ViewModes      []devnetBlockViewModeOption
		Result         *devnetBlockView
		Warnings       []string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedNode,
		Height:         height,
		Hash:           hash,
		ViewMode:       string(mode),
		ViewModes:      devnetBlockViewModeOptions(),
		Warnings:       warnings,
	}

	if len(manifest.Nodes) > 0 && selectedNode != "" {
		view.Result = s.fetchBlockView(ctx, manifest.Nodes, selectedNode, height, hash, mode)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := devnetBlockTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *devnetServer) handleBlocksHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/blocks" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	manifest, warnings := s.loadManifest()
	selectedNode := strings.TrimSpace(r.URL.Query().Get("node"))
	countValue := strings.TrimSpace(r.URL.Query().Get("count"))
	mode := normalizeDevnetBlockViewMode(devnetBlockViewMode(r.URL.Query().Get("view")))

	if selectedNode == "" && len(manifest.Nodes) > 0 {
		selectedNode = manifest.Nodes[0].Name
	}

	count, countErr := parseBlockListCount(countValue)
	view := struct {
		Nodes        []devnetNode
		SelectedNode string
		Count        int
		CountValue   string
		ViewMode     string
		ViewModes    []devnetBlockViewModeOption
		Result       *devnetBlocksView
		Warnings     []string
		CountError   string
	}{
		Nodes:        manifest.Nodes,
		SelectedNode: selectedNode,
		Count:        count,
		CountValue:   countValue,
		ViewMode:     string(mode),
		ViewModes:    devnetBlockViewModeOptions(),
		Warnings:     warnings,
	}
	if countErr != nil {
		view.CountError = countErr.Error()
	} else if len(manifest.Nodes) > 0 && selectedNode != "" {
		view.Result = s.fetchBlocksView(ctx, manifest.Nodes, selectedNode, count, mode)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := devnetBlocksTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *devnetServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(s.collectSnapshot(ctx))
}

func (s *devnetServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	snapshot := s.collectSnapshot(ctx)
	if snapshot.Summary.HealthyNodes == 0 {
		http.Error(w, "no healthy devnet nodes\n", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *devnetServer) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "devnet actions are only allowed from loopback clients", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	action := strings.TrimSpace(r.Form.Get("action"))
	actionLabel, spec, err := resolveDevnetAction(action, r.Form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.actionTimeout)
	defer cancel()

	result := s.runner.Run(ctx, actionLabel, spec)
	s.setLastAction(&result)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *devnetServer) collectSnapshot(ctx context.Context) *devnetDashboardSnapshot {
	manifest, warnings := s.loadManifest()
	snapshot := &devnetDashboardSnapshot{
		GeneratedAt: time.Now().UTC(),
		Network:     manifest.Network,
		NodeCount:   manifest.NodeCount,
		DataDir:     manifest.DataDir,
		Summary: devnetSummary{
			ConfiguredNodes: len(manifest.Nodes),
		},
		Warnings: warnings,
	}

	var (
		maxHeight int32
		minHeight int32
	)

	for idx, node := range manifest.Nodes {
		nodeSnapshot := devnetNodeSnapshot{Node: node}
		status, err := s.source.Snapshot(ctx, node)
		if err != nil {
			nodeSnapshot.Error = err.Error()
			snapshot.Warnings = append(snapshot.Warnings,
				fmt.Sprintf("%s unavailable: %v", node.Name, err))
		} else {
			nodeSnapshot.Healthy = true
			nodeSnapshot.Snapshot = status
			snapshot.Summary.HealthyNodes++
			snapshot.Summary.TotalMempoolTxs += status.Mempool.Transactions

			height := status.Chain.Blocks
			if snapshot.Summary.HealthyNodes == 1 || height > maxHeight {
				maxHeight = height
			}
			if snapshot.Summary.HealthyNodes == 1 || height < minHeight {
				minHeight = height
			}
		}

		if idx == 0 {
			snapshot.Network = manifest.Network
		}
		snapshot.Nodes = append(snapshot.Nodes, nodeSnapshot)
	}

	if snapshot.Summary.HealthyNodes > 0 {
		snapshot.Summary.BestHeight = maxHeight
		snapshot.Summary.HeightSpread = maxHeight - minHeight
		snapshot.Summary.Synced = snapshot.Summary.HealthyNodes == snapshot.Summary.ConfiguredNodes &&
			snapshot.Summary.HeightSpread == 0
	}

	sort.Strings(snapshot.Warnings)
	snapshot.LastAction = s.getLastAction()
	return snapshot
}

func (s *devnetServer) fetchBlockView(ctx context.Context, nodes []devnetNode, selectedNode, height, hash string, mode devnetBlockViewMode) *devnetBlockView {
	node, err := findDevnetNode(nodes, selectedNode)
	if err != nil {
		return &devnetBlockView{Error: err.Error()}
	}

	queryLabel := "latest"
	switch {
	case strings.TrimSpace(hash) != "":
		queryLabel = "hash " + strings.TrimSpace(hash)
	case strings.TrimSpace(height) != "":
		queryLabel = "height " + strings.TrimSpace(height)
	}

	result, err := s.blockFetcher.FetchBlock(ctx, node, height, hash, mode)
	if err != nil {
		return &devnetBlockView{
			Node:       node,
			QueryLabel: queryLabel,
			Error:      err.Error(),
		}
	}

	return &devnetBlockView{
		Node:       node,
		QueryLabel: queryLabel,
		Result:     result,
	}
}

func (s *devnetServer) fetchBlocksView(ctx context.Context, nodes []devnetNode, selectedNode string, count int, mode devnetBlockViewMode) *devnetBlocksView {
	node, err := findDevnetNode(nodes, selectedNode)
	if err != nil {
		return &devnetBlocksView{Error: err.Error()}
	}

	blocks, err := s.blockFetcher.FetchRecentBlocks(ctx, node, count, mode)
	if err != nil {
		return &devnetBlocksView{
			Node:  node,
			Count: count,
			Error: err.Error(),
		}
	}

	return &devnetBlocksView{
		Node:   node,
		Count:  count,
		Blocks: blocks,
	}
}

func (s *devnetServer) loadManifest() (*devnetManifest, []string) {
	manifest, err := readManifestFile(s.manifestPath)
	if err == nil {
		return manifest, nil
	}

	warnings := []string{
		fmt.Sprintf("devnet manifest unavailable: %v", err),
		"showing planned local node layout until the devnet starts",
	}
	return plannedManifest(s.cfg), warnings
}

func (s *devnetServer) setLastAction(result *devnetActionResult) {
	if result == nil {
		return
	}
	copyResult := *result
	copyResult.Args = append([]string(nil), result.Args...)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAction = &copyResult
}

func (s *devnetServer) getLastAction() *devnetActionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastAction == nil {
		return nil
	}

	copyResult := *s.lastAction
	copyResult.Args = append([]string(nil), s.lastAction.Args...)
	return &copyResult
}

func findDevnetNode(nodes []devnetNode, name string) (devnetNode, error) {
	for _, node := range nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return devnetNode{}, fmt.Errorf("unknown node %q", name)
}

func parseBlockListCount(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 20, nil
	}

	count, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("block count must be an integer between 1 and 50")
	}
	if count < 1 || count > 50 {
		return 0, fmt.Errorf("block count must be between 1 and 50")
	}
	return count, nil
}

func readManifestFile(path string) (*devnetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest devnetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.NodeCount == 0 || len(manifest.Nodes) == 0 {
		return nil, fmt.Errorf("manifest contains no nodes")
	}

	return &manifest, nil
}

func plannedManifest(cfg *config) *devnetManifest {
	dataDir := filepath.Dir(cfg.DevnetManifest)
	nodes := make([]devnetNode, 0, cfg.DevnetNodes)
	for idx := 1; idx <= cfg.DevnetNodes; idx++ {
		nodes = append(nodes, devnetNode{
			Index:     idx,
			Name:      fmt.Sprintf("node%d", idx),
			Role:      plannedRole(idx),
			RPCServer: fmt.Sprintf("127.0.0.1:%d", 18556+idx-1),
			P2PServer: fmt.Sprintf("127.0.0.1:%d", 19555+idx-1),
			DataDir:   filepath.Join(dataDir, fmt.Sprintf("node%d", idx)),
		})
	}

	return &devnetManifest{
		GeneratedAt: time.Now().UTC(),
		Network:     cfg.NetworkName,
		NodeCount:   cfg.DevnetNodes,
		DataDir:     dataDir,
		Nodes:       nodes,
	}
}

func plannedRole(idx int) string {
	switch idx {
	case 1:
		return "miner"
	case 2:
		return "peer"
	default:
		return "relay"
	}
}

func trimActionOutput(output string) string {
	const maxBytes = 12000
	if len(output) <= maxBytes {
		return output
	}
	return "...\n" + output[len(output)-maxBytes:]
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func resolveDevnetAction(action string, form map[string][]string) (string, devnetActionSpec, error) {
	if spec, ok := devnetActionSpecs()[action]; ok {
		return action, spec, nil
	}

	switch action {
	case "mine-custom":
		return buildDevnetMineActionSpec(form)
	case "spam-custom":
		return buildDevnetSpamActionSpec(form)
	default:
		return "", devnetActionSpec{}, fmt.Errorf("unknown action")
	}
}

func buildDevnetMineActionSpec(form map[string][]string) (string, devnetActionSpec, error) {
	blocks, err := parseDevnetFormInt(form, "blocks", defaultDevnetMineFormDefaults().Blocks, 1, 10000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}

	return fmt.Sprintf("mine %d", blocks), devnetActionSpec{
		Label: "Mine",
		Args:  []string{"mine", strconv.Itoa(blocks)},
	}, nil
}

func buildDevnetSpamActionSpec(form map[string][]string) (string, devnetActionSpec, error) {
	target := strings.ToLower(strings.TrimSpace(firstFormValue(form, "target")))
	if target == "" {
		target = "primary"
	}
	command := "spam"
	actionLabel := "spam"
	if target == "peer" {
		command = "spam-peer"
		actionLabel = "spam-peer"
	} else if target != "primary" {
		return "", devnetActionSpec{}, fmt.Errorf("unknown spam target %q", target)
	}

	mode := strings.ToLower(strings.TrimSpace(firstFormValue(form, "mode")))
	if mode == "" {
		mode = defaultDevnetSpamFormDefaults().Mode
	}
	if !isAllowedDevnetTrafficMode(mode) {
		return "", devnetActionSpec{}, fmt.Errorf("unknown spam mode %q", mode)
	}

	count, err := parseDevnetFormInt(form, "count", defaultDevnetSpamFormDefaults().Count, 1, 50000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	value, err := parseDevnetFormInt64(form, "value", defaultDevnetSpamFormDefaults().Value, 1, 1000000000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	feeRate, err := parseDevnetFormInt64(form, "fee_rate", defaultDevnetSpamFormDefaults().FeeRate, 1, 1000000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	prepare, err := parseDevnetFormInt(form, "prepare", defaultDevnetSpamFormDefaults().Prepare, 0, 100000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	prepareValue, err := parseDevnetFormInt64(form, "prepare_value", defaultDevnetSpamFormDefaults().PrepareValue, 1, 1000000000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	paceMS, err := parseDevnetFormInt(form, "pace_ms", defaultDevnetSpamFormDefaults().PaceMS, 0, 60000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}

	valueMinRaw := strings.TrimSpace(firstFormValue(form, "value_min"))
	valueMaxRaw := strings.TrimSpace(firstFormValue(form, "value_max"))
	valueMin, err := parseOptionalDevnetFormInt64(valueMinRaw, "value_min", 1, 1000000000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	valueMax, err := parseOptionalDevnetFormInt64(valueMaxRaw, "value_max", 1, 1000000000)
	if err != nil {
		return "", devnetActionSpec{}, err
	}
	if valueMin > 0 && valueMax > 0 && valueMin > valueMax {
		return "", devnetActionSpec{}, fmt.Errorf("value_min must be less than or equal to value_max")
	}

	randomizeInputs := devnetFormBool(form, "randomize_inputs")
	randSeedRaw := strings.TrimSpace(firstFormValue(form, "rand_seed"))
	randSeed, err := parseOptionalDevnetFormInt64(randSeedRaw, "rand_seed", -1<<62, 1<<62)
	if err != nil {
		return "", devnetActionSpec{}, err
	}

	args := []string{
		command,
		"--count", strconv.Itoa(count),
		"--mode", mode,
		"--value", strconv.FormatInt(value, 10),
		"--fee-rate", strconv.FormatInt(feeRate, 10),
	}
	if prepare > 0 {
		args = append(args,
			"--prepare", strconv.Itoa(prepare),
			"--prepare-value", strconv.FormatInt(prepareValue, 10),
		)
	}
	if paceMS > 0 {
		args = append(args, "--pace-ms", strconv.Itoa(paceMS))
	}
	if valueMin > 0 {
		args = append(args, "--value-min", strconv.FormatInt(valueMin, 10))
	}
	if valueMax > 0 {
		args = append(args, "--value-max", strconv.FormatInt(valueMax, 10))
	}
	if randomizeInputs {
		args = append(args, "--randomize-inputs")
	}
	if randSeed != 0 {
		args = append(args, "--rand-seed", strconv.FormatInt(randSeed, 10))
	}

	return actionLabel, devnetActionSpec{
		Label: "Spam",
		Args:  args,
	}, nil
}

func firstFormValue(form map[string][]string, key string) string {
	if values, ok := form[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

func parseDevnetFormInt(form map[string][]string, key string, defaultValue, minValue, maxValue int) (int, error) {
	raw := strings.TrimSpace(firstFormValue(form, key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minValue, maxValue)
	}
	return value, nil
}

func parseDevnetFormInt64(form map[string][]string, key string, defaultValue, minValue, maxValue int64) (int64, error) {
	raw := strings.TrimSpace(firstFormValue(form, key))
	if raw == "" {
		return defaultValue, nil
	}
	return parseOptionalDevnetFormInt64(raw, key, minValue, maxValue)
}

func parseOptionalDevnetFormInt64(raw, key string, minValue, maxValue int64) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minValue, maxValue)
	}
	return value, nil
}

func devnetFormBool(form map[string][]string, key string) bool {
	value := strings.ToLower(strings.TrimSpace(firstFormValue(form, key)))
	return value == "1" || value == "true" || value == "on" || value == "yes"
}

func isAllowedDevnetTrafficMode(mode string) bool {
	for _, option := range devnetTrafficModeOptions() {
		if option.Value == mode {
			return true
		}
	}
	return false
}

func devnetActionSpecs() map[string]devnetActionSpec {
	return map[string]devnetActionSpec{
		"start":                {Label: "Start", Args: []string{"start"}},
		"stop":                 {Label: "Stop", Args: []string{"stop"}},
		"restart":              {Label: "Restart", Args: []string{"restart"}},
		"mine1":                {Label: "Mine 1", Args: []string{"mine", "1"}},
		"validate":             {Label: "Validate", Args: []string{"validate-obtc"}},
		"prepare":              {Label: "Prepare", Args: []string{"prepare", "512", "300000"}},
		"prepare-peer":         {Label: "Prepare Peer", Args: []string{"prepare-peer", "256", "240000"}},
		"scenario-dynamic":     {Label: "Scenario Dynamic", Args: []string{"scenario", "dynamic"}},
		"scenario-multisource": {Label: "Scenario Multisource", Args: []string{"scenario", "multisource"}},
	}
}

func devnetTrafficModeOptions() []devnetSelectOption {
	return []devnetSelectOption{
		{
			Value:       "simple",
			Label:       "simple",
			Description: "Straightforward payment traffic for quick volume buildup and basic smoke testing.",
		},
		{
			Value:       "mixed",
			Label:       "mixed",
			Description: "A mix of common transaction shapes, combining simple payments with more complex spend paths.",
		},
		{
			Value:       "chain",
			Label:       "chain",
			Description: "Creates parent-child dependency chains to observe unconfirmed dependencies, package order, and ancestor relationships.",
		},
		{
			Value:       "consolidate",
			Label:       "consolidate",
			Description: "Consolidates many small UTXOs into fewer outputs, simulating wallet cleanup and large-input transactions.",
		},
		{
			Value:       "feemarket",
			Label:       "feemarket",
			Description: "Generates transactions across multiple fee tiers to simulate congestion pricing and miner selection.",
		},
		{
			Value:       "conflict",
			Label:       "conflict",
			Description: "Injects conflicting transactions to observe node rejection, conflict relay, and double-spend-like behavior.",
		},
	}
}

func devnetTrafficTargetOptions() []devnetSelectOption {
	return []devnetSelectOption{
		{Value: "primary", Label: "primary"},
		{Value: "peer", Label: "peer"},
	}
}

func defaultDevnetMineFormDefaults() devnetMineFormDefaults {
	return devnetMineFormDefaults{
		Blocks: 1,
	}
}

func defaultDevnetSpamFormDefaults() devnetSpamFormDefaults {
	return devnetSpamFormDefaults{
		Target:       "primary",
		Mode:         "mixed",
		Count:        200,
		Value:        150000,
		FeeRate:      10,
		Prepare:      512,
		PrepareValue: 300000,
		PaceMS:       0,
		ValueMin:     0,
		ValueMax:     0,
		RandSeed:     42,
	}
}

func devnetActionButtons() []devnetActionButton {
	keys := []string{
		"start",
		"stop",
		"restart",
		"mine1",
		"validate",
		"prepare",
		"prepare-peer",
		"scenario-dynamic",
		"scenario-multisource",
	}

	specs := devnetActionSpecs()
	buttons := make([]devnetActionButton, 0, len(keys))
	for _, key := range keys {
		spec := specs[key]
		buttons = append(buttons, devnetActionButton{
			Key:   key,
			Label: spec.Label,
		})
	}
	return buttons
}

var devnetTemplate = template.Must(template.New("devnet").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Devnet Dashboard</title>
  <style>
    :root {
      --bg: #f5efe3;
      --card: rgba(255, 252, 246, 0.92);
      --ink: #1f2a37;
      --muted: #64748b;
      --line: #dccfb0;
      --accent: #b45309;
      --accent-deep: #7c2d12;
      --good: #166534;
      --bad: #991b1b;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(237, 201, 175, 0.55), transparent 32%),
        linear-gradient(180deg, #f9f4ea 0%, var(--bg) 100%);
    }
    main { max-width: 1360px; margin: 0 auto; padding: 32px 24px 64px; }
    h1, h2, h3, p { margin: 0; }
    .hero {
      display: grid;
      gap: 14px;
      padding: 24px;
      border: 1px solid var(--line);
      border-radius: 24px;
      background: linear-gradient(135deg, rgba(255,255,255,0.86), rgba(246,236,220,0.9));
      box-shadow: 0 20px 45px rgba(124, 45, 18, 0.08);
    }
    .hero-top {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
    }
    .kpis {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 12px;
    }
    .kpi, .panel, .node {
      background: var(--card);
      border: 1px solid var(--line);
      border-radius: 18px;
      box-shadow: 0 14px 30px rgba(124, 45, 18, 0.06);
    }
    .kpi { padding: 14px 16px; }
    .kpi .label { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; }
    .kpi .value { font-size: 28px; font-weight: 700; margin-top: 4px; }
    .layout { display: grid; grid-template-columns: 1.15fr 0.85fr; gap: 18px; margin-top: 20px; }
    .panel { padding: 20px; min-width: 0; }
    .panel h2 { margin-bottom: 12px; }
    .actions {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
      gap: 10px;
    }
    .control-stack {
      display: grid;
      gap: 18px;
    }
    .custom-form {
      display: grid;
      gap: 14px;
      margin-top: 18px;
      padding-top: 18px;
      border-top: 1px solid rgba(220, 207, 176, 0.9);
    }
    .custom-form h3 {
      margin: 0;
      font-size: 18px;
    }
    .field-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: 12px;
    }
    .field-grid label,
    .advanced-grid label {
      display: grid;
      gap: 6px;
      font-size: 13px;
      color: var(--muted);
      font-weight: 600;
    }
    .advanced-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
      gap: 12px;
      margin-top: 12px;
    }
    .custom-form input,
    .custom-form select {
      width: 100%;
      border-radius: 12px;
      border: 1px solid var(--line);
      padding: 11px 12px;
      font: inherit;
      background: #fffdfa;
      color: var(--ink);
    }
	    .custom-form details {
	      border: 1px solid rgba(220, 207, 176, 0.9);
	      border-radius: 14px;
	      padding: 12px 14px;
	      background: rgba(255,255,255,0.45);
	    }
	    .mode-help {
	      border: 1px solid rgba(220, 207, 176, 0.9);
	      border-radius: 14px;
	      padding: 14px;
	      background: rgba(255,255,255,0.6);
	    }
	    .mode-help .meta {
	      margin: 0 0 10px;
	    }
	    .mode-list {
	      margin: 0;
	      padding: 0;
	      list-style: none;
	      display: grid;
	      gap: 10px;
	    }
	    .mode-list li {
	      display: grid;
	      gap: 4px;
	      padding: 10px 12px;
	      border-radius: 12px;
	      background: rgba(255, 250, 244, 0.9);
	      border: 1px solid rgba(220, 207, 176, 0.65);
	    }
	    .mode-list code {
	      width: fit-content;
	      padding: 3px 8px;
	      border-radius: 999px;
	      background: rgba(180, 83, 9, 0.12);
	      color: var(--accent-deep);
	      font-size: 12px;
	      font-weight: 700;
	    }
	    .mode-list span {
	      font-size: 13px;
	      color: var(--muted);
	      line-height: 1.5;
	    }
	    .custom-form summary {
	      cursor: pointer;
	      font-weight: 700;
	      color: var(--accent-deep);
	    }
    .checkbox-line {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-top: 12px;
      color: var(--muted);
      font-size: 13px;
      font-weight: 600;
    }
    .checkbox-line input {
      width: auto;
      margin: 0;
    }
    .action-form { margin: 0; }
    .toolbar-link,
    .node-link {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 10px 12px;
      border-radius: 12px;
      text-decoration: none;
      font-weight: 700;
      background: rgba(180, 83, 9, 0.1);
      color: var(--accent-deep);
      border: 1px solid rgba(180, 83, 9, 0.2);
    }
    .toolbar-link:hover,
    .node-link:hover { background: rgba(180, 83, 9, 0.16); }
    button {
      width: 100%;
      padding: 12px 14px;
      border: 0;
      border-radius: 12px;
      cursor: pointer;
      background: linear-gradient(135deg, var(--accent) 0%, var(--accent-deep) 100%);
      color: #fff;
      font-weight: 700;
      letter-spacing: 0.02em;
    }
    button:hover { filter: brightness(1.05); }
    .meta, .warns li {
      color: var(--muted);
      font-size: 14px;
      overflow-wrap: anywhere;
    }
    .warns { margin: 12px 0 0; padding-left: 18px; }
    .last-action pre {
      margin: 0;
      padding: 14px;
      border-radius: 14px;
      background: #111827;
      color: #f9fafb;
      overflow: auto;
      white-space: pre-wrap;
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 12px;
      line-height: 1.5;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 6px 10px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.06em;
    }
    .status.good { background: rgba(22, 101, 52, 0.1); color: var(--good); }
    .status.bad { background: rgba(153, 27, 27, 0.08); color: var(--bad); }
    .nodes {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 20px;
      margin-top: 24px;
      align-items: start;
    }
    .node {
      padding: 22px;
      display: grid;
      gap: 16px;
      min-width: 0;
    }
    .node-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }
    .node-head {
      display: flex;
      flex-wrap: wrap;
      justify-content: space-between;
      align-items: flex-start;
      gap: 12px;
    }
    .node-head > div:first-child {
      min-width: 0;
    }
    .node-role {
      color: var(--accent-deep);
      font-size: 12px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    .metric-list {
      display: grid;
      gap: 10px;
      font-size: 14px;
    }
    .metric-list div {
      display: grid;
      grid-template-columns: minmax(92px, 116px) minmax(0, 1fr);
      align-items: start;
      gap: 12px;
      border-top: 1px solid rgba(220, 207, 176, 0.7);
      padding-top: 10px;
      min-width: 0;
    }
    .metric-list span {
      color: var(--muted);
      font-weight: 600;
    }
    .metric-list code,
    .metric-list strong {
      display: block;
      min-width: 0;
      width: 100%;
      overflow-wrap: anywhere;
      word-break: break-word;
      white-space: normal;
      line-height: 1.45;
    }
    code {
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 12px;
      background: rgba(148, 163, 184, 0.14);
      padding: 2px 6px;
      border-radius: 8px;
    }
    .last-action pre { max-height: 360px; }
    @media (max-width: 960px) {
      .layout { grid-template-columns: 1fr; }
      .hero-top { align-items: flex-start; }
    }
    @media (max-width: 720px) {
      main { padding: 24px 16px 40px; }
      .nodes { grid-template-columns: 1fr; }
      .metric-list div { grid-template-columns: 1fr; gap: 6px; }
    }
  </style>
</head>
<body>
<main>
  <section class="hero">
    <div class="hero-top">
      <div>
        <h1>OBTC Devnet Dashboard</h1>
        <p class="meta">Local devnet control panel aggregating chain status, mempool state, and OBTC-specific signals for each node.</p>
      </div>
      <div class="status {{if .Snapshot.Summary.Synced}}good{{else}}bad{{end}}">
        {{if .Snapshot.Summary.Synced}}Synced{{else}}Attention{{end}}
      </div>
    </div>
	    <div>
	      <a class="toolbar-link" href="/blocks?view=raw">Open Block List</a>
	      <a class="toolbar-link" href="/block?view=raw">Open Block Viewer</a>
	      <a class="toolbar-link" href="/reap">Open REAP Monitor</a>
	      <a class="toolbar-link" href="/expiryindex">Open ExpiryIndex Ordering</a>
	    </div>
    <div class="kpis">
      <div class="kpi">
        <div class="label">Network</div>
        <div class="value">{{.Snapshot.Network}}</div>
      </div>
      <div class="kpi">
        <div class="label">Healthy Nodes</div>
        <div class="value">{{.Snapshot.Summary.HealthyNodes}} / {{.Snapshot.Summary.ConfiguredNodes}}</div>
      </div>
      <div class="kpi">
        <div class="label">Best Height</div>
        <div class="value">{{.Snapshot.Summary.BestHeight}}</div>
      </div>
      <div class="kpi">
        <div class="label">Total Mempool</div>
        <div class="value">{{.Snapshot.Summary.TotalMempoolTxs}}</div>
      </div>
    </div>
  </section>

  <section class="layout">
    <div class="panel">
      <div class="control-stack">
	        <div>
	          <h2>Common Actions</h2>
	          <p class="meta">Buttons are limited to loopback clients and invoke the local <code>scripts/devnet-up.sh</code> helper.</p>
	          <div class="actions">
            {{range .Actions}}
            <form class="action-form" method="post" action="/action">
              <input type="hidden" name="action" value="{{.Key}}">
              <button type="submit">{{.Label}}</button>
            </form>
            {{end}}
	          </div>
	        </div>

	        <form class="custom-form" method="post" action="/action">
	          <input type="hidden" name="action" value="mine-custom">
	          <div>
	            <h3>Mine Blocks</h3>
	            <p class="meta">The default quick action remains <code>Mine 1</code>; use this form to mine multiple blocks at once.</p>
	          </div>
	          <div class="field-grid">
	            <label>
	              Block Count
	              <input type="number" name="blocks" min="1" max="10000" value="{{.MineDefaults.Blocks}}">
	            </label>
	          </div>
	          <button type="submit">Start Mining</button>
	        </form>

	        <form class="custom-form" method="post" action="/action">
	          <input type="hidden" name="action" value="spam-custom">
	          <div>
            <h3>Traffic Injection</h3>
            <p class="meta">Start <code>spam</code> or <code>spam-peer</code> directly from the web UI. Keep the basic parameters above and the advanced randomization and preparation controls below.</p>
          </div>
	          <div class="field-grid">
            <label>
              Target Wallet
              <select name="target">
                {{range .TrafficTargets}}
                <option value="{{.Value}}" {{if eq $.SpamDefaults.Target .Value}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </label>
            <label>
              Mode
              <select name="mode">
                {{range .TrafficModes}}
                <option value="{{.Value}}" {{if eq $.SpamDefaults.Mode .Value}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </label>
            <label>
              Transaction Count
              <input type="number" name="count" min="1" value="{{.SpamDefaults.Count}}">
            </label>
            <label>
              Base Value
              <input type="number" name="value" min="1" value="{{.SpamDefaults.Value}}">
	            </label>
	          </div>
	          <div class="mode-help">
	            <p class="meta">Mode Guide</p>
	            <ul class="mode-list">
	              {{range .TrafficModes}}
	              <li>
	                <code>{{.Value}}</code>
	                <span>{{.Description}}</span>
	              </li>
	              {{end}}
	            </ul>
	          </div>
	          <details>
	            <summary>Advanced Options</summary>
            <div class="advanced-grid">
              <label>
                Fee Rate
                <input type="number" name="fee_rate" min="1" value="{{.SpamDefaults.FeeRate}}">
              </label>
              <label>
                Prepare UTXOs
                <input type="number" name="prepare" min="0" value="{{.SpamDefaults.Prepare}}">
              </label>
              <label>
                Prepare Value
                <input type="number" name="prepare_value" min="1" value="{{.SpamDefaults.PrepareValue}}">
              </label>
              <label>
                Pace ms
                <input type="number" name="pace_ms" min="0" value="{{.SpamDefaults.PaceMS}}">
              </label>
              <label>
                Value Min
                <input type="number" name="value_min" min="1" value="{{if gt .SpamDefaults.ValueMin 0}}{{.SpamDefaults.ValueMin}}{{end}}" placeholder="optional">
              </label>
              <label>
                Value Max
                <input type="number" name="value_max" min="1" value="{{if gt .SpamDefaults.ValueMax 0}}{{.SpamDefaults.ValueMax}}{{end}}" placeholder="optional">
              </label>
              <label>
                Rand Seed
                <input type="number" name="rand_seed" value="{{.SpamDefaults.RandSeed}}">
              </label>
            </div>
            <label class="checkbox-line">
              <input type="checkbox" name="randomize_inputs" value="true">
              Randomize spendable inputs
            </label>
          </details>
          <button type="submit">Start Traffic</button>
        </form>

        {{if .Snapshot.Warnings}}
        <ul class="warns">
          {{range .Snapshot.Warnings}}<li>{{.}}</li>{{end}}
        </ul>
        {{end}}
      </div>
    </div>

    <div class="panel last-action">
      <h2>Latest Action</h2>
      {{if .Snapshot.LastAction}}
      <p class="meta">
        {{.Snapshot.LastAction.At}} | {{.Snapshot.LastAction.Action}} |
        {{if .Snapshot.LastAction.Success}}succeeded{{else}}failed{{end}}
      </p>
      {{if .Snapshot.LastAction.Error}}
      <p class="meta">Error: {{.Snapshot.LastAction.Error}}</p>
      {{end}}
      <pre>{{.Snapshot.LastAction.Output}}</pre>
      {{else}}
      <p class="meta">No dashboard action has been run yet. After devnet starts, the latest command result appears here.</p>
      {{end}}
    </div>
  </section>

  <section class="nodes">
    {{range .Snapshot.Nodes}}
    <article class="node">
      <div class="node-head">
        <div>
          <div class="node-role">{{.Node.Role}}</div>
          <h3>{{.Node.Name}}</h3>
        </div>
        <div class="status {{if .Healthy}}good{{else}}bad{{end}}">
          {{if .Healthy}}Healthy{{else}}Down{{end}}
        </div>
      </div>

      <div class="metric-list">
        <div><span>RPC</span><code>{{.Node.RPCServer}}</code></div>
        <div><span>P2P</span><code>{{.Node.P2PServer}}</code></div>
        {{if .Healthy}}
        <div><span>Height</span><strong>{{.Snapshot.Chain.Blocks}}</strong></div>
        <div><span>Peers</span><strong>{{.Snapshot.Peers.Count}}</strong></div>
        <div><span>Mempool</span><strong>{{.Snapshot.Mempool.Transactions}}</strong></div>
        <div><span>Expiry Index</span><strong>{{if .Snapshot.ExpiryIndex.Available}}{{if .Snapshot.ExpiryIndex.Disabled}}Disabled{{else}}Active{{end}}{{else}}N/A{{end}}</strong></div>
        <div><span>Commitment</span><strong>{{if .Snapshot.ExpiryCommitment.Active}}Active{{else}}Inactive{{end}}</strong></div>
        <div><span>REAP Picked</span><strong>{{.Snapshot.ReapPlan.Picked}}</strong></div>
        <div><span>Best Block</span><code>{{.Snapshot.Chain.BestBlockHash}}</code></div>
        {{else}}
        <div><span>Status</span><strong>{{.Error}}</strong></div>
        {{end}}
      </div>
	      <div class="node-actions">
	        <a class="node-link" href="/blocks?node={{.Node.Name}}&view=raw">Browse Block List</a>
	        <a class="node-link" href="/block?node={{.Node.Name}}&view=raw">View Latest Block JSON</a>
	        <a class="node-link" href="/reap?node={{.Node.Name}}">View REAP Details</a>
	        <a class="node-link" href="/expiryindex?node={{.Node.Name}}">View ExpiryIndex Ordering</a>
	        {{if .Healthy}}<a class="node-link" href="/block?node={{.Node.Name}}&hash={{.Snapshot.Chain.BestBlockHash}}&view=raw">View Current Best Block</a>{{end}}
	      </div>
    </article>
    {{end}}
  </section>
</main>
</body>
</html>`))

var devnetBlocksTemplate = template.Must(template.New("devnet-blocks").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>OBTC Block List</title>
  <style>
    :root {
      --bg: #f5efe3;
      --card: rgba(255, 252, 246, 0.94);
      --ink: #1f2a37;
      --muted: #64748b;
      --line: #dccfb0;
      --accent: #b45309;
      --accent-deep: #7c2d12;
      --bad: #991b1b;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(237, 201, 175, 0.55), transparent 32%),
        linear-gradient(180deg, #f9f4ea 0%, var(--bg) 100%);
    }
    main { max-width: 1440px; margin: 0 auto; padding: 28px 24px 56px; }
    .panel {
      background: var(--card);
      border: 1px solid var(--line);
      border-radius: 20px;
      box-shadow: 0 14px 30px rgba(124, 45, 18, 0.06);
      padding: 22px;
      margin-bottom: 18px;
    }
    h1, h2, p { margin: 0; }
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 16px;
    }
    .links {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
    }
    .link {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 10px 12px;
      border-radius: 12px;
      text-decoration: none;
      font-weight: 700;
      background: rgba(180, 83, 9, 0.1);
      color: var(--accent-deep);
      border: 1px solid rgba(180, 83, 9, 0.2);
    }
    .link:hover { background: rgba(180, 83, 9, 0.16); }
    .meta {
      color: var(--muted);
      margin-top: 6px;
      overflow-wrap: anywhere;
      line-height: 1.5;
    }
    form {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
      align-items: end;
    }
    label {
      display: grid;
      gap: 6px;
      font-size: 14px;
      color: var(--muted);
      font-weight: 600;
    }
    input, select, button {
      width: 100%;
      border-radius: 12px;
      border: 1px solid var(--line);
      padding: 12px 14px;
      font: inherit;
      background: #fffdfa;
      color: var(--ink);
    }
    button {
      cursor: pointer;
      font-weight: 700;
      background: linear-gradient(135deg, var(--accent) 0%, var(--accent-deep) 100%);
      border: 0;
      color: #fff;
    }
    .warnings {
      margin: 12px 0 0;
      padding-left: 18px;
      color: var(--muted);
    }
    .error {
      color: var(--bad);
      font-weight: 700;
      overflow-wrap: anywhere;
    }
    .block-list {
      display: grid;
      gap: 14px;
    }
    details {
      border: 1px solid var(--line);
      border-radius: 16px;
      background: rgba(255,255,255,0.55);
      overflow: hidden;
    }
    summary {
      cursor: pointer;
      list-style: none;
      display: grid;
      gap: 8px;
      padding: 16px 18px;
    }
    summary::-webkit-details-marker { display: none; }
    .block-meta {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
    }
    .height-badge {
      display: inline-flex;
      align-items: center;
      padding: 6px 10px;
      border-radius: 999px;
      background: rgba(180, 83, 9, 0.12);
      color: var(--accent-deep);
      font-size: 12px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.06em;
    }
    code {
      font-family: "SFMono-Regular", Menlo, monospace;
      font-size: 12px;
      background: rgba(148, 163, 184, 0.14);
      padding: 2px 6px;
      border-radius: 8px;
      overflow-wrap: anywhere;
      word-break: break-word;
    }
    .hash-line {
      display: block;
      line-height: 1.6;
      overflow-wrap: anywhere;
    }
    .json-wrap {
      padding: 0 18px 18px;
    }
    pre {
      margin: 0;
      padding: 18px;
      border-radius: 16px;
      background: #111827;
      color: #f9fafb;
      overflow: auto;
      white-space: pre-wrap;
      word-break: break-word;
      line-height: 1.55;
      font-size: 12px;
      font-family: "SFMono-Regular", Menlo, monospace;
    }
    @media (max-width: 720px) {
      main { padding: 24px 16px 40px; }
      .block-meta { align-items: flex-start; }
    }
  </style>
</head>
<body>
<main>
  <section class="panel">
    <div class="topbar">
      <div>
        <h1>Block List</h1>
        <p class="meta">Choose a node to list recent blocks. Each entry shows the height and hash and expands to the full pretty JSON. Raw mode omits derived fields such as <code>nextblockhash</code>.</p>
      </div>
      <div class="links">
        <a class="link" href="/">Back to Dashboard</a>
        <a class="link" href="/block?view={{.ViewMode}}">Open Block Viewer</a>
      </div>
    </div>

    <form method="get" action="/blocks">
      <label>
        Node
        <select name="node">
          {{range .Nodes}}
          <option value="{{.Name}}" {{if eq $.SelectedNode .Name}}selected{{end}}>{{.Name}} ({{.Role}})</option>
          {{end}}
        </select>
      </label>
      <label>
        Block Count
        <input type="text" name="count" value="{{if .CountValue}}{{.CountValue}}{{else}}{{.Count}}{{end}}" placeholder="default 20, max 50">
      </label>
      <label>
        View
        <select name="view">
          {{range .ViewModes}}
          <option value="{{.Value}}" {{if eq $.ViewMode .Value}}selected{{end}}>{{.Label}}</option>
          {{end}}
        </select>
      </label>
      <button type="submit">Load Block List</button>
    </form>

    {{if .CountError}}
    <p class="error">Invalid parameter: {{.CountError}}</p>
    {{end}}

    {{if .Warnings}}
    <ul class="warnings">
      {{range .Warnings}}<li>{{.}}</li>{{end}}
    </ul>
    {{end}}
  </section>

  <section class="panel">
    {{if .Result}}
      {{if .Result.Error}}
      <p class="error">Query failed: {{.Result.Error}}</p>
      {{else}}
      <h2>{{.Result.Node.Name}} | Latest {{.Result.Count}} Blocks</h2>
      <p class="meta">Open any entry to expand the full block JSON. Current view: <code>{{.ViewMode}}</code></p>
      <div class="block-list">
        {{range $idx, $block := .Result.Blocks}}
        <details {{if eq $idx 0}}open{{end}}>
          <summary>
            <div class="block-meta">
              <span class="height-badge">Height {{$block.Height}}</span>
            </div>
            <code class="hash-line">{{$block.Hash}}</code>
          </summary>
          <div class="json-wrap">
            <pre>{{$block.PrettyJSON}}</pre>
          </div>
        </details>
        {{end}}
      </div>
      {{end}}
    {{else}}
    <p class="meta">The block list appears here after you select a node and load data.</p>
    {{end}}
  </section>
</main>
</body>
</html>`))

var devnetBlockTemplate = template.Must(template.New("devnet-block").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>OBTC Block Viewer</title>
  <style>
    :root {
      --bg: #f5efe3;
      --card: rgba(255, 252, 246, 0.94);
      --ink: #1f2a37;
      --muted: #64748b;
      --line: #dccfb0;
      --accent: #b45309;
      --accent-deep: #7c2d12;
      --bad: #991b1b;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(237, 201, 175, 0.55), transparent 32%),
        linear-gradient(180deg, #f9f4ea 0%, var(--bg) 100%);
    }
    main { max-width: 1360px; margin: 0 auto; padding: 28px 24px 56px; }
    .panel {
      background: var(--card);
      border: 1px solid var(--line);
      border-radius: 20px;
      box-shadow: 0 14px 30px rgba(124, 45, 18, 0.06);
      padding: 22px;
      margin-bottom: 18px;
    }
    h1, h2, p { margin: 0; }
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 16px;
    }
    .back-link {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 10px 12px;
      border-radius: 12px;
      text-decoration: none;
      font-weight: 700;
      background: rgba(180, 83, 9, 0.1);
      color: var(--accent-deep);
      border: 1px solid rgba(180, 83, 9, 0.2);
    }
    .back-link:hover { background: rgba(180, 83, 9, 0.16); }
    .meta { color: var(--muted); margin-top: 6px; overflow-wrap: anywhere; }
    form {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
      align-items: end;
    }
    label {
      display: grid;
      gap: 6px;
      font-size: 14px;
      color: var(--muted);
      font-weight: 600;
    }
    input, select, button {
      width: 100%;
      border-radius: 12px;
      border: 1px solid var(--line);
      padding: 12px 14px;
      font: inherit;
      background: #fffdfa;
      color: var(--ink);
    }
    button {
      cursor: pointer;
      font-weight: 700;
      background: linear-gradient(135deg, var(--accent) 0%, var(--accent-deep) 100%);
      border: 0;
      color: #fff;
    }
    pre {
      margin: 0;
      padding: 18px;
      border-radius: 16px;
      background: #111827;
      color: #f9fafb;
      overflow: auto;
      white-space: pre-wrap;
      word-break: break-word;
      line-height: 1.55;
      font-size: 12px;
      font-family: "SFMono-Regular", Menlo, monospace;
    }
    .error {
      color: var(--bad);
      font-weight: 700;
      overflow-wrap: anywhere;
    }
    .warnings {
      margin: 12px 0 0;
      padding-left: 18px;
      color: var(--muted);
    }
  </style>
</head>
<body>
<main>
  <section class="panel">
    <div class="topbar">
      <div>
        <h1>Block Viewer</h1>
        <p class="meta">View the latest block by node, or enter a block height or block hash. Raw mode shows JSON closer to the low-level block-byte decode.</p>
      </div>
      <a class="back-link" href="/">Back to Dashboard</a>
    </div>

    <form method="get" action="/block">
      <label>
        Node
        <select name="node">
          {{range .Nodes}}
          <option value="{{.Name}}" {{if eq $.SelectedNode .Name}}selected{{end}}>{{.Name}} ({{.Role}})</option>
          {{end}}
        </select>
      </label>
      <label>
        Block Height
        <input type="text" name="height" value="{{.Height}}" placeholder="leave empty to use the latest block">
      </label>
      <label>
        Block Hash
        <input type="text" name="hash" value="{{.Hash}}" placeholder="takes precedence over height">
      </label>
      <label>
        View
        <select name="view">
          {{range .ViewModes}}
          <option value="{{.Value}}" {{if eq $.ViewMode .Value}}selected{{end}}>{{.Label}}</option>
          {{end}}
        </select>
      </label>
      <button type="submit">View Block JSON</button>
    </form>

    {{if .Warnings}}
    <ul class="warnings">
      {{range .Warnings}}<li>{{.}}</li>{{end}}
    </ul>
    {{end}}
  </section>

  <section class="panel">
    {{if .Result}}
      {{if .Result.Error}}
      <p class="error">Query failed: {{.Result.Error}}</p>
      {{else}}
      <h2>{{.Result.Node.Name}} | {{.Result.QueryLabel}}</h2>
      <p class="meta">Block Hash: {{.Result.Result.BlockHash}}</p>
      <pre>{{.Result.Result.PrettyJSON}}</pre>
      {{end}}
    {{else}}
    <p class="meta">The block JSON appears here after you select a node and submit the form.</p>
    {{end}}
  </section>
</main>
</body>
</html>`))
