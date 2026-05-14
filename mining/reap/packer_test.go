// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"bytes"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildBlueprintTotalsAndMarker(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 1100, 1)
	op2 := addUtxo(t, view, 2001, 2)

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2}}
	plan.TaxTotal = taxForValue(1100, p) + taxForValue(2001, p)
	plan.RefundTotal = 3101 - plan.TaxTotal

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

func TestBuildBlueprintIsFinalizedAtTargetHeight(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 1000, 1)

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op}}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if !blockchain.IsFinalizedTransaction(btcutil.NewTx(tx), plan.Height, time.Now()) {
		t.Fatalf("expected REAP blueprint tx to be finalized at target height")
	}
}

func TestBuildBlueprintRefundOutputsGroupedByScript(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op1 := addUtxo(t, view, 1100, 1)
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

func TestMarkerScriptDirect(t *testing.T) {
	op := wire.OutPoint{Index: 7}
	s, err := markerScript(321, 1, []wire.OutPoint{op})
	if err != nil {
		t.Fatalf("markerScript failed: %v", err)
	}
	payload, ok := ExtractMarkerPayload(s)
	if !ok {
		t.Fatalf("expected valid marker payload")
	}
	if payload == "" {
		t.Fatalf("expected non-empty marker payload")
	}
}

func TestMarkerScriptEmptyAndDeterministic(t *testing.T) {
	s1, err := markerScript(0, 0, nil)
	if err != nil {
		t.Fatalf("markerScript(nil) failed: %v", err)
	}
	s2, err := markerScript(0, 0, []wire.OutPoint{})
	if err != nil {
		t.Fatalf("markerScript(empty) failed: %v", err)
	}
	if !bytes.Equal(s1, s2) {
		t.Fatalf("nil/empty inputs should produce identical marker script")
	}
}

func TestBuildBlueprintLargeAmountInvariant(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 9_000_000_000_000_000, 42)
	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 9, Inputs: []wire.OutPoint{op}}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(tx.TxOut) < 2 || tx.TxOut[len(tx.TxOut)-1].Value != 0 {
		t.Fatalf("expected marker output at tail")
	}
	if tx.TxOut[0].Value <= 0 {
		t.Fatalf("refund should remain positive for large amount")
	}
}
