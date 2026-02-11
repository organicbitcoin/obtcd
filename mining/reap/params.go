package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

type REAPParams struct {
	Sort         SortMode
	MaxInputs    int
	WeightBudget int64
	ScanBatch    int
	BurnPolicy   BurnPolicy
	TaxNum       int64
	TaxDen       int64
}

func DefaultREAPParams(mode SortMode) REAPParams {
	return REAPParams{
		Sort:         mode,
		MaxInputs:    1000,
		WeightBudget: 400_000,
		ScanBatch:    10_000,
		BurnPolicy:   BurnPolicyOpReturn,
		TaxNum:       30,
		TaxDen:       100,
	}
}

// DefaultREAPParamsForNet returns network-aware defaults.
//
// Current week3.1 kickoff keeps consensus-affecting values unchanged from
// DefaultREAPParams and only adjusts operational knobs per network.
func DefaultREAPParamsForNet(net *chaincfg.Params, mode SortMode) REAPParams {
	p := DefaultREAPParams(mode)
	if net == nil {
		return p
	}

	switch net.Net {
	case chaincfg.ObtcRegTestParams.Net:
		p.ScanBatch = 2_000
		p.MaxInputs = 200
	case chaincfg.ObtcTestNetParams.Net:
		p.ScanBatch = 5_000
		p.MaxInputs = 500
	}

	return p
}

func (p REAPParams) Validate() error {
	if p.MaxInputs <= 0 {
		return fmt.Errorf("MaxInputs must be > 0")
	}
	if p.ScanBatch <= 0 {
		return fmt.Errorf("ScanBatch must be > 0")
	}
	if p.TaxDen <= 0 {
		return fmt.Errorf("TaxDen must be > 0")
	}
	if p.TaxNum < 0 {
		return fmt.Errorf("TaxNum must be >= 0")
	}
	return nil
}
