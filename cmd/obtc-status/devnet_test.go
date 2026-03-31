package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type fakeNodeSnapshotSource struct {
	snapshots map[string]*statusSnapshot
	errs      map[string]error
}

func (f *fakeNodeSnapshotSource) Snapshot(ctx context.Context, node devnetNode) (*statusSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err, ok := f.errs[node.Name]; ok {
		return nil, err
	}
	if snapshot, ok := f.snapshots[node.Name]; ok {
		return snapshot, nil
	}
	return nil, nil
}

type fakeDevnetActionRunner struct {
	lastAction string
	lastArgs   []string
	result     devnetActionResult
}

func (f *fakeDevnetActionRunner) Run(ctx context.Context, action string, spec devnetActionSpec) devnetActionResult {
	f.lastAction = action
	f.lastArgs = append([]string(nil), spec.Args...)
	result := f.result
	result.Action = action
	result.Args = append([]string(nil), spec.Args...)
	if result.At.IsZero() {
		result.At = time.Now().UTC()
	}
	return result
}

type fakeDevnetBlockFetcher struct {
	lastNode     devnetNode
	lastHeight   string
	lastHash     string
	lastMode     devnetBlockViewMode
	lastListFor  devnetNode
	lastLimit    int
	lastListMode devnetBlockViewMode
	result       *devnetBlockResult
	listResult   []devnetBlockListItem
	err          error
	listErr      error
}

func (f *fakeDevnetBlockFetcher) FetchBlock(ctx context.Context, node devnetNode, height, hash string, mode devnetBlockViewMode) (*devnetBlockResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.lastNode = node
	f.lastHeight = height
	f.lastHash = hash
	f.lastMode = mode
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeDevnetBlockFetcher) FetchRecentBlocks(ctx context.Context, node devnetNode, limit int, mode devnetBlockViewMode) ([]devnetBlockListItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.lastListFor = node
	f.lastLimit = limit
	f.lastListMode = mode
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResult, nil
}

type fakeDevnetDiagnosticsSource struct {
	lastReapNode   devnetNode
	lastReapCount  int
	lastExpiryNode devnetNode
	lastStart      int32
	lastEnd        int32
	lastLimit      int
	reap           *devnetReapHistoryData
	expiry         *devnetExpiryIndexData
	reapErr        error
	expiryErr      error
}

func (f *fakeDevnetDiagnosticsSource) LoadReapHistory(ctx context.Context, node devnetNode, count int) (*devnetReapHistoryData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.lastReapNode = node
	f.lastReapCount = count
	if f.reapErr != nil {
		return nil, f.reapErr
	}
	return f.reap, nil
}

func (f *fakeDevnetDiagnosticsSource) LoadExpiryOrdering(ctx context.Context, node devnetNode, startHeight, endHeight int32, limit int) (*devnetExpiryIndexData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.lastExpiryNode = node
	f.lastStart = startHeight
	f.lastEnd = endHeight
	f.lastLimit = limit
	if f.expiryErr != nil {
		return nil, f.expiryErr
	}
	return f.expiry, nil
}

func TestDevnetServerHandlers(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}

	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source: &fakeNodeSnapshotSource{
			snapshots: map[string]*statusSnapshot{
				"node1": makeNodeSnapshot("127.0.0.1:18556", 145, 8),
				"node2": makeNodeSnapshot("127.0.0.1:18557", 145, 5),
				"node3": makeNodeSnapshot("127.0.0.1:18558", 145, 3),
			},
		},
		runner: &fakeDevnetActionRunner{},
	}

	jsonReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	jsonReq.RemoteAddr = "127.0.0.1:33333"
	jsonRec := httptest.NewRecorder()
	server.routes().ServeHTTP(jsonRec, jsonReq)
	if jsonRec.Code != http.StatusOK {
		t.Fatalf("expected json status 200, got %d", jsonRec.Code)
	}
	if !strings.Contains(jsonRec.Body.String(), "\"healthy_nodes\": 3") {
		t.Fatalf("unexpected json body: %s", jsonRec.Body.String())
	}

	htmlReq := httptest.NewRequest(http.MethodGet, "/", nil)
	htmlReq.RemoteAddr = "127.0.0.1:33333"
	htmlRec := httptest.NewRecorder()
	server.routes().ServeHTTP(htmlRec, htmlReq)
	if htmlRec.Code != http.StatusOK {
		t.Fatalf("expected html status 200, got %d", htmlRec.Code)
	}
	if !strings.Contains(htmlRec.Body.String(), "OBTC Devnet Dashboard") ||
		!strings.Contains(htmlRec.Body.String(), "node3") ||
		!strings.Contains(htmlRec.Body.String(), "区块数量") ||
		!strings.Contains(htmlRec.Body.String(), "模式说明") ||
		!strings.Contains(htmlRec.Body.String(), "模拟拥堵下的费率竞争") {
		t.Fatalf("unexpected html body: %s", htmlRec.Body.String())
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthReq.RemoteAddr = "127.0.0.1:33333"
	healthRec := httptest.NewRecorder()
	server.routes().ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected healthz 200, got %d", healthRec.Code)
	}
}

func TestDevnetServerAction(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	runner := &fakeDevnetActionRunner{
		result: devnetActionResult{
			Success: true,
			Output:  "ok",
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        runner,
	}

	form := url.Values{}
	form.Set("action", "mine1")
	req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:44444"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status, got %d", rec.Code)
	}
	if runner.lastAction != "mine1" {
		t.Fatalf("expected mine1 action, got %q", runner.lastAction)
	}

	last := server.getLastAction()
	if last == nil || !last.Success || last.Action != "mine1" {
		t.Fatalf("unexpected last action: %+v", last)
	}
}

func TestDevnetServerCustomSpamAction(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	runner := &fakeDevnetActionRunner{
		result: devnetActionResult{
			Success: true,
			Output:  "spam ok",
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        runner,
	}

	form := url.Values{}
	form.Set("action", "spam-custom")
	form.Set("target", "peer")
	form.Set("mode", "feemarket")
	form.Set("count", "25")
	form.Set("value", "150000")
	form.Set("fee_rate", "15")
	form.Set("prepare", "128")
	form.Set("prepare_value", "250000")
	form.Set("pace_ms", "5")
	form.Set("value_min", "80000")
	form.Set("value_max", "180000")
	form.Set("rand_seed", "42")
	form.Set("randomize_inputs", "true")

	req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:44444"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status, got %d", rec.Code)
	}
	if runner.lastAction != "spam-peer" {
		t.Fatalf("expected spam-peer action, got %q", runner.lastAction)
	}

	wantArgs := []string{
		"spam-peer",
		"--count", "25",
		"--mode", "feemarket",
		"--value", "150000",
		"--fee-rate", "15",
		"--prepare", "128",
		"--prepare-value", "250000",
		"--pace-ms", "5",
		"--value-min", "80000",
		"--value-max", "180000",
		"--randomize-inputs",
		"--rand-seed", "42",
	}
	if strings.Join(runner.lastArgs, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected spam args: got=%v want=%v", runner.lastArgs, wantArgs)
	}
}

func TestDevnetServerCustomMineAction(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	runner := &fakeDevnetActionRunner{
		result: devnetActionResult{
			Success: true,
			Output:  "mine ok",
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        runner,
	}

	form := url.Values{}
	form.Set("action", "mine-custom")
	form.Set("blocks", "12")

	req := httptest.NewRequest(http.MethodPost, "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:44444"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status, got %d", rec.Code)
	}
	if runner.lastAction != "mine 12" {
		t.Fatalf("expected mine 12 action, got %q", runner.lastAction)
	}

	wantArgs := []string{"mine", "12"}
	if strings.Join(runner.lastArgs, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected mine args: got=%v want=%v", runner.lastArgs, wantArgs)
	}
}

func TestDevnetServerBlockPage(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	blockFetcher := &fakeDevnetBlockFetcher{
		result: &devnetBlockResult{
			BlockHash:  "best-hash",
			PrettyJSON: "{\n  \"hash\": \"best-hash\"\n}",
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        &fakeDevnetActionRunner{},
		blockFetcher:  blockFetcher,
	}

	req := httptest.NewRequest(http.MethodGet, "/block?node=node2&height=145&view=raw", nil)
	req.RemoteAddr = "127.0.0.1:55555"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected block page 200, got %d", rec.Code)
	}
	if blockFetcher.lastNode.Name != "node2" {
		t.Fatalf("expected node2 query, got %+v", blockFetcher.lastNode)
	}
	if blockFetcher.lastHeight != "145" {
		t.Fatalf("expected height 145, got %q", blockFetcher.lastHeight)
	}
	if blockFetcher.lastMode != devnetBlockViewModeRaw {
		t.Fatalf("expected raw mode, got %q", blockFetcher.lastMode)
	}
	if !strings.Contains(rec.Body.String(), "best-hash") ||
		!strings.Contains(rec.Body.String(), "区块查看器") {
		t.Fatalf("unexpected block page body: %s", rec.Body.String())
	}
}

func TestDevnetServerBlocksPage(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	blockFetcher := &fakeDevnetBlockFetcher{
		listResult: []devnetBlockListItem{
			{
				Height:     145,
				Hash:       "hash-145",
				PrettyJSON: "{\n  \"height\": 145,\n  \"hash\": \"hash-145\"\n}",
			},
			{
				Height:     144,
				Hash:       "hash-144",
				PrettyJSON: "{\n  \"height\": 144,\n  \"hash\": \"hash-144\"\n}",
			},
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        &fakeDevnetActionRunner{},
		blockFetcher:  blockFetcher,
	}

	req := httptest.NewRequest(http.MethodGet, "/blocks?node=node3&count=2&view=raw", nil)
	req.RemoteAddr = "127.0.0.1:55555"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected blocks page 200, got %d", rec.Code)
	}
	if blockFetcher.lastListFor.Name != "node3" {
		t.Fatalf("expected node3 list query, got %+v", blockFetcher.lastListFor)
	}
	if blockFetcher.lastLimit != 2 {
		t.Fatalf("expected count 2, got %d", blockFetcher.lastLimit)
	}
	if blockFetcher.lastListMode != devnetBlockViewModeRaw {
		t.Fatalf("expected raw list mode, got %q", blockFetcher.lastListMode)
	}
	if !strings.Contains(rec.Body.String(), "区块列表页") ||
		!strings.Contains(rec.Body.String(), "hash-145") ||
		!strings.Contains(rec.Body.String(), "Height 145") {
		t.Fatalf("unexpected blocks page body: %s", rec.Body.String())
	}
}

func TestDevnetServerReapPage(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	diagnostics := &fakeDevnetDiagnosticsSource{
		reap: &devnetReapHistoryData{
			Node:             devnetNode{Name: "node2", Role: "peer"},
			BestHeight:       145,
			BlocksRequested:  3,
			BlocksInspected:  3,
			BlocksWithREAP:   1,
			TotalExpiredUTXO: 2,
			Blocks: []devnetReapBlockData{
				{
					Height:           145,
					Hash:             "hash-145",
					BlockLink:        "/block?node=node2&hash=hash-145&view=raw",
					HasREAP:          true,
					REAPTxID:         "reap-tx",
					MarkerPayload:    "REAP:145:2:abcd",
					InputCount:       2,
					ComputedTaxTotal: 300,
					ComputedRefund:   700,
					OnChainRefund:    700,
					TotalsMatch:      true,
					Rows: []devnetReapInputRow{
						{
							Order:         1,
							OutPoint:      "tx1:0",
							SourceAddress: "oabc123",
							ScriptClass:   "pubkeyhash",
							AmountSat:     1000,
							TaxSat:        300,
							RefundSat:     700,
							CreateHeight:  1,
							ExpiryHeight:  145,
						},
					},
				},
			},
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        &fakeDevnetActionRunner{},
		diagnostics:   diagnostics,
	}

	req := httptest.NewRequest(http.MethodGet, "/reap?node=node2&count=3", nil)
	req.RemoteAddr = "127.0.0.1:55555"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected reap page 200, got %d", rec.Code)
	}
	if diagnostics.lastReapNode.Name != "node2" {
		t.Fatalf("expected node2 reap query, got %+v", diagnostics.lastReapNode)
	}
	if diagnostics.lastReapCount != 3 {
		t.Fatalf("expected count 3, got %d", diagnostics.lastReapCount)
	}
	if !strings.Contains(rec.Body.String(), "REAP 区块观察页") ||
		!strings.Contains(rec.Body.String(), "reap-tx") ||
		!strings.Contains(rec.Body.String(), "oabc123") {
		t.Fatalf("unexpected reap page body: %s", rec.Body.String())
	}
}

func TestDevnetServerExpiryIndexPage(t *testing.T) {
	cfg := &config{
		Devnet:         true,
		DevnetNodes:    3,
		DevnetManifest: filepath.Join(t.TempDir(), "manifest.json"),
		NetworkName:    "obtcregtest",
	}
	rowA := &devnetExpiryIndexRow{
		ScanRank:        1,
		StrictRank:      2,
		Picked:          false,
		OutPoint:        "txA:0",
		TxID:            "txA",
		Vout:            0,
		Address:         "oaddrA",
		ScriptClass:     "pubkeyhash",
		AmountSat:       5000,
		ExpiryHeight:    140,
		CreateHeight:    10,
		BlocksToExpiry:  -5,
		SelectorHashHex: "aaaa",
	}
	rowB := &devnetExpiryIndexRow{
		ScanRank:        2,
		StrictRank:      1,
		Picked:          true,
		OutPoint:        "txB:1",
		TxID:            "txB",
		Vout:            1,
		Address:         "oaddrB",
		ScriptClass:     "witness_v0_keyhash",
		AmountSat:       3000,
		ExpiryHeight:    140,
		CreateHeight:    10,
		BlocksToExpiry:  -5,
		SelectorHashHex: "bbbb",
	}
	diagnostics := &fakeDevnetDiagnosticsSource{
		expiry: &devnetExpiryIndexData{
			Node:                   devnetNode{Name: "node1", Role: "miner"},
			TipHeight:              145,
			StartHeight:            0,
			EndHeight:              145,
			Limit:                  10,
			Returned:               2,
			PreviewPicked:          1,
			MaxInputs:              200,
			WeightBudget:           400000,
			ScanOrderDescription:   "expiry_height -> canonical txid string -> vout",
			StrictOrderDescription: "expiry_height -> amount_sat -> raw hash bytes -> vout",
			ScanRows:               []*devnetExpiryIndexRow{rowA, rowB},
			StrictRows:             []*devnetExpiryIndexRow{rowB, rowA},
		},
	}
	server := &devnetServer{
		cfg:           cfg,
		manifestPath:  cfg.DevnetManifest,
		refresh:       5 * time.Second,
		timeout:       2 * time.Second,
		actionTimeout: time.Minute,
		source:        &fakeNodeSnapshotSource{},
		runner:        &fakeDevnetActionRunner{},
		diagnostics:   diagnostics,
	}

	req := httptest.NewRequest(http.MethodGet, "/expiryindex?node=node1&start=0&end=145&limit=10", nil)
	req.RemoteAddr = "127.0.0.1:55555"

	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected expiry page 200, got %d", rec.Code)
	}
	if diagnostics.lastExpiryNode.Name != "node1" {
		t.Fatalf("expected node1 expiry query, got %+v", diagnostics.lastExpiryNode)
	}
	if diagnostics.lastStart != 0 || diagnostics.lastEnd != 145 || diagnostics.lastLimit != 10 {
		t.Fatalf("unexpected expiry args: start=%d end=%d limit=%d", diagnostics.lastStart, diagnostics.lastEnd, diagnostics.lastLimit)
	}
	if !strings.Contains(rec.Body.String(), "ExpiryIndex 排序验证页") ||
		!strings.Contains(rec.Body.String(), "oaddrB") ||
		!strings.Contains(rec.Body.String(), "picked") {
		t.Fatalf("unexpected expiry page body: %s", rec.Body.String())
	}
}

func TestRenderRawBlockPrettyJSON(t *testing.T) {
	signatureScript, err := txscript.NewScriptBuilder().
		AddData([]byte("hello-signature-payload")).
		Script()
	if err != nil {
		t.Fatalf("build signature script: %v", err)
	}
	pkScript, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("REAP:500:1:abcd")).
		Script()
	if err != nil {
		t.Fatalf("build pk script: %v", err)
	}

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Index: wire.MaxPrevOutIndex,
		},
		SignatureScript: signatureScript,
		Sequence:        wire.MaxTxInSequenceNum,
	})
	coinbase.AddTxOut(&wire.TxOut{
		Value:    50,
		PkScript: pkScript,
	})

	block := wire.NewMsgBlock(&wire.BlockHeader{
		Version:    1,
		Bits:       0x1d00ffff,
		Nonce:      42,
		Timestamp:  time.Unix(1234567890, 0),
		MerkleRoot: coinbase.TxHash(),
	})
	if err := block.AddTransaction(coinbase); err != nil {
		t.Fatalf("add transaction: %v", err)
	}

	var buf bytes.Buffer
	if err := block.Serialize(&buf); err != nil {
		t.Fatalf("serialize block: %v", err)
	}

	jsonText, err := renderRawBlockPrettyJSON(hexString(buf.Bytes()))
	if err != nil {
		t.Fatalf("render raw block json: %v", err)
	}
	if !strings.Contains(jsonText, "\"block_hash\"") ||
		!strings.Contains(jsonText, "\"signature_script_hex\"") {
		t.Fatalf("unexpected raw json: %s", jsonText)
	}
	if !strings.Contains(jsonText, "\"signature_script_asm\"") ||
		!strings.Contains(jsonText, "\"pk_script_asm\"") {
		t.Fatalf("expected decoded script fields in raw json: %s", jsonText)
	}
	if !strings.Contains(jsonText, "\"signature_script_payloads\"") ||
		!strings.Contains(jsonText, "\"pk_script_payloads\"") ||
		!strings.Contains(jsonText, "hello-signature-payload") ||
		!strings.Contains(jsonText, "REAP:500:1:abcd") {
		t.Fatalf("expected parsed payload fields in raw json: %s", jsonText)
	}
	if strings.Contains(jsonText, "nextblockhash") ||
		strings.Contains(jsonText, "confirmations") {
		t.Fatalf("raw json should not contain derived fields: %s", jsonText)
	}
}

func hexString(data []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = hexChars[b>>4]
		out[i*2+1] = hexChars[b&0x0f]
	}
	return string(out)
}

func makeNodeSnapshot(rpcServer string, height int32, mempool int64) *statusSnapshot {
	return &statusSnapshot{
		GeneratedAt: time.Now().UTC(),
		RPCServer:   rpcServer,
		Chain: chainStatus{
			Name:          "obtcregtest",
			Blocks:        height,
			Headers:       height,
			BestBlockHash: "best-hash",
		},
		Peers: peerStatus{
			Count:    2,
			Outbound: 2,
		},
		Mempool: mempoolStatus{
			Transactions: mempool,
			Bytes:        mempool * 100,
		},
		ExpiryIndex: expiryIndexStatus{
			Available: true,
			Disabled:  false,
		},
		ExpiryCommitment: expiryCommitmentStatus{
			Available: true,
			Active:    true,
		},
		ReapPlan: reapPlanStatus{
			Available: true,
			Active:    true,
			Picked:    1,
		},
	}
}
