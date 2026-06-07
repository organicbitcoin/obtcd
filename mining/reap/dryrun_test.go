// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildDryRunSummary(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 1100, 11)
	op2 := addUtxo(t, view, 2001, 12)

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 99, Inputs: []wire.OutPoint{op1, op2}}

	s, err := BuildDryRunSummary(plan, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if s.Picked != 2 {
		t.Fatalf("picked mismatch: %d", s.Picked)
	}
	wantTax := taxForValue(1100, p) + taxForValue(2001, p)
	if s.TaxTotal != wantTax {
		t.Fatalf("tax mismatch got=%d want=%d", s.TaxTotal, wantTax)
	}
	if s.RefundTotal != 3101-wantTax {
		t.Fatalf("refund mismatch got=%d", s.RefundTotal)
	}
	if s.EstWeight != EstimateBlueprintWeight(2) {
		t.Fatalf("weight mismatch got=%d", s.EstWeight)
	}
	if s.MarkerHash == "" {
		t.Fatalf("marker hash should not be empty")
	}
}

func TestBuildDryRunSummaryMissingUTXO(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)
	_, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{{}}}, view, p)
	if err == nil {
		t.Fatalf("expected missing utxo error")
	}
}

func TestBuildDryRunSummaryDustFoldingTotals(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 700, 33) // tax=210 refund=490, dust-fold to tax

	p := DefaultREAPParams(SortModeStrict)
	p.DustThresholdSat = 720

	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{op}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if s.TaxTotal != 700 || s.RefundTotal != 0 {
		t.Fatalf("dust folding mismatch tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}
}

func TestBuildDryRunSummaryTwoTierDustWeightEstimate(t *testing.T) {
	const dustMax = 1024

	view := blockchain.NewUtxoViewpoint()
	inputs := make([]wire.OutPoint, 0, dustMax)
	for i := 0; i < dustMax; i++ {
		inputs = append(inputs, addUtxo(t, view, 719, uint32(1000+i)))
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 256
	p.DustMaxInputs = dustMax
	p.WeightBudget = 200_000

	s, err := BuildDryRunSummary(REAPPlan{Inputs: inputs}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	wantWeight := EstimateTieredBlueprintWeight(dustMax, 0)
	if s.EstWeight != wantWeight {
		t.Fatalf("weight mismatch got=%d want=%d", s.EstWeight, wantWeight)
	}
	if s.EstWeight >= EstimateBlueprintWeight(dustMax) {
		t.Fatalf("two-tier dust estimate should be below legacy upper bound: got=%d legacy=%d",
			s.EstWeight, EstimateBlueprintWeight(dustMax))
	}
	if s.EstWeight > p.WeightBudget {
		t.Fatalf("dust-only dry-run estimate should fit budget: got=%d budget=%d",
			s.EstWeight, p.WeightBudget)
	}
}
