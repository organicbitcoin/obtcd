// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mempool

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TestREAPNonV3TxNotRejected verifies that transactions with REAP-like outputs
// but version != 3 are NOT flagged as REAP system transactions.
func TestREAPNonV3TxNotRejected(t *testing.T) {
	harness, spendableOuts, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	// Version 1 tx with REAP-like marker should not trigger REAP rejection.
	msg := wire.NewMsgTx(1)
	msg.AddTxIn(&wire.TxIn{
		PreviousOutPoint: spendableOuts[0].outPoint,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	msg.AddTxOut(&wire.TxOut{
		Value:    int64(spendableOuts[0].amount - 1000),
		PkScript: harness.payScript,
	})
	marker, _ := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("REAP:100:1:abcd")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})

	sigScript, err := txscript.SignatureScript(
		msg, 0, harness.payScript,
		txscript.SigHashAll, harness.signKey, true,
	)
	if err != nil {
		t.Fatalf("sign error: %v", err)
	}
	msg.TxIn[0].SignatureScript = sigScript

	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), false, false, 0)
	if err != nil && strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("version 1 tx with REAP marker should not be rejected as REAP: %v", err)
	}
}

// TestREAPV3TxWithoutMarkerNotRejected verifies that a v3 tx without a
// REAP marker output is not flagged as a REAP system transaction.
func TestREAPV3TxWithoutMarkerNotRejected(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	// Regular output, no REAP marker
	msg.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), false, false, 0)
	if err != nil && strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("v3 tx without REAP marker should not be rejected as REAP: %v", err)
	}
}

// TestREAPRejectionWithMultipleOutputs verifies REAP detection works when
// the marker is the last output among many.
func TestREAPRejectionWithMultipleOutputs(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})

	// Add several regular outputs before the REAP marker
	for i := 0; i < 5; i++ {
		msg.AddTxOut(&wire.TxOut{
			Value:    int64(1000 * (i + 1)),
			PkScript: []byte{txscript.OP_TRUE},
		})
	}

	// REAP marker as last output
	marker, _ := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("REAP:100:1:deadbeef")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})

	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), false, false, 0)
	if err == nil {
		t.Fatalf("expected REAP rejection for v3 tx with trailing REAP marker")
	}
	if !strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("expected reap system transaction error, got: %v", err)
	}
}

// TestREAPRejectionNotBypassedByAllowHighFees verifies that the allowHighFees
// parameter does not bypass REAP rejection.
func TestREAPRejectionNotBypassedByAllowHighFees(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	marker, _ := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("REAP:100:1:abcd")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})

	// Try with allowHighFees=true
	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), true, false, 0)
	if err == nil {
		t.Fatalf("expected REAP rejection even with allowHighFees=true")
	}
	if !strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("expected reap system transaction error, got: %v", err)
	}
}

// TestREAPMarkerNonLastOutputNotRejected verifies that a REAP marker in a
// non-last output position is NOT detected as a REAP tx.
func TestREAPMarkerNonLastOutputNotRejected(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})

	// REAP marker as first output (not last)
	marker, _ := txscript.NewScriptBuilder().
		AddOp(txscript.OP_RETURN).
		AddData([]byte("REAP:100:1:abcd")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})
	// Regular output as last
	msg.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{txscript.OP_TRUE}})

	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), false, false, 0)
	if err != nil && strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("REAP marker in non-last position should not trigger rejection: %v", err)
	}
}
