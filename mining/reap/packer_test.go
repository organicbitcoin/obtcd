package reap

import (
	"bytes"
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
	plan.RefundTotal = 3001 - plan.TaxTotal

	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(tx.TxIn) != 2 {
		t.Fatalf("unexpected input count in=%d", len(tx.TxIn))
	}
	if len(tx.TxOut) != 2 {
		t.Fatalf("expected one aggregated refund output + marker, got out=%d", len(tx.TxOut))
	}
	if tx.TxOut[0].Value != plan.RefundTotal {
		t.Fatalf("refund mismatch got=%d want=%d", tx.TxOut[0].Value, plan.RefundTotal)
	}
	if tx.TxOut[1].Value != 0 {
		t.Fatalf("marker output must be zero value")
	}
}

func TestBuildBlueprintRefundOutputsGroupedByScript(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 1000, 1)
	op2 := addUtxoWithScript(t, view, 2000, 2, []byte{0x52})
	op3 := addUtxoWithScript(t, view, 3000, 3, []byte{0x52})

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2, op3}}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(tx.TxOut) != 3 {
		t.Fatalf("expected 2 refund outputs + marker, got %d", len(tx.TxOut))
	}
	if tx.TxOut[2].Value != 0 {
		t.Fatalf("last output should be marker")
	}
	if bytes.Equal(tx.TxOut[0].PkScript, tx.TxOut[1].PkScript) {
		t.Fatalf("refund outputs should be unique scripts after grouping")
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
