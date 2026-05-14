// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type config struct {
	RPCHost            string
	RPCUser            string
	RPCPass            string
	RPCCert            string
	Network            string
	StartHeight        int32
	EndHeight          int32
	EndSet             bool
	Verbose            bool
	JSON               bool
	OutputFile         string
	MaxIssues          int
	CheckReapSelection bool
}

type trackedUTXO struct {
	OutPoint    wire.OutPoint
	Value       int64
	PkScript    []byte
	BlockHeight int32
	IsCoinbase  bool
}

type auditIssue struct {
	Category  string `json:"category"`
	Height    int32  `json:"height"`
	BlockHash string `json:"block_hash"`
	TxID      string `json:"txid,omitempty"`
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
}

type heavyReapBlock struct {
	Height int32  `json:"height"`
	Inputs int    `json:"inputs"`
	TxID   string `json:"txid"`
}

type auditSummary struct {
	Network             string           `json:"network"`
	StartHeight         int32            `json:"start_height"`
	EndHeight           int32            `json:"end_height"`
	TipHeight           int32            `json:"tip_height"`
	AuditedBlocks       int              `json:"audited_blocks"`
	ReapBlocks          int              `json:"reap_blocks"`
	MaxReapInputs       int              `json:"max_reap_inputs"`
	MaxBlockTxs         int              `json:"max_block_txs"`
	IssueCount          int              `json:"issue_count"`
	ConsensusIssueCount int              `json:"consensus_issue_count"`
	AuditIssueCount     int              `json:"audit_issue_count"`
	HeavyReapBlocks     []heavyReapBlock `json:"heavy_reap_blocks,omitempty"`
}

type auditReport struct {
	Summary     auditSummary `json:"summary"`
	FirstIssues []auditIssue `json:"first_issues,omitempty"`
}

type replayAuditor struct {
	cfg          *config
	net          *chaincfg.Params
	expiryParams *chaincfg.ExpiryParams
	reapParams   reap.REAPParams
	liveUTXOs    map[wire.OutPoint]*trackedUTXO
	accumulator  *expiryindex.MuHash
	report       auditReport
	prevHash     *chainhash.Hash
}

type reapCandidate struct {
	OutPoint wire.OutPoint
	Expiry   uint64
	Amount   int64
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fatalf("configuration error: %v", err)
	}

	net, err := resolveNetwork(cfg.Network)
	if err != nil {
		fatalf("resolve network: %v", err)
	}

	client, err := createRPCClient(cfg)
	if err != nil {
		fatalf("create RPC client: %v", err)
	}
	defer client.Shutdown()

	tipHeight, err := client.GetBlockCount()
	if err != nil {
		fatalf("get block count: %v", err)
	}
	if !cfg.EndSet {
		cfg.EndHeight = int32(tipHeight)
	}
	if cfg.EndHeight > int32(tipHeight) {
		fatalf("requested end height %d exceeds tip %d", cfg.EndHeight, tipHeight)
	}
	if cfg.StartHeight < 0 || cfg.StartHeight > cfg.EndHeight {
		fatalf("invalid height range start=%d end=%d", cfg.StartHeight, cfg.EndHeight)
	}

	report, err := runAudit(client, cfg, net, int32(tipHeight))
	if err != nil {
		fatalf("run audit: %v", err)
	}

	if cfg.OutputFile != "" {
		if err := writeJSONReport(cfg.OutputFile, report); err != nil {
			fatalf("write JSON report: %v", err)
		}
	}

	if cfg.JSON {
		printJSON(report)
	} else {
		printText(report)
	}

	if report.Summary.IssueCount > 0 {
		os.Exit(1)
	}
}

func parseFlags() (*config, error) {
	cfg := &config{}
	var startHeight int
	var endHeight int

	flag.StringVar(&cfg.RPCHost, "rpchost", "", "RPC server host:port (auto-detected if empty)")
	flag.StringVar(&cfg.RPCUser, "rpcuser", "", "RPC username")
	flag.StringVar(&cfg.RPCPass, "rpcpass", "", "RPC password")
	flag.StringVar(&cfg.RPCCert, "rpccert", "", "RPC TLS certificate path")
	flag.StringVar(&cfg.Network, "network", "obtcregtest", "network: obtcregtest|obtctestnet|obtcmainnet|regtest|testnet|mainnet")
	flag.IntVar(&startHeight, "start", 0, "first block height to count in the audit report")
	flag.IntVar(&endHeight, "end", -1, "last block height to audit (defaults to tip)")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "print progress to stderr")
	flag.BoolVar(&cfg.JSON, "json", false, "print JSON report to stdout")
	flag.StringVar(&cfg.OutputFile, "output", "", "write JSON report to file")
	flag.IntVar(&cfg.MaxIssues, "max-issues", 20, "maximum issues to keep in the report body")
	flag.BoolVar(&cfg.CheckReapSelection, "check-reap-selection", false, "also enforce the current deterministic REAP selection policy")
	flag.Parse()

	if cfg.RPCUser == "" || cfg.RPCPass == "" {
		return nil, errors.New("rpcuser and rpcpass are required")
	}
	if cfg.MaxIssues < 0 {
		return nil, fmt.Errorf("max-issues must be >= 0")
	}
	if endHeight >= 0 {
		cfg.EndHeight = int32(endHeight)
		cfg.EndSet = true
	}
	cfg.StartHeight = int32(startHeight)
	if cfg.RPCHost == "" {
		switch strings.ToLower(cfg.Network) {
		case "obtcregtest":
			cfg.RPCHost = "127.0.0.1:29528"
		case "obtctestnet":
			cfg.RPCHost = "127.0.0.1:19528"
		case "obtcmainnet":
			cfg.RPCHost = "127.0.0.1:9528"
		case "regtest":
			cfg.RPCHost = "127.0.0.1:18334"
		case "testnet":
			cfg.RPCHost = "127.0.0.1:18334"
		case "mainnet":
			cfg.RPCHost = "127.0.0.1:8334"
		default:
			cfg.RPCHost = "127.0.0.1:18556"
		}
	}

	return cfg, nil
}

func resolveNetwork(name string) (*chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "obtcregtest", "obtc-regtest":
		return &chaincfg.ObtcRegTestParams, nil
	case "obtctestnet", "obtc-testnet":
		return &chaincfg.ObtcTestNetParams, nil
	case "obtcmainnet", "obtc-mainnet":
		return &chaincfg.ObtcMainNetParams, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	case "testnet", "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	default:
		return nil, fmt.Errorf("unsupported network %q", name)
	}
}

func createRPCClient(cfg *config) (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.RPCHost,
		User:         cfg.RPCUser,
		Pass:         cfg.RPCPass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	if cfg.RPCCert != "" {
		cert, err := os.ReadFile(filepath.Clean(cfg.RPCCert))
		if err != nil {
			return nil, fmt.Errorf("read RPC cert: %w", err)
		}
		connCfg.DisableTLS = false
		connCfg.Certificates = cert
	}
	return rpcclient.New(connCfg, nil)
}

func runAudit(client *rpcclient.Client, cfg *config, net *chaincfg.Params, tipHeight int32) (*auditReport, error) {
	auditor := &replayAuditor{
		cfg:          cfg,
		net:          net,
		expiryParams: chaincfg.GetExpiryParams(net),
		reapParams:   reap.DefaultREAPParamsForNet(net, reap.SortModeStrict),
		liveUTXOs:    make(map[wire.OutPoint]*trackedUTXO),
		accumulator:  expiryindex.NewMuHash(),
		report: auditReport{
			Summary: auditSummary{
				Network:     net.Name,
				StartHeight: cfg.StartHeight,
				EndHeight:   cfg.EndHeight,
				TipHeight:   tipHeight,
			},
		},
	}

	for height := int32(0); height <= cfg.EndHeight; height++ {
		if cfg.Verbose && (height == 0 || height == cfg.EndHeight || height%100 == 0) {
			fmt.Fprintf(os.Stderr, "[audit] replaying block %d/%d\n", height, cfg.EndHeight)
		}

		blockHash, err := client.GetBlockHash(int64(height))
		if err != nil {
			return nil, fmt.Errorf("get block hash at %d: %w", height, err)
		}
		block, err := client.GetBlock(blockHash)
		if err != nil {
			return nil, fmt.Errorf("get block %s: %w", blockHash, err)
		}

		auditor.auditBlock(block, height, height >= cfg.StartHeight)
	}

	return &auditor.report, nil
}

func (a *replayAuditor) auditBlock(block *wire.MsgBlock, height int32, countInReport bool) {
	blockHash := block.BlockHash()
	blockHashStr := blockHash.String()

	if countInReport {
		a.report.Summary.AuditedBlocks++
		if len(block.Transactions) > a.report.Summary.MaxBlockTxs {
			a.report.Summary.MaxBlockTxs = len(block.Transactions)
		}
	}

	if a.prevHash != nil && block.Header.PrevBlock != *a.prevHash {
		a.addIssue("consensus", height, blockHashStr, "", "prev_hash_mismatch",
			fmt.Sprintf("got prev=%s want prev=%s", block.Header.PrevBlock, *a.prevHash))
	}
	a.prevHash = &blockHash

	if len(block.Transactions) == 0 {
		a.addIssue("consensus", height, blockHashStr, "", "missing_coinbase",
			"block has no transactions")
		return
	}
	if !blockchain.IsCoinBaseTx(block.Transactions[0]) {
		a.addIssue("consensus", height, blockHashStr, block.Transactions[0].TxHash().String(),
			"first_tx_not_coinbase", "first transaction is not coinbase")
	}
	for i := 1; i < len(block.Transactions); i++ {
		if blockchain.IsCoinBaseTx(block.Transactions[i]) {
			a.addIssue("consensus", height, blockHashStr, block.Transactions[i].TxHash().String(),
				"multiple_coinbase", fmt.Sprintf("extra coinbase found at index %d", i))
		}
	}

	preStateRoot := a.accumulator.Digest()
	a.auditExpiryCommitment(block.Transactions[0], height, blockHashStr, preStateRoot)

	expectedReapInputs := a.expectedReapInputs(height)
	reapCount := 0
	for i, tx := range block.Transactions {
		txHashStr := tx.TxHash().String()
		isReap := reap.IsLikelyREAPTx(tx)
		if i > 0 && isReap {
			reapCount++
			if countInReport {
				a.report.Summary.ReapBlocks++
				if len(tx.TxIn) > a.report.Summary.MaxReapInputs {
					a.report.Summary.MaxReapInputs = len(tx.TxIn)
				}
				if len(tx.TxIn) == a.reapParams.MaxInputs {
					a.report.Summary.HeavyReapBlocks = append(a.report.Summary.HeavyReapBlocks, heavyReapBlock{
						Height: height,
						Inputs: len(tx.TxIn),
						TxID:   txHashStr,
					})
				}
			}
		}
		if reapCount > 1 {
			a.addIssue("consensus", height, blockHashStr, txHashStr,
				"multiple_reap_txs", "block contains more than one REAP transaction")
		}

		a.auditTransaction(tx, height, blockHashStr)
		if isReap {
			a.auditReapTx(tx, height, blockHashStr, expectedReapInputs)
		}
		a.applyTransaction(tx, height)
	}
}

func (a *replayAuditor) auditExpiryCommitment(coinbase *wire.MsgTx, height int32, blockHash string,
	expectedRoot [expiryindex.AccumulatorDigestSize]byte) {

	if a.expiryParams == nil || height < a.expiryParams.ExpiryCommitmentEnableAtHeight {
		return
	}

	coinbaseTx := btcutil.NewTx(coinbase)
	count := expiryindex.CountExpiryCommitments(coinbaseTx)
	if count != 1 {
		a.addIssue("consensus", height, blockHash, coinbase.TxHash().String(),
			"expiry_commitment_count", fmt.Sprintf("expected exactly 1 expiry commitment, got %d", count))
		return
	}

	version, root, found := expiryindex.ExtractExpiryCommitment(coinbaseTx)
	if !found {
		a.addIssue("consensus", height, blockHash, coinbase.TxHash().String(),
			"missing_expiry_commitment", "coinbase is missing the required expiry commitment")
		return
	}
	if version != expiryindex.ExpiryCommitmentVersion {
		a.addIssue("consensus", height, blockHash, coinbase.TxHash().String(),
			"expiry_commitment_version", fmt.Sprintf("got version=%d want version=%d",
				version, expiryindex.ExpiryCommitmentVersion))
	}
	if root != expectedRoot {
		a.addIssue("consensus", height, blockHash, coinbase.TxHash().String(),
			"expiry_commitment_root", fmt.Sprintf("got=%x want=%x", root, expectedRoot))
	}
}

func (a *replayAuditor) auditTransaction(tx *wire.MsgTx, height int32, blockHash string) {
	if blockchain.IsCoinBaseTx(tx) {
		return
	}

	isReap := reap.IsLikelyREAPTx(tx)
	for _, txIn := range tx.TxIn {
		utxo, ok := a.liveUTXOs[txIn.PreviousOutPoint]
		if !ok {
			a.addIssue("consensus", height, blockHash, tx.TxHash().String(),
				"missing_prevout", fmt.Sprintf("missing prevout %s", txIn.PreviousOutPoint))
			continue
		}

		if utxo.IsCoinbase {
			blocksSincePrev := height - utxo.BlockHeight
			if blocksSincePrev < int32(a.net.CoinbaseMaturity) {
				a.addIssue("consensus", height, blockHash, tx.TxHash().String(),
					"immature_coinbase_spend", fmt.Sprintf("prevout %s from height %d spent at height %d before maturity %d",
						txIn.PreviousOutPoint, utxo.BlockHeight, height, a.net.CoinbaseMaturity))
			}
		}

		expired := a.isExpiredAtHeight(utxo, height)
		if isReap && !expired {
			a.addIssue("consensus", height, blockHash, tx.TxHash().String(),
				"reap_spends_live_utxo", fmt.Sprintf("prevout %s is not expired", txIn.PreviousOutPoint))
		}
		if !isReap && expired {
			a.addIssue("consensus", height, blockHash, tx.TxHash().String(),
				"nonreap_spends_expired_utxo", fmt.Sprintf("prevout %s is expired", txIn.PreviousOutPoint))
		}
	}
}

func (a *replayAuditor) auditReapTx(tx *wire.MsgTx, height int32, blockHash string, expectedInputs []wire.OutPoint) {
	txHashStr := tx.TxHash().String()
	if a.expiryParams == nil {
		return
	}

	if height >= a.expiryParams.ReapConsensusAtHeight && a.expiryParams.ReapMaxInputs > 0 &&
		len(tx.TxIn) > a.expiryParams.ReapMaxInputs {
		a.addIssue("consensus", height, blockHash, txHashStr,
			"reap_max_inputs", fmt.Sprintf("got %d inputs want <= %d", len(tx.TxIn), a.expiryParams.ReapMaxInputs))
	}

	if len(tx.TxOut) == 0 {
		a.addIssue("consensus", height, blockHash, txHashStr,
			"reap_missing_marker", "REAP transaction has no outputs")
		return
	}
	payload, ok := reap.ExtractMarkerPayload(tx.TxOut[len(tx.TxOut)-1].PkScript)
	if !ok {
		a.addIssue("consensus", height, blockHash, txHashStr,
			"reap_marker_parse", "unable to decode REAP marker output")
	} else {
		markerHeight, markerCount, markerDigest, err := parseReapMarker(payload)
		if err != nil {
			a.addIssue("consensus", height, blockHash, txHashStr,
				"reap_marker_payload", err.Error())
		} else {
			if markerHeight != height {
				a.addIssue("consensus", height, blockHash, txHashStr,
					"reap_marker_height", fmt.Sprintf("got %d want %d", markerHeight, height))
			}
			if markerCount != len(tx.TxIn) {
				a.addIssue("consensus", height, blockHash, txHashStr,
					"reap_marker_count", fmt.Sprintf("got %d want %d", markerCount, len(tx.TxIn)))
			}
			if markerDigest != reap.MarkerDigest(outPointsFromTx(tx)) {
				a.addIssue("consensus", height, blockHash, txHashStr,
					"reap_marker_digest", fmt.Sprintf("got %s want %s", markerDigest, reap.MarkerDigest(outPointsFromTx(tx))))
			}
		}
	}

	if height >= a.expiryParams.ReapConsensusAtHeight {
		for i := 1; i < len(tx.TxIn); i++ {
			prev := tx.TxIn[i-1].PreviousOutPoint
			cur := tx.TxIn[i].PreviousOutPoint
			if compareCandidates(a.makeCandidate(prev), a.makeCandidate(cur)) > 0 {
				a.addIssue("consensus", height, blockHash, txHashStr,
					"reap_input_order", fmt.Sprintf("inputs out of canonical order at positions %d and %d", i-1, i))
				break
			}
		}
	}

	if a.cfg.CheckReapSelection {
		actualInputs := outPointsFromTx(tx)
		if !sameOutPoints(actualInputs, expectedInputs) {
			a.addIssue("audit", height, blockHash, txHashStr,
				"reap_selection_mismatch", fmt.Sprintf("actual_inputs=%d expected_inputs=%d", len(actualInputs), len(expectedInputs)))
		}
	}

	expectedRefunds := make(map[string]int64)
	var expectedRefundTotal int64
	for _, txIn := range tx.TxIn {
		utxo, ok := a.liveUTXOs[txIn.PreviousOutPoint]
		if !ok {
			continue
		}
		tax := reapTax(utxo.Value, a.expiryParams)
		refund := utxo.Value - tax
		refund, _ = applyDustRule(utxo.Value, refund, tax, a.expiryParams)
		if refund > 0 {
			expectedRefunds[string(utxo.PkScript)] += refund
		}
		expectedRefundTotal += refund
	}

	actualRefunds := make(map[string]int64)
	var actualRefundTotal int64
	for _, txOut := range tx.TxOut[:len(tx.TxOut)-1] {
		if txOut.Value <= 0 {
			a.addIssue("consensus", height, blockHash, txHashStr,
				"reap_refund_output_value", "refund outputs must be positive")
		}
		actualRefunds[string(txOut.PkScript)] += txOut.Value
		actualRefundTotal += txOut.Value
	}
	if actualRefundTotal != expectedRefundTotal {
		a.addIssue("consensus", height, blockHash, txHashStr,
			"reap_refund_total", fmt.Sprintf("got %d want %d", actualRefundTotal, expectedRefundTotal))
	}
	if len(actualRefunds) != len(expectedRefunds) {
		a.addIssue("consensus", height, blockHash, txHashStr,
			"reap_refund_set_size", fmt.Sprintf("got %d want %d", len(actualRefunds), len(expectedRefunds)))
	} else {
		for script, expected := range expectedRefunds {
			if actualRefunds[script] != expected {
				a.addIssue("consensus", height, blockHash, txHashStr,
					"reap_refund_distribution", fmt.Sprintf("script=%x got=%d want=%d", []byte(script), actualRefunds[script], expected))
				break
			}
		}
	}
}

func (a *replayAuditor) applyTransaction(tx *wire.MsgTx, height int32) {
	if !blockchain.IsCoinBaseTx(tx) {
		for _, txIn := range tx.TxIn {
			if utxo, ok := a.liveUTXOs[txIn.PreviousOutPoint]; ok {
				delete(a.liveUTXOs, txIn.PreviousOutPoint)
				if a.expiryParams != nil {
					a.accumulator.Remove(encodeAccumulatorEntry(utxo.OutPoint, a.expiryParams.CalculateExpiryKey(utxo.BlockHeight)))
				}
			}
		}
	}

	isCoinbase := blockchain.IsCoinBaseTx(tx)
	txHash := tx.TxHash()
	for i, txOut := range tx.TxOut {
		if txscript.IsUnspendable(txOut.PkScript) {
			continue
		}

		op := wire.OutPoint{Hash: txHash, Index: uint32(i)}
		if height == 0 && isCoinbase {
			if a.expiryParams != nil {
				a.accumulator.Add(encodeAccumulatorEntry(op, a.expiryParams.CalculateExpiryKey(height)))
			}
			continue
		}

		a.liveUTXOs[op] = &trackedUTXO{
			OutPoint:    op,
			Value:       txOut.Value,
			PkScript:    append([]byte(nil), txOut.PkScript...),
			BlockHeight: height,
			IsCoinbase:  isCoinbase,
		}
		if a.expiryParams != nil {
			a.accumulator.Add(encodeAccumulatorEntry(op, a.expiryParams.CalculateExpiryKey(height)))
		}
	}
}

func (a *replayAuditor) expectedReapInputs(height int32) []wire.OutPoint {
	if a.expiryParams == nil || height < a.expiryParams.EnableAtHeight {
		return nil
	}

	candidates := make([]reapCandidate, 0, len(a.liveUTXOs))
	for _, utxo := range a.liveUTXOs {
		if !a.isExpiredAtHeight(utxo, height) {
			continue
		}
		candidates = append(candidates, reapCandidate{
			OutPoint: utxo.OutPoint,
			Expiry:   a.expiryParams.CalculateExpiryKey(utxo.BlockHeight),
			Amount:   utxo.Value,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareCandidates(candidates[i], candidates[j]) < 0
	})

	selected := make([]wire.OutPoint, 0, len(candidates))
	for _, candidate := range candidates {
		if a.reapParams.MaxInputs > 0 && len(selected) >= a.reapParams.MaxInputs {
			break
		}
		if a.reapParams.WeightBudget > 0 &&
			reap.EstimateBlueprintWeight(len(selected)+1) > a.reapParams.WeightBudget {
			break
		}
		selected = append(selected, candidate.OutPoint)
	}
	return selected
}

func (a *replayAuditor) isExpiredAtHeight(utxo *trackedUTXO, spendHeight int32) bool {
	if utxo == nil || a.expiryParams == nil || spendHeight < a.expiryParams.EnableAtHeight {
		return false
	}
	return spendHeight >= int32(a.expiryParams.CalculateExpiryKey(utxo.BlockHeight))
}

func (a *replayAuditor) makeCandidate(op wire.OutPoint) reapCandidate {
	utxo := a.liveUTXOs[op]
	if utxo == nil {
		return reapCandidate{OutPoint: op}
	}
	return reapCandidate{
		OutPoint: op,
		Expiry:   a.expiryParams.CalculateExpiryKey(utxo.BlockHeight),
		Amount:   utxo.Value,
	}
}

func (a *replayAuditor) addIssue(category string, height int32, blockHash, txid, kind, detail string) {
	a.report.Summary.IssueCount++
	switch category {
	case "consensus":
		a.report.Summary.ConsensusIssueCount++
	default:
		a.report.Summary.AuditIssueCount++
	}
	if len(a.report.FirstIssues) < a.cfg.MaxIssues {
		a.report.FirstIssues = append(a.report.FirstIssues, auditIssue{
			Category:  category,
			Height:    height,
			BlockHash: blockHash,
			TxID:      txid,
			Kind:      kind,
			Detail:    detail,
		})
	}
}

func compareCandidates(a, b reapCandidate) int {
	if a.Expiry != b.Expiry {
		if a.Expiry < b.Expiry {
			return -1
		}
		return 1
	}
	if a.Amount != b.Amount {
		if a.Amount < b.Amount {
			return -1
		}
		return 1
	}
	hcmp := bytes.Compare(a.OutPoint.Hash[:], b.OutPoint.Hash[:])
	if hcmp != 0 {
		return hcmp
	}
	switch {
	case a.OutPoint.Index < b.OutPoint.Index:
		return -1
	case a.OutPoint.Index > b.OutPoint.Index:
		return 1
	default:
		return 0
	}
}

func parseReapMarker(payload string) (int32, int, string, error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 4 || parts[0] != "REAP" {
		return 0, 0, "", fmt.Errorf("invalid REAP marker payload %q", payload)
	}
	height, err := parseInt32(parts[1])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker height: %w", err)
	}
	count, err := parseInt(parts[2])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker count: %w", err)
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker digest: %w", err)
	}
	return height, count, parts[3], nil
}

func parseInt32(s string) (int32, error) {
	v, err := parseInt(s)
	return int32(v), err
}

func parseInt(s string) (int, error) {
	var out int
	_, err := fmt.Sscanf(s, "%d", &out)
	return out, err
}

func outPointsFromTx(tx *wire.MsgTx) []wire.OutPoint {
	out := make([]wire.OutPoint, 0, len(tx.TxIn))
	for _, txIn := range tx.TxIn {
		out = append(out, txIn.PreviousOutPoint)
	}
	return out
}

func sameOutPoints(a, b []wire.OutPoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func reapTax(value int64, params *chaincfg.ExpiryParams) int64 {
	if params == nil || value <= 0 || params.ReapTaxNumerator <= 0 || params.ReapTaxDenominator <= 0 {
		return 0
	}
	return (value * params.ReapTaxNumerator) / params.ReapTaxDenominator
}

func applyDustRule(value, refund, tax int64, params *chaincfg.ExpiryParams) (int64, int64) {
	if params == nil || params.ReapDustThresholdSat <= 0 {
		return refund, tax
	}
	if value > 0 && value < params.ReapDustThresholdSat {
		return 0, value
	}
	return refund, tax
}

func encodeAccumulatorEntry(op wire.OutPoint, expiryKey uint64) []byte {
	data := make([]byte, 44)
	copy(data[0:32], op.Hash[:])
	binary.LittleEndian.PutUint32(data[32:36], op.Index)
	binary.BigEndian.PutUint64(data[36:44], expiryKey)
	return data
}

func printText(report *auditReport) {
	fmt.Printf("network=%s\n", report.Summary.Network)
	fmt.Printf("range=%d..%d\n", report.Summary.StartHeight, report.Summary.EndHeight)
	fmt.Printf("tip_height=%d\n", report.Summary.TipHeight)
	fmt.Printf("audited_blocks=%d\n", report.Summary.AuditedBlocks)
	fmt.Printf("reap_blocks=%d\n", report.Summary.ReapBlocks)
	fmt.Printf("max_reap_inputs=%d\n", report.Summary.MaxReapInputs)
	fmt.Printf("max_block_txs=%d\n", report.Summary.MaxBlockTxs)
	fmt.Printf("issue_count=%d\n", report.Summary.IssueCount)
	fmt.Printf("consensus_issue_count=%d\n", report.Summary.ConsensusIssueCount)
	fmt.Printf("audit_issue_count=%d\n", report.Summary.AuditIssueCount)
	if len(report.FirstIssues) == 0 {
		fmt.Println("status=pass")
		return
	}
	fmt.Println("status=fail")
	fmt.Println("first_issues:")
	for _, issue := range report.FirstIssues {
		fmt.Printf("- [%s] height=%d block=%s tx=%s kind=%s detail=%s\n",
			issue.Category, issue.Height, issue.BlockHash, issue.TxID, issue.Kind, issue.Detail)
	}
}

func printJSON(report *auditReport) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatalf("marshal JSON report: %v", err)
	}
	fmt.Println(string(payload))
}

func writeJSONReport(path string, report *auditReport) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), payload, 0o644)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "replay_block_audit: "+format+"\n", args...)
	os.Exit(1)
}
