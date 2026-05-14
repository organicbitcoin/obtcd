// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/wire"
)

type candidate struct {
	op     wire.OutPoint
	expiry uint64
	amount int64
}

type expiringScanner interface {
	ScanExpiringUTXOs(fromKey, toKey uint64, maxResults int, startAfter *wire.OutPoint) ([]*expiryindex.ExpiringUTXO, bool, error)
}

func SelectCandidates(ctx context.Context, tip int32, idx *expiryindex.ExpiryIndex,
	view *blockchain.UtxoViewpoint, p REAPParams) (REAPPlan, error) {
	if idx == nil {
		return REAPPlan{}, ErrNilIndex
	}
	return selectCandidatesWithScanner(ctx, tip, idx, view, p)
}

// SelectCandidatesWithScanner is an integration-friendly variant that accepts
// any scanner implementation with the ExpiringUTXO scan contract.
func SelectCandidatesWithScanner(ctx context.Context, tip int32, scanner interface {
	ScanExpiringUTXOs(fromKey, toKey uint64, maxResults int, startAfter *wire.OutPoint) ([]*expiryindex.ExpiringUTXO, bool, error)
}, view *blockchain.UtxoViewpoint, p REAPParams) (REAPPlan, error) {
	if scanner == nil {
		return REAPPlan{}, ErrNilIndex
	}
	return selectCandidatesWithScanner(ctx, tip, scanner, view, p)
}

func selectCandidatesWithScanner(ctx context.Context, tip int32, scanner expiringScanner,
	view *blockchain.UtxoViewpoint, p REAPParams) (REAPPlan, error) {
	if view == nil {
		return REAPPlan{}, ErrNilView
	}
	if p.MaxInputs <= 0 {
		return REAPPlan{}, fmt.Errorf("invalid MaxInputs: %d", p.MaxInputs)
	}
	if p.ScanBatch <= 0 {
		p.ScanBatch = 10_000
	}
	p.debugLogf("REAP select start tip=%d maxInputs=%d weightBudget=%d scanBatch=%d sort=%d",
		tip, p.MaxInputs, p.WeightBudget, p.ScanBatch, p.Sort)

	var all []candidate
	var fromKey uint64
	if tip >= 0 {
		fromKey = 0
	}
	toKey := uint64(tip)
	var startAfter *wire.OutPoint

	for {
		if err := ctx.Err(); err != nil {
			return REAPPlan{}, err
		}
		rows, hasMore, err := scanner.ScanExpiringUTXOs(fromKey, toKey, p.ScanBatch, startAfter)
		if err != nil {
			return REAPPlan{}, err
		}
		liveRows := 0
		for _, row := range rows {
			entry := view.LookupEntry(row.OutPoint)
			if entry == nil || entry.IsSpent() {
				continue
			}
			all = append(all, candidate{op: row.OutPoint, expiry: row.ExpiryKey, amount: entry.Amount()})
			liveRows++
		}
		p.debugLogf("REAP select scan page from=%d to=%d rows=%d liveRows=%d accumulated=%d hasMore=%t",
			fromKey, toKey, len(rows), liveRows, len(all), hasMore)
		if !hasMore || len(rows) == 0 {
			break
		}
		last := rows[len(rows)-1]
		fromKey = last.ExpiryKey
		op := last.OutPoint
		startAfter = &op
	}

	sortCandidates(all, p.Sort)

	plan := REAPPlan{Height: tip}
	plan.Stats.Candidates = len(all)

	for _, c := range all {
		if err := ctx.Err(); err != nil {
			return REAPPlan{}, err
		}
		if len(plan.Inputs) >= p.MaxInputs {
			plan.Stats.Skipped += len(all) - len(plan.Inputs)
			p.debugLogf("REAP select stop reason=max-inputs picked=%d skipped=%d",
				len(plan.Inputs), plan.Stats.Skipped)
			break
		}
		nextWeight := EstimateBlueprintWeight(len(plan.Inputs) + 1)
		if p.WeightBudget > 0 && nextWeight > p.WeightBudget {
			plan.Stats.Skipped += len(all) - len(plan.Inputs)
			p.debugLogf("REAP select stop reason=weight-budget picked=%d nextWeight=%d budget=%d skipped=%d",
				len(plan.Inputs), nextWeight, p.WeightBudget, plan.Stats.Skipped)
			break
		}
		plan.Inputs = append(plan.Inputs, c.op)
		tax := taxForValue(c.amount, p)
		refund := c.amount - tax
		refund, tax = applyDustRule(c.amount, refund, tax, p.DustThresholdSat)
		plan.TaxTotal += tax
		plan.RefundTotal += refund
	}

	plan.Stats.Picked = len(plan.Inputs)
	plan.Stats.EstWeight = EstimateBlueprintWeight(len(plan.Inputs))
	p.debugLogf("REAP select done tip=%d candidates=%d picked=%d skipped=%d refund=%d tax=%d estWeight=%d",
		tip, plan.Stats.Candidates, plan.Stats.Picked, plan.Stats.Skipped,
		plan.RefundTotal, plan.TaxTotal, plan.Stats.EstWeight)
	return plan, nil
}

func sortCandidates(cs []candidate, mode SortMode) {
	slices.SortFunc(cs, func(a, b candidate) int {
		if a.expiry != b.expiry {
			if a.expiry < b.expiry {
				return -1
			}
			return 1
		}
		if mode == SortModeStrict && a.amount != b.amount {
			if a.amount < b.amount {
				return -1
			}
			return 1
		}
		hcmp := bytes.Compare(a.op.Hash[:], b.op.Hash[:])
		if hcmp != 0 {
			return hcmp
		}
		switch {
		case a.op.Index < b.op.Index:
			return -1
		case a.op.Index > b.op.Index:
			return 1
		default:
			return 0
		}
	})
}

func taxForValue(v int64, p REAPParams) int64 {
	if v <= 0 || p.TaxDen <= 0 || p.TaxNum <= 0 {
		return 0
	}
	return (v * p.TaxNum) / p.TaxDen
}
