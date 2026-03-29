package main

import (
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestFeeRateForAttempt(t *testing.T) {
	base := btcutil.Amount(10)
	got := []btcutil.Amount{
		feeRateForAttempt(base, 0),
		feeRateForAttempt(base, 1),
		feeRateForAttempt(base, 2),
		feeRateForAttempt(base, 3),
		feeRateForAttempt(base, 4),
		feeRateForAttempt(base, 5),
		feeRateForAttempt(base, 6),
		feeRateForAttempt(base, 7),
		feeRateForAttempt(base, 8),
	}
	want := []btcutil.Amount{10, 10, 20, 20, 30, 50, 80, 130, 10}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt %d: want fee %d, got %d", i, want[i], got[i])
		}
	}
}

func TestCreateSweepTxUsesRequestedInputCount(t *testing.T) {
	w := newTestWallet(t)
	for i := 1; i <= 4; i++ {
		addConfirmedUTXO(t, w, uint32(i), 100_000, byte(i))
	}

	tx, err := w.createSweepTx(3, 10, selectSmallestConfirmed, false)
	if err != nil {
		t.Fatalf("createSweepTx: %v", err)
	}
	if len(tx.TxIn) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(tx.TxIn))
	}
	if len(tx.TxOut) != 1 {
		t.Fatalf("expected 1 output, got %d", len(tx.TxOut))
	}
	for i, txIn := range tx.TxIn {
		if len(txIn.SignatureScript) == 0 {
			t.Fatalf("input %d missing signature script", i)
		}
	}
}

func TestCreateSweepFromUTXOsRejectsDust(t *testing.T) {
	w := newTestWallet(t)
	addConfirmedUTXO(t, w, 1, 1_200, 1)

	_, err := w.createSweepTx(1, 10, selectSmallestConfirmed, false)
	if err == nil {
		t.Fatal("expected sweep to fail for dust-sized input")
	}
}

func TestDeriveSeedByTag(t *testing.T) {
	primary := deriveSeed("primary")
	defaultSeed := deriveSeed("")
	peer := deriveSeed("peer")

	if primary != defaultHDSeed {
		t.Fatal("primary seed should match default seed")
	}
	if defaultSeed != defaultHDSeed {
		t.Fatal("empty seed tag should match default seed")
	}
	if peer == defaultHDSeed {
		t.Fatal("peer seed tag should derive a different seed")
	}
	if peer != deriveSeed("peer") {
		t.Fatal("seed derivation should be deterministic")
	}
}

func TestCreatePaymentTxPaysExternalAddress(t *testing.T) {
	payer := newTestWallet(t)
	addConfirmedUTXO(t, payer, 1, 150_000, 1)

	targetKM, err := loadKeyManager(filepath.Join(t.TempDir(), "peer-state.json"), &chaincfg.SimNetParams, "peer")
	if err != nil {
		t.Fatalf("loadKeyManager(target): %v", err)
	}
	addr, _, err := targetKM.newAddress()
	if err != nil {
		t.Fatalf("newAddress(target): %v", err)
	}

	tx, err := payer.createPaymentTx([]paymentOutput{{
		Address: addr,
		Amount:  90_000,
	}}, 10, selectSmallestConfirmed, false)
	if err != nil {
		t.Fatalf("createPaymentTx: %v", err)
	}
	if len(tx.TxIn) == 0 {
		t.Fatal("expected payment tx to have inputs")
	}
	if len(tx.TxOut) == 0 {
		t.Fatal("expected payment tx to have outputs")
	}

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript(target): %v", err)
	}
	if string(tx.TxOut[0].PkScript) != string(pkScript) {
		t.Fatal("first output does not pay the requested external address")
	}
	if len(tx.TxIn[0].SignatureScript) == 0 {
		t.Fatal("expected signed payment input")
	}
}

func TestSignatureHashTypeSwitchesForOBTCReplayProtection(t *testing.T) {
	simnetWallet := newTestWallet(t)
	if got := simnetWallet.signatureHashType(); got != txscript.SigHashAll {
		t.Fatalf("simnet should use SigHashAll, got %v", got)
	}

	obtcWallet := newTestWalletWithNet(t, &chaincfg.ObtcRegTestParams)
	obtcWallet.currentHeight = chaincfg.GetOBTCReplayProtectionHeight(&chaincfg.ObtcRegTestParams)
	want := txscript.SigHashOBTCReplayProtection | txscript.SigHashAll
	if got := obtcWallet.signatureHashType(); got != want {
		t.Fatalf("obtcregtest should use replay-protected sighash, want %v got %v", want, got)
	}
}

func newTestWallet(t *testing.T) *wallet {
	return newTestWalletWithNet(t, &chaincfg.SimNetParams)
}

func newTestWalletWithNet(t *testing.T, net *chaincfg.Params) *wallet {
	t.Helper()

	km, err := loadKeyManager(filepath.Join(t.TempDir(), "state.json"), net, "primary")
	if err != nil {
		t.Fatalf("loadKeyManager: %v", err)
	}

	w := newWallet(km)
	w.currentHeight = 500
	return w
}

func addConfirmedUTXO(t *testing.T, w *wallet, keyIndex uint32, value int64, hashByte byte) {
	t.Helper()

	addr, err := w.km.ensureAddress(keyIndex)
	if err != nil {
		t.Fatalf("ensureAddress(%d): %v", keyIndex, err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: %v", err)
	}

	var hash chainhash.Hash
	hash[0] = hashByte
	outPoint := wire.OutPoint{Hash: hash, Index: 0}
	w.confirmed[outPoint] = &walletUTXO{
		OutPoint:       outPoint,
		PkScript:       pkScript,
		Value:          btcutil.Amount(value),
		KeyIndex:       keyIndex,
		MaturityHeight: 0,
		Confirmed:      true,
	}
}
