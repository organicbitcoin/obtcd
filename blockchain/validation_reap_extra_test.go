// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

// ---------------------------------------------------------------------------
// reapTaxForValue
// ---------------------------------------------------------------------------

func TestReapTaxForValue(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	tests := []struct {
		name  string
		value int64
		want  int64
	}{
		{"standard 1000 sat", 1000, (1000 * ep.ReapTaxNumerator) / ep.ReapTaxDenominator},
		{"zero value", 0, 0},
		{"negative value", -100, 0},
		{"one satoshi", 1, (1 * ep.ReapTaxNumerator) / ep.ReapTaxDenominator},
		{"large value", 100_000_000, (100_000_000 * ep.ReapTaxNumerator) / ep.ReapTaxDenominator},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reapTaxForValue(tc.value, ep)
			if got != tc.want {
				t.Fatalf("reapTaxForValue(%d) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestReapTaxForValueNilParams(t *testing.T) {
	if got := reapTaxForValue(1000, nil); got != 0 {
		t.Fatalf("expected 0 tax with nil params, got %d", got)
	}
}

func TestReapTaxForValueZeroDenominator(t *testing.T) {
	ep := &chaincfg.ExpiryParams{
		ReapTaxNumerator:   30,
		ReapTaxDenominator: 0,
	}
	if got := reapTaxForValue(1000, ep); got != 0 {
		t.Fatalf("expected 0 tax with zero denominator, got %d", got)
	}
}

func TestReapTaxForValueZeroNumerator(t *testing.T) {
	ep := &chaincfg.ExpiryParams{
		ReapTaxNumerator:   0,
		ReapTaxDenominator: 100,
	}
	if got := reapTaxForValue(1000, ep); got != 0 {
		t.Fatalf("expected 0 tax with zero numerator, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// applyReapDustRule
// ---------------------------------------------------------------------------

func TestApplyReapDustRule(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}

	tests := []struct {
		name       string
		value      int64
		refund     int64
		tax        int64
		wantRefund int64
		wantTax    int64
	}{
		{
			name:       "above dust threshold",
			value:      1000,
			refund:     700,
			tax:        300,
			wantRefund: 700,
			wantTax:    300,
		},
		{
			name:       "below dust threshold",
			value:      500, // < 720
			refund:     350,
			tax:        150,
			wantRefund: 0,
			wantTax:    500,
		},
		{
			name:       "at dust threshold",
			value:      ep.ReapDustThresholdSat,
			refund:     504,
			tax:        216,
			wantRefund: 504,
			wantTax:    216,
		},
		{
			name:       "zero value",
			value:      0,
			refund:     0,
			tax:        0,
			wantRefund: 0,
			wantTax:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRefund, gotTax := applyReapDustRule(tc.value, tc.refund, tc.tax, ep)
			if gotRefund != tc.wantRefund || gotTax != tc.wantTax {
				t.Fatalf("applyReapDustRule(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.value, tc.refund, tc.tax,
					gotRefund, gotTax, tc.wantRefund, tc.wantTax)
			}
		})
	}
}

func TestApplyReapDustRuleNilParams(t *testing.T) {
	refund, tax := applyReapDustRule(100, 70, 30, nil)
	if refund != 70 || tax != 30 {
		t.Fatalf("expected passthrough with nil params, got refund=%d tax=%d", refund, tax)
	}
}

func TestApplyReapDustRuleDisabledThreshold(t *testing.T) {
	ep := &chaincfg.ExpiryParams{ReapDustThresholdSat: 0}
	refund, tax := applyReapDustRule(100, 70, 30, ep)
	if refund != 70 || tax != 30 {
		t.Fatalf("expected passthrough with zero threshold, got refund=%d tax=%d", refund, tax)
	}
}

// ---------------------------------------------------------------------------
// compareReapInputOrderKey
// ---------------------------------------------------------------------------

func TestCompareReapInputOrderKey(t *testing.T) {
	a := reapInputOrderKey{expiry: 100, amount: 500, op: wire.OutPoint{Index: 0}}
	b := reapInputOrderKey{expiry: 200, amount: 500, op: wire.OutPoint{Index: 0}}
	c := reapInputOrderKey{expiry: 100, amount: 600, op: wire.OutPoint{Index: 0}}
	d := reapInputOrderKey{expiry: 100, amount: 500, op: wire.OutPoint{Index: 1}}

	tests := []struct {
		name string
		a, b reapInputOrderKey
		want int
	}{
		{"lower expiry first", a, b, -1},
		{"higher expiry second", b, a, 1},
		{"same expiry, lower amount first", a, c, -1},
		{"same expiry, higher amount second", c, a, 1},
		{"same expiry+amount, lower index first", a, d, -1},
		{"same expiry+amount, higher index second", d, a, 1},
		{"equal keys", a, a, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareReapInputOrderKey(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("compareReapInputOrderKey = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isLikelyReapTx edge cases
// ---------------------------------------------------------------------------

func TestIsLikelyReapTxNil(t *testing.T) {
	if isLikelyReapTx(nil) {
		t.Fatalf("nil tx should not be identified as REAP")
	}
}

func TestIsLikelyReapTxEmptyOutputs(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	if isLikelyReapTx(tx) {
		t.Fatalf("tx with no outputs should not be identified as REAP")
	}
}

func TestIsLikelyReapTxNonZeroMarkerValue(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
	tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: markerForTx(t, tx, 100)})
	if isLikelyReapTx(tx) {
		t.Fatalf("tx with non-zero marker value should not be identified as REAP")
	}
}

func TestIsLikelyReapTxWrongVersion(t *testing.T) {
	for _, v := range []int32{1, 2, 4} {
		tx := wire.NewMsgTx(v)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 1}})
		tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 100)})
		if isLikelyReapTx(tx) {
			t.Fatalf("version %d tx should not be identified as REAP", v)
		}
	}
}

// ---------------------------------------------------------------------------
// checkReapBlockHardening edge cases
// ---------------------------------------------------------------------------

func TestCheckReapBlockHardeningNilBlock(t *testing.T) {
	if err := checkReapBlockHardening(nil, 500, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("nil block should not cause error: %v", err)
	}
}

func TestCheckReapBlockHardeningNilParams(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	block := makeBlockWithTxs(makeValidReapTx(t, view, 500, op))
	if err := checkReapBlockHardening(block, 500, nil); err != nil {
		t.Fatalf("nil params should not cause error: %v", err)
	}
}

func TestCheckReapBlockHardeningNonOBTCParams(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	block := makeBlockWithTxs(makeValidReapTx(t, view, 500, op))
	// Bitcoin MainNet has no ExpiryParams, so it should be a no-op.
	if err := checkReapBlockHardening(block, 500, &chaincfg.MainNetParams); err != nil {
		t.Fatalf("bitcoin params should not enforce REAP hardening: %v", err)
	}
}

func TestCheckReapBlockHardeningEmptyBlockAccepted(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcRegTestParams)
	if ep == nil {
		t.Fatalf("expected expiry params")
	}
	// Block with only coinbase (no REAP txs)
	block := makeBlockWithTxs()
	if err := checkReapBlockHardening(block, ep.ReapConsensusAtHeight, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("empty block should pass hardening: %v", err)
	}
}

// ---------------------------------------------------------------------------
// checkExpirySpendRules edge cases
// ---------------------------------------------------------------------------

func TestCheckExpirySpendRulesNonOBTCParams(t *testing.T) {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 0}})
	tx.AddTxOut(&wire.TxOut{Value: 100, PkScript: []byte{0x51}})
	if err := checkExpirySpendRules(tx, 500, NewUtxoViewpoint(), &chaincfg.MainNetParams); err != nil {
		t.Fatalf("bitcoin params should skip expiry enforcement: %v", err)
	}
}

// ---------------------------------------------------------------------------
// checkReapConsensusHardening edge cases
// ---------------------------------------------------------------------------

func TestCheckReapConsensusHardeningNilChainParams(t *testing.T) {
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 0}})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 500)})
	// nil chainParams is handled by the function (returns nil early)
	if err := checkReapConsensusHardening(tx, 500, NewUtxoViewpoint(), nil); err != nil {
		t.Fatalf("nil params should skip enforcement: %v", err)
	}
}

func TestCheckReapConsensusHardeningSingleInputSkipsOrderCheck(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	tx := makeValidReapTx(t, view, 500, op)

	if err := checkReapConsensusHardening(tx, 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("single input REAP should pass ordering check: %v", err)
	}
}

// ---------------------------------------------------------------------------
// checkReapTaxRules edge cases
// ---------------------------------------------------------------------------

func TestCheckReapTaxRulesNonOBTCParams(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	tx := wire.NewMsgTx(reapTxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerForTx(t, tx, 500)})
	// Bitcoin MainNet has no ExpiryParams, so tax rules should be a no-op.
	if err := checkReapTaxRules(tx, 500, view, &chaincfg.MainNetParams); err != nil {
		t.Fatalf("bitcoin params should skip tax enforcement: %v", err)
	}
}

func TestCheckReapTaxRulesNonReapTxSkipped(t *testing.T) {
	view := NewUtxoViewpoint()
	op := addUtxoToView(t, view, 1000, 1)
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	tx.AddTxOut(&wire.TxOut{Value: 900, PkScript: []byte{0x51}})
	if err := checkReapTaxRules(tx, 500, view, &chaincfg.ObtcRegTestParams); err != nil {
		t.Fatalf("non-REAP tx should skip tax rules: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExpiryChainAccessor
// ---------------------------------------------------------------------------

func TestNewExpiryChainAccessorNotNil(t *testing.T) {
	// We can't easily construct a full BlockChain in unit tests, but we can
	// verify the constructor doesn't panic with a nil chain (defensive).
	accessor := NewExpiryChainAccessor(nil)
	if accessor == nil {
		t.Fatalf("expected non-nil accessor")
	}
}
