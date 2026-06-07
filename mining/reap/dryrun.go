// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/blockchain"
)

type DryRunSummary struct {
	Picked      int
	TaxTotal    int64
	RefundTotal int64
	EstWeight   int64
	MarkerHash  string
}

func BuildDryRunSummary(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (DryRunSummary, error) {
	if view == nil {
		return DryRunSummary{}, ErrNilView
	}

	taxTotal := int64(0)
	refundTotal := int64(0)
	dustCount := 0
	normalCount := 0
	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			return DryRunSummary{}, fmt.Errorf("missing utxo in view: %s", op.String())
		}
		amt := entry.Amount()
		tax := taxForValue(amt, p)
		refund := amt - tax
		if refund < 0 {
			return DryRunSummary{}, fmt.Errorf("negative refund total")
		}
		refund, tax = applyDustRule(amt, refund, tax, p.DustThresholdSat)
		taxTotal += tax
		refundTotal += refund
		if isDustTierAmount(amt, p.DustThresholdSat) {
			dustCount++
		} else {
			normalCount++
		}
	}

	return DryRunSummary{
		Picked:      len(plan.Inputs),
		TaxTotal:    taxTotal,
		RefundTotal: refundTotal,
		EstWeight:   estimatePlanWeight(dustCount, normalCount, len(plan.Inputs), p),
		MarkerHash:  MarkerDigest(plan.Inputs),
	}, nil
}
