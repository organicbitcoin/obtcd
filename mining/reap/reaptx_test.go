package reap

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestIsLikelyREAPTx(t *testing.T) {
	tx := wire.NewMsgTx(REAPTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 0}})
	burn, _ := burnScript(BurnPolicyOpReturn)
	tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: burn})
	m, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:100:1:abcd")).Script()
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: m})

	if !IsLikelyREAPTx(tx) {
		t.Fatalf("expected reap tx")
	}

	tx.TxOut[1].Value = 1
	if IsLikelyREAPTx(tx) {
		t.Fatalf("marker output must be zero")
	}
}

func TestExtractMarkerPayload(t *testing.T) {
	s, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:1:2:x")).Script()
	p, ok := ExtractMarkerPayload(s)
	if !ok || p != "REAP:1:2:x" {
		t.Fatalf("unexpected payload ok=%v p=%q", ok, p)
	}

	p, ok = ExtractMarkerPayload([]byte{txscript.OP_TRUE})
	if ok || p != "" {
		t.Fatalf("expected invalid marker script")
	}
}
