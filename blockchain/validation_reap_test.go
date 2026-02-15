package blockchain

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

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

	if err := checkReapMarker(tx, 222); err != nil {
		t.Fatalf("expected valid marker check, got %v", err)
	}
	if err := checkReapMarker(tx, 223); err == nil {
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
	if err := checkReapMarker(tx, 100); err == nil {
		t.Fatalf("expected marker count mismatch error")
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
