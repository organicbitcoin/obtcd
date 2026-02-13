package reap

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
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

	pTestnet := DefaultREAPParamsForNet(&chaincfg.ObtcTestNetParams, SortModeStrict)
	if pTestnet.MaxInputs != 500 || pTestnet.ScanBatch != 5_000 {
		t.Fatalf("unexpected defaults for obtc testnet: %+v", pTestnet)
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
