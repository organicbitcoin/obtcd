// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func obtcReplayProtectedSigHashMatrix() []struct {
	name string
	hash SigHashType
} {
	return []struct {
		name string
		hash SigHashType
	}{
		{
			name: "all",
			hash: SigHashOBTCReplayProtection | SigHashAll,
		},
		{
			name: "none",
			hash: SigHashOBTCReplayProtection | SigHashNone,
		},
		{
			name: "single",
			hash: SigHashOBTCReplayProtection | SigHashSingle,
		},
		{
			name: "anyonecanpay_all",
			hash: SigHashOBTCReplayProtection | SigHashAnyOneCanPay |
				SigHashAll,
		},
		{
			name: "anyonecanpay_none",
			hash: SigHashOBTCReplayProtection | SigHashAnyOneCanPay |
				SigHashNone,
		},
		{
			name: "anyonecanpay_single",
			hash: SigHashOBTCReplayProtection | SigHashAnyOneCanPay |
				SigHashSingle,
		},
	}
}

func obtcReplayInvalidSigHashMatrix() []struct {
	name string
	hash SigHashType
} {
	return []struct {
		name string
		hash SigHashType
	}{
		{
			name: "missing replay bit",
			hash: SigHashAll,
		},
		{
			name: "unknown extra bits",
			hash: SigHashOBTCReplayProtection | 0x20 | SigHashAll,
		},
		{
			name: "base type zero",
			hash: SigHashOBTCReplayProtection | SigHashDefault,
		},
	}
}

func makeOBTCReplaySighashTestTx() (*wire.MsgTx, PrevOutputFetcher, *TxSigHashes) {
	tx := wire.NewMsgTx(2)

	var prevHash chainhash.Hash
	prevHash[0] = 1
	prevOut := wire.NewOutPoint(&prevHash, 0)
	txIn := wire.NewTxIn(prevOut, nil, nil)
	txIn.Sequence = 0xfffffffd
	tx.AddTxIn(txIn)

	const prevValue = int64(10_000)
	pkScript := []byte{OP_TRUE}
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 500,
		PkScript: pkScript,
	})

	prevOutFetcher := NewCannedPrevOutputFetcher(pkScript, prevValue)
	sigHashes := NewTxSigHashes(tx, prevOutFetcher)

	return tx, prevOutFetcher, sigHashes
}

func TestIsValidTaprootSigHashOBTCReplayProtectedRejectedByDefault(t *testing.T) {
	if !isValidTaprootSigHash(SigHashAll) {
		t.Fatalf("expected SigHashAll to remain a valid taproot sighash")
	}

	if isValidTaprootSigHash(SigHashOBTCReplayProtection | SigHashAll) {
		t.Fatalf("OBTC replay-protected taproot sighash should require explicit enablement")
	}
}

func TestCalcTaprootSignatureHashRawOBTCReplayProtectionGated(t *testing.T) {
	tx, prevOutFetcher, sigHashes := makeOBTCReplaySighashTestTx()
	hashType := SigHashOBTCReplayProtection | SigHashAll

	if _, err := calcTaprootSignatureHashRaw(
		sigHashes, hashType, tx, 0, prevOutFetcher,
	); err == nil {
		t.Fatalf("expected OBTC replay-protected taproot sighash to be rejected without option")
	}

	if _, err := calcTaprootSignatureHashRaw(
		sigHashes, hashType, tx, 0, prevOutFetcher,
		WithOBTCReplayProtectionSighash(),
	); err != nil {
		t.Fatalf("expected OBTC replay-protected taproot sighash with option to succeed: %v", err)
	}
}

func TestLegacyAndWitnessReplayDomainTagGated(t *testing.T) {
	tx, _, sigHashes := makeOBTCReplaySighashTestTx()
	hashType := SigHashOBTCReplayProtection | SigHashAll
	subScript := []byte{OP_TRUE}

	legacyReplay := calcSignatureHashWithReplayProtection(
		subScript, hashType, tx, 0, true,
	)
	legacyBase := calcSignatureHashWithReplayProtection(
		subScript, hashType, tx, 0, false,
	)
	if bytes.Equal(legacyReplay, legacyBase) {
		t.Fatalf("expected legacy replay-protected and base domains to differ")
	}

	witnessReplay, err := calcWitnessSignatureHashRawWithReplayProtection(
		subScript, sigHashes, hashType, tx, 0, 10_000, true,
	)
	if err != nil {
		t.Fatalf("unexpected witness replay-domain hash error: %v", err)
	}

	witnessBase, err := calcWitnessSignatureHashRawWithReplayProtection(
		subScript, sigHashes, hashType, tx, 0, 10_000, false,
	)
	if err != nil {
		t.Fatalf("unexpected witness base-domain hash error: %v", err)
	}

	if bytes.Equal(witnessReplay, witnessBase) {
		t.Fatalf("expected witness replay-protected and base domains to differ")
	}
}

func TestCalcWitnessSigHashOBTCReplayProtectionWrapper(t *testing.T) {
	tx, _, sigHashes := makeOBTCReplaySighashTestTx()
	hashType := SigHashOBTCReplayProtection | SigHashAll
	subScript := []byte{OP_TRUE}

	got, err := CalcWitnessSigHash(
		subScript, sigHashes, hashType, tx, 0, 10_000,
	)
	if err != nil {
		t.Fatalf("unexpected witness wrapper error: %v", err)
	}

	want, err := calcWitnessSignatureHashRawWithReplayProtection(
		subScript, sigHashes, hashType, tx, 0, 10_000, true,
	)
	if err != nil {
		t.Fatalf("unexpected witness raw error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("expected witness wrapper hash to match raw helper")
	}
}

func makeOBTCReplayEngineSpendTx(value int64) *wire.MsgTx {
	tx := wire.NewMsgTx(2)

	var prevHash chainhash.Hash
	prevHash[0] = 0x41
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    value - 1000,
		PkScript: []byte{OP_TRUE},
	})

	return tx
}

func makeOBTCReplayP2PKHScript(t *testing.T,
	privKey *btcec.PrivateKey) []byte {

	t.Helper()

	pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
	addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	pkScript, err := PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("PayToAddrScript: %v", err)
	}
	return pkScript
}

func makeOBTCReplayP2WPKHScripts(t *testing.T,
	privKey *btcec.PrivateKey) (fundingScript, subScript []byte) {

	t.Helper()

	pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(
		pubKeyHash, &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("NewAddressWitnessPubKeyHash: %v", err)
	}
	fundingScript, err = PayToAddrScript(witnessAddr)
	if err != nil {
		t.Fatalf("PayToAddrScript witness: %v", err)
	}

	pubKeyAddr, err := btcutil.NewAddressPubKeyHash(
		pubKeyHash, &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	subScript, err = PayToAddrScript(pubKeyAddr)
	if err != nil {
		t.Fatalf("PayToAddrScript p2pkh subscript: %v", err)
	}

	return fundingScript, subScript
}

func makeOBTCReplayP2WSHMultisigScripts(t *testing.T,
	privKey *btcec.PrivateKey) (fundingScript, witnessScript []byte) {

	t.Helper()

	pubKeyOne, err := btcutil.NewAddressPubKey(
		privKey.PubKey().SerializeCompressed(),
		&chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("NewAddressPubKey one: %v", err)
	}

	secondKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey second: %v", err)
	}
	pubKeyTwo, err := btcutil.NewAddressPubKey(
		secondKey.PubKey().SerializeCompressed(),
		&chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("NewAddressPubKey two: %v", err)
	}

	witnessScript, err = MultiSigScript(
		[]*btcutil.AddressPubKey{pubKeyOne, pubKeyTwo}, 1,
	)
	if err != nil {
		t.Fatalf("MultiSigScript: %v", err)
	}

	scriptHash := sha256.Sum256(witnessScript)
	witnessAddr, err := btcutil.NewAddressWitnessScriptHash(
		scriptHash[:], &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("NewAddressWitnessScriptHash: %v", err)
	}
	fundingScript, err = PayToAddrScript(witnessAddr)
	if err != nil {
		t.Fatalf("PayToAddrScript p2wsh: %v", err)
	}

	return fundingScript, witnessScript
}

func signOBTCReplayP2WSHMultisigWitness(t *testing.T, tx *wire.MsgTx,
	sigHashes *TxSigHashes, prevValue int64, witnessScript []byte,
	hashType SigHashType, privKey *btcec.PrivateKey) wire.TxWitness {

	t.Helper()

	sig, err := RawTxInWitnessSignature(
		tx, sigHashes, 0, prevValue, witnessScript, hashType, privKey,
	)
	if err != nil {
		t.Fatalf("RawTxInWitnessSignature: %v", err)
	}

	return wire.TxWitness{nil, sig, witnessScript}
}

func TestLegacyVMRequiresOBTCReplayProtectedHashType(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	pkScript := makeOBTCReplayP2PKHScript(t, privKey)
	const prevValue = int64(10_000)

	tests := []struct {
		name    string
		hash    SigHashType
		wantErr bool
	}{
		{
			name:    "legacy sighash rejected after replay activation",
			hash:    SigHashAll,
			wantErr: true,
		},
		{
			name: "replay-protected sighash accepted",
			hash: SigHashOBTCReplayProtection | SigHashAll,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			sigScript, err := SignatureScript(
				tx, 0, pkScript, test.hash, privKey, true,
			)
			if err != nil {
				t.Fatalf("SignatureScript: %v", err)
			}
			tx.TxIn[0].SignatureScript = sigScript

			vm, err := NewEngine(
				pkScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, nil, prevValue, nil,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}

			err = vm.Execute()
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected replay-protection failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected replay-protected legacy spend to pass: %v", err)
			}
		})
	}
}

func TestLegacyVMOBTCReplayProtectedSigHashMatrix(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	pkScript := makeOBTCReplayP2PKHScript(t, privKey)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayProtectedSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			sigScript, err := SignatureScript(
				tx, 0, pkScript, test.hash, privKey, true,
			)
			if err != nil {
				t.Fatalf("SignatureScript: %v", err)
			}
			tx.TxIn[0].SignatureScript = sigScript

			vm, err := NewEngine(
				pkScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, nil, prevValue, nil,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err != nil {
				t.Fatalf("expected legacy %s spend to pass: %v",
					test.name, err)
			}
		})
	}
}

func TestLegacyVMRejectsInvalidOBTCReplayHashTypes(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	pkScript := makeOBTCReplayP2PKHScript(t, privKey)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayInvalidSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			sigScript, err := SignatureScript(
				tx, 0, pkScript, test.hash, privKey, true,
			)
			if err != nil {
				t.Fatalf("SignatureScript: %v", err)
			}
			tx.TxIn[0].SignatureScript = sigScript

			vm, err := NewEngine(
				pkScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, nil, prevValue, nil,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err == nil {
				t.Fatalf("expected legacy %s spend to fail",
					test.name)
			}
		})
	}
}

func TestSegWitV0VMRequiresOBTCReplayProtectedHashType(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	fundingScript, subScript := makeOBTCReplayP2WPKHScripts(t, privKey)
	const prevValue = int64(10_000)

	tests := []struct {
		name    string
		hash    SigHashType
		wantErr bool
	}{
		{
			name:    "legacy segwit sighash rejected after replay activation",
			hash:    SigHashAll,
			wantErr: true,
		},
		{
			name: "replay-protected segwit sighash accepted",
			hash: SigHashOBTCReplayProtection | SigHashAll,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			prevFetcher := NewCannedPrevOutputFetcher(fundingScript, prevValue)
			sigHashes := NewTxSigHashes(tx, prevFetcher)
			witness, err := WitnessSignature(
				tx, sigHashes, 0, prevValue, subScript, test.hash,
				privKey, true,
			)
			if err != nil {
				t.Fatalf("WitnessSignature: %v", err)
			}
			tx.TxIn[0].Witness = witness

			vm, err := NewEngine(
				fundingScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyWitness|
					ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevValue, prevFetcher,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}

			err = vm.Execute()
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected replay-protection failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected replay-protected segwit spend to pass: %v", err)
			}
		})
	}
}

func TestSegWitV0P2WPKHVMOBTCReplayProtectedSigHashMatrix(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	fundingScript, subScript := makeOBTCReplayP2WPKHScripts(t, privKey)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayProtectedSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			prevFetcher := NewCannedPrevOutputFetcher(
				fundingScript, prevValue,
			)
			sigHashes := NewTxSigHashes(tx, prevFetcher)
			witness, err := WitnessSignature(
				tx, sigHashes, 0, prevValue, subScript, test.hash,
				privKey, true,
			)
			if err != nil {
				t.Fatalf("WitnessSignature: %v", err)
			}
			tx.TxIn[0].Witness = witness

			vm, err := NewEngine(
				fundingScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevValue, prevFetcher,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err != nil {
				t.Fatalf("expected p2wpkh %s spend to pass: %v",
					test.name, err)
			}
		})
	}
}

func TestSegWitV0P2WPKHVMRejectsInvalidOBTCReplayHashTypes(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	fundingScript, subScript := makeOBTCReplayP2WPKHScripts(t, privKey)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayInvalidSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			prevFetcher := NewCannedPrevOutputFetcher(
				fundingScript, prevValue,
			)
			sigHashes := NewTxSigHashes(tx, prevFetcher)
			witness, err := WitnessSignature(
				tx, sigHashes, 0, prevValue, subScript, test.hash,
				privKey, true,
			)
			if err != nil {
				t.Fatalf("WitnessSignature: %v", err)
			}
			tx.TxIn[0].Witness = witness

			vm, err := NewEngine(
				fundingScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevValue, prevFetcher,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err == nil {
				t.Fatalf("expected p2wpkh %s spend to fail",
					test.name)
			}
		})
	}
}

func TestSegWitV0P2WSHMultisigVMOBTCReplayProtectedSigHashMatrix(
	t *testing.T) {

	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	fundingScript, witnessScript := makeOBTCReplayP2WSHMultisigScripts(
		t, privKey,
	)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayProtectedSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			prevFetcher := NewCannedPrevOutputFetcher(
				fundingScript, prevValue,
			)
			sigHashes := NewTxSigHashes(tx, prevFetcher)
			tx.TxIn[0].Witness = signOBTCReplayP2WSHMultisigWitness(
				t, tx, sigHashes, prevValue, witnessScript,
				test.hash, privKey,
			)

			vm, err := NewEngine(
				fundingScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevValue, prevFetcher,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err != nil {
				t.Fatalf("expected p2wsh multisig %s spend to pass: %v",
					test.name, err)
			}
		})
	}
}

func TestSegWitV0P2WSHMultisigVMRejectsInvalidOBTCReplayHashTypes(
	t *testing.T) {

	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	fundingScript, witnessScript := makeOBTCReplayP2WSHMultisigScripts(
		t, privKey,
	)
	const prevValue = int64(10_000)

	for _, test := range obtcReplayInvalidSigHashMatrix() {
		t.Run(test.name, func(t *testing.T) {
			tx := makeOBTCReplayEngineSpendTx(prevValue)
			prevFetcher := NewCannedPrevOutputFetcher(
				fundingScript, prevValue,
			)
			sigHashes := NewTxSigHashes(tx, prevFetcher)
			tx.TxIn[0].Witness = signOBTCReplayP2WSHMultisigWitness(
				t, tx, sigHashes, prevValue, witnessScript,
				test.hash, privKey,
			)

			vm, err := NewEngine(
				fundingScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevValue, prevFetcher,
			)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if err := vm.Execute(); err == nil {
				t.Fatalf("expected p2wsh multisig %s spend to fail",
					test.name)
			}
		})
	}
}
