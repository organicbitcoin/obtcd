package expiryindex

import (
	"testing"
)

func TestExpiryParamsHelpers(t *testing.T) {
	p := &ExpiryParams{WindowBlocks: 144, ListBatchLimit: 100, StartScanHeight: 50, EnableAtHeight: 80}

	if !p.IsIndexingEnabled(50) || p.IsIndexingEnabled(49) {
		t.Fatalf("IsIndexingEnabled boundary incorrect")
	}
	if !p.IsExpiryEnabled(80) || p.IsExpiryEnabled(79) {
		t.Fatalf("IsExpiryEnabled boundary incorrect")
	}

	if err := p.ValidateListParams(1); err != nil {
		t.Fatalf("expected valid limit: %v", err)
	}
	if err := p.ValidateListParams(0); err == nil {
		t.Fatalf("expected error for zero limit")
	}
	if err := p.ValidateListParams(101); err == nil {
		t.Fatalf("expected error for over-limit")
	}

	from, to := p.CalculateExpiryRange(1000, 500)
	if from != 1000 || to != 1500 {
		t.Fatalf("unexpected range: %d..%d", from, to)
	}

	// Overflow case.
	from2, to2 := p.CalculateExpiryRange(^uint64(0)-10, 20)
	if from2 != ^uint64(0)-10 || to2 != ^uint64(0) {
		t.Fatalf("overflow range incorrect: %d..%d", from2, to2)
	}

	if p.GetDefaultHorizon() != 144 {
		t.Fatalf("default horizon mismatch")
	}
}
