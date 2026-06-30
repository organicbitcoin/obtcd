// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func makeOrdinarySpend(op wire.OutPoint, value, fee int64) *wire.MsgTx {
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{
		Value:    value - fee,
		PkScript: []byte{txscript.OP_TRUE},
	})
	return tx
}

func requireRuleErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected rule error %v", want)
	}
	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError %v, got %T: %v", want, err, err)
	}
	if ruleErr.ErrorCode != want {
		t.Fatalf("unexpected error code: got %v want %v: %v",
			ruleErr.ErrorCode, want, err)
	}
}

func TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries(t *testing.T) {
	t.Run("mainnet expired before activation is current-compatible", func(t *testing.T) {
		ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
		if ep == nil {
			t.Fatalf("expected mainnet expiry params")
		}

		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)
		tx := makeOrdinarySpend(op, 1000, 100)
		expiryHeight := int32(ep.CalculateExpiryKey(1))
		if expiryHeight >= ep.EnableAtHeight {
			t.Fatalf("test setup needs expiry before activation: expiry=%d activation=%d",
				expiryHeight, ep.EnableAtHeight)
		}

		if _, err := CheckTransactionInputs(
			btcutil.NewTx(tx), ep.EnableAtHeight-1, view,
			&chaincfg.ObtcMainNetParams,
		); err != nil {
			t.Fatalf("pre-activation expired ordinary spend should follow legacy behavior: %v", err)
		}

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), ep.EnableAtHeight, view,
			&chaincfg.ObtcMainNetParams,
		)
		requireRuleErrorCode(t, err, ErrBadTxInput)
		if !strings.Contains(err.Error(), "expired utxo") {
			t.Fatalf("expected expired-utxo rejection, got %v", err)
		}
	})

	tests := []struct {
		name   string
		params *chaincfg.Params
	}{
		{"mainnet", &chaincfg.ObtcMainNetParams},
		{"testnet", &chaincfg.ObtcTestNetParams},
		{"regtest", &chaincfg.ObtcRegTestParams},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ep := chaincfg.GetExpiryParams(tc.params)
			if ep == nil {
				t.Fatalf("expected expiry params")
			}

			createHeight := ep.EnableAtHeight
			expiryHeight := int32(ep.CalculateExpiryKey(createHeight))
			if expiryHeight <= ep.EnableAtHeight {
				t.Fatalf("test setup expected expiry after activation: expiry=%d activation=%d",
					expiryHeight, ep.EnableAtHeight)
			}

			view := NewUtxoViewpoint()
			op := addUtxoToView(t, view, 1000, createHeight)
			tx := makeOrdinarySpend(op, 1000, 100)

			if _, err := CheckTransactionInputs(
				btcutil.NewTx(tx), expiryHeight-1, view, tc.params,
			); err != nil {
				t.Fatalf("ordinary spend before expiry should pass for %s: %v",
					tc.name, err)
			}

			_, err := CheckTransactionInputs(
				btcutil.NewTx(tx), expiryHeight, view, tc.params,
			)
			requireRuleErrorCode(t, err, ErrBadTxInput)

			_, err = CheckTransactionInputs(
				btcutil.NewTx(tx), expiryHeight+1, view, tc.params,
			)
			requireRuleErrorCode(t, err, ErrBadTxInput)
		})
	}
}

func TestOBTCREAPInputValidityEdgeCases(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected regtest expiry params")
	}
	height := int32(ep.CalculateExpiryKey(1))

	t.Run("empty input REAP is rejected by transaction sanity", func(t *testing.T) {
		tx := wire.NewMsgTx(reapTxVersion)
		tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, height)})

		err := CheckTransactionSanity(btcutil.NewTx(tx))
		requireRuleErrorCode(t, err, ErrNoTxInputs)
	})

	t.Run("missing outpoint is rejected", func(t *testing.T) {
		var missingHash chainhash.Hash
		missingHash[0] = 0x99
		missing := wire.OutPoint{Hash: missingHash, Index: 0}

		tx := wire.NewMsgTx(reapTxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: missing})
		tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, height)})

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), height, NewUtxoViewpoint(),
			&chaincfg.ObtcRegTestParams,
		)
		requireRuleErrorCode(t, err, ErrMissingTxOut)
	})

	t.Run("duplicate input is rejected by transaction sanity", func(t *testing.T) {
		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)

		tx := makeValidReapTx(t, view, height, op, op)
		err := CheckTransactionSanity(btcutil.NewTx(tx))
		requireRuleErrorCode(t, err, ErrDuplicateTxInputs)
	})

	t.Run("version one marker spoof remains non-REAP and cannot spend expired UTXO", func(t *testing.T) {
		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)

		tx := wire.NewMsgTx(wire.TxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
		tx.AddTxOut(&wire.TxOut{Value: 700, PkScript: []byte{txscript.OP_TRUE}})
		tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, height)})

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), height, view, &chaincfg.ObtcRegTestParams,
		)
		requireRuleErrorCode(t, err, ErrBadTxInput)
		if !strings.Contains(err.Error(), "non-reap transaction") {
			t.Fatalf("expected non-REAP expired spend rejection, got %v", err)
		}
	})
}

func TestOBTCREAPMarkerMissingMultipleAndPlacement(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected regtest expiry params")
	}
	height := int32(ep.CalculateExpiryKey(1))

	t.Run("missing marker is treated as non-REAP expired spend", func(t *testing.T) {
		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)

		tx := wire.NewMsgTx(reapTxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
		tx.AddTxOut(&wire.TxOut{Value: 700, PkScript: []byte{txscript.OP_TRUE}})

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), height, view, &chaincfg.ObtcRegTestParams,
		)
		requireRuleErrorCode(t, err, ErrBadTxInput)
		if !strings.Contains(err.Error(), "non-reap transaction") {
			t.Fatalf("expected non-REAP expired spend rejection, got %v", err)
		}
	})

	t.Run("multiple marker outputs are rejected by refund validation", func(t *testing.T) {
		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)

		tx := makeValidReapTx(t, view, height, op)
		extraMarker := &wire.TxOut{
			Value:    0,
			PkScript: markerForTx(t, tx, height),
		}
		tx.TxOut = append(tx.TxOut[:len(tx.TxOut)-1],
			append([]*wire.TxOut{extraMarker}, tx.TxOut[len(tx.TxOut)-1])...)

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), height, view, &chaincfg.ObtcRegTestParams,
		)
		requireRuleErrorCode(t, err, ErrBadTxOutValue)
		if !strings.Contains(err.Error(), "refund outputs must be positive") {
			t.Fatalf("expected refund-output rejection, got %v", err)
		}
	})

	t.Run("marker in non-tail position is not recognized as REAP", func(t *testing.T) {
		view := NewUtxoViewpoint()
		op := addUtxoToView(t, view, 1000, 1)

		tx := wire.NewMsgTx(reapTxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
		tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, height)})
		tx.AddTxOut(&wire.TxOut{Value: 700, PkScript: []byte{txscript.OP_TRUE}})

		_, err := CheckTransactionInputs(
			btcutil.NewTx(tx), height, view, &chaincfg.ObtcRegTestParams,
		)
		requireRuleErrorCode(t, err, ErrBadTxInput)
		if !strings.Contains(err.Error(), "non-reap transaction") {
			t.Fatalf("expected non-REAP expired spend rejection, got %v", err)
		}
	})
}

func TestOBTCREAPTaxRefundDustAccountingMatrix(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected regtest expiry params")
	}
	height := int32(ep.CalculateExpiryKey(1))

	values := []int64{1, ep.ReapDustThresholdSat - 1, ep.ReapDustThresholdSat, 1000, 1_000_000}
	view := NewUtxoViewpoint()
	inputs := make([]wire.OutPoint, 0, len(values))
	var wantTax, wantRefund int64
	for i, value := range values {
		inputs = append(inputs, addUniqueUtxoToView(t, view, value, 1, uint32(i)))

		tax := reapTaxForValue(value, ep)
		refund := value - tax
		refund, tax = applyReapDustRule(value, refund, tax, ep)
		wantTax += tax
		wantRefund += refund
	}

	tx := makeValidReapTx(t, view, height, inputs...)
	var gotRefund int64
	for _, out := range tx.TxOut[:len(tx.TxOut)-1] {
		gotRefund += out.Value
	}
	if gotRefund != wantRefund {
		t.Fatalf("refund output total: got %d want %d", gotRefund, wantRefund)
	}

	gotFee, err := CheckTransactionInputs(
		btcutil.NewTx(tx), height, view, &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("expected valid mixed dust/normal REAP tx: %v", err)
	}
	if gotFee != wantTax {
		t.Fatalf("REAP fee/tax mismatch: got %d want %d", gotFee, wantTax)
	}

	if wantTax+wantRefund != sumInt64(values) {
		t.Fatalf("test accounting invariant broken: tax=%d refund=%d inputs=%d",
			wantTax, wantRefund, sumInt64(values))
	}

	badTx := tx.Copy()
	if len(badTx.TxOut) < 2 {
		t.Fatalf("test setup expected at least one refund output plus marker")
	}
	badTx.TxOut[0].Value++
	_, err = CheckTransactionInputs(
		btcutil.NewTx(badTx), height, view, &chaincfg.ObtcRegTestParams,
	)
	requireRuleErrorCode(t, err, ErrBadTxOutValue)
	if !strings.Contains(err.Error(), "refund total mismatch") {
		t.Fatalf("expected refund total mismatch, got %v", err)
	}
}

func sumInt64(values []int64) int64 {
	var sum int64
	for _, v := range values {
		sum += v
	}
	return sum
}

func TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-fullblock-reap-coinbase-overclaim",
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	chain.TstSetCoinbaseMaturity(1)

	blocks, spends := buildOBTCBlocks(t, chain, 144)
	expired := spends[1][0]

	view := loadReapView(t, chain, expired.PrevOut)
	reapTx := makeValidReapTx(t, view, 145, expired.PrevOut)
	reapFee, err := CheckTransactionInputs(
		btcutil.NewTx(reapTx), 145, view, &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		t.Fatalf("valid REAP tx failed input accounting: %v", err)
	}
	if reapFee <= 0 {
		t.Fatalf("test setup expected positive REAP tax/fee, got %d", reapFee)
	}

	chain.reapPrefixSource = staticReapPrefixSource{
		tipHeight: 144,
		candidates: []ReapPrefixCandidate{{
			OutPoint: expired.PrevOut,
		}},
	}

	block := newOBTCBlock(t, chain, blocks[143], reapTx)
	block.MsgBlock().Transactions[0].TxOut[0].Value += reapFee + 1
	block.MsgBlock().Header.MerkleRoot = calcMerkleRoot(block.MsgBlock().Transactions)

	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	requireRuleErrorCode(t, err, ErrBadCoinbaseValue)
}
