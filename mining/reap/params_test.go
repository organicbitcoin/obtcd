package reap

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestDefaultREAPParamsForNet(t *testing.T) {
	btcMain := DefaultREAPParamsForNet(&chaincfg.MainNetParams, SortModeStrict)
	if btcMain.MaxInputs != 1000 || btcMain.ScanBatch != 10_000 || btcMain.WeightBudget != 400_000 {
		t.Fatalf("unexpected bitcoin mainnet defaults: %+v", btcMain)
	}

	obtcMain := DefaultREAPParamsForNet(&chaincfg.ObtcMainNetParams, SortModeStrict)
	if obtcMain.MaxInputs != 256 || obtcMain.WeightBudget != 200_000 {
		t.Fatalf("unexpected obtc mainnet defaults: %+v", obtcMain)
	}

	reg := DefaultREAPParamsForNet(&chaincfg.ObtcRegTestParams, SortModeStrict)
	if reg.MaxInputs != 200 || reg.ScanBatch != 2_000 {
		t.Fatalf("unexpected regtest defaults: %+v", reg)
	}
}

func TestREAPParamsValidate(t *testing.T) {
	p := DefaultREAPParams(SortModeStrict)
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid params: %v", err)
	}

	p.MaxInputs = 0
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid MaxInputs")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.DustThresholdSat = -1
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid DustThresholdSat")
	}
}
