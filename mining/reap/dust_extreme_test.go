package reap

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func TestDustExtremeCliff778Vs779(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	if p.DustThresholdSat != 546 {
		t.Fatalf("unexpected default dust threshold: %d", p.DustThresholdSat)
	}

	// amount=778 => tax=floor(778*0.3)=233, refund=545 (<546), so refund is folded.
	tax778 := taxForValue(778, p)
	refund778 := int64(778) - tax778
	adjRefund778, adjTax778 := applyDustRule(refund778, tax778, p.DustThresholdSat)
	if adjRefund778 != 0 || adjTax778 != 778 {
		t.Fatalf("778 cliff mismatch: got refund=%d tax=%d", adjRefund778, adjTax778)
	}

	// amount=779 => tax=233, refund=546 (==threshold), so no folding.
	tax779 := taxForValue(779, p)
	refund779 := int64(779) - tax779
	adjRefund779, adjTax779 := applyDustRule(refund779, tax779, p.DustThresholdSat)
	if adjRefund779 != 546 || adjTax779 != 233 {
		t.Fatalf("779 cliff mismatch: got refund=%d tax=%d", adjRefund779, adjTax779)
	}

	view := blockchain.NewUtxoViewpoint()
	opA := addUtxo(t, view, 778, 71)
	opB := addUtxo(t, view, 779, 72)
	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{opA, opB}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if s.TaxTotal != 1011 || s.RefundTotal != 546 {
		t.Fatalf("unexpected dryrun totals: tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}
}

func TestDustExtremePerInputFoldingDiffersFromAggregate(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	view := blockchain.NewUtxoViewpoint()

	// Two inputs with same script and same value.
	// Per-input rule: each 700 => tax=210 refund=490 (<546) => fold twice.
	op1 := addUtxoWithScript(t, view, 700, 81, []byte{0x51})
	op2 := addUtxoWithScript(t, view, 700, 82, []byte{0x51})

	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{op1, op2}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if s.TaxTotal != 1400 || s.RefundTotal != 0 {
		t.Fatalf("expected full fold per input: tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}

	// Hypothetical aggregate-first (NOT implemented):
	// total=1400 => tax=420 refund=980 (>=546, would not fold).
	hypTax := taxForValue(1400, p)
	hypRefund := int64(1400) - hypTax
	hypRefund, hypTax = applyDustRule(hypRefund, hypTax, p.DustThresholdSat)
	if hypTax != 420 || hypRefund != 980 {
		t.Fatalf("aggregate baseline changed unexpectedly: tax=%d refund=%d", hypTax, hypRefund)
	}

	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{op1, op2}, TaxTotal: 1400, RefundTotal: 0}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build blueprint failed: %v", err)
	}
	if len(tx.TxOut) != 1 {
		t.Fatalf("expected marker-only tx under per-input fold, got %d outputs", len(tx.TxOut))
	}
}

func TestDustExtremeTaxNumZeroStillFoldsSubDustRefund(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	p.TaxNum = 0
	view := blockchain.NewUtxoViewpoint()

	opSmall := addUtxoWithScript(t, view, 545, 91, []byte{0x51})
	opEdge := addUtxoWithScript(t, view, 546, 92, []byte{0x51})

	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{opSmall, opEdge}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	// TaxNum=0 does not disable dust folding:
	// 545 => folded to tax, 546 => refunded.
	if s.TaxTotal != 545 || s.RefundTotal != 546 {
		t.Fatalf("unexpected zero-taxnum dust behavior: tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}

	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{opSmall, opEdge}, TaxTotal: 545, RefundTotal: 546}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build blueprint failed: %v", err)
	}
	if len(tx.TxOut) != 2 {
		t.Fatalf("expected one refund output plus marker, got %d outputs", len(tx.TxOut))
	}
	if tx.TxOut[0].Value != 546 {
		t.Fatalf("unexpected refund output value: %d", tx.TxOut[0].Value)
	}
	if tx.TxOut[1].Value != 0 {
		t.Fatalf("marker output must be zero-valued")
	}
}
