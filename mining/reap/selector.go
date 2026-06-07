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

// SelectPrefixCandidates builds a REAP plan from candidates that are already
// ordered by the consensus global-prefix source.
func SelectPrefixCandidates(ctx context.Context, tip int32,
	candidates []blockchain.ReapPrefixCandidate, p REAPParams) (REAPPlan, error) {

	if p.MaxInputs <= 0 {
		return REAPPlan{}, fmt.Errorf("invalid MaxInputs: %d", p.MaxInputs)
	}
	if p.DustMaxInputs < 0 {
		return REAPPlan{}, fmt.Errorf("invalid DustMaxInputs: %d", p.DustMaxInputs)
	}

	plan := REAPPlan{Height: tip}
	plan.Stats.Candidates = len(candidates)
	dustCount := 0
	normalCount := 0

	for i, c := range candidates {
		if err := ctx.Err(); err != nil {
			return REAPPlan{}, err
		}
		dustTier := isDustTierAmount(c.Amount, p.DustThresholdSat)
		if !canSelectTierCandidate(dustTier, dustCount, normalCount, p) {
			plan.Stats.Skipped += len(candidates) - i
			break
		}
		nextWeight := estimateNextWeight(dustTier, dustCount, normalCount, len(plan.Inputs), p)
		if p.WeightBudget > 0 && nextWeight > p.WeightBudget &&
			len(plan.Inputs) > 0 {
			plan.Stats.Skipped += len(candidates) - i
			break
		}
		plan.Inputs = append(plan.Inputs, c.OutPoint)
		if dustTier {
			dustCount++
		} else {
			normalCount++
		}
		tax := taxForValue(c.Amount, p)
		refund := c.Amount - tax
		refund, tax = applyDustRule(c.Amount, refund, tax, p.DustThresholdSat)
		plan.TaxTotal += tax
		plan.RefundTotal += refund
		if tierLimitReached(dustCount, normalCount, p) {
			plan.Stats.Skipped += len(candidates) - i - 1
			break
		}
	}

	plan.Stats.Picked = len(plan.Inputs)
	plan.Stats.EstWeight = estimatePlanWeight(dustCount, normalCount, len(plan.Inputs), p)
	return plan, nil
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
	if p.DustMaxInputs < 0 {
		return REAPPlan{}, fmt.Errorf("invalid DustMaxInputs: %d", p.DustMaxInputs)
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
	dustCount := 0
	normalCount := 0

	for i, c := range all {
		if err := ctx.Err(); err != nil {
			return REAPPlan{}, err
		}
		dustTier := isDustTierAmount(c.amount, p.DustThresholdSat)
		if !canSelectTierCandidate(dustTier, dustCount, normalCount, p) {
			plan.Stats.Skipped += len(all) - i
			p.debugLogf("REAP select stop reason=max-inputs picked=%d skipped=%d",
				len(plan.Inputs), plan.Stats.Skipped)
			break
		}
		nextWeight := estimateNextWeight(dustTier, dustCount, normalCount, len(plan.Inputs), p)
		if p.WeightBudget > 0 && nextWeight > p.WeightBudget &&
			len(plan.Inputs) > 0 {
			plan.Stats.Skipped += len(all) - len(plan.Inputs)
			p.debugLogf("REAP select stop reason=weight-budget picked=%d nextWeight=%d budget=%d skipped=%d",
				len(plan.Inputs), nextWeight, p.WeightBudget, plan.Stats.Skipped)
			break
		}
		plan.Inputs = append(plan.Inputs, c.op)
		if dustTier {
			dustCount++
		} else {
			normalCount++
		}
		tax := taxForValue(c.amount, p)
		refund := c.amount - tax
		refund, tax = applyDustRule(c.amount, refund, tax, p.DustThresholdSat)
		plan.TaxTotal += tax
		plan.RefundTotal += refund
		if tierLimitReached(dustCount, normalCount, p) {
			plan.Stats.Skipped += len(all) - i - 1
			p.debugLogf("REAP select stop reason=tier-max-inputs picked=%d skipped=%d dust=%d normal=%d",
				len(plan.Inputs), plan.Stats.Skipped, dustCount, normalCount)
			break
		}
	}

	plan.Stats.Picked = len(plan.Inputs)
	plan.Stats.EstWeight = estimatePlanWeight(dustCount, normalCount, len(plan.Inputs), p)
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

func isDustTierAmount(amount, dustThresholdSat int64) bool {
	return dustThresholdSat > 0 && amount > 0 && amount < dustThresholdSat
}

func canSelectTierCandidate(dustTier bool, dustCount, normalCount int, p REAPParams) bool {
	if p.DustMaxInputs <= 0 {
		return dustCount+normalCount < p.MaxInputs
	}
	if dustCount >= p.DustMaxInputs || normalCount >= p.MaxInputs {
		return false
	}
	if dustTier {
		return dustCount < p.DustMaxInputs
	}
	return normalCount < p.MaxInputs
}

func tierLimitReached(dustCount, normalCount int, p REAPParams) bool {
	if p.DustMaxInputs <= 0 {
		return dustCount+normalCount >= p.MaxInputs
	}
	return dustCount >= p.DustMaxInputs || normalCount >= p.MaxInputs
}

func estimateNextWeight(dustTier bool, dustCount, normalCount, inputCount int,
	p REAPParams) int64 {

	if p.DustMaxInputs <= 0 {
		return EstimateBlueprintWeight(inputCount + 1)
	}
	nextDust := dustCount
	nextNormal := normalCount
	if dustTier {
		nextDust++
	} else {
		nextNormal++
	}
	return EstimateTieredBlueprintWeight(nextDust, nextNormal)
}

func estimatePlanWeight(dustCount, normalCount, inputCount int, p REAPParams) int64 {
	if p.DustMaxInputs <= 0 {
		return EstimateBlueprintWeight(inputCount)
	}
	return EstimateTieredBlueprintWeight(dustCount, normalCount)
}

func taxForValue(v int64, p REAPParams) int64 {
	if v <= 0 || p.TaxDen <= 0 || p.TaxNum <= 0 {
		return 0
	}
	return (v * p.TaxNum) / p.TaxDen
}
