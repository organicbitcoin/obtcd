// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

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
