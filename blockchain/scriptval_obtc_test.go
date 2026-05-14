// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TestValidateTransactionScriptsSkipsREAPTx verifies that REAP system
// transactions bypass script validation entirely.
func TestValidateTransactionScriptsSkipsREAPTx(t *testing.T) {
	// Construct a REAP tx with invalid scripts that would normally fail
	// script validation. ValidateTransactionScripts should return nil
	// because REAP txs are skipped before any script execution.
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
		// Deliberately invalid signature script
		SignatureScript: []byte{0xde, 0xad},
	})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 500)})

	btx := btcutil.NewTx(tx)
	view := NewUtxoViewpoint()

	flags := txscript.ScriptBip16 | txscript.ScriptVerifyWitness
	err := ValidateTransactionScripts(btx, view, flags, nil, nil)
	if err != nil {
		t.Fatalf("expected REAP tx to skip script validation, got: %v", err)
	}
}

// TestValidateTransactionScriptsDoesNotSkipNonREAP verifies that non-REAP
// transactions still go through normal script validation.
func TestValidateTransactionScriptsDoesNotSkipNonREAP(t *testing.T) {
	prevTx := wire.NewMsgTx(1)
	prevTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
	})
	prevTx.AddTxOut(&wire.TxOut{Value: 100, PkScript: []byte{txscript.OP_FALSE}})
	prevBtx := btcutil.NewTx(prevTx)

	// A non-REAP tx spending a real prevout with an always-failing script
	// must still execute script validation and return an error.
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *prevBtx.Hash(), Index: 0},
	})
	tx.AddTxOut(&wire.TxOut{Value: 100, PkScript: []byte{txscript.OP_TRUE}})

	btx := btcutil.NewTx(tx)
	view := NewUtxoViewpoint()
	view.AddTxOut(prevBtx, 0, 100)

	flags := txscript.ScriptBip16
	err := ValidateTransactionScripts(btx, view, flags, nil, nil)
	if err == nil {
		t.Fatalf("expected non-REAP tx to run script validation and fail")
	}
}

// TestCheckBlockScriptsSkipsREAPTx verifies that checkBlockScripts skips
// REAP transactions in blocks.
func TestCheckBlockScriptsSkipsREAPTx(t *testing.T) {
	// Build a block containing only a coinbase and a REAP tx with invalid scripts.
	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
	})
	coinbase.AddTxOut(&wire.TxOut{Value: 5000, PkScript: []byte{txscript.OP_TRUE}})

	reapTx := wire.NewMsgTx(reapTxVersion)
	reapTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
		SignatureScript:  []byte{0xba, 0xad}, // invalid script
	})
	reapTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, reapTx, 500)})

	msgBlock := &wire.MsgBlock{}
	msgBlock.AddTransaction(coinbase)
	msgBlock.AddTransaction(reapTx)
	block := btcutil.NewBlock(msgBlock)

	view := NewUtxoViewpoint()
	flags := txscript.ScriptBip16

	err := checkBlockScripts(block, view, flags, nil, nil)
	if err != nil {
		t.Fatalf("expected block with only coinbase+REAP to pass script validation, got: %v", err)
	}
}
