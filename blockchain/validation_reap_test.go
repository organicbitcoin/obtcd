package blockchain

import (
	"fmt"
	"strings"
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
