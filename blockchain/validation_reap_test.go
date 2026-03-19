package blockchain

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func makeValidReapTx(t *testing.T, view *UtxoViewpoint, height int32,
	inputs ...wire.OutPoint) *wire.MsgTx {

	t.Helper()

	tx := wire.NewMsgTx(reapTxVersion)
	for _, op := range inputs {
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	}

	expiryParams := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if expiryParams == nil {
		t.Fatalf("expected expiry params")
	}

	refundByScript := make(map[string]int64)
	for _, op := range inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			t.Fatalf("missing utxo for reap test input %v", op)
		}
		amount := entry.Amount()
		tax := reapTaxForValue(amount, expiryParams)
		refund := amount - tax
		if refund < 0 {
			t.Fatalf("negative refund for input %v", op)
		}
		refund, _ = applyReapDustRule(amount, refund, tax, expiryParams)
		if refund > 0 {
			refundByScript[string(entry.PkScript())] += refund
		}
	}

	scripts := make([]string, 0, len(refundByScript))
	for script := range refundByScript {
		scripts = append(scripts, script)
	}
	sort.Slice(scripts, func(i, j int) bool {
		return bytes.Compare([]byte(scripts[i]), []byte(scripts[j])) < 0
	})
	for _, script := range scripts {
		tx.AddTxOut(&wire.TxOut{
			Value:    refundByScript[script],
			PkScript: []byte(script),
		})
	}

	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, height)})
	return tx
}

func addUtxoToView(t *testing.T, view *UtxoViewpoint, value int64, height int32) wire.OutPoint {
	t.Helper()
	tx := wire.NewMsgTx(1)
	tx.AddTxOut(&wire.TxOut{Value: value, PkScript: []byte{txscript.OP_TRUE}})
	btx := btcutil.NewTx(tx)
	view.AddTxOut(btx, 0, height)
	return wire.OutPoint{Hash: *btx.Hash(), Index: 0}
}

func markerForTx(t *testing.T, tx *wire.MsgTx, height int32) []byte {
	t.Helper()
	payload := fmt.Sprintf("REAP:%d:%d:%s", height, len(tx.TxIn), reapInputDigest(tx))
	s, err := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte(payload)).Script()
	if err != nil {
		t.Fatalf("marker script: %v", err)
	}
	return s
}

func TestNonREAPExpiredSpendRejected(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 900, PkScript: []byte{txscript.OP_TRUE}})

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 200, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "expired utxo") {
		t.Fatalf("expected expired spend rejection, got: %v", err)
	}
}

func makeBlockWithTxs(txs ...*wire.MsgTx) *btcutil.Block {
	msgBlock := &wire.MsgBlock{}
	msgBlock.AddTransaction(wire.NewMsgTx(1))
	for _, tx := range txs {
		msgBlock.AddTransaction(tx)
	}
	return btcutil.NewBlock(msgBlock)
}

func TestREAPNonExpiredSpendRejected(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 180)

	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 700, PkScript: []byte{txscript.OP_TRUE}})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 200)})

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 200, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "non-expired utxo") {
		t.Fatalf("expected non-expired rejection, got: %v", err)
	}
}

func TestREAPMarkerDigestMismatchRejected(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)

	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 700, PkScript: []byte{txscript.OP_TRUE}})
	bad, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:200:1:deadbeef")).Script()
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: bad})

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 200, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch rejection, got: %v", err)
	}
}

func TestValidationREAPHelpersDirect(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})
	if isLikelyReapTx(tx) {
		t.Fatalf("non-marker tx should not be identified as reap")
	}

	tx.TxOut[0].Value = 0
	tx.TxOut[0].PkScript = markerForTx(t, tx, 200)
	if !isLikelyReapTx(tx) {
		t.Fatalf("valid marker tx should be identified as reap")
	}

	payload, ok := extractMarkerPayload(tx.TxOut[0].PkScript)
	if !ok || !strings.HasPrefix(payload, "REAP:") {
		t.Fatalf("expected marker payload, got ok=%v payload=%q", ok, payload)
	}

	h, c, d, err := parseReapMarkerPayload(payload)
	if err != nil || h != 200 || c != 1 || d == "" {
		t.Fatalf("unexpected parse result h=%d c=%d d=%q err=%v", h, c, d, err)
	}
	if _, _, _, err := parseReapMarkerPayload("bad"); err == nil {
		t.Fatalf("expected parse error for invalid payload")
	}

	if got := reapInputDigest(tx); got == "" {
		t.Fatalf("expected non-empty digest")
	}
}

func TestCheckReapMarkerDirect(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 2}})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 222)})

	if err := checkReapMarker(tx, 222, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected valid marker check, got %v", err)
	}
	if err := checkReapMarker(tx, 223, &chaincfg.ObtcRegTestParams); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestParseReapMarkerPayloadEdgeCases(t *testing.T) {
	tests := []string{
		"",                   // empty
		"reap:1:1:abcd",      // case mismatch
		"REAP:x:1:abcd",      // bad height
		"REAP:1:x:abcd",      // bad count
		"REAP:1:1:not_hex_🧪", // unicode + invalid digest
	}
	for _, in := range tests {
		if _, _, _, err := parseReapMarkerPayload(in); err == nil {
			t.Fatalf("expected parse error for %q", in)
		}
	}
}

func TestCheckReapMarkerCountMismatch(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	bad, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:100:2:deadbeef")).Script()
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: bad})
	if err := checkReapMarker(tx, 100, &chaincfg.ObtcRegTestParams); err == nil {
		t.Fatalf("expected marker count mismatch error")
	}
}

func TestReapBlockHardeningRejectsMultipleReapTx(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	view := NewUtxoViewpoint()
	opA := addUtxoToView(t, view, 1000, 1)
	opB := addUtxoToView(t, view, 1200, 1)

	block := makeBlockWithTxs(
		makeValidReapTx(t, view, ep.ReapConsensusAtHeight, opA),
		makeValidReapTx(t, view, ep.ReapConsensusAtHeight, opB),
	)

	err := checkReapBlockHardening(block, ep.ReapConsensusAtHeight, &chaincfg.ObtcRegTestParams)
	if err == nil {
		t.Fatalf("expected multiple REAP tx rejection")
	}

	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T", err)
	}
	if ruleErr.ErrorCode != ErrMultipleReapTx {
		t.Fatalf("unexpected error code: got %v want %v", ruleErr.ErrorCode, ErrMultipleReapTx)
	}
}

func TestReapBlockHardeningAllowsSingleReapTx(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)

	block := makeBlockWithTxs(makeValidReapTx(t, view, ep.ReapConsensusAtHeight, op))
	if err := checkReapBlockHardening(block, ep.ReapConsensusAtHeight, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected single REAP tx to pass, got %v", err)
	}
}

func TestReapBlockHardeningNoopBeforeActivation(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}
	if ep.ReapConsensusAtHeight <= 0 {
		t.Fatalf("expected positive ReapConsensusAtHeight")
	}

	view := NewUtxoViewpoint()
	opA := addUtxoToView(t, view, 1000, 1)
	opB := addUtxoToView(t, view, 1200, 1)

	block := makeBlockWithTxs(
		makeValidReapTx(t, view, ep.ReapConsensusAtHeight-1, opA),
		makeValidReapTx(t, view, ep.ReapConsensusAtHeight-1, opB),
	)

	if err := checkReapBlockHardening(block, ep.ReapConsensusAtHeight-1, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected no block-level REAP enforcement before activation, got %v", err)
	}
}

func TestREAPCanonicalInputOrderEnforced(t *testing.T) {
	view := NewUtxoViewpoint()
	highAmount := addUtxoToView(t, view, 2000, 1)
	lowAmount := addUtxoToView(t, view, 1000, 1)

	// Wrong strict order: same expiry but larger amount comes first.
	tx := makeValidReapTx(t, view, 500, highAmount, lowAmount)

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "canonical order") {
		t.Fatalf("expected canonical order rejection, got: %v", err)
	}
}

func TestREAPCanonicalInputOrderAcceptedWhenSorted(t *testing.T) {
	view := NewUtxoViewpoint()
	highAmount := addUtxoToView(t, view, 2000, 1)
	lowAmount := addUtxoToView(t, view, 1000, 1)

	tx := makeValidReapTx(t, view, 500, lowAmount, highAmount)

	if _, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected sorted reap tx to pass, got: %v", err)
	}
}

func TestREAPInputCountConsensusLimit(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil || ep.ReapMaxInputs <= 0 {
		t.Fatalf("expected positive reap max input consensus limit")
	}

	view := NewUtxoViewpoint()
	inputs := make([]wire.OutPoint, 0, ep.ReapMaxInputs+1)
	for i := 0; i < ep.ReapMaxInputs+1; i++ {
		inputs = append(inputs, addUtxoToView(t, view, 1000, 1))
	}

	sort.Slice(inputs, func(i, j int) bool {
		hcmp := bytes.Compare(inputs[i].Hash[:], inputs[j].Hash[:])
		if hcmp != 0 {
			return hcmp < 0
		}
		return inputs[i].Index < inputs[j].Index
	})

	tx := makeValidReapTx(t, view, 500, inputs...)

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "exceeds consensus limit") {
		t.Fatalf("expected reap input count limit rejection, got: %v", err)
	}
}

func TestREAPExpiredSpendRequiresExpectedRefundDistribution(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)

	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 500)})

	_, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "refund total mismatch") {
		t.Fatalf("expected reap refund mismatch rejection, got: %v", err)
	}
}

func TestREAPExpiredSpendWithExpectedRefundAccepted(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)

	tx := makeValidReapTx(t, view, 500, op)
	if _, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected valid reap tax distribution, got: %v", err)
	}
}

func TestREAPExpiredDustOnlyInputAcceptedWithoutRefundOutput(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 700, 1) // below 720 dust threshold -> full tax

	tx := makeValidReapTx(t, view, 500, op)
	if len(tx.TxOut) != 1 {
		t.Fatalf("expected dust-only reap tx to contain only marker output, got %d outputs", len(tx.TxOut))
	}
	if _, err := CheckTransactionInputs(btcutil.NewTx(tx), 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected dust-only reap tx to pass, got: %v", err)
	}
}

func TestCheckExpirySpendRulesEnableBoundaryAndIdempotent(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 900, PkScript: []byte{txscript.OP_TRUE}})

	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}
	below := ep.EnableAtHeight - 1
	for i := 0; i < 2; i++ { // idempotent repeat
		if err := checkExpirySpendRules(tx, below, view, &chaincfg.ObtcRegTestParams); err != nil {
			t.Fatalf("expected no enforcement below enable height, got %v", err)
		}
	}
}

func TestReapInputDigestConcurrentStable(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	for i := 0; i < 8; i++ {
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: uint32(i)}})
	}
	want := reapInputDigest(tx)
	if want == "" {
		t.Fatalf("expected non-empty digest")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := reapInputDigest(tx); got != want {
				errCh <- fmt.Errorf("digest mismatch got=%s want=%s", got, want)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestCheckExpirySpendRulesDirect(t *testing.T) {
	view := NewUtxoViewpoint()
	expiredOp := addUtxoToView(t, view, 1000, 1)
	nonExpiredOp := addUtxoToView(t, view, 1001, 495)
	if e := view.LookupEntry(expiredOp); e == nil || e.BlockHeight() != 1 {
		t.Fatalf("unexpected expired utxo height entry=%v", e)
	}

	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params on obtc regtest")
	}

	nonReap := wire.NewMsgTx(1)
	nonReap.AddTxIn(&wire.TxIn{PreviousOutPoint: expiredOp})
	nonReap.AddTxOut(&wire.TxOut{Value: 900, PkScript: []byte{txscript.OP_TRUE}})
	if err := checkExpirySpendRules(nonReap, 500, view, &chaincfg.ObtcRegTestParams); err == nil {
		t.Fatalf("expected non-reap expired spend error")
	}

	reapTx := wire.NewMsgTx(reapTxVersion)
	reapTx.AddTxIn(&wire.TxIn{PreviousOutPoint: nonExpiredOp})
	reapTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, reapTx, 500)})
	if err := checkExpirySpendRules(reapTx, 500, view, &chaincfg.ObtcRegTestParams); err == nil {
		t.Fatalf("expected reap non-expired spend error")
	}
}

func TestCheckExpirySpendRulesAllowsUnconfirmedParentSpend(t *testing.T) {
	view := NewUtxoViewpoint()
	tx := wire.NewMsgTx(1)
	tx.AddTxOut(&wire.TxOut{Value: 1_000, PkScript: []byte{txscript.OP_TRUE}})
	parent := btcutil.NewTx(tx)
	view.AddTxOut(parent, 0, unminedInputHeight)

	spend := wire.NewMsgTx(1)
	spend.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *parent.Hash(), Index: 0},
	})
	spend.AddTxOut(&wire.TxOut{Value: 900, PkScript: []byte{txscript.OP_TRUE}})

	if err := checkExpirySpendRules(spend, 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("expected non-REAP spend of unconfirmed parent to bypass expiry enforcement, got %v", err)
	}
}

func TestCheckExpirySpendRulesRejectsReapUnconfirmedParentSpend(t *testing.T) {
	view := NewUtxoViewpoint()
	tx := wire.NewMsgTx(1)
	tx.AddTxOut(&wire.TxOut{Value: 1_000, PkScript: []byte{txscript.OP_TRUE}})
	parent := btcutil.NewTx(tx)
	view.AddTxOut(parent, 0, unminedInputHeight)

	reapTx := wire.NewMsgTx(reapTxVersion)
	reapTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *parent.Hash(), Index: 0},
	})
	reapTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, reapTx, 500)})

	err := checkExpirySpendRules(reapTx, 500, view, &chaincfg.ObtcRegTestParams)
	if err == nil || !strings.Contains(err.Error(), "unconfirmed utxo") {
		t.Fatalf("expected REAP unconfirmed spend rejection, got %v", err)
	}
}
