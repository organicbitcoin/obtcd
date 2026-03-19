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
	if btcMain.DebugEnabled {
		t.Fatalf("bitcoin mainnet debug logs should stay disabled")
	}

	obtcMain := DefaultREAPParamsForNet(&chaincfg.ObtcMainNetParams, SortModeStrict)
	if obtcMain.MaxInputs != 256 || obtcMain.ScanBatch != 10_000 || obtcMain.WeightBudget != 200_000 {
		t.Fatalf("unexpected obtc mainnet defaults: %+v", obtcMain)
	}
	if obtcMain.DebugEnabled {
		t.Fatalf("obtc mainnet debug logs should stay disabled")
	}

	reg := DefaultREAPParamsForNet(&chaincfg.ObtcRegTestParams, SortModeStrict)
	if reg.MaxInputs != 200 || reg.ScanBatch != 2_000 {
		t.Fatalf("unexpected regtest defaults: %+v", reg)
	}
	if !reg.DebugEnabled {
		t.Fatalf("regtest debug logs should be enabled")
	}
}

func TestOBTCMainnetConsensusAndTemplateMaxInputsAligned(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if ep == nil {
		t.Fatalf("expected expiry params for obtc mainnet")
	}

	p := DefaultREAPParamsForNet(&chaincfg.ObtcMainNetParams, SortModeStrict)
	if p.MaxInputs != ep.ReapMaxInputs {
		t.Fatalf("obtc mainnet max input mismatch: template=%d consensus=%d", p.MaxInputs, ep.ReapMaxInputs)
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
