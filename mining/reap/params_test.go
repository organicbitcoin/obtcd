package reap

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestDefaultREAPParamsForNet(t *testing.T) {
	main := DefaultREAPParamsForNet(&chaincfg.MainNetParams, SortModeStrict)
	if main.MaxInputs != 1000 || main.ScanBatch != 10_000 {
		t.Fatalf("unexpected main defaults: %+v", main)
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
}
