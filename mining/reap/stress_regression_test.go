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
