// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func largeSpendableScript(size int, marker byte) []byte {
	script := bytes.Repeat([]byte{0x51}, size)
	if size > 0 {
		script[size-1] = marker
	}
	return script
}

func blueprintWeight(t *testing.T, plan REAPPlan, view *blockchain.UtxoViewpoint,
	p REAPParams) int64 {

	t.Helper()
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("BuildBlueprint failed: %v", err)
	}
	return blockchain.GetTransactionWeight(btcutil.NewTx(tx))
}

func TestBuildBudgetedBlueprintEmptyPlan(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)

	tx, plan, weight, err := BuildBudgetedBlueprint(REAPPlan{}, view, p, 0)
	if err != nil {
		t.Fatalf("empty plan should not fail: %v", err)
	}
	if tx != nil || weight != 0 || len(plan.Inputs) != 0 {
		t.Fatalf("unexpected empty plan result tx=%v weight=%d inputs=%d",
			tx, weight, len(plan.Inputs))
	}
}

func TestBuildBudgetedBlueprintKeepsPlanWithinSoftBudget(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 10_000, 1)
	op2 := addUtxo(t, view, 20_000, 2)

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 1_000_000
	plan := REAPPlan{
		Height: 123,
		Inputs: []wire.OutPoint{op1, op2},
		Stats:  REAPStats{Candidates: 2},
	}

	tx, gotPlan, weight, err := BuildBudgetedBlueprint(plan, view, p, 0)
	if err != nil {
		t.Fatalf("budgeted build failed: %v", err)
	}
	if tx == nil {
		t.Fatalf("expected tx")
	}
	if len(gotPlan.Inputs) != 2 {
		t.Fatalf("expected both inputs to fit, got %d", len(gotPlan.Inputs))
	}
	if weight > p.WeightBudget {
		t.Fatalf("weight should fit soft budget: got %d budget %d", weight, p.WeightBudget)
	}
	if gotPlan.Stats.EstWeight != weight {
		t.Fatalf("stats weight mismatch: got %d want %d", gotPlan.Stats.EstWeight, weight)
	}
}

func TestBuildBudgetedBlueprintNormalizesCandidateStats(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 10_000, 3)

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 1_000_000
	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op}}

	_, gotPlan, _, err := BuildBudgetedBlueprint(plan, view, p, 0)
	if err != nil {
		t.Fatalf("budgeted build failed: %v", err)
	}
	if gotPlan.Stats.Candidates != 1 || gotPlan.Stats.Picked != 1 ||
		gotPlan.Stats.Skipped != 0 {
		t.Fatalf("unexpected normalized stats: %+v", gotPlan.Stats)
	}
}

func TestNormalizePlanTwoTierEstimate(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	dust1 := addUtxo(t, view, 719, 4)
	dust2 := addUtxo(t, view, 700, 5)
	normal := addUtxo(t, view, 720, 6)

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 256
	p.DustMaxInputs = 1024

	gotPlan, err := normalizePlan(REAPPlan{
		Inputs: []wire.OutPoint{dust1, dust2, normal},
		Stats:  REAPStats{Candidates: 5},
	}, view, p)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	wantWeight := EstimateTieredBlueprintWeight(2, 1)
	if gotPlan.Stats.EstWeight != wantWeight {
		t.Fatalf("weight mismatch got=%d want=%d", gotPlan.Stats.EstWeight, wantWeight)
	}
	if gotPlan.Stats.Picked != 3 || gotPlan.Stats.Skipped != 2 {
		t.Fatalf("unexpected stats: %+v", gotPlan.Stats)
	}
}

func TestBuildBudgetedBlueprintTrimsToActualSoftBudget(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxoWithScript(t, view, 10_000, 11, largeSpendableScript(4_000, 0x52))
	op2 := addUtxoWithScript(t, view, 10_000, 12, largeSpendableScript(4_000, 0x53))

	p := DefaultREAPParams(SortModeStrict)
	oneWeight := blueprintWeight(t, REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1}}, view, p)
	twoWeight := blueprintWeight(t, REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2}}, view, p)
	if oneWeight >= twoWeight {
		t.Fatalf("test setup expected two inputs to weigh more: one=%d two=%d",
			oneWeight, twoWeight)
	}
	p.WeightBudget = oneWeight + (twoWeight-oneWeight)/2

	plan := REAPPlan{
		Height: 123,
		Inputs: []wire.OutPoint{op1, op2},
		Stats:  REAPStats{Candidates: 2},
	}
	tx, gotPlan, weight, err := BuildBudgetedBlueprint(plan, view, p, 0)
	if err != nil {
		t.Fatalf("budgeted build failed: %v", err)
	}
	if tx == nil {
		t.Fatalf("expected trimmed tx")
	}
	if len(gotPlan.Inputs) != 1 || gotPlan.Inputs[0] != op1 {
		t.Fatalf("expected trailing input trim, got %v", gotPlan.Inputs)
	}
	if weight > p.WeightBudget {
		t.Fatalf("trimmed weight exceeds soft budget: got %d budget %d", weight, p.WeightBudget)
	}
	if gotPlan.Stats.Picked != 1 || gotPlan.Stats.Skipped != 1 {
		t.Fatalf("unexpected stats picked=%d skipped=%d",
			gotPlan.Stats.Picked, gotPlan.Stats.Skipped)
	}
}

func TestBuildBudgetedBlueprintAllowsSingleSoftOverage(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxoWithScript(t, view, 10_000, 21, largeSpendableScript(4_000, 0x52))

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 1
	plan := REAPPlan{
		Height: 123,
		Inputs: []wire.OutPoint{op},
		Stats:  REAPStats{Candidates: 1},
	}

	tx, gotPlan, weight, err := BuildBudgetedBlueprint(plan, view, p, 0)
	if err != nil {
		t.Fatalf("single-input soft overage should be allowed: %v", err)
	}
	if tx == nil || len(gotPlan.Inputs) != 1 {
		t.Fatalf("expected single-input tx, got tx=%v inputs=%d", tx, len(gotPlan.Inputs))
	}
	if weight <= p.WeightBudget {
		t.Fatalf("test setup expected soft overage: got %d budget %d", weight, p.WeightBudget)
	}
}

func TestBuildBudgetedBlueprintTrimsToHardLimit(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxoWithScript(t, view, 10_000, 31, largeSpendableScript(4_000, 0x52))
	op2 := addUtxoWithScript(t, view, 10_000, 32, largeSpendableScript(4_000, 0x53))

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 0
	oneWeight := blueprintWeight(t, REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1}}, view, p)
	twoWeight := blueprintWeight(t, REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2}}, view, p)
	if oneWeight >= twoWeight {
		t.Fatalf("test setup expected two inputs to weigh more: one=%d two=%d",
			oneWeight, twoWeight)
	}

	plan := REAPPlan{
		Height: 123,
		Inputs: []wire.OutPoint{op1, op2},
		Stats:  REAPStats{Candidates: 2},
	}
	tx, gotPlan, weight, err := BuildBudgetedBlueprint(plan, view, p, twoWeight)
	if err != nil {
		t.Fatalf("hard-limit trim should succeed: %v", err)
	}
	if tx == nil || len(gotPlan.Inputs) != 1 {
		t.Fatalf("expected one-input tx after hard trim, got tx=%v inputs=%d",
			tx, len(gotPlan.Inputs))
	}
	if weight >= twoWeight {
		t.Fatalf("hard-trimmed weight should be below hard limit: got %d limit %d",
			weight, twoWeight)
	}
}

func TestBuildBudgetedBlueprintRejectsSingleOverHardLimit(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxoWithScript(t, view, 10_000, 41, largeSpendableScript(4_000, 0x52))

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 0
	oneWeight := blueprintWeight(t, REAPPlan{Height: 123, Inputs: []wire.OutPoint{op}}, view, p)

	plan := REAPPlan{
		Height: 123,
		Inputs: []wire.OutPoint{op},
		Stats:  REAPStats{Candidates: 1},
	}
	tx, gotPlan, weight, err := BuildBudgetedBlueprint(plan, view, p, oneWeight)
	if err == nil {
		t.Fatalf("expected hard-limit error")
	}
	if tx != nil {
		t.Fatalf("hard-limit error should not return tx")
	}
	if len(gotPlan.Inputs) != 1 || weight != oneWeight {
		t.Fatalf("unexpected rejection details inputs=%d weight=%d wantWeight=%d",
			len(gotPlan.Inputs), weight, oneWeight)
	}
	if !strings.Contains(err.Error(), "single-input REAP transaction weight") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildBudgetedBlueprintMissingUTXO(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)

	_, _, _, err := BuildBudgetedBlueprint(
		REAPPlan{Inputs: []wire.OutPoint{{Index: 1}}}, view, p, 0,
	)
	if err == nil {
		t.Fatalf("expected missing utxo error")
	}
}

func TestBuildBudgetedBlueprintNilView(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	_, _, _, err := BuildBudgetedBlueprint(
		REAPPlan{Inputs: []wire.OutPoint{{Index: 1}}}, nil, p, 0,
	)
	if err != ErrNilView {
		t.Fatalf("expected ErrNilView, got %v", err)
	}
}

func TestBuildBudgetedBlueprintNegativeRefund(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 1_000, 51)

	p := DefaultREAPParams(SortModeStrict)
	p.TaxNum = 2
	p.TaxDen = 1

	_, _, _, err := BuildBudgetedBlueprint(
		REAPPlan{Inputs: []wire.OutPoint{op}}, view, p, 0,
	)
	if err == nil {
		t.Fatalf("expected negative refund error")
	}
	if !strings.Contains(err.Error(), "negative refund amount") {
		t.Fatalf("unexpected error: %v", err)
	}
}
