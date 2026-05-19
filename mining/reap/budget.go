// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
)

// BuildBudgetedBlueprint builds a REAP blueprint and trims trailing inputs
// until the actual transaction weight fits the configured soft budget.  A
// single input is allowed to exceed the soft budget so oversized refund scripts
// do not prevent forward progress.  When maxTxWeight is positive, it is treated
// as a hard exclusive upper bound because template assembly requires the final
// block weight to remain strictly below the block max.
func BuildBudgetedBlueprint(plan REAPPlan, view *blockchain.UtxoViewpoint,
	p REAPParams, maxTxWeight int64) (*btcutil.Tx, REAPPlan, int64, error) {

	if len(plan.Inputs) == 0 {
		return nil, plan, 0, nil
	}

	for {
		normalized, err := normalizePlan(plan, view, p)
		if err != nil {
			return nil, plan, 0, err
		}

		msgTx, err := BuildBlueprint(normalized, view, p)
		if err != nil {
			return nil, plan, 0, err
		}

		tx := btcutil.NewTx(msgTx)
		txWeight := blockchain.GetTransactionWeight(tx)
		hardOver := maxTxWeight > 0 && txWeight >= maxTxWeight
		softOver := p.WeightBudget > 0 && txWeight > p.WeightBudget

		if hardOver {
			if len(plan.Inputs) == 1 {
				return nil, normalized, txWeight, fmt.Errorf(
					"single-input REAP transaction weight %d exceeds hard limit %d",
					txWeight, maxTxWeight)
			}
			plan.Inputs = plan.Inputs[:len(plan.Inputs)-1]
			continue
		}

		if softOver && len(plan.Inputs) > 1 {
			plan.Inputs = plan.Inputs[:len(plan.Inputs)-1]
			continue
		}

		normalized.Stats.EstWeight = txWeight
		return tx, normalized, txWeight, nil
	}
}

func normalizePlan(plan REAPPlan, view *blockchain.UtxoViewpoint,
	p REAPParams) (REAPPlan, error) {

	if view == nil {
		return REAPPlan{}, ErrNilView
	}

	normalized := plan
	normalized.TaxTotal = 0
	normalized.RefundTotal = 0

	for _, op := range normalized.Inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			return REAPPlan{}, fmt.Errorf("missing utxo in view: %s", op.String())
		}

		amt := entry.Amount()
		tax := taxForValue(amt, p)
		refund := amt - tax
		if refund < 0 {
			return REAPPlan{}, fmt.Errorf("negative refund amount")
		}
		refund, tax = applyDustRule(amt, refund, tax, p.DustThresholdSat)
		normalized.TaxTotal += tax
		normalized.RefundTotal += refund
	}

	normalized.Stats.Picked = len(normalized.Inputs)
	if normalized.Stats.Candidates < normalized.Stats.Picked {
		normalized.Stats.Candidates = normalized.Stats.Picked
	}
	normalized.Stats.Skipped = normalized.Stats.Candidates - normalized.Stats.Picked
	normalized.Stats.EstWeight = EstimateBlueprintWeight(len(normalized.Inputs))

	return normalized, nil
}
