// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

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
	if obtcMain.MaxInputs != 256 || obtcMain.DustMaxInputs != 1024 ||
		obtcMain.ScanBatch != 10_000 || obtcMain.WeightBudget != 400_000 {
		t.Fatalf("unexpected obtc mainnet defaults: %+v", obtcMain)
	}
	if obtcMain.DebugEnabled {
		t.Fatalf("obtc mainnet debug logs should stay disabled")
	}

	reg := DefaultREAPParamsForNet(&chaincfg.ObtcRegTestParams, SortModeStrict)
	if reg.MaxInputs != 200 || reg.DustMaxInputs != 400 || reg.ScanBatch != 2_000 {
		t.Fatalf("unexpected regtest defaults: %+v", reg)
	}
	if !reg.DebugEnabled {
		t.Fatalf("regtest debug logs should be enabled")
	}
}

func TestOBTCConsensusAndTemplateLimitsAligned(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params *chaincfg.Params
	}{
		{name: "obtc mainnet", params: &chaincfg.ObtcMainNetParams},
		{name: "obtc testnet", params: &chaincfg.ObtcTestNetParams},
		{name: "obtc regtest", params: &chaincfg.ObtcRegTestParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ep := chaincfg.GetExpiryParams(tc.params)
			if ep == nil {
				t.Fatalf("expected expiry params")
			}

			p := DefaultREAPParamsForNet(tc.params, SortModeStrict)
			if p.MaxInputs != ep.ReapMaxInputs {
				t.Fatalf("max input mismatch: template=%d consensus=%d", p.MaxInputs, ep.ReapMaxInputs)
			}
			if p.DustMaxInputs != ep.ReapDustMaxInputs {
				t.Fatalf("dust max input mismatch: template=%d consensus=%d", p.DustMaxInputs, ep.ReapDustMaxInputs)
			}
			if p.WeightBudget != ep.ReapMaxWeight {
				t.Fatalf("weight budget mismatch: template=%d consensus=%d", p.WeightBudget, ep.ReapMaxWeight)
			}
		})
	}
}

func TestOBTCREAPWeightBudgetIs400k(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params *chaincfg.Params
	}{
		{name: "obtc mainnet", params: &chaincfg.ObtcMainNetParams},
		{name: "obtc testnet", params: &chaincfg.ObtcTestNetParams},
		{name: "obtc regtest", params: &chaincfg.ObtcRegTestParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultREAPParamsForNet(tc.params, SortModeStrict)
			if p.WeightBudget != 400_000 {
				t.Fatalf("unexpected OBTC REAP weight budget: got %d want %d",
					p.WeightBudget, int64(400_000))
			}
		})
	}
}

func TestOBTCMainnetTierEstimateFitsWeightBudget(t *testing.T) {
	ep := chaincfg.GetExpiryParams(&chaincfg.ObtcMainNetParams)
	if ep == nil {
		t.Fatalf("expected expiry params for obtc mainnet")
	}
	p := DefaultREAPParamsForNet(&chaincfg.ObtcMainNetParams, SortModeStrict)
	dustInputs := ep.ReapDustMaxInputs - 1
	normalInputs := ep.ReapMaxInputs
	estWeight := EstimateTieredBlueprintWeight(dustInputs, normalInputs)
	if estWeight >= p.WeightBudget {
		t.Fatalf("near-full tier estimate should fit budget: dust=%d normal=%d estimate=%d budget=%d",
			dustInputs, normalInputs, estWeight, p.WeightBudget)
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
	p.DustMaxInputs = -1
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid DustMaxInputs")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.WeightBudget = -1
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid WeightBudget")
	}

	p = DefaultREAPParams(SortModeStrict)
	p.DustThresholdSat = -1
	if err := p.Validate(); err == nil {
		t.Fatalf("expected invalid DustThresholdSat")
	}
}
