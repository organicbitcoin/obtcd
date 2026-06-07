// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
)

type REAPParams struct {
	Sort             SortMode
	MaxInputs        int
	DustMaxInputs    int
	WeightBudget     int64
	ScanBatch        int
	TaxNum           int64
	TaxDen           int64
	DustThresholdSat int64
	DebugEnabled     bool
}

func DefaultREAPParams(mode SortMode) REAPParams {
	return REAPParams{
		Sort:             mode,
		MaxInputs:        1000,
		WeightBudget:     400_000,
		ScanBatch:        10_000,
		TaxNum:           30,
		TaxDen:           100,
		DustThresholdSat: 720, // 6! (factorial of 6)
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

	if expiryParams := chaincfg.GetExpiryParams(net); expiryParams != nil {
		if expiryParams.ReapMaxInputs > 0 {
			p.MaxInputs = expiryParams.ReapMaxInputs
		}
		if expiryParams.ReapDustMaxInputs > 0 {
			p.DustMaxInputs = expiryParams.ReapDustMaxInputs
		}
		if expiryParams.ReapTaxNumerator > 0 {
			p.TaxNum = expiryParams.ReapTaxNumerator
		}
		if expiryParams.ReapTaxDenominator > 0 {
			p.TaxDen = expiryParams.ReapTaxDenominator
		}
		if expiryParams.ReapDustThresholdSat >= 0 {
			p.DustThresholdSat = expiryParams.ReapDustThresholdSat
		}
	}

	switch net.Net {
	case chaincfg.ObtcMainNetParams.Net:
		// Mainnet starts conservative to avoid starving normal transactions.
		p.WeightBudget = 200_000
	case chaincfg.ObtcRegTestParams.Net:
		p.ScanBatch = 2_000
		p.DebugEnabled = true
	case chaincfg.ObtcTestNetParams.Net:
		p.ScanBatch = 5_000
	}

	return p
}

func (p REAPParams) Validate() error {
	if p.MaxInputs <= 0 {
		return fmt.Errorf("MaxInputs must be > 0")
	}
	if p.DustMaxInputs < 0 {
		return fmt.Errorf("DustMaxInputs must be >= 0")
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
	if p.DustThresholdSat < 0 {
		return fmt.Errorf("DustThresholdSat must be >= 0")
	}
	return nil
}

// PrefixCandidateLimit returns how many global-prefix candidates mining should
// fetch before applying local tier and weight limits.
func (p REAPParams) PrefixCandidateLimit() int {
	if p.DustMaxInputs <= 0 {
		return p.MaxInputs
	}
	return p.MaxInputs + p.DustMaxInputs
}
