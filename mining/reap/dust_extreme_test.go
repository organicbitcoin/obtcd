package reap

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func TestDustExtremeCliff1027Vs1028(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	if p.DustThresholdSat != 720 {
		t.Fatalf("unexpected default dust threshold: %d", p.DustThresholdSat)
	}

	// amount=1027 => tax=floor(1027*0.3)=308, refund=719 (<720), so refund is folded.
	tax1027 := taxForValue(1027, p)
	refund1027 := int64(1027) - tax1027
	adjRefund1027, adjTax1027 := applyDustRule(refund1027, tax1027, p.DustThresholdSat)
	if adjRefund1027 != 0 || adjTax1027 != 1027 {
		t.Fatalf("1027 cliff mismatch: got refund=%d tax=%d", adjRefund1027, adjTax1027)
	}

	// amount=1028 => tax=308, refund=720 (==threshold), so no folding.
	tax1028 := taxForValue(1028, p)
	refund1028 := int64(1028) - tax1028
	adjRefund1028, adjTax1028 := applyDustRule(refund1028, tax1028, p.DustThresholdSat)
	if adjRefund1028 != 720 || adjTax1028 != 308 {
		t.Fatalf("1028 cliff mismatch: got refund=%d tax=%d", adjRefund1028, adjTax1028)
	}

	view := blockchain.NewUtxoViewpoint()
	opA := addUtxo(t, view, 1027, 71)
	opB := addUtxo(t, view, 1028, 72)
	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{opA, opB}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if s.TaxTotal != 1335 || s.RefundTotal != 720 {
		t.Fatalf("unexpected dryrun totals: tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}
}

func TestDustExtremePerInputFoldingDiffersFromAggregate(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	view := blockchain.NewUtxoViewpoint()

	// Two inputs with same script and same value.
	// Per-input rule: each 700 => tax=210 refund=490 (<720) => fold twice.
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
	// total=1400 => tax=420 refund=980 (>=720, would not fold).
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

	opSmall := addUtxoWithScript(t, view, 719, 91, []byte{0x51})
	opEdge := addUtxoWithScript(t, view, 720, 92, []byte{0x51})

	s, err := BuildDryRunSummary(REAPPlan{Inputs: []wire.OutPoint{opSmall, opEdge}}, view, p)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	// TaxNum=0 does not disable dust folding:
	// 719 => folded to tax, 720 => refunded.
	if s.TaxTotal != 719 || s.RefundTotal != 720 {
		t.Fatalf("unexpected zero-taxnum dust behavior: tax=%d refund=%d", s.TaxTotal, s.RefundTotal)
	}

	plan := REAPPlan{Height: 123, Inputs: []wire.OutPoint{opSmall, opEdge}, TaxTotal: 719, RefundTotal: 720}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build blueprint failed: %v", err)
	}
	if len(tx.TxOut) != 2 {
		t.Fatalf("expected one refund output plus marker, got %d outputs", len(tx.TxOut))
	}
	if tx.TxOut[0].Value != 720 {
		t.Fatalf("unexpected refund output value: %d", tx.TxOut[0].Value)
	}
	if tx.TxOut[1].Value != 0 {
		t.Fatalf("marker output must be zero-valued")
	}
}
