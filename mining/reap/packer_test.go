package reap

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildBlueprintTotalsAndMarker(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 1000, 1)
	op2 := addUtxo(t, view, 2001, 2)

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2}}
	plan.TaxTotal = taxForValue(1000, p) + taxForValue(2001, p)
	plan.BurnTotal = 3001 - plan.TaxTotal

	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(tx.TxIn) != 2 || len(tx.TxOut) != 2 {
		t.Fatalf("unexpected io counts in=%d out=%d", len(tx.TxIn), len(tx.TxOut))
	}
	if tx.TxOut[0].Value != plan.BurnTotal {
		t.Fatalf("burn mismatch got=%d want=%d", tx.TxOut[0].Value, plan.BurnTotal)
	}
	if tx.TxOut[1].Value != 0 {
		t.Fatalf("marker output must be zero value")
	}
}

func TestBuildBlueprintMissingUTXO(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 1, Inputs: []wire.OutPoint{{}}}
	_, err := BuildBlueprint(plan, view, p)
	if err == nil {
		t.Fatalf("expected error for missing utxo")
	}
}
