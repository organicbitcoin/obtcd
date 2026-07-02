// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func makeOBTCReplayTaprootKeySpendTestTx(t *testing.T) (*btcec.PrivateKey,
	*wire.MsgTx, *wire.TxOut, PrevOutputFetcher, *TxSigHashes) {

	t.Helper()

	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	outputKey := ComputeTaprootKeyNoScript(privKey.PubKey())
	pkScript, err := PayToTaprootScript(outputKey)
	require.NoError(t, err)

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
	})

	prevOut := &wire.TxOut{
		Value:    1e8,
		PkScript: pkScript,
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    prevOut.Value - 1_000,
		PkScript: []byte{OP_TRUE},
	})

	prevFetcher := NewCannedPrevOutputFetcher(prevOut.PkScript, prevOut.Value)
	sigHashes := NewTxSigHashes(tx, prevFetcher)

	return privKey, tx, prevOut, prevFetcher, sigHashes
}

func makeOBTCReplayTapscriptSpendTestTx(t *testing.T) (*btcec.PrivateKey,
	*wire.MsgTx, *wire.TxOut, TapLeaf, []byte, PrevOutputFetcher, *TxSigHashes) {

	t.Helper()

	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	internalKey := privKey.PubKey()
	tapScript, err := NewScriptBuilder().
		AddData(schnorr.SerializePubKey(internalKey)).
		AddOp(OP_CHECKSIG).
		Script()
	require.NoError(t, err)

	tapLeaf := NewBaseTapLeaf(tapScript)
	tapScriptTree := AssembleTaprootScriptTree(tapLeaf)
	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		internalKey,
	)
	ctrlBlockBytes, err := ctrlBlock.ToBytes()
	require.NoError(t, err)

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := ComputeTaprootOutputKey(internalKey, tapScriptRootHash[:])
	pkScript, err := PayToTaprootScript(outputKey)
	require.NoError(t, err)

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
	})

	prevOut := &wire.TxOut{
		Value:    1e8,
		PkScript: pkScript,
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    prevOut.Value - 1_000,
		PkScript: []byte{OP_TRUE},
	})

	prevFetcher := NewCannedPrevOutputFetcher(prevOut.PkScript, prevOut.Value)
	sigHashes := NewTxSigHashes(tx, prevFetcher)

	return privKey, tx, prevOut, tapLeaf, ctrlBlockBytes, prevFetcher, sigHashes
}

func TestCalcTaprootSignatureHashOBTCReplayProtectionOption(t *testing.T) {
	t.Parallel()

	_, tx, prevOut, prevFetcher, sigHashes := makeOBTCReplayTaprootKeySpendTestTx(t)
	hashType := SigHashOBTCReplayProtection | SigHashAll

	_, err := CalcTaprootSignatureHash(sigHashes, hashType, tx, 0, prevFetcher)
	require.Error(t, err)

	hash, err := CalcTaprootSignatureHash(
		sigHashes, hashType, tx, 0, prevFetcher,
		WithOBTCReplayProtectionSighash(),
	)
	require.NoError(t, err)
	require.Len(t, hash, 32)
	require.NotEqual(t, make([]byte, 32), hash)
	require.NotNil(t, prevOut)
}

func TestRawTxInTaprootSignatureOBTCReplayProtectionOption(t *testing.T) {
	t.Parallel()

	privKey, tx, prevOut, prevFetcher, sigHashes := makeOBTCReplayTaprootKeySpendTestTx(t)
	hashType := SigHashOBTCReplayProtection | SigHashAll

	_, err := RawTxInTaprootSignature(
		tx, sigHashes, 0, prevOut.Value, prevOut.PkScript, nil,
		hashType, privKey,
	)
	require.Error(t, err)

	sig, err := RawTxInTaprootSignature(
		tx, sigHashes, 0, prevOut.Value, prevOut.PkScript, nil,
		hashType, privKey, WithOBTCReplayProtectionSighash(),
	)
	require.NoError(t, err)

	txCopy := tx.Copy()
	txCopy.TxIn[0].Witness = wire.TxWitness{sig}
	vm, err := NewEngine(
		prevOut.PkScript, txCopy, 0,
		StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
		nil, sigHashes, prevOut.Value, prevFetcher,
	)
	require.NoError(t, err)
	require.NoError(t, vm.Execute())
}

func TestTaprootKeyPathVMOBTCReplayProtectedSigHashMatrix(t *testing.T) {
	t.Parallel()

	for _, test := range obtcReplayProtectedSigHashMatrix() {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			privKey, tx, prevOut, prevFetcher, sigHashes :=
				makeOBTCReplayTaprootKeySpendTestTx(t)
			sig, err := RawTxInTaprootSignature(
				tx, sigHashes, 0, prevOut.Value, prevOut.PkScript,
				nil, test.hash, privKey,
				WithOBTCReplayProtectionSighash(),
			)
			require.NoError(t, err)

			txCopy := tx.Copy()
			txCopy.TxIn[0].Witness = wire.TxWitness{sig}
			vm, err := NewEngine(
				prevOut.PkScript, txCopy, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevOut.Value, prevFetcher,
			)
			require.NoError(t, err)
			require.NoError(t, vm.Execute())
		})
	}
}

func TestTaprootKeyPathVMRejectsInvalidOBTCReplayHashTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		makeSig func(t *testing.T, privKey *btcec.PrivateKey,
			tx *wire.MsgTx, prevOut *wire.TxOut,
			sigHashes *TxSigHashes) []byte
	}{
		{
			name: "missing replay bit",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTaprootSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, nil, SigHashAll, privKey,
				)
				require.NoError(t, err)
				return sig
			},
		},
		{
			name: "taproot default",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTaprootSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, nil, SigHashDefault,
					privKey,
				)
				require.NoError(t, err)
				return sig
			},
		},
		{
			name: "unknown extra bits",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTaprootSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, nil,
					SigHashOBTCReplayProtection|SigHashAll,
					privKey, WithOBTCReplayProtectionSighash(),
				)
				require.NoError(t, err)
				require.Len(t, sig, schnorr.SignatureSize+1)
				sig[schnorr.SignatureSize] = byte(
					SigHashOBTCReplayProtection | 0x20 | SigHashAll,
				)
				return sig
			},
		},
		{
			name: "base type zero",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTaprootSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, nil,
					SigHashOBTCReplayProtection|SigHashAll,
					privKey, WithOBTCReplayProtectionSighash(),
				)
				require.NoError(t, err)
				require.Len(t, sig, schnorr.SignatureSize+1)
				sig[schnorr.SignatureSize] = byte(
					SigHashOBTCReplayProtection | SigHashDefault,
				)
				return sig
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			privKey, tx, prevOut, prevFetcher, sigHashes :=
				makeOBTCReplayTaprootKeySpendTestTx(t)
			tx.TxIn[0].Witness = wire.TxWitness{
				test.makeSig(t, privKey, tx, prevOut, sigHashes),
			}
			vm, err := NewEngine(
				prevOut.PkScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevOut.Value, prevFetcher,
			)
			require.NoError(t, err)
			require.Error(t, vm.Execute())
		})
	}
}

func TestCalcTapscriptSignatureHashOBTCReplayProtectionOption(t *testing.T) {
	t.Parallel()

	_, tx, _, tapLeaf, _, prevFetcher, sigHashes := makeOBTCReplayTapscriptSpendTestTx(t)
	hashType := SigHashOBTCReplayProtection | SigHashAll

	_, err := CalcTapscriptSignaturehash(
		sigHashes, hashType, tx, 0, prevFetcher, tapLeaf,
	)
	require.Error(t, err)

	hash, err := CalcTapscriptSignaturehash(
		sigHashes, hashType, tx, 0, prevFetcher, tapLeaf,
		WithOBTCReplayProtectionSighash(),
	)
	require.NoError(t, err)
	require.Len(t, hash, 32)
	require.NotEqual(t, make([]byte, 32), hash)
}

func TestRawTxInTapscriptSignatureOBTCReplayProtectionOption(t *testing.T) {
	t.Parallel()

	privKey, tx, prevOut, tapLeaf, ctrlBlockBytes, prevFetcher,
		sigHashes := makeOBTCReplayTapscriptSpendTestTx(t)
	hashType := SigHashOBTCReplayProtection | SigHashAll

	_, err := RawTxInTapscriptSignature(
		tx, sigHashes, 0, prevOut.Value, prevOut.PkScript, tapLeaf,
		hashType, privKey,
	)
	require.Error(t, err)

	sig, err := RawTxInTapscriptSignature(
		tx, sigHashes, 0, prevOut.Value, prevOut.PkScript, tapLeaf,
		hashType, privKey, WithOBTCReplayProtectionSighash(),
	)
	require.NoError(t, err)

	tapScript := tapLeaf.Script
	txCopy := tx.Copy()
	txCopy.TxIn[0].Witness = wire.TxWitness{
		sig, tapScript, ctrlBlockBytes,
	}
	vm, err := NewEngine(
		prevOut.PkScript, txCopy, 0,
		StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
		nil, sigHashes, prevOut.Value, prevFetcher,
	)
	require.NoError(t, err)
	require.NoError(t, vm.Execute())
}

func TestTaprootScriptPathVMOBTCReplayProtectedSigHashMatrix(t *testing.T) {
	t.Parallel()

	for _, test := range obtcReplayProtectedSigHashMatrix() {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			privKey, tx, prevOut, tapLeaf, ctrlBlockBytes,
				prevFetcher, sigHashes :=
				makeOBTCReplayTapscriptSpendTestTx(t)
			sig, err := RawTxInTapscriptSignature(
				tx, sigHashes, 0, prevOut.Value, prevOut.PkScript,
				tapLeaf, test.hash, privKey,
				WithOBTCReplayProtectionSighash(),
			)
			require.NoError(t, err)

			txCopy := tx.Copy()
			txCopy.TxIn[0].Witness = wire.TxWitness{
				sig, tapLeaf.Script, ctrlBlockBytes,
			}
			vm, err := NewEngine(
				prevOut.PkScript, txCopy, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevOut.Value, prevFetcher,
			)
			require.NoError(t, err)
			require.NoError(t, vm.Execute())
		})
	}
}

func TestTaprootScriptPathVMRejectsInvalidOBTCReplayHashTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		makeSig func(t *testing.T, privKey *btcec.PrivateKey,
			tx *wire.MsgTx, prevOut *wire.TxOut, tapLeaf TapLeaf,
			sigHashes *TxSigHashes) []byte
	}{
		{
			name: "missing replay bit",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				tapLeaf TapLeaf,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTapscriptSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, tapLeaf, SigHashAll,
					privKey,
				)
				require.NoError(t, err)
				return sig
			},
		},
		{
			name: "taproot default",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				tapLeaf TapLeaf,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTapscriptSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, tapLeaf, SigHashDefault,
					privKey,
				)
				require.NoError(t, err)
				return sig
			},
		},
		{
			name: "unknown extra bits",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				tapLeaf TapLeaf,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTapscriptSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, tapLeaf,
					SigHashOBTCReplayProtection|SigHashAll,
					privKey, WithOBTCReplayProtectionSighash(),
				)
				require.NoError(t, err)
				require.Len(t, sig, schnorr.SignatureSize+1)
				sig[schnorr.SignatureSize] = byte(
					SigHashOBTCReplayProtection | 0x20 | SigHashAll,
				)
				return sig
			},
		},
		{
			name: "base type zero",
			makeSig: func(t *testing.T, privKey *btcec.PrivateKey,
				tx *wire.MsgTx, prevOut *wire.TxOut,
				tapLeaf TapLeaf,
				sigHashes *TxSigHashes) []byte {

				t.Helper()

				sig, err := RawTxInTapscriptSignature(
					tx, sigHashes, 0, prevOut.Value,
					prevOut.PkScript, tapLeaf,
					SigHashOBTCReplayProtection|SigHashAll,
					privKey, WithOBTCReplayProtectionSighash(),
				)
				require.NoError(t, err)
				require.Len(t, sig, schnorr.SignatureSize+1)
				sig[schnorr.SignatureSize] = byte(
					SigHashOBTCReplayProtection | SigHashDefault,
				)
				return sig
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			privKey, tx, prevOut, tapLeaf, ctrlBlockBytes,
				prevFetcher, sigHashes :=
				makeOBTCReplayTapscriptSpendTestTx(t)
			tx.TxIn[0].Witness = wire.TxWitness{
				test.makeSig(
					t, privKey, tx, prevOut, tapLeaf, sigHashes,
				),
				tapLeaf.Script,
				ctrlBlockBytes,
			}
			vm, err := NewEngine(
				prevOut.PkScript, tx, 0,
				StandardVerifyFlags|ScriptVerifyOBTCReplayProtection,
				nil, sigHashes, prevOut.Value, prevFetcher,
			)
			require.NoError(t, err)
			require.Error(t, vm.Execute())
		})
	}
}
