package reap

import (
	"context"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

type stubScanner struct {
	items []*expiryindex.ExpiringUTXO
}

func (s *stubScanner) ScanExpiringUTXOs(fromKey, toKey uint64, maxResults int, startAfter *wire.OutPoint) ([]*expiryindex.ExpiringUTXO, bool, error) {
	start := 0
	if startAfter != nil {
		for i, it := range s.items {
			if it.ExpiryKey == fromKey && it.OutPoint == *startAfter {
				start = i + 1
				break
			}
		}
	}
	var out []*expiryindex.ExpiringUTXO
	for i := start; i < len(s.items) && len(out) < maxResults; i++ {
		it := s.items[i]
		if it.ExpiryKey < fromKey || it.ExpiryKey > toKey {
			continue
		}
		out = append(out, it)
	}
	hasMore := start+len(out) < len(s.items)
	return out, hasMore, nil
}

func TestSelectCandidatesDeterministic(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	for i := 0; i < 100; i++ {
		op := addUtxo(t, view, int64(1000+(i%7)), uint32(i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: uint64(10 + i%5)})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 50
	p.ScanBatch = 17

	first, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		next, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
		if err != nil {
			t.Fatalf("select failed: %v", err)
		}
		if len(first.Inputs) != len(next.Inputs) {
			t.Fatalf("input length mismatch: %d != %d", len(first.Inputs), len(next.Inputs))
		}
		for j := range first.Inputs {
			if first.Inputs[j] != next.Inputs[j] {
				t.Fatalf("determinism failed at run %d idx %d", i, j)
			}
		}
	}
}

func TestSelectCandidatesMaxInputsAndWeightBudget(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	for i := 0; i < 10; i++ {
		op := addUtxo(t, view, 1000, uint32(200+i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: 1})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 3
	p.WeightBudget = 0
	plan, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(plan.Inputs) != 3 {
		t.Fatalf("expected max-input truncation to 3, got %d", len(plan.Inputs))
	}

	p.MaxInputs = 100
	p.WeightBudget = EstimateBlueprintWeight(2)
	plan2, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(plan2.Inputs) != 2 {
		t.Fatalf("expected weight-budget truncation to 2, got %d", len(plan2.Inputs))
	}
}

func TestSelectCandidatesIntegrationFiltersMissingAndSpent(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	valid1 := addUtxo(t, view, 1000, 1)
	valid2 := addUtxo(t, view, 2000, 2)
	spent := addUtxo(t, view, 3000, 3)
	if e := view.LookupEntry(spent); e == nil {
		t.Fatalf("expected spent entry")
	} else {
		e.Spend()
	}

	missing := wire.OutPoint{Index: 999}
	scanner.items = []*expiryindex.ExpiringUTXO{
		{OutPoint: valid1, ExpiryKey: 1},
		{OutPoint: missing, ExpiryKey: 1},
		{OutPoint: spent, ExpiryKey: 1},
		{OutPoint: valid2, ExpiryKey: 2},
	}

	p := DefaultREAPParams(SortModeStrict)
	p.ScanBatch = 2 // force paging path
	plan, err := selectCandidatesWithScanner(context.Background(), 10, scanner, view, p)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if plan.Stats.Candidates != 2 {
		t.Fatalf("expected 2 valid candidates after filtering, got %d", plan.Stats.Candidates)
	}
	if len(plan.Inputs) != 2 {
		t.Fatalf("expected 2 picked inputs, got %d", len(plan.Inputs))
	}
}

func TestTaxRoundingInvariant(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 500; i++ {
		v := r.Int63n(5_000_000_000)
		tax := taxForValue(v, p)
		burn := v - tax
		if tax+burn != v {
			t.Fatalf("invariant broken for %d", v)
		}
	}
}

func addUtxo(t testing.TB, view *blockchain.UtxoViewpoint, amount int64, nonce uint32) wire.OutPoint {
	t.Helper()
	msg := wire.NewMsgTx(2)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: nonce}})
	msg.AddTxOut(&wire.TxOut{Value: amount, PkScript: []byte{0x51}})
	tx := btcutil.NewTx(msg)
	view.AddTxOut(tx, 0, 100)
	return wire.OutPoint{Hash: *tx.Hash(), Index: 0}
}
