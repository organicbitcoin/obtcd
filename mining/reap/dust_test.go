package reap

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func TestApplyDustRuleDirect(t *testing.T) {
	refund, tax := applyDustRule(719, 300, 720)
	if refund != 0 || tax != 1019 {
		t.Fatalf("unexpected dust fold result refund=%d tax=%d", refund, tax)
	}

	refund, tax = applyDustRule(720, 300, 720)
	if refund != 720 || tax != 300 {
		t.Fatalf("threshold boundary mismatch refund=%d tax=%d", refund, tax)
	}

	refund, tax = applyDustRule(100, 300, 0)
	if refund != 100 || tax != 300 {
		t.Fatalf("disabled dust rule should be no-op")
	}
}

func TestBuildBlueprintDustRefundFoldedToTax(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 700, 1) // 30% tax=210, refund=490 (<720)

	p := DefaultREAPParams(SortModeStrict)
	p.DustThresholdSat = 720

	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op}}
	plan.TaxTotal = 700
	plan.RefundTotal = 0

	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(tx.TxOut) != 1 {
		t.Fatalf("expected only marker output when refund is dust, got %d", len(tx.TxOut))
	}
	if tx.TxOut[0].Value != 0 {
		t.Fatalf("marker value must be zero")
	}
}
