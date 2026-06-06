// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type stubScanner struct {
	items []*expiryindex.ExpiringUTXO
}

func makeSelectorOutPoint(index uint32) wire.OutPoint {
	var hash chainhash.Hash
	hash[0] = byte(index)
	return wire.OutPoint{Hash: hash, Index: index}
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

func TestSelectPrefixCandidatesTruncatesTailOnly(t *testing.T) {
	candidates := make([]blockchain.ReapPrefixCandidate, 0, 5)
	for i := 0; i < 5; i++ {
		candidates = append(candidates, blockchain.ReapPrefixCandidate{
			OutPoint: makeSelectorOutPoint(uint32(i)),
			Amount:   int64(1000 + i),
		})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = 3
	p.WeightBudget = 0

	plan, err := SelectPrefixCandidates(context.Background(), 123, candidates, p)
	if err != nil {
		t.Fatalf("select prefix: %v", err)
	}
	if len(plan.Inputs) != 3 {
		t.Fatalf("input count: got %d want 3", len(plan.Inputs))
	}
	for i := range plan.Inputs {
		if plan.Inputs[i] != candidates[i].OutPoint {
			t.Fatalf("input %d: got %s want %s", i, plan.Inputs[i], candidates[i].OutPoint)
		}
	}
	if plan.Stats.Skipped != 2 {
		t.Fatalf("skipped: got %d want 2", plan.Stats.Skipped)
	}

	p.MaxInputs = 5
	p.WeightBudget = EstimateBlueprintWeight(1)
	plan, err = SelectPrefixCandidates(context.Background(), 123, candidates, p)
	if err != nil {
		t.Fatalf("select prefix with budget: %v", err)
	}
	if len(plan.Inputs) != 1 || plan.Inputs[0] != candidates[0].OutPoint {
		t.Fatalf("budget should keep only first prefix candidate, got %+v", plan.Inputs)
	}
	if plan.Stats.Skipped != len(candidates)-1 {
		t.Fatalf("budget skipped: got %d want %d", plan.Stats.Skipped, len(candidates)-1)
	}
}

func TestSelectCandidatesAllowsSingleInputOverWeightBudget(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	for i := 0; i < 3; i++ {
		op := addUtxo(t, view, 1000, uint32(300+i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{
			OutPoint:  op,
			ExpiryKey: 1,
		})
	}

	p := DefaultREAPParams(SortModeStrict)
	p.WeightBudget = 1
	plan, err := selectCandidatesWithScanner(context.Background(), 100, scanner, view, p)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(plan.Inputs) != 1 {
		t.Fatalf("expected single-input escape over tiny budget, got %d", len(plan.Inputs))
	}
	if plan.Stats.Picked != 1 || plan.Stats.Skipped != 2 {
		t.Fatalf("unexpected stats picked=%d skipped=%d",
			plan.Stats.Picked, plan.Stats.Skipped)
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

func TestSelectCandidatesWithScannerWrapper(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 1000, 300)
	scanner := &stubScanner{items: []*expiryindex.ExpiringUTXO{{OutPoint: op, ExpiryKey: 1}}}
	p := DefaultREAPParams(SortModeStrict)

	plan, err := SelectCandidatesWithScanner(context.Background(), 10, scanner, view, p)
	if err != nil {
		t.Fatalf("wrapper select failed: %v", err)
	}
	if len(plan.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(plan.Inputs))
	}
}

func TestSelectCandidatesWithScannerNilScanner(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)

	_, err := SelectCandidatesWithScanner(context.Background(), 10, nil, view, p)
	if err != ErrNilIndex {
		t.Fatalf("expected ErrNilIndex, got %v", err)
	}
}

func TestSortCandidatesByExpiryThenOutpoint(t *testing.T) {
	h := func(b byte) chainhash.Hash { return chainhash.Hash{b} }
	cs := []candidate{
		{op: wire.OutPoint{Hash: h(0x02), Index: 3}, expiry: 2, amount: 100},
		{op: wire.OutPoint{Hash: h(0x01), Index: 9}, expiry: 1, amount: 999},
		{op: wire.OutPoint{Hash: h(0x01), Index: 1}, expiry: 1, amount: 111},
		{op: wire.OutPoint{Hash: h(0x01), Index: 0}, expiry: 1, amount: 111},
	}

	sortCandidates(cs, SortModeSimple)

	want := []wire.OutPoint{
		{Hash: h(0x01), Index: 0},
		{Hash: h(0x01), Index: 1},
		{Hash: h(0x01), Index: 9},
		{Hash: h(0x02), Index: 3},
	}
	for i := range want {
		if cs[i].op != want[i] {
			t.Fatalf("order mismatch at %d: got %v want %v", i, cs[i].op, want[i])
		}
	}
}

func TestSortCandidatesStrictUsesAmountTieBreaker(t *testing.T) {
	h := func(b byte) chainhash.Hash { return chainhash.Hash{b} }
	base := []candidate{
		{op: wire.OutPoint{Hash: h(0x01), Index: 0}, expiry: 7, amount: 200},
		{op: wire.OutPoint{Hash: h(0x02), Index: 0}, expiry: 7, amount: 100},
	}

	det := append([]candidate(nil), base...)
	strict := append([]candidate(nil), base...)
	sortCandidates(det, SortModeSimple)
	sortCandidates(strict, SortModeStrict)

	if det[0].amount != 200 {
		t.Fatalf("deterministic mode should not prioritize amount; got first amount %d", det[0].amount)
	}
	if strict[0].amount != 100 {
		t.Fatalf("strict mode should prioritize smaller amount; got first amount %d", strict[0].amount)
	}
}

func TestTaxRoundingInvariant(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 500; i++ {
		v := r.Int63n(5_000_000_000)
		tax := taxForValue(v, p)
		refund := v - tax
		if tax+refund != v {
			t.Fatalf("invariant broken for %d", v)
		}
	}
}

func TestTaxForValueLargeValue(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	v := int64(math.MaxInt64 / 64)
	tax := taxForValue(v, p)
	if tax <= 0 || tax >= v {
		t.Fatalf("unexpected tax for large value: v=%d tax=%d", v, tax)
	}
}

func addUtxo(t testing.TB, view *blockchain.UtxoViewpoint, amount int64, nonce uint32) wire.OutPoint {
	return addUtxoWithScript(t, view, amount, nonce, []byte{0x51})
}

func addUtxoWithScript(t testing.TB, view *blockchain.UtxoViewpoint, amount int64, nonce uint32, pkScript []byte) wire.OutPoint {
	t.Helper()
	msg := wire.NewMsgTx(2)
	msg.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: nonce}})
	msg.AddTxOut(&wire.TxOut{Value: amount, PkScript: pkScript})
	tx := btcutil.NewTx(msg)
	view.AddTxOut(tx, 0, 100)
	return wire.OutPoint{Hash: *tx.Hash(), Index: 0}
}

func addManualUtxo(view *blockchain.UtxoViewpoint, op wire.OutPoint, amount int64) {
	view.Entries()[op] = blockchain.NewUtxoEntry(&wire.TxOut{Value: amount, PkScript: []byte{0x51}}, 100, false)
}

func TestSelectCandidatesSortModesDivergeOnSameExpiry(t *testing.T) {
	mkOutPoint := func(hashPrefix byte, index uint32) wire.OutPoint {
		var h chainhash.Hash
		h[0] = hashPrefix
		return wire.OutPoint{Hash: h, Index: index}
	}

	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}

	// In simple mode, deterministic outpoint order prefers opLowHash first.
	// In strict mode, amount tie-break should prefer opHighHashLowerAmount first.
	opLowHashHigherAmount := mkOutPoint(0x01, 0)
	opHighHashLowerAmount := mkOutPoint(0x02, 0)
	addManualUtxo(view, opLowHashHigherAmount, 5_000)
	addManualUtxo(view, opHighHashLowerAmount, 1_000)
	scanner.items = []*expiryindex.ExpiringUTXO{
		{OutPoint: opHighHashLowerAmount, ExpiryKey: 10},
		{OutPoint: opLowHashHigherAmount, ExpiryKey: 10},
	}

	p := DefaultREAPParams(SortModeSimple)
	p.ScanBatch = 1
	p.MaxInputs = 10
	p.WeightBudget = 0

	simplePlan, err := selectCandidatesWithScanner(context.Background(), 20, scanner, view, p)
	if err != nil {
		t.Fatalf("simple mode selection failed: %v", err)
	}
	if len(simplePlan.Inputs) != 2 {
		t.Fatalf("simple mode expected 2 inputs, got %d", len(simplePlan.Inputs))
	}
	if simplePlan.Inputs[0] != opLowHashHigherAmount {
		t.Fatalf("simple mode should prioritize deterministic outpoint order")
	}

	p.Sort = SortModeStrict
	strictPlan, err := selectCandidatesWithScanner(context.Background(), 20, scanner, view, p)
	if err != nil {
		t.Fatalf("strict mode selection failed: %v", err)
	}
	if len(strictPlan.Inputs) != 2 {
		t.Fatalf("strict mode expected 2 inputs, got %d", len(strictPlan.Inputs))
	}
	if strictPlan.Inputs[0] != opHighHashLowerAmount {
		t.Fatalf("strict mode should prioritize lower amount when expiry ties")
	}
}

func TestSelectCandidatesTipCutoffAndStats(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}
	expiryByOutpoint := make(map[wire.OutPoint]uint64)

	for i := 1; i <= 6; i++ {
		op := addUtxo(t, view, 1_000, uint32(600+i))
		expiry := uint64(i)
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: expiry})
		expiryByOutpoint[op] = expiry
	}

	p := DefaultREAPParams(SortModeStrict)
	p.ScanBatch = 2
	p.MaxInputs = 10
	p.WeightBudget = 0

	tip := int32(3)
	plan, err := selectCandidatesWithScanner(context.Background(), tip, scanner, view, p)
	if err != nil {
		t.Fatalf("selection failed: %v", err)
	}
	if plan.Stats.Candidates != 3 {
		t.Fatalf("expected 3 candidates under tip cutoff, got %d", plan.Stats.Candidates)
	}
	if plan.Stats.Picked != 3 || len(plan.Inputs) != 3 {
		t.Fatalf("expected 3 picked inputs, got stats=%d len=%d", plan.Stats.Picked, len(plan.Inputs))
	}
	if plan.Stats.Skipped != 0 {
		t.Fatalf("expected no skipped candidates, got %d", plan.Stats.Skipped)
	}
	for i, op := range plan.Inputs {
		if expiryByOutpoint[op] > uint64(tip) {
			t.Fatalf("picked unexpired outpoint at %d with expiry=%d", i, expiryByOutpoint[op])
		}
	}
}

func TestSelectCandidatesSkippedStatsForLimitScenarios(t *testing.T) {
	setup := func(tb testing.TB, n int) (*blockchain.UtxoViewpoint, *stubScanner) {
		tb.Helper()
		view := blockchain.NewUtxoViewpoint()
		scanner := &stubScanner{}
		for i := 0; i < n; i++ {
			op := addUtxo(tb, view, 1_000, uint32(1_000+i))
			scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: 1})
		}
		return view, scanner
	}

	tests := []struct {
		name       string
		maxInputs  int
		budget     int64
		wantPicked int
	}{
		{name: "max_inputs", maxInputs: 4, budget: 0, wantPicked: 4},
		{name: "weight_budget_strictly_less_than_3_inputs", maxInputs: 10, budget: EstimateBlueprintWeight(3) - 1, wantPicked: 2},
		{name: "weight_budget_exactly_3_inputs", maxInputs: 10, budget: EstimateBlueprintWeight(3), wantPicked: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			view, scanner := setup(t, 10)
			p := DefaultREAPParams(SortModeStrict)
			p.ScanBatch = 3
			p.MaxInputs = tc.maxInputs
			p.WeightBudget = tc.budget

			plan, err := selectCandidatesWithScanner(context.Background(), 10, scanner, view, p)
			if err != nil {
				t.Fatalf("selection failed: %v", err)
			}

			if plan.Stats.Candidates != 10 {
				t.Fatalf("expected 10 candidates, got %d", plan.Stats.Candidates)
			}
			if got := len(plan.Inputs); got != tc.wantPicked {
				t.Fatalf("picked count mismatch: got %d want %d", got, tc.wantPicked)
			}
			if plan.Stats.Picked != tc.wantPicked {
				t.Fatalf("stats picked mismatch: got %d want %d", plan.Stats.Picked, tc.wantPicked)
			}
			wantSkipped := 10 - tc.wantPicked
			if plan.Stats.Skipped != wantSkipped {
				t.Fatalf("skipped count mismatch: got %d want %d", plan.Stats.Skipped, wantSkipped)
			}
		})
	}
}

func TestSelectCandidatesRespectsCanceledContext(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	op := addUtxo(t, view, 1_000, 777)
	scanner := &stubScanner{items: []*expiryindex.ExpiringUTXO{{OutPoint: op, ExpiryKey: 1}}}
	p := DefaultREAPParams(SortModeStrict)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := selectCandidatesWithScanner(ctx, 10, scanner, view, p)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
