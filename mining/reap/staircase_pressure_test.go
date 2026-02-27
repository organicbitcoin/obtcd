package reap

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

type candidateMeta struct {
	expiry uint64
	amount int64
}

func TestSelectCandidatesStaircasePressureCapsAndOrder(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}
	meta := make(map[wire.OutPoint]candidateMeta)

	const total = 9000
	for i := 0; i < total; i++ {
		amount := int64(600 + (i % 1800))
		op := addUtxo(t, view, amount, uint32(100_000+i))
		expiryKey := uint64(100 + i/30)
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{
			OutPoint:  op,
			ExpiryKey: expiryKey,
		})
		meta[op] = candidateMeta{expiry: expiryKey, amount: amount}

		// Simulate index lag / stale references: selected path must filter spent.
		if i%37 == 0 {
			view.LookupEntry(op).Spend()
		}
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 600
	p.WeightBudget = 220_000
	p.ScanBatch = 97

	maxByWeight := maxInputsByWeightBudget(p.WeightBudget)
	effectiveCap := p.MaxInputs
	if maxByWeight > 0 && maxByWeight < effectiveCap {
		effectiveCap = maxByWeight
	}

	tips := []int32{120, 180, 240, 320, 420}
	for _, tip := range tips {
		plan, err := selectCandidatesWithScanner(context.Background(), tip, scanner, view, p)
		if err != nil {
			t.Fatalf("tip=%d select failed: %v", tip, err)
		}

		if plan.Stats.Picked != len(plan.Inputs) {
			t.Fatalf("tip=%d stats mismatch picked=%d len=%d", tip, plan.Stats.Picked, len(plan.Inputs))
		}
		if len(plan.Inputs) > effectiveCap {
			t.Fatalf("tip=%d picked over cap: got %d cap %d", tip, len(plan.Inputs), effectiveCap)
		}
		if plan.Stats.EstWeight != EstimateBlueprintWeight(len(plan.Inputs)) {
			t.Fatalf("tip=%d unexpected estimated weight", tip)
		}

		for i, op := range plan.Inputs {
			m := meta[op]
			if m.expiry > uint64(tip) {
				t.Fatalf("tip=%d picked unexpired input[%d] expiry=%d", tip, i, m.expiry)
			}

			entry := view.LookupEntry(op)
			if entry == nil || entry.IsSpent() {
				t.Fatalf("tip=%d picked stale/spent input[%d]=%s", tip, i, op.String())
			}
			if entry.Amount() != m.amount {
				t.Fatalf("tip=%d picked input[%d] amount mismatch: view=%d meta=%d", tip, i, entry.Amount(), m.amount)
			}

			if i == 0 {
				continue
			}
			prevOp := plan.Inputs[i-1]
			prev := meta[prevOp]

			if prev.expiry > m.expiry {
				t.Fatalf("tip=%d order broken at %d: expiry %d > %d", tip, i, prev.expiry, m.expiry)
			}
			if prev.expiry == m.expiry {
				if prev.amount > m.amount {
					t.Fatalf("tip=%d strict order broken at %d: amount %d > %d", tip, i, prev.amount, m.amount)
				}
				if prev.amount == m.amount && compareOutPointDeterministic(prevOp, op) > 0 {
					t.Fatalf("tip=%d deterministic tie-break broken at %d", tip, i)
				}
			}
		}

		// Determinism check: same inputs and same order for same tip.
		again, err := selectCandidatesWithScanner(context.Background(), tip, scanner, view, p)
		if err != nil {
			t.Fatalf("tip=%d select retry failed: %v", tip, err)
		}
		if len(again.Inputs) != len(plan.Inputs) {
			t.Fatalf("tip=%d determinism length mismatch %d vs %d", tip, len(plan.Inputs), len(again.Inputs))
		}
		for i := range plan.Inputs {
			if plan.Inputs[i] != again.Inputs[i] {
				t.Fatalf("tip=%d determinism mismatch at index %d", tip, i)
			}
		}
	}
}

func TestBuildBlueprintStaircaseDustAndInvariants(t *testing.T) {
	stages := []int{64, 256, 512}
	for _, n := range stages {
		view := blockchain.NewUtxoViewpoint()
		plan := REAPPlan{Height: int32(1000 + n)}
		p := DefaultREAPParams(SortModeStrict)

		var expectedTax int64
		var inTotal int64

		for i := 0; i < n; i++ {
			var amount int64
			switch i % 4 {
			case 0:
				amount = 700 // refund 490 -> dust fold into tax
			case 1:
				amount = 779 // refund 546 -> equals threshold, should remain refund
			case 2:
				amount = 1200
			default:
				amount = 5000
			}

			var script []byte
			switch i % 4 {
			case 0:
				script = []byte{0x51}
			case 1:
				script = []byte{0x52}
			case 2:
				script = []byte{0x53}
			default:
				script = []byte{0x54}
			}
			op := addUtxoWithScript(t, view, amount, uint32(200_000+i), script)
			plan.Inputs = append(plan.Inputs, op)

			tax := taxForValue(amount, p)
			refund := amount - tax
			refund, tax = applyDustRule(refund, tax, p.DustThresholdSat)
			expectedTax += tax
			inTotal += amount
		}

		tx, err := BuildBlueprint(plan, view, p)
		if err != nil {
			t.Fatalf("n=%d build failed: %v", n, err)
		}
		if len(tx.TxIn) != n {
			t.Fatalf("n=%d input count mismatch: got %d", n, len(tx.TxIn))
		}
		if tx.LockTime != uint32(plan.Height-1) {
			t.Fatalf("n=%d unexpected locktime: got %d want %d", n, tx.LockTime, plan.Height-1)
		}
		if !blockchain.IsFinalizedTransaction(btcutil.NewTx(tx), plan.Height, time.Unix(0, 0)) {
			t.Fatalf("n=%d expected blueprint tx to be finalized at target height", n)
		}
		if len(tx.TxOut) < 1 || tx.TxOut[len(tx.TxOut)-1].Value != 0 {
			t.Fatalf("n=%d expected zero-valued marker output", n)
		}

		var refundTotal int64
		for i := 0; i < len(tx.TxOut)-1; i++ {
			v := tx.TxOut[i].Value
			if v > 0 && v < p.DustThresholdSat {
				t.Fatalf("n=%d refund output below dust threshold: %d", n, v)
			}
			refundTotal += v
		}

		if inTotal-refundTotal != expectedTax {
			t.Fatalf("n=%d invariant mismatch: in=%d refund=%d expectedTax=%d", n, inTotal, refundTotal, expectedTax)
		}
	}
}

func maxInputsByWeightBudget(weightBudget int64) int {
	if weightBudget <= 0 {
		return 0
	}
	maxInputs := 0
	for i := 1; ; i++ {
		if EstimateBlueprintWeight(i) > weightBudget {
			return maxInputs
		}
		maxInputs = i
	}
}

func compareOutPointDeterministic(a, b wire.OutPoint) int {
	hcmp := bytes.Compare(a.Hash[:], b.Hash[:])
	if hcmp != 0 {
		return hcmp
	}
	switch {
	case a.Index < b.Index:
		return -1
	case a.Index > b.Index:
		return 1
	default:
		return 0
	}
}
