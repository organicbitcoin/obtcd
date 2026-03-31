package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type devnetDiagnosticsSource interface {
	LoadReapHistory(ctx context.Context, node devnetNode, count int) (*devnetReapHistoryData, error)
	LoadExpiryOrdering(ctx context.Context, node devnetNode, startHeight, endHeight int32, limit int) (*devnetExpiryIndexData, error)
}

type rpcDevnetDiagnosticsSource struct {
	cfg *config

	mu    sync.RWMutex
	cache map[string]*cachedDevnetChainState
}

type cachedDevnetChainState struct {
	bestHash   string
	bestHeight int64
	state      *devnetChainState
}

type devnetChainState struct {
	BestHeight int64
	BestHash   string
	Blocks     []devnetChainBlock
	Outputs    map[wire.OutPoint]devnetOutputMeta
}

type devnetChainBlock struct {
	Height    int64
	Hash      string
	Timestamp time.Time
	Txs       []*wire.MsgTx
}

type devnetOutputMeta struct {
	Amount       int64
	PkScript     []byte
	Address      string
	ScriptClass  string
	CreateHeight int64
	TxID         string
	Vout         uint32
	IsCoinbase   bool
}

type devnetReapHistoryData struct {
	Node             devnetNode
	BestHeight       int64
	BlocksRequested  int
	BlocksInspected  int
	BlocksWithREAP   int
	TotalExpiredUTXO int
	Blocks           []devnetReapBlockData
}

type devnetReapBlockData struct {
	Height           int64
	Hash             string
	Timestamp        string
	BlockLink        string
	HasREAP          bool
	REAPTxID         string
	MarkerPayload    string
	InputCount       int
	ComputedTaxTotal int64
	ComputedRefund   int64
	OnChainRefund    int64
	TotalsMatch      bool
	WeightEstimate   int64
	MissingPrevouts  int
	Rows             []devnetReapInputRow
}

type devnetReapInputRow struct {
	Order           int
	OutPoint        string
	TxID            string
	Vout            uint32
	SourceAddress   string
	ScriptClass     string
	AmountSat       int64
	TaxSat          int64
	RefundSat       int64
	CreateHeight    int64
	ExpiryHeight    int64
	SelectorHashHex string
	SourceKnown     bool
}

type devnetExpiryIndexData struct {
	Node                   devnetNode
	TipHeight              int32
	StartHeight            int32
	EndHeight              int32
	Limit                  int
	Returned               int
	PreviewPicked          int
	MaxInputs              int
	WeightBudget           int64
	Truncated              bool
	NextCursor             string
	ScanOrderDescription   string
	StrictOrderDescription string
	ScanRows               []*devnetExpiryIndexRow
	StrictRows             []*devnetExpiryIndexRow
}

type devnetExpiryIndexRow struct {
	ScanRank        int
	StrictRank      int
	Picked          bool
	OutPoint        string
	TxID            string
	Vout            uint32
	Address         string
	ScriptClass     string
	AmountSat       int64
	ExpiryHeight    uint64
	CreateHeight    uint64
	BlocksToExpiry  int64
	SelectorHashHex string
}

func newRPCDevnetDiagnosticsSource(cfg *config) devnetDiagnosticsSource {
	return &rpcDevnetDiagnosticsSource{
		cfg:   cfg,
		cache: make(map[string]*cachedDevnetChainState),
	}
}

func (s *rpcDevnetDiagnosticsSource) LoadReapHistory(ctx context.Context, node devnetNode, count int) (*devnetReapHistoryData, error) {
	if count < 1 {
		count = 1
	}
	if count > 200 {
		count = 200
	}

	params, err := diagnosticsNetworkParams(s.cfg)
	if err != nil {
		return nil, err
	}
	expiryParams := chaincfg.GetExpiryParams(params)
	if expiryParams == nil {
		return nil, fmt.Errorf("expiry params unavailable for %s", params.Name)
	}

	state, err := s.loadChainState(ctx, node, params)
	if err != nil {
		return nil, err
	}

	start := maxInt64(0, state.BestHeight-int64(count)+1)
	report := &devnetReapHistoryData{
		Node:            node,
		BestHeight:      state.BestHeight,
		BlocksRequested: count,
	}

	for _, block := range state.Blocks {
		if block.Height < start {
			continue
		}

		item := devnetReapBlockData{
			Height:    block.Height,
			Hash:      block.Hash,
			Timestamp: block.Timestamp.UTC().Format(time.RFC3339),
			BlockLink: fmt.Sprintf("/block?node=%s&hash=%s&view=raw", node.Name, block.Hash),
		}

		for _, tx := range block.Txs {
			if !reap.IsLikelyREAPTx(tx) {
				continue
			}

			item.HasREAP = true
			item.REAPTxID = tx.TxHash().String()
			item.InputCount = len(tx.TxIn)
			item.WeightEstimate = reap.EstimateBlueprintWeight(len(tx.TxIn))
			item.MarkerPayload = reapMarkerPayload(tx)
			item.OnChainRefund = totalNonMarkerOutputs(tx)

			for idx, txIn := range tx.TxIn {
				row := devnetReapInputRow{
					Order:    idx + 1,
					OutPoint: formatOutPoint(txIn.PreviousOutPoint),
					TxID:     txIn.PreviousOutPoint.Hash.String(),
					Vout:     txIn.PreviousOutPoint.Index,
				}

				meta, ok := state.Outputs[txIn.PreviousOutPoint]
				if !ok {
					row.SourceAddress = "unresolved"
					row.ScriptClass = "unknown"
					item.MissingPrevouts++
					item.Rows = append(item.Rows, row)
					continue
				}

				row.SourceKnown = true
				row.SourceAddress = meta.Address
				row.ScriptClass = meta.ScriptClass
				row.AmountSat = meta.Amount
				row.CreateHeight = meta.CreateHeight
				row.ExpiryHeight = computeExpiryHeight(meta.CreateHeight, expiryParams)
				row.SelectorHashHex = hex.EncodeToString(txIn.PreviousOutPoint.Hash[:])
				row.TaxSat, row.RefundSat = reapBreakdown(meta.Amount, expiryParams)

				item.ComputedTaxTotal += row.TaxSat
				item.ComputedRefund += row.RefundSat
				item.Rows = append(item.Rows, row)
			}

			item.TotalsMatch = item.ComputedRefund == item.OnChainRefund
			report.BlocksWithREAP++
			report.TotalExpiredUTXO += len(item.Rows)
			break
		}

		report.Blocks = append(report.Blocks, item)
		report.BlocksInspected++
	}

	return report, nil
}

func (s *rpcDevnetDiagnosticsSource) LoadExpiryOrdering(ctx context.Context, node devnetNode, startHeight, endHeight int32, limit int) (*devnetExpiryIndexData, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 5000 {
		limit = 5000
	}

	params, err := diagnosticsNetworkParams(s.cfg)
	if err != nil {
		return nil, err
	}
	reapParams := reap.DefaultREAPParamsForNet(params, reap.SortModeStrict)

	caller, err := newJSONRPCCallerForEndpoint(s.cfg, node.RPCServer)
	if err != nil {
		return nil, err
	}

	var chainInfo btcjson.GetBlockChainInfoResult
	if err := caller.Call(ctx, "getblockchaininfo", nil, &chainInfo); err != nil {
		return nil, err
	}

	if endHeight < 0 {
		endHeight = chainInfo.Blocks
	}
	if startHeight < 0 {
		startHeight = 0
	}
	if endHeight < startHeight {
		return nil, fmt.Errorf("end height must be greater than or equal to start height")
	}

	var result btcjson.ListExpiringResult
	paramsList := []interface{}{startHeight, endHeight, limit}
	if err := caller.Call(ctx, "listexpiring", paramsList, &result); err != nil {
		return nil, err
	}

	state, err := s.loadChainState(ctx, node, params)
	if err != nil {
		return nil, err
	}

	rows := make([]*devnetExpiryIndexRow, 0, len(result.ExpiringUTXOs))
	for idx, utxo := range result.ExpiringUTXOs {
		op, err := parseResultOutPoint(utxo.TxID, utxo.Vout)
		if err != nil {
			return nil, err
		}

		address := "unresolved"
		scriptClass := "unknown"
		if meta, ok := state.Outputs[op]; ok {
			address = meta.Address
			scriptClass = meta.ScriptClass
		}

		rows = append(rows, &devnetExpiryIndexRow{
			ScanRank:        idx + 1,
			OutPoint:        formatOutPoint(op),
			TxID:            utxo.TxID,
			Vout:            utxo.Vout,
			Address:         address,
			ScriptClass:     scriptClass,
			AmountSat:       utxo.AmountSat,
			ExpiryHeight:    utxo.ExpiryHeight,
			CreateHeight:    utxo.CreateHeight,
			BlocksToExpiry:  utxo.BlocksToExpiry,
			SelectorHashHex: hex.EncodeToString(op.Hash[:]),
		})
	}

	strictRows := append([]*devnetExpiryIndexRow(nil), rows...)
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

	picked := 0
	for idx, row := range strictRows {
		row.StrictRank = idx + 1
		nextWeight := reap.EstimateBlueprintWeight(picked + 1)
		if picked < reapParams.MaxInputs && (reapParams.WeightBudget <= 0 || nextWeight <= reapParams.WeightBudget) {
			row.Picked = true
			picked++
		}
	}

	report := &devnetExpiryIndexData{
		Node:                   node,
		TipHeight:              chainInfo.Blocks,
		StartHeight:            result.StartHeight,
		EndHeight:              result.EndHeight,
		Limit:                  limit,
		Returned:               len(rows),
		PreviewPicked:          picked,
		MaxInputs:              reapParams.MaxInputs,
		WeightBudget:           reapParams.WeightBudget,
		Truncated:              result.NextHeight != nil && result.NextOutpoint != nil,
		ScanOrderDescription:   "expiry_height -> canonical txid string -> vout",
		StrictOrderDescription: "expiry_height -> amount_sat -> raw hash bytes -> vout",
		ScanRows:               rows,
		StrictRows:             strictRows,
	}

	if result.NextHeight != nil && result.NextOutpoint != nil {
		report.NextCursor = fmt.Sprintf("next_height=%d, start_after=%s", *result.NextHeight, *result.NextOutpoint)
	}

	return report, nil
}

func (s *rpcDevnetDiagnosticsSource) loadChainState(ctx context.Context, node devnetNode, params *chaincfg.Params) (*devnetChainState, error) {
	caller, err := newJSONRPCCallerForEndpoint(s.cfg, node.RPCServer)
	if err != nil {
		return nil, err
	}

	var chainInfo btcjson.GetBlockChainInfoResult
	if err := caller.Call(ctx, "getblockchaininfo", nil, &chainInfo); err != nil {
		return nil, err
	}

	cacheKey := node.RPCServer

	s.mu.RLock()
	cached := s.cache[cacheKey]
	s.mu.RUnlock()
	if cached != nil && cached.bestHash == chainInfo.BestBlockHash && cached.bestHeight == int64(chainInfo.Blocks) {
		return cached.state, nil
	}

	state, err := buildChainState(ctx, caller, int64(chainInfo.Blocks), chainInfo.BestBlockHash, params)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[cacheKey] = &cachedDevnetChainState{
		bestHash:   chainInfo.BestBlockHash,
		bestHeight: int64(chainInfo.Blocks),
		state:      state,
	}
	s.mu.Unlock()

	return state, nil
}

func buildChainState(ctx context.Context, caller *jsonRPCCaller, bestHeight int64, bestHash string, params *chaincfg.Params) (*devnetChainState, error) {
	state := &devnetChainState{
		BestHeight: bestHeight,
		BestHash:   bestHash,
		Outputs:    make(map[wire.OutPoint]devnetOutputMeta),
	}

	for height := int64(0); height <= bestHeight; height++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var blockHash string
		if err := caller.Call(ctx, "getblockhash", []interface{}{height}, &blockHash); err != nil {
			return nil, err
		}

		msgBlock, err := fetchRawBlock(ctx, caller, blockHash)
		if err != nil {
			return nil, err
		}

		block := devnetChainBlock{
			Height:    height,
			Hash:      blockHash,
			Timestamp: msgBlock.Header.Timestamp,
			Txs:       msgBlock.Transactions,
		}
		state.Blocks = append(state.Blocks, block)

		for txIndex, tx := range msgBlock.Transactions {
			isCoinbase := txIndex == 0
			txID := tx.TxHash().String()
			for vout, txOut := range tx.TxOut {
				if txscript.IsUnspendable(txOut.PkScript) {
					continue
				}

				address, class := describePkScript(txOut.PkScript, params)
				op := wire.OutPoint{Hash: tx.TxHash(), Index: uint32(vout)}
				state.Outputs[op] = devnetOutputMeta{
					Amount:       txOut.Value,
					PkScript:     append([]byte(nil), txOut.PkScript...),
					Address:      address,
					ScriptClass:  class,
					CreateHeight: height,
					TxID:         txID,
					Vout:         uint32(vout),
					IsCoinbase:   isCoinbase,
				}
			}
		}
	}

	return state, nil
}

func fetchRawBlock(ctx context.Context, caller *jsonRPCCaller, blockHash string) (*wire.MsgBlock, error) {
	var rawHex string
	if err := caller.Call(ctx, "getblock", []interface{}{blockHash, 0}, &rawHex); err != nil {
		return nil, err
	}

	rawBytes, err := hex.DecodeString(strings.TrimSpace(rawHex))
	if err != nil {
		return nil, fmt.Errorf("decode raw block hex: %w", err)
	}

	var msgBlock wire.MsgBlock
	if err := msgBlock.Deserialize(bytes.NewReader(rawBytes)); err != nil {
		return nil, fmt.Errorf("deserialize raw block: %w", err)
	}

	return &msgBlock, nil
}

func describePkScript(pkScript []byte, params *chaincfg.Params) (string, string) {
	class, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, params)
	classLabel := class.String()
	if err == nil && len(addrs) > 0 {
		parts := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			parts = append(parts, addr.EncodeAddress())
		}
		return strings.Join(parts, ", "), classLabel
	}

	if class == txscript.NullDataTy {
		return "OP_RETURN", classLabel
	}
	if classLabel == "" || classLabel == "nonstandard" {
		return trimMiddle(disassembleScript(pkScript), 64), classLabel
	}
	return classLabel, classLabel
}

func diagnosticsNetworkParams(cfg *config) (*chaincfg.Params, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if params, err := networkParams(cfg); err == nil {
		return params, nil
	}

	switch cfg.NetworkName {
	case "obtctestnet":
		return &chaincfg.ObtcTestNetParams, nil
	case "obtcregtest":
		return &chaincfg.ObtcRegTestParams, nil
	default:
		return &chaincfg.ObtcMainNetParams, nil
	}
}

func computeExpiryHeight(createHeight int64, expiryParams *chaincfg.ExpiryParams) int64 {
	if expiryParams == nil {
		return 0
	}
	return createHeight + int64(expiryParams.WindowBlocks)
}

func reapBreakdown(amount int64, expiryParams *chaincfg.ExpiryParams) (tax, refund int64) {
	if expiryParams == nil || amount <= 0 {
		return 0, amount
	}
	if expiryParams.ReapTaxNumerator > 0 && expiryParams.ReapTaxDenominator > 0 {
		tax = (amount * expiryParams.ReapTaxNumerator) / expiryParams.ReapTaxDenominator
	}
	refund = amount - tax
	if expiryParams.ReapDustThresholdSat > 0 && amount < expiryParams.ReapDustThresholdSat {
		return amount, 0
	}
	return tax, refund
}

func totalNonMarkerOutputs(tx *wire.MsgTx) int64 {
	if tx == nil || len(tx.TxOut) == 0 {
		return 0
	}

	total := int64(0)
	for idx, txOut := range tx.TxOut {
		if idx == len(tx.TxOut)-1 && txOut.Value == 0 {
			if payload, ok := reap.ExtractMarkerPayload(txOut.PkScript); ok && strings.HasPrefix(payload, "REAP:") {
				continue
			}
		}
		total += txOut.Value
	}
	return total
}

func reapMarkerPayload(tx *wire.MsgTx) string {
	if tx == nil || len(tx.TxOut) == 0 {
		return ""
	}
	payload, ok := reap.ExtractMarkerPayload(tx.TxOut[len(tx.TxOut)-1].PkScript)
	if !ok {
		return ""
	}
	return payload
}

func parseResultOutPoint(txID string, vout uint32) (wire.OutPoint, error) {
	hash, err := chainhash.NewHashFromStr(txID)
	if err != nil {
		return wire.OutPoint{}, err
	}
	return wire.OutPoint{Hash: *hash, Index: vout}, nil
}

func formatOutPoint(op wire.OutPoint) string {
	return fmt.Sprintf("%s:%d", op.Hash.String(), op.Index)
}

func trimMiddle(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	head := max / 2
	tail := max - head - 1
	if tail < 0 {
		tail = 0
	}
	return s[:head] + "..." + s[len(s)-tail:]
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func parseReapHistoryCount(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 20, nil
	}
	count, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("block count must be an integer between 1 and 200")
	}
	if count < 1 || count > 200 {
		return 0, fmt.Errorf("block count must be between 1 and 200")
	}
	return count, nil
}

func parseExpiryHeight(raw string, defaultValue int32) (int32, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("height parameter must be an integer")
	}
	if value < 0 {
		return 0, fmt.Errorf("height parameter must not be negative")
	}
	return int32(value), nil
}

func parseExpiryLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 200, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer between 1 and 5000")
	}
	if value < 1 || value > 5000 {
		return 0, fmt.Errorf("limit must be between 1 and 5000")
	}
	return value, nil
}

func (s *devnetServer) handleReapHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/reap" {
		http.NotFound(w, r)
		return
	}
	if s.diagnostics == nil {
		http.Error(w, "REAP diagnostics unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
	defer cancel()

	manifest, warnings := s.loadManifest()
	selectedNode := strings.TrimSpace(r.URL.Query().Get("node"))
	countRaw := strings.TrimSpace(r.URL.Query().Get("count"))
	if selectedNode == "" && len(manifest.Nodes) > 0 {
		selectedNode = manifest.Nodes[0].Name
	}

	count, countErr := parseReapHistoryCount(countRaw)
	view := struct {
		RefreshSeconds int
		Nodes          []devnetNode
		SelectedNode   string
		CountValue     string
		Count          int
		Result         *devnetReapHistoryData
		Warnings       []string
		Error          string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedNode,
		CountValue:     countRaw,
		Count:          count,
		Warnings:       warnings,
	}

	if countErr != nil {
		view.Error = countErr.Error()
	} else if len(manifest.Nodes) > 0 && selectedNode != "" {
		node, err := findDevnetNode(manifest.Nodes, selectedNode)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Result, err = s.diagnostics.LoadReapHistory(ctx, node, count)
			if err != nil {
				view.Error = err.Error()
			}
		}
	}

	if view.CountValue == "" {
		view.CountValue = strconv.Itoa(view.Count)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := devnetReapTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *devnetServer) handleExpiryIndexHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/expiryindex" {
		http.NotFound(w, r)
		return
	}
	if s.diagnostics == nil {
		http.Error(w, "ExpiryIndex diagnostics unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxDuration(s.timeout, 30*time.Second))
	defer cancel()

	manifest, warnings := s.loadManifest()
	selectedNode := strings.TrimSpace(r.URL.Query().Get("node"))
	startRaw := strings.TrimSpace(r.URL.Query().Get("start"))
	endRaw := strings.TrimSpace(r.URL.Query().Get("end"))
	limitRaw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if selectedNode == "" && len(manifest.Nodes) > 0 {
		selectedNode = manifest.Nodes[0].Name
	}

	startHeight, startErr := parseExpiryHeight(startRaw, 0)
	endHeight, endErr := parseExpiryHeight(endRaw, -1)
	limit, limitErr := parseExpiryLimit(limitRaw)

	view := struct {
		RefreshSeconds int
		Nodes          []devnetNode
		SelectedNode   string
		StartValue     string
		EndValue       string
		LimitValue     string
		Result         *devnetExpiryIndexData
		Warnings       []string
		Error          string
	}{
		RefreshSeconds: maxInt(1, int(s.refresh/time.Second)),
		Nodes:          manifest.Nodes,
		SelectedNode:   selectedNode,
		StartValue:     startRaw,
		EndValue:       endRaw,
		LimitValue:     limitRaw,
		Warnings:       warnings,
	}

	switch {
	case startErr != nil:
		view.Error = startErr.Error()
	case endErr != nil:
		view.Error = endErr.Error()
	case limitErr != nil:
		view.Error = limitErr.Error()
	case len(manifest.Nodes) > 0 && selectedNode != "":
		node, err := findDevnetNode(manifest.Nodes, selectedNode)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Result, err = s.diagnostics.LoadExpiryOrdering(ctx, node, startHeight, endHeight, limit)
			if err != nil {
				view.Error = err.Error()
			}
		}
	}

	if view.StartValue == "" && view.Result != nil {
		view.StartValue = strconv.FormatInt(int64(view.Result.StartHeight), 10)
	}
	if view.EndValue == "" && view.Result != nil {
		view.EndValue = strconv.FormatInt(int64(view.Result.EndHeight), 10)
	}
	if view.LimitValue == "" {
		view.LimitValue = strconv.Itoa(limit)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := devnetExpiryIndexTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var devnetReapTemplate = template.Must(template.New("devnet-reap").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Devnet REAP Monitor</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f1;
      --panel: #ffffff;
      --ink: #182123;
      --muted: #607074;
      --line: #d7dfd8;
      --accent: #2b6c5a;
      --accent-soft: #dff0e8;
      --bad: #b24333;
      --bad-soft: #fbe3dd;
      --mono: "SFMono-Regular", "SF Mono", Menlo, Consolas, monospace;
      --serif: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      background: radial-gradient(circle at top left, #fdf5da, transparent 32%), var(--bg);
      color: var(--ink);
    }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .page { max-width: 1440px; margin: 0 auto; padding: 28px 24px 48px; }
    .hero, .panel { background: var(--panel); border: 1px solid var(--line); border-radius: 18px; box-shadow: 0 20px 50px rgba(24, 33, 35, 0.06); }
    .hero { padding: 24px 26px; }
    .hero h1 { margin: 0; font-family: var(--serif); font-size: 34px; }
    .hero p { margin: 10px 0 0; color: var(--muted); max-width: 78ch; line-height: 1.55; }
    .toolbar { margin-top: 14px; display: flex; flex-wrap: wrap; gap: 10px; }
    .toolbar a { padding: 9px 13px; border-radius: 999px; border: 1px solid var(--line); background: #f9fbfa; }
    .form-panel { margin-top: 18px; padding: 18px; }
    .form-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 14px; align-items: end; }
    label { display: block; font-size: 13px; color: var(--muted); margin-bottom: 6px; }
    input, select, button {
      width: 100%; border: 1px solid var(--line); border-radius: 12px; padding: 11px 12px;
      font: inherit; background: #fff;
    }
    button { cursor: pointer; background: var(--accent); color: #fff; border-color: var(--accent); font-weight: 600; }
    .notice { margin-top: 16px; padding: 12px 14px; border-radius: 12px; font-size: 14px; }
    .notice.warn { background: #fff7d9; border: 1px solid #edd48f; }
    .notice.err { background: var(--bad-soft); border: 1px solid #efc0b6; color: var(--bad); }
    .kpis { margin-top: 18px; display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 14px; }
    .kpi { padding: 16px; border: 1px solid var(--line); border-radius: 16px; background: linear-gradient(180deg, #fcfffd, #f4f8f5); }
    .kpi .label { font-size: 12px; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
    .kpi .value { margin-top: 8px; font-size: 28px; font-weight: 700; }
    .summary { margin-top: 22px; padding: 18px; }
    .summary table, .detail table { width: 100%; border-collapse: collapse; }
    .summary th, .summary td, .detail th, .detail td { text-align: left; padding: 10px 10px; border-top: 1px solid var(--line); vertical-align: top; }
    .summary thead th, .detail thead th { border-top: none; color: var(--muted); font-size: 12px; letter-spacing: 0.05em; text-transform: uppercase; }
    .mono { font-family: var(--mono); font-size: 12px; overflow-wrap: anywhere; }
    .tag { display: inline-flex; align-items: center; padding: 4px 9px; border-radius: 999px; font-size: 12px; font-weight: 700; }
    .tag.good { background: var(--accent-soft); color: var(--accent); }
    .tag.empty { background: #ecefed; color: #667173; }
    .detail { margin-top: 18px; padding: 18px; }
    .detail h2 { margin: 0; font-size: 22px; }
    .detail .meta { margin-top: 6px; color: var(--muted); font-size: 14px; }
    .detail + .detail { margin-top: 16px; }
    .metrics { margin-top: 14px; display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
    .metric { padding: 12px; border-radius: 14px; background: #f8faf8; border: 1px solid var(--line); }
    .metric .label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
    .metric .value { margin-top: 6px; font-weight: 700; }
    .muted { color: var(--muted); }
    @media (max-width: 900px) {
      .page { padding: 20px 16px 36px; }
      .hero h1 { font-size: 28px; }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="hero">
      <h1>REAP Block Monitor</h1>
      <p>This page lists the expired UTXOs consumed by REAP transactions block by block and expands each input with its source address, original amount, tax amount, post-tax refund amount, and in-transaction order so you can compare on-chain results with locally derived expectations.</p>
      <div class="toolbar">
        <a href="/">Back to Dashboard</a>
        <a href="/blocks?view=raw">Block List</a>
        <a href="/expiryindex">ExpiryIndex Ordering</a>
      </div>
    </section>

    <section class="panel form-panel">
      <form method="get" action="/reap">
        <div class="form-row">
          <div>
            <label for="node">Node</label>
            <select id="node" name="node">
              {{range .Nodes}}
              <option value="{{.Name}}" {{if eq $.SelectedNode .Name}}selected{{end}}>{{.Name}} | {{.Role}}</option>
              {{end}}
            </select>
          </div>
          <div>
            <label for="count">Recent Block Count</label>
            <input id="count" name="count" value="{{.CountValue}}" inputmode="numeric">
          </div>
          <div>
            <button type="submit">Refresh REAP View</button>
          </div>
        </div>
      </form>

      {{range .Warnings}}<div class="notice warn">{{.}}</div>{{end}}
      {{if .Error}}<div class="notice err">{{.Error}}</div>{{end}}
    </section>

    {{if .Result}}
    <section class="kpis">
      <div class="kpi"><div class="label">Node</div><div class="value">{{.Result.Node.Name}}</div></div>
      <div class="kpi"><div class="label">Best Height</div><div class="value">{{.Result.BestHeight}}</div></div>
      <div class="kpi"><div class="label">Blocks Inspected</div><div class="value">{{.Result.BlocksInspected}}</div></div>
      <div class="kpi"><div class="label">Blocks With REAP</div><div class="value">{{.Result.BlocksWithREAP}}</div></div>
      <div class="kpi"><div class="label">Expired UTXOs Listed</div><div class="value">{{.Result.TotalExpiredUTXO}}</div></div>
    </section>

    <section class="panel summary">
      <table>
        <thead>
          <tr>
            <th>Height</th>
            <th>Block</th>
            <th>Status</th>
            <th>Inputs</th>
            <th>Tax</th>
            <th>Refund</th>
            <th>Check</th>
          </tr>
        </thead>
        <tbody>
          {{range .Result.Blocks}}
          <tr>
            <td>{{.Height}}</td>
            <td><a class="mono" href="{{.BlockLink}}">{{.Hash}}</a></td>
            <td>{{if .HasREAP}}<span class="tag good">REAP</span>{{else}}<span class="tag empty">No REAP</span>{{end}}</td>
            <td>{{.InputCount}}</td>
            <td>{{.ComputedTaxTotal}}</td>
            <td>{{.ComputedRefund}}</td>
            <td>{{if .HasREAP}}{{if .TotalsMatch}}ok{{else}}mismatch{{end}}{{else}}-{{end}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    {{range .Result.Blocks}}
    {{if .HasREAP}}
    <section class="panel detail">
      <h2>Height {{.Height}} | REAP {{.REAPTxID}}</h2>
      <div class="meta">
        <a href="{{.BlockLink}}">View Block JSON</a>
        <span> | Time {{.Timestamp}}</span>
        {{if .MarkerPayload}}<span> | Marker {{.MarkerPayload}}</span>{{end}}
      </div>
      <div class="metrics">
        <div class="metric"><div class="label">Inputs</div><div class="value">{{.InputCount}}</div></div>
        <div class="metric"><div class="label">Computed Tax</div><div class="value">{{.ComputedTaxTotal}}</div></div>
        <div class="metric"><div class="label">Computed Refund</div><div class="value">{{.ComputedRefund}}</div></div>
        <div class="metric"><div class="label">On-chain Refund</div><div class="value">{{.OnChainRefund}}</div></div>
        <div class="metric"><div class="label">Est. Weight</div><div class="value">{{.WeightEstimate}}</div></div>
        <div class="metric"><div class="label">Missing Prevouts</div><div class="value">{{.MissingPrevouts}}</div></div>
      </div>
      <table>
        <thead>
          <tr>
            <th>#</th>
            <th>OutPoint</th>
            <th>Source Address</th>
            <th>Script Type</th>
            <th>Amount</th>
            <th>Tax</th>
            <th>Refund</th>
            <th>Create</th>
            <th>Expiry</th>
          </tr>
        </thead>
        <tbody>
          {{range .Rows}}
          <tr>
            <td>{{.Order}}</td>
            <td class="mono">{{.OutPoint}}</td>
            <td>{{.SourceAddress}}</td>
            <td>{{.ScriptClass}}</td>
            <td>{{.AmountSat}}</td>
            <td>{{.TaxSat}}</td>
            <td>{{.RefundSat}}</td>
            <td>{{.CreateHeight}}</td>
            <td>{{.ExpiryHeight}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </section>
    {{end}}
    {{end}}
    {{end}}
  </main>
</body>
</html>`))

var devnetExpiryIndexTemplate = template.Must(template.New("devnet-expiryindex").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="{{.RefreshSeconds}}">
  <title>OBTC Devnet ExpiryIndex Ordering</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f6fb;
      --panel: #ffffff;
      --ink: #182123;
      --muted: #61707a;
      --line: #d6dce6;
      --accent: #205b84;
      --accent-soft: #e2eef7;
      --picked: #1e6a4e;
      --picked-soft: #def2e8;
      --warn: #8a5d16;
      --warn-soft: #fff5da;
      --mono: "SFMono-Regular", "SF Mono", Menlo, Consolas, monospace;
      --serif: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Avenir Next", "Segoe UI", sans-serif;
      background: radial-gradient(circle at top right, #dbedf9, transparent 28%), var(--bg);
      color: var(--ink);
    }
    a { color: var(--accent); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .page { max-width: 1580px; margin: 0 auto; padding: 28px 24px 48px; }
    .hero, .panel { background: var(--panel); border: 1px solid var(--line); border-radius: 18px; box-shadow: 0 20px 50px rgba(24, 33, 35, 0.06); }
    .hero { padding: 24px 26px; }
    .hero h1 { margin: 0; font-family: var(--serif); font-size: 34px; }
    .hero p { margin: 10px 0 0; color: var(--muted); max-width: 86ch; line-height: 1.55; }
    .toolbar { margin-top: 14px; display: flex; flex-wrap: wrap; gap: 10px; }
    .toolbar a { padding: 9px 13px; border-radius: 999px; border: 1px solid var(--line); background: #f9fbff; }
    .form-panel { margin-top: 18px; padding: 18px; }
    .form-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; align-items: end; }
    label { display: block; font-size: 13px; color: var(--muted); margin-bottom: 6px; }
    input, select, button {
      width: 100%; border: 1px solid var(--line); border-radius: 12px; padding: 11px 12px;
      font: inherit; background: #fff;
    }
    button { cursor: pointer; background: var(--accent); color: #fff; border-color: var(--accent); font-weight: 600; }
    .notice { margin-top: 16px; padding: 12px 14px; border-radius: 12px; font-size: 14px; }
    .notice.warn { background: var(--warn-soft); border: 1px solid #e7c677; color: var(--warn); }
    .notice.err { background: #fbe3dd; border: 1px solid #efc0b6; color: #b24333; }
    .kpis { margin-top: 18px; display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 14px; }
    .kpi { padding: 16px; border: 1px solid var(--line); border-radius: 16px; background: linear-gradient(180deg, #fcfeff, #f2f6fa); }
    .kpi .label { font-size: 12px; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
    .kpi .value { margin-top: 8px; font-size: 28px; font-weight: 700; }
    .note-grid { margin-top: 18px; display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 14px; }
    .note { padding: 15px 16px; border-radius: 16px; border: 1px solid var(--line); background: #fbfcff; }
    .note h2 { margin: 0 0 8px; font-size: 15px; }
    .note p { margin: 0; color: var(--muted); line-height: 1.5; }
    .table-panel { margin-top: 18px; padding: 18px; overflow-x: auto; }
    .table-panel h2 { margin: 0 0 10px; font-size: 22px; }
    table { width: 100%; border-collapse: collapse; min-width: 1080px; }
    th, td { text-align: left; padding: 10px 10px; border-top: 1px solid var(--line); vertical-align: top; }
    thead th { border-top: none; color: var(--muted); font-size: 12px; letter-spacing: 0.05em; text-transform: uppercase; }
    .mono { font-family: var(--mono); font-size: 12px; overflow-wrap: anywhere; }
    .picked { background: linear-gradient(90deg, var(--picked-soft), transparent); }
    .tag { display: inline-flex; align-items: center; padding: 4px 9px; border-radius: 999px; font-size: 12px; font-weight: 700; }
    .tag.good { background: var(--picked-soft); color: var(--picked); }
    .tag.idle { background: #ecefed; color: #667173; }
    @media (max-width: 900px) {
      .page { padding: 20px 16px 36px; }
      .hero h1 { font-size: 28px; }
    }
  </style>
</head>
<body>
  <main class="page">
    <section class="hero">
      <h1>ExpiryIndex Ordering Validator</h1>
      <p>This page expands the current <code>listexpiring</code> results in two orders at once: the actual ExpiryIndex or RPC return order, and a deterministic preview of the REAP strict selector ordering. This makes scan order, reordering, and current-page input priority visible at a glance.</p>
      <div class="toolbar">
        <a href="/">Back to Dashboard</a>
        <a href="/reap">REAP Monitor</a>
        <a href="/blocks?view=raw">Block List</a>
      </div>
    </section>

    <section class="panel form-panel">
      <form method="get" action="/expiryindex">
        <div class="form-row">
          <div>
            <label for="node">Node</label>
            <select id="node" name="node">
              {{range .Nodes}}
              <option value="{{.Name}}" {{if eq $.SelectedNode .Name}}selected{{end}}>{{.Name}} | {{.Role}}</option>
              {{end}}
            </select>
          </div>
          <div>
            <label for="start">start height</label>
            <input id="start" name="start" value="{{.StartValue}}" inputmode="numeric">
          </div>
          <div>
            <label for="end">end height</label>
            <input id="end" name="end" value="{{.EndValue}}" inputmode="numeric">
          </div>
          <div>
            <label for="limit">limit</label>
            <input id="limit" name="limit" value="{{.LimitValue}}" inputmode="numeric">
          </div>
          <div>
            <button type="submit">Refresh Ordering View</button>
          </div>
        </div>
      </form>

      {{range .Warnings}}<div class="notice warn">{{.}}</div>{{end}}
      {{if .Error}}<div class="notice err">{{.Error}}</div>{{end}}
    </section>

    {{if .Result}}
    <section class="kpis">
      <div class="kpi"><div class="label">Node</div><div class="value">{{.Result.Node.Name}}</div></div>
      <div class="kpi"><div class="label">Tip Height</div><div class="value">{{.Result.TipHeight}}</div></div>
      <div class="kpi"><div class="label">Returned Rows</div><div class="value">{{.Result.Returned}}</div></div>
      <div class="kpi"><div class="label">Preview Picked</div><div class="value">{{.Result.PreviewPicked}}</div></div>
      <div class="kpi"><div class="label">Max Inputs</div><div class="value">{{.Result.MaxInputs}}</div></div>
      <div class="kpi"><div class="label">Weight Budget</div><div class="value">{{.Result.WeightBudget}}</div></div>
    </section>

    <section class="note-grid">
      <article class="note">
        <h2>ExpiryIndex / RPC Order</h2>
        <p class="mono">{{.Result.ScanOrderDescription}}</p>
      </article>
      <article class="note">
        <h2>REAP Strict Order</h2>
        <p class="mono">{{.Result.StrictOrderDescription}}</p>
      </article>
      {{if .Result.Truncated}}
      <article class="note">
        <h2>Pagination Notice</h2>
        <p class="mono">{{.Result.NextCursor}}</p>
      </article>
      {{end}}
    </section>

    <section class="panel table-panel">
      <h2>RPC Scan Order</h2>
      <table>
        <thead>
          <tr>
            <th>Scan</th>
            <th>Strict</th>
            <th>Pick</th>
            <th>Expiry</th>
            <th>Create</th>
            <th>Blocks Left</th>
            <th>Amount</th>
            <th>Address</th>
            <th>OutPoint</th>
          </tr>
        </thead>
        <tbody>
          {{range .Result.ScanRows}}
          <tr class="{{if .Picked}}picked{{end}}">
            <td>{{.ScanRank}}</td>
            <td>{{.StrictRank}}</td>
            <td>{{if .Picked}}<span class="tag good">picked</span>{{else}}<span class="tag idle">idle</span>{{end}}</td>
            <td>{{.ExpiryHeight}}</td>
            <td>{{.CreateHeight}}</td>
            <td>{{.BlocksToExpiry}}</td>
            <td>{{.AmountSat}}</td>
            <td>{{.Address}}<div class="mono">{{.ScriptClass}}</div></td>
            <td class="mono">{{.OutPoint}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </section>

    <section class="panel table-panel">
      <h2>REAP Strict Preview</h2>
      <table>
        <thead>
          <tr>
            <th>Strict</th>
            <th>Scan</th>
            <th>Pick</th>
            <th>Expiry</th>
            <th>Amount</th>
            <th>Selector Hash</th>
            <th>Address</th>
            <th>OutPoint</th>
          </tr>
        </thead>
        <tbody>
          {{range .Result.StrictRows}}
          <tr class="{{if .Picked}}picked{{end}}">
            <td>{{.StrictRank}}</td>
            <td>{{.ScanRank}}</td>
            <td>{{if .Picked}}<span class="tag good">picked</span>{{else}}<span class="tag idle">idle</span>{{end}}</td>
            <td>{{.ExpiryHeight}}</td>
            <td>{{.AmountSat}}</td>
            <td class="mono">{{.SelectorHashHex}}</td>
            <td>{{.Address}}<div class="mono">{{.ScriptClass}}</div></td>
            <td class="mono">{{.OutPoint}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </section>
    {{end}}
  </main>
</body>
</html>`))
