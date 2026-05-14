// Copyright (c) 2026 The OBTC developers
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

func TestRejectREAPSystemTxFromMempool(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	// Allow version 3 so this test specifically exercises REAP rejection
	// instead of generic version gating.
	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	burn, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP_BURN")).Script()
	marker, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:100:1:abcd")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 1000, PkScript: burn})
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})

	_, err = harness.txPool.ProcessTransaction(btcutil.NewTx(msg), false, false, 0)
	if err == nil {
		t.Fatalf("expected mempool rejection for REAP tx")
	}
	if !strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("unexpected error: %v", err)
	}
}
