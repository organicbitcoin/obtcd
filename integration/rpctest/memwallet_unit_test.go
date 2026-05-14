// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpctest

import (
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestMemWalletConfirmedBalanceSkipsExpiredUTXOsOnOBTC(t *testing.T) {
	w := newTestMemWallet(t, &chaincfg.ObtcRegTestParams)
	w.currentHeight = 241
	addTestUTXO(t, w, 0, 110_000, 1, 98)
	addTestUTXO(t, w, 0, 120_000, 2, 99)

	if got := w.ConfirmedBalance(); got != 120_000 {
		t.Fatalf("expected confirmed balance 120000, got %d", got)
	}
}

func TestMemWalletCreateTransactionRejectsExpiredUTXOOnOBTC(t *testing.T) {
	w := newTestMemWallet(t, &chaincfg.ObtcRegTestParams)
	w.currentHeight = 241
	addTestUTXO(t, w, 0, 150_000, 1, 98)

	output := newTestOutput(t, w.coinbaseAddr, 100_000)
	if _, err := w.CreateTransaction([]*wire.TxOut{output}, 10, false); err == nil {
		t.Fatal("expected transaction creation to fail when only expired utxos remain")
	}
}

func TestMemWalletCreateTransactionUsesReplayProtectedSigHashOnOBTC(t *testing.T) {
	w := newTestMemWallet(t, &chaincfg.ObtcRegTestParams)
	w.currentHeight = chaincfg.GetOBTCReplayProtectionHeight(&chaincfg.ObtcRegTestParams) - 1
	addTestUTXO(t, w, 0, 150_000, 1, 120)

	output := newTestOutput(t, w.coinbaseAddr, 100_000)
	tx, err := w.CreateTransaction([]*wire.TxOut{output}, 10, false)
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}
	if len(tx.TxIn) != 1 {
		t.Fatalf("expected 1 input, got %d", len(tx.TxIn))
	}

	pushed, err := txscript.PushedData(tx.TxIn[0].SignatureScript)
	if err != nil {
		t.Fatalf("PushedData: %v", err)
	}
	if len(pushed) == 0 || len(pushed[0]) == 0 {
		t.Fatal("expected signature push in signature script")
	}

	got := txscript.SigHashType(pushed[0][len(pushed[0])-1])
	want := txscript.SigHashAll | txscript.SigHashOBTCReplayProtection
	if got != want {
		t.Fatalf("unexpected sighash type: got %v want %v", got, want)
	}
}

func TestMemWalletCreateTransactionKeepsStandardSigHashOnSimnet(t *testing.T) {
	w := newTestMemWallet(t, &chaincfg.SimNetParams)
	w.currentHeight = 500
	addTestUTXO(t, w, 0, 150_000, 1, 120)

	output := newTestOutput(t, w.coinbaseAddr, 100_000)
	tx, err := w.CreateTransaction([]*wire.TxOut{output}, 10, false)
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	pushed, err := txscript.PushedData(tx.TxIn[0].SignatureScript)
	if err != nil {
		t.Fatalf("PushedData: %v", err)
	}
	got := txscript.SigHashType(pushed[0][len(pushed[0])-1])
	if got != txscript.SigHashAll {
		t.Fatalf("unexpected sighash type on simnet: got %v want %v", got, txscript.SigHashAll)
	}
}

func newTestMemWallet(t *testing.T, net *chaincfg.Params) *memWallet {
	t.Helper()

	w, err := newMemWallet(net, 7)
	if err != nil {
		t.Fatalf("newMemWallet: %v", err)
	}
	return w
}

func addTestUTXO(t *testing.T, w *memWallet, keyIndex uint32, value int64, hashByte byte, blockHeight int32) {
	t.Helper()

	addr, ok := w.addrs[keyIndex]
	if !ok {
		t.Fatalf("missing wallet address for key index %d", keyIndex)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: %v", err)
	}

	var hash chainhash.Hash
	hash[0] = hashByte
	outPoint := wire.OutPoint{Hash: hash, Index: 0}
	w.utxos[outPoint] = &utxo{
		pkScript:       pkScript,
		value:          btcutil.Amount(value),
		keyIndex:       keyIndex,
		blockHeight:    blockHeight,
		maturityHeight: 0,
	}
}

func newTestOutput(t *testing.T, addr btcutil.Address, value int64) *wire.TxOut {
	t.Helper()

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: %v", err)
	}
	return &wire.TxOut{
		Value:    value,
		PkScript: pkScript,
	}
}
