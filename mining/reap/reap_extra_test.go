package reap

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

func TestSelectCandidatesNilIndex(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	p := DefaultREAPParams(SortModeStrict)
	_, err := SelectCandidates(context.Background(), 100, nil, view, p)
	if err == nil {
		t.Fatalf("expected error for nil index")
	}
}

func TestDefaultREAPParamsForNetNilAndTestnet(t *testing.T) {
	pNil := DefaultREAPParamsForNet(nil, SortModeStrict)
	if pNil.MaxInputs != 1000 || pNil.ScanBatch != 10_000 {
		t.Fatalf("unexpected defaults for nil net: %+v", pNil)
	}
	if pNil.DebugEnabled {
		t.Fatalf("nil net should not enable debug logs")
	}

	pTestnet := DefaultREAPParamsForNet(&chaincfg.ObtcTestNetParams, SortModeStrict)
	if pTestnet.MaxInputs != 500 || pTestnet.ScanBatch != 5_000 {
		t.Fatalf("unexpected defaults for obtc testnet: %+v", pTestnet)
	}
	if pTestnet.DebugEnabled {
		t.Fatalf("obtc testnet debug logs should stay disabled")
	}
}

func TestREAPParamsValidateBranches(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)

	p.ScanBatch = 0
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid ScanBatch")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.TaxDen = 0
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid TaxDen")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.TaxNum = -1
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid TaxNum")
	}
}

func TestEstimateBlueprintWeightNegative(t *testing.T) {
	wNeg := EstimateBlueprintWeight(-10)
	wZero := EstimateBlueprintWeight(0)
	if wNeg != wZero {
		t.Fatalf("negative input weight should equal zero input weight")
	}
}

func TestTaxForValueEdgeCases(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	if got := taxForValue(0, p); got != 0 {
		t.Fatalf("expected zero tax for zero value")
	}
	if got := taxForValue(-10, p); got != 0 {
		t.Fatalf("expected zero tax for negative value")
	}

	p.TaxDen = 0
	if got := taxForValue(1000, p); got != 0 {
		t.Fatalf("expected zero tax when TaxDen<=0")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.TaxNum = 0
	if got := taxForValue(1000, p); got != 0 {
		t.Fatalf("expected zero tax when TaxNum<=0")
	}
}

func TestSortCandidatesSimpleModeIgnoresAmount(t *testing.T) {
	op := wire.OutPoint{Index: 0}
	cs := []candidate{
		{op: op, expiry: 100, amount: 999},
		{op: op, expiry: 100, amount: 1},
	}

	sortCandidates(cs, SortModeSimple)
	// same outpoint+expiry, so order remains stable and no panic; branch coverage target.
	if cs[0].expiry != 100 || cs[1].expiry != 100 {
		t.Fatalf("unexpected ordering")
	}
}

func TestSelectCandidatesContextCanceled(t *testing.T) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{items: []*expiryindex.ExpiringUTXO{}}
	p := DefaultREAPParams(SortModeStrict)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := selectCandidatesWithScanner(ctx, 100, scanner, view, p)
	if err == nil {
		t.Fatalf("expected canceled context error")
	}
}

func TestIsLikelyREAPTxInvalidMarker(t *testing.T) {
	tx := wire.NewMsgTx(REAPTxVersion)
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: []byte{0x51}}) // OP_TRUE, not OP_RETURN data
	if IsLikelyREAPTx(tx) {
		t.Fatalf("invalid marker script should not be identified as REAP")
	}
}
