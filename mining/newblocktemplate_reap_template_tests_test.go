// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/wire"
)

func findREAPTxInTemplate(t *testing.T, tmpl *BlockTemplate) (int, *wire.MsgTx) {
	t.Helper()

	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		if reap.IsLikelyREAPTx(tx) {
			return i, tx
		}
	}
	return -1, nil
}

func reapTemplateInputs(tx *wire.MsgTx) []wire.OutPoint {
	inputs := make([]wire.OutPoint, len(tx.TxIn))
	for i, txIn := range tx.TxIn {
		inputs[i] = txIn.PreviousOutPoint
	}
	return inputs
}

func assertREAPMarkerMatchesInputs(t *testing.T, tx *wire.MsgTx, height int32) {
	t.Helper()

	if len(tx.TxOut) == 0 {
		t.Fatalf("REAP tx has no marker output")
	}
	markerOut := tx.TxOut[len(tx.TxOut)-1]
	if markerOut.Value != 0 {
		t.Fatalf("REAP marker value: got %d want 0", markerOut.Value)
	}

	payload, ok := reap.ExtractMarkerPayload(markerOut.PkScript)
	if !ok {
		t.Fatalf("REAP marker payload missing")
	}
	parts := strings.Split(payload, ":")
	if len(parts) != 4 || parts[0] != "REAP" {
		t.Fatalf("invalid REAP marker payload: %q", payload)
	}

	gotHeight, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		t.Fatalf("invalid marker height %q: %v", parts[1], err)
	}
	if int32(gotHeight) != height {
		t.Fatalf("marker height: got %d want %d", gotHeight, height)
	}

	gotCount, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("invalid marker count %q: %v", parts[2], err)
	}
	if gotCount != len(tx.TxIn) {
		t.Fatalf("marker count: got %d want %d", gotCount, len(tx.TxIn))
	}

	inputs := reapTemplateInputs(tx)
	if got, want := parts[3], reap.MarkerDigest(inputs); got != want {
		t.Fatalf("marker digest: got %s want %s", got, want)
	}
}

func expectedREAPRefundAndTax(t *testing.T, tx *wire.MsgTx,
	view *blockchain.UtxoViewpoint, params *chaincfg.Params) (map[string]int64, int64, int64) {

	t.Helper()

	ep := chaincfg.GetExpiryParams(params)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	refundByScript := make(map[string]int64)
	var inputTotal int64
	var taxTotal int64
	for _, txIn := range tx.TxIn {
		entry := view.LookupEntry(txIn.PreviousOutPoint)
		if entry == nil || entry.IsSpent() {
			t.Fatalf("missing REAP input view entry for %s", txIn.PreviousOutPoint)
		}

		value := entry.Amount()
		inputTotal += value
		tax := (value * ep.ReapTaxNumerator) / ep.ReapTaxDenominator
		refund := value - tax
		if ep.ReapDustThresholdSat > 0 && value > 0 && value < ep.ReapDustThresholdSat {
			tax = value
			refund = 0
		}
		taxTotal += tax
		if refund > 0 {
			refundByScript[string(entry.PkScript())] += refund
		}
	}

	return refundByScript, taxTotal, inputTotal
}

func actualREAPRefundOutputs(tx *wire.MsgTx) map[string]int64 {
	refundByScript := make(map[string]int64)
	for _, txOut := range tx.TxOut[:len(tx.TxOut)-1] {
		refundByScript[string(txOut.PkScript)] += txOut.Value
	}
	return refundByScript
}

func TestNewBlockTemplateREAPNotAppendedWithoutExpiredCandidates(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 120, []int32{1, 100})
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, plannedREAPFee, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx != nil || plannedREAPFee != 0 {
		t.Fatalf("expected no REAP before any indexed UTXO expires, got tx=%v fee=%d",
			plannedREAPTx, plannedREAPFee)
	}

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	if templateHasREAPTx(tmpl) {
		t.Fatalf("did not expect REAP tx with only unexpired indexed UTXOs")
	}
	if len(tmpl.Block.Transactions) != 1 {
		t.Fatalf("expected coinbase-only template, got %d txs", len(tmpl.Block.Transactions))
	}
}

func TestNewBlockTemplateREAPAppendStructureRefundsAndAccounting(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 243, []int32{100, 120})
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, plannedREAPFee, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx == nil {
		t.Fatalf("expected REAP plan at height %d", nextHeight)
	}

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}

	reapIdx, reapTx := findREAPTxInTemplate(t, tmpl)
	if reapTx == nil {
		t.Fatalf("expected REAP tx in template")
	}
	if got, want := reapTx.TxHash(), *plannedREAPTx.Hash(); got != want {
		t.Fatalf("template REAP tx hash mismatch: got %s want %s", got, want)
	}
	if reapTx.Version != reap.REAPTxVersion {
		t.Fatalf("REAP tx version: got %d want %d", reapTx.Version, reap.REAPTxVersion)
	}
	if reapTx.LockTime != uint32(tmpl.Height-1) {
		t.Fatalf("REAP locktime: got %d want %d", reapTx.LockTime, tmpl.Height-1)
	}
	for i, txIn := range reapTx.TxIn {
		if txIn.Sequence != 0xfffffffe {
			t.Fatalf("REAP input %d sequence: got %x want fffffffe", i, txIn.Sequence)
		}
	}
	assertREAPMarkerMatchesInputs(t, reapTx, tmpl.Height)

	if !txHasInput(btcutil.NewTx(reapTx), h.spendable[100]) {
		t.Fatalf("expected expired height 100 output to be selected")
	}
	if txHasInput(btcutil.NewTx(reapTx), h.spendable[120]) {
		t.Fatalf("did not expect unexpired height 120 output to be selected")
	}

	view, err := h.chain.FetchUtxoView(btcutil.NewTx(reapTx))
	if err != nil {
		t.Fatalf("FetchUtxoView for REAP tx failed: %v", err)
	}
	ep := chaincfg.GetExpiryParams(h.params)
	for i, txIn := range reapTx.TxIn {
		entry := view.LookupEntry(txIn.PreviousOutPoint)
		if entry == nil {
			t.Fatalf("missing input %d view entry", i)
		}
		expiryHeight := int32(ep.CalculateExpiryKey(entry.BlockHeight()))
		if tmpl.Height < expiryHeight {
			t.Fatalf("REAP input %d is not expired: create=%d expiry=%d template=%d",
				i, entry.BlockHeight(), expiryHeight, tmpl.Height)
		}
	}

	expectedRefunds, expectedTax, inputTotal := expectedREAPRefundAndTax(t, reapTx, view, h.params)
	if !reflect.DeepEqual(actualREAPRefundOutputs(reapTx), expectedRefunds) {
		t.Fatalf("refund outputs mismatch: got %+v want %+v",
			actualREAPRefundOutputs(reapTx), expectedRefunds)
	}
	if plannedREAPFee != expectedTax {
		t.Fatalf("planned REAP fee/tax mismatch: got %d want %d", plannedREAPFee, expectedTax)
	}
	if tmpl.Fees[reapIdx] != expectedTax {
		t.Fatalf("template REAP fee mismatch: got %d want %d", tmpl.Fees[reapIdx], expectedTax)
	}

	subsidy := blockchain.CalcBlockSubsidy(tmpl.Height, h.params)
	coinbaseValue := tmpl.Block.Transactions[0].TxOut[0].Value
	if want := subsidy + expectedTax; coinbaseValue != want {
		t.Fatalf("coinbase value mismatch: got %d want %d", coinbaseValue, want)
	}
	if coinbaseValue == subsidy+inputTotal {
		t.Fatalf("coinbase incorrectly appears to include refund value as tax")
	}
}

func TestNewBlockTemplateREAPUsesCanonicalPrefixAndIsStable(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 360, nil)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	p := reap.DefaultREAPParamsForNet(h.params, reap.SortModeStrict)
	candidates, err := h.reapIndex.ReapPrefixCandidates(nextHeight, p.PrefixCandidateLimit())
	if err != nil {
		t.Fatalf("ReapPrefixCandidates failed: %v", err)
	}
	if len(candidates) <= p.MaxInputs {
		t.Fatalf("test setup expected backlog beyond normal cap: candidates=%d cap=%d",
			len(candidates), p.MaxInputs)
	}

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	_, reapTx := findREAPTxInTemplate(t, tmpl)
	if reapTx == nil {
		t.Fatalf("expected REAP tx in template")
	}

	inputs := reapTemplateInputs(reapTx)
	if len(inputs) != p.MaxInputs {
		t.Fatalf("REAP input cap mismatch: got %d want %d", len(inputs), p.MaxInputs)
	}
	for i, input := range inputs {
		if want := candidates[i].OutPoint; input != want {
			t.Fatalf("REAP input %d is not canonical prefix: got %s want %s",
				i, input, want)
		}
	}
	assertREAPMarkerMatchesInputs(t, reapTx, tmpl.Height)

	weight := blockchain.GetTransactionWeight(btcutil.NewTx(reapTx))
	if weight > p.WeightBudget {
		t.Fatalf("REAP tx exceeds reserved weight budget: got %d budget %d",
			weight, p.WeightBudget)
	}

	repeat, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("repeat NewBlockTemplate failed: %v", err)
	}
	_, repeatREAPTx := findREAPTxInTemplate(t, repeat)
	if repeatREAPTx == nil {
		t.Fatalf("expected repeat REAP tx")
	}
	if got := reapTemplateInputs(repeatREAPTx); !reflect.DeepEqual(got, inputs) {
		t.Fatalf("repeat template REAP inputs changed")
	}
	if repeatREAPTx.TxHash() != reapTx.TxHash() {
		t.Fatalf("repeat template REAP tx hash changed: got %s want %s",
			repeatREAPTx.TxHash(), reapTx.TxHash())
	}
}
