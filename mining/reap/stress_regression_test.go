package reap

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/wire"
)

func TestSelectCandidatesHighLoadDeterministic(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	for i := 0; i < 5000; i++ {
		op := addUtxo(t, view, int64(1000+(i%97)), uint32(10_000+i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{
			OutPoint:  op,
			ExpiryKey: uint64(100 + i/100),
		})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 2000
	p.WeightBudget = 0
	p.ScanBatch = 113

	first, err := selectCandidatesWithScanner(context.Background(), 1000, scanner, view, p)
	if err != nil {
		t.Fatalf("first select failed: %v", err)
	}
	if len(first.Inputs) != p.MaxInputs {
		t.Fatalf("expected truncation to %d, got %d", p.MaxInputs, len(first.Inputs))
	}

	for i := 0; i < 3; i++ {
		next, err := selectCandidatesWithScanner(context.Background(), 1000, scanner, view, p)
		if err != nil {
			t.Fatalf("repeat select failed: %v", err)
		}
		if len(next.Inputs) != len(first.Inputs) {
			t.Fatalf("determinism length mismatch")
		}
		for j := range first.Inputs {
			if first.Inputs[j] != next.Inputs[j] {
				t.Fatalf("determinism mismatch run=%d idx=%d", i, j)
			}
		}
	}
}

func TestBuildBlueprintHighLoadInvariant(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	inputs := make([]wire.OutPoint, 0, 1200)
	for i := 0; i < 1200; i++ {
		script := []byte{0x51}
		if i%3 == 1 {
			script = []byte{0x52}
		} else if i%3 == 2 {
			script = []byte{0x53}
		}
		op := addUtxoWithScript(t, view, int64(10000+i), uint32(20_000+i), script)
		inputs = append(inputs, op)
	}

	p := DefaultREAPParams(SortModeStrict)
	plan := REAPPlan{Height: 777, Inputs: inputs}
	tx, err := BuildBlueprint(plan, view, p)
	if err != nil {
		t.Fatalf("build blueprint failed: %v", err)
	}
	if len(tx.TxIn) != len(inputs) {
		t.Fatalf("expected %d inputs, got %d", len(inputs), len(tx.TxIn))
	}
	if len(tx.TxOut) < 2 {
		t.Fatalf("expected refund outputs + marker")
	}
	if tx.TxOut[len(tx.TxOut)-1].Value != 0 {
		t.Fatalf("last output should be marker")
	}
}

func TestSelectCandidatesConcurrentRegression(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}
	for i := 0; i < 1000; i++ {
		op := addUtxo(t, view, int64(5000+i%11), uint32(30_000+i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: uint64(5 + i/100)})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 400
	p.ScanBatch = 31

	base, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
	if err != nil {
		t.Fatalf("base select failed: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plan, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
			if err != nil {
				errCh <- err
				return
			}
			if len(plan.Inputs) != len(base.Inputs) {
				errCh <- fmt.Errorf("length mismatch")
				return
			}
			for i := range base.Inputs {
				if plan.Inputs[i] != base.Inputs[i] {
					errCh <- fmt.Errorf("input mismatch at idx=%d", i)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent regression failure: %v", err)
	}
}

func TestLongRunSelectionStableAcrossScannerOrder(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	baseItems := make([]*expiryindex.ExpiringUTXO, 0, 3000)
	for i := 0; i < 3000; i++ {
		op := addUtxo(t, view, int64(2000+(i%113)), uint32(40_000+i))
		baseItems = append(baseItems, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: uint64(20 + i/30)})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 500
	p.ScanBatch = 10_000
	p.WeightBudget = 0

	forward := &stubScanner{items: append([]*expiryindex.ExpiringUTXO(nil), baseItems...)}
	reversed := &stubScanner{items: append([]*expiryindex.ExpiringUTXO(nil), baseItems...)}
	for l, r := 0, len(reversed.items)-1; l < r; l, r = l+1, r-1 {
		reversed.items[l], reversed.items[r] = reversed.items[r], reversed.items[l]
	}

	planA, err := selectCandidatesWithScanner(context.Background(), 500, forward, view, p)
	if err != nil {
		t.Fatalf("forward select failed: %v", err)
	}
	planB, err := selectCandidatesWithScanner(context.Background(), 500, reversed, view, p)
	if err != nil {
		t.Fatalf("reverse select failed: %v", err)
	}
	if len(planA.Inputs) != len(planB.Inputs) {
		t.Fatalf("length mismatch: %d != %d", len(planA.Inputs), len(planB.Inputs))
	}
	for i := range planA.Inputs {
		if planA.Inputs[i] != planB.Inputs[i] {
			t.Fatalf("order mismatch at idx=%d", i)
		}
	}
}

func TestLongRunContinuousSelectionAndBuild(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	for i := 0; i < 2500; i++ {
		op := addUtxo(t, view, int64(5000+(i%29)), uint32(50_000+i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: uint64(10 + i/25)})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 300
	p.ScanBatch = 33
	p.WeightBudget = 0

	nonEmptyRounds := 0
	for round := 0; round < 120; round++ {
		tip := int32(120 + round)
		plan, err := selectCandidatesWithScanner(context.Background(), tip, scanner, view, p)
		if err != nil {
			t.Fatalf("round %d select failed: %v", round, err)
		}
		if len(plan.Inputs) > 0 {
			nonEmptyRounds++
			tx, err := BuildBlueprint(plan, view, p)
			if err != nil {
				t.Fatalf("round %d build failed: %v", round, err)
			}
			if tx.TxOut[len(tx.TxOut)-1].Value != 0 {
				t.Fatalf("round %d marker value must be zero", round)
			}

			// Simulate confirmed REAP consumption: mark selected entries spent.
			for _, op := range plan.Inputs {
				if e := view.LookupEntry(op); e != nil && !e.IsSpent() {
					e.Spend()
				}
			}
		}

		// Simulate new-chain growth and occasional reorg-like reintroduction via fresh outputs.
		if round%6 == 0 {
			for k := 0; k < 40; k++ {
				nonce := uint32(80_000 + round*100 + k)
				op := addUtxo(t, view, int64(7000+(k%17)), nonce)
				// Keep newly appended keys moving forward with tip so they are not
				// permanently skipped by scanner pagination fromKey advancement.
				nextExpiryKey := uint64(tip + 1 + int32(k/20))
				scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: nextExpiryKey})
			}
		}
	}
	if nonEmptyRounds < 5 {
		t.Fatalf("expected substantial non-empty rounds, got %d", nonEmptyRounds)
	}
}
