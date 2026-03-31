package main

import (
	"path/filepath"
	"reflect"
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

func TestSelectUTXOsRandomConfirmedDeterministic(t *testing.T) {
	w1 := newTestWallet(t)
	for i := 1; i <= 5; i++ {
		addConfirmedUTXO(t, w1, uint32(i), int64(90_000+i*1_000), byte(i))
	}
	w1.setRandSeed(77)

	selected1, err := w1.selectUTXOs(selectRandomConfirmed, false, 3)
	if err != nil {
		t.Fatalf("selectUTXOs(random, first): %v", err)
	}

	w2 := newTestWallet(t)
	for i := 1; i <= 5; i++ {
		addConfirmedUTXO(t, w2, uint32(i), int64(90_000+i*1_000), byte(i))
	}
	w2.setRandSeed(77)

	selected2, err := w2.selectUTXOs(selectRandomConfirmed, false, 3)
	if err != nil {
		t.Fatalf("selectUTXOs(random, second): %v", err)
	}

	got1 := outPointsOf(selected1)
	got2 := outPointsOf(selected2)
	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("expected deterministic random selection, got %v and %v", got1, got2)
	}
}

func TestCandidatesRandomPendingFirstPrefersPending(t *testing.T) {
	w := newTestWallet(t)
	addConfirmedUTXO(t, w, 1, 100_000, 1)
	w.pending[wire.OutPoint{Hash: chainhash.Hash{2}, Index: 0}] = &walletUTXO{
		OutPoint:  wire.OutPoint{Hash: chainhash.Hash{2}, Index: 0},
		PkScript:  []byte{0x51},
		Value:     90_000,
		KeyIndex:  1,
		Confirmed: false,
	}
	w.setRandSeed(9)

	candidates := w.candidates(selectRandomPendingFirst, true)
	if len(candidates) < 2 {
		t.Fatalf("expected pending and confirmed candidates, got %d", len(candidates))
	}
	if !candidates[0].isPending {
		t.Fatalf("expected pending candidate first, got %+v", candidates[0])
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

func TestSpendableCountSkipsExpiredUTXOsOnOBTC(t *testing.T) {
	w := newTestWalletWithNet(t, &chaincfg.ObtcRegTestParams)
	w.currentHeight = 241
	addConfirmedUTXOAtHeight(t, w, 1, 110_000, 1, 98)
	addConfirmedUTXOAtHeight(t, w, 2, 120_000, 2, 99)

	if got := w.spendableCount(); got != 1 {
		t.Fatalf("expected 1 spendable utxo before expiry boundary, got %d", got)
	}
	if got := w.spendableBalance(); got != 120_000 {
		t.Fatalf("expected spendable balance 120000, got %d", got)
	}

	candidates := w.candidates(selectLargestConfirmed, false)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate after expiry filtering, got %d", len(candidates))
	}
	if candidates[0].utxo.BlockHeight != 99 {
		t.Fatalf("expected live candidate from height 99, got %d", candidates[0].utxo.BlockHeight)
	}
}

func TestSpendableCountIgnoresExpiryOnNonOBTC(t *testing.T) {
	w := newTestWallet(t)
	w.currentHeight = 241
	addConfirmedUTXOAtHeight(t, w, 1, 110_000, 1, 98)
	addConfirmedUTXOAtHeight(t, w, 2, 120_000, 2, 99)

	if got := w.spendableCount(); got != 2 {
		t.Fatalf("expected simnet wallet to keep both utxos spendable, got %d", got)
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
	addConfirmedUTXOAtHeight(t, w, keyIndex, value, hashByte, 0)
}

func addConfirmedUTXOAtHeight(t *testing.T, w *wallet, keyIndex uint32, value int64,
	hashByte byte, blockHeight int32) {

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
		BlockHeight:    blockHeight,
		MaturityHeight: 0,
		Confirmed:      true,
	}
}

func outPointsOf(utxos []*walletUTXO) []wire.OutPoint {
	points := make([]wire.OutPoint, 0, len(utxos))
	for _, utxo := range utxos {
		points = append(points, utxo.OutPoint)
	}
	return points
}
