// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"context"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/wire"
)

func (g *BlkTmplGenerator) maybeBuildREAPTx(nextBlockHeight int32) (*btcutil.Tx, int64, error) {
	if g.reapIndex == nil || !chaincfg.IsOBTC(g.chainParams) {
		logOBTCDevf(g.chainParams,
			"REAP build skipped nextHeight=%d reason=index-or-network reapIndex=%t isOBTC=%t",
			nextBlockHeight, g.reapIndex != nil, chaincfg.IsOBTC(g.chainParams))
		return nil, 0, nil
	}
	expiryParams := chaincfg.GetExpiryParams(g.chainParams)
	if expiryParams == nil || nextBlockHeight < expiryParams.EnableAtHeight {
		activationHeight := int32(-1)
		if expiryParams != nil {
			activationHeight = expiryParams.EnableAtHeight
		}
		logOBTCDevf(g.chainParams,
			"REAP build skipped nextHeight=%d reason=before-activation activationHeight=%d",
			nextBlockHeight, activationHeight)
		return nil, 0, nil
	}

	p := reap.DefaultREAPParamsForNet(g.chainParams, reap.SortModeStrict)
	if err := p.Validate(); err != nil {
		logOBTCDevf(g.chainParams,
			"REAP build failed nextHeight=%d reason=invalid-params err=%v", nextBlockHeight, err)
		return nil, 0, err
	}
	logOBTCDevf(g.chainParams,
		"REAP build start nextHeight=%d maxInputs=%d weightBudget=%d scanBatch=%d",
		nextBlockHeight, p.MaxInputs, p.WeightBudget, p.ScanBatch)

	opSet, err := g.collectExpiredOutpoints(nextBlockHeight, p)
	if err != nil {
		logOBTCDevf(g.chainParams,
			"REAP build failed nextHeight=%d reason=collect-expired err=%v", nextBlockHeight, err)
		return nil, 0, err
	}
	if len(opSet) == 0 {
		logOBTCDevf(g.chainParams,
			"REAP build skipped nextHeight=%d reason=no-expired-outpoints", nextBlockHeight)
		return nil, 0, nil
	}

	dummy := wire.NewMsgTx(1)
	for _, op := range opSet {
		dummy.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	}
	view, err := g.chain.FetchUtxoView(btcutil.NewTx(dummy))
	if err != nil {
		logOBTCDevf(g.chainParams,
			"REAP build failed nextHeight=%d reason=fetch-view err=%v candidateOutpoints=%d",
			nextBlockHeight, err, len(opSet))
		return nil, 0, err
	}
	logOBTCDevf(g.chainParams,
		"REAP build fetched utxo view nextHeight=%d candidateOutpoints=%d",
		nextBlockHeight, len(opSet))

	plan, err := reap.SelectCandidates(context.Background(), nextBlockHeight, g.reapIndex, view, p)
	if err != nil {
		logOBTCDevf(g.chainParams,
			"REAP build failed nextHeight=%d reason=select err=%v", nextBlockHeight, err)
		return nil, 0, err
	}
	if len(plan.Inputs) == 0 {
		logOBTCDevf(g.chainParams,
			"REAP build skipped nextHeight=%d reason=empty-plan candidates=%d",
			nextBlockHeight, plan.Stats.Candidates)
		return nil, 0, nil
	}
	logOBTCDevf(g.chainParams,
		"REAP build plan nextHeight=%d candidates=%d picked=%d skipped=%d refund=%d tax=%d estWeight=%d",
		nextBlockHeight, plan.Stats.Candidates, plan.Stats.Picked, plan.Stats.Skipped,
		plan.RefundTotal, plan.TaxTotal, plan.Stats.EstWeight)

	tx, err := reap.BuildBlueprint(plan, view, p)
	if err != nil {
		logOBTCDevf(g.chainParams,
			"REAP build failed nextHeight=%d reason=blueprint err=%v", nextBlockHeight, err)
		return nil, 0, err
	}
	builtTx := btcutil.NewTx(tx)
	logOBTCDevf(g.chainParams,
		"REAP build done nextHeight=%d tx=%s inputs=%d outputs=%d fee=%d",
		nextBlockHeight, builtTx.Hash(), len(tx.TxIn), len(tx.TxOut), plan.TaxTotal)
	return builtTx, plan.TaxTotal, nil
}

// normalTxWeightLimit returns the weight cap used while selecting regular
// mempool transactions before attempting to append a REAP system tx.
//
// When REAP is active, we reserve up to REAP's weight budget so heavily loaded
// mempools still leave headroom for expiry processing.
func (g *BlkTmplGenerator) normalTxWeightLimit(nextBlockHeight int32, reserveForREAP bool) uint32 {
	limit := g.policy.BlockMaxWeight
	if !reserveForREAP {
		return limit
	}

	reserve := g.reservedREAPWeight(nextBlockHeight)
	if reserve > 0 && reserve < limit {
		return limit - reserve
	}
	return limit
}

func (g *BlkTmplGenerator) reservedREAPWeight(nextBlockHeight int32) uint32 {
	if g.reapIndex == nil || g.chainParams == nil || !chaincfg.IsOBTC(g.chainParams) {
		return 0
	}

	expiryParams := chaincfg.GetExpiryParams(g.chainParams)
	if expiryParams == nil || nextBlockHeight < expiryParams.EnableAtHeight {
		return 0
	}

	p := reap.DefaultREAPParamsForNet(g.chainParams, reap.SortModeStrict)
	if p.WeightBudget <= 0 {
		return 0
	}

	maxWeight := uint64(g.policy.BlockMaxWeight)
	reserve := uint64(p.WeightBudget)
	if maxWeight == 0 || reserve >= maxWeight {
		return 0
	}

	return uint32(reserve)
}

func (g *BlkTmplGenerator) collectExpiredOutpoints(nextBlockHeight int32, p reap.REAPParams) ([]wire.OutPoint, error) {
	fromKey := uint64(0)
	toKey := uint64(nextBlockHeight)
	var startAfter *wire.OutPoint
	maxCandidates := p.MaxInputs * 20
	if maxCandidates < p.MaxInputs {
		maxCandidates = p.MaxInputs
	}
	if maxCandidates > 50000 {
		maxCandidates = 50000
	}

	out := make([]wire.OutPoint, 0, maxCandidates)
	logOBTCDevf(g.chainParams,
		"REAP collect start nextHeight=%d maxCandidates=%d scanBatch=%d",
		nextBlockHeight, maxCandidates, p.ScanBatch)
	for len(out) < maxCandidates {
		rows, hasMore, err := g.reapIndex.ScanExpiringUTXOs(fromKey, toKey, p.ScanBatch, startAfter)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			logOBTCDevf(g.chainParams,
				"REAP collect page nextHeight=%d rows=0 total=%d hasMore=%t",
				nextBlockHeight, len(out), hasMore)
			break
		}
		for _, r := range rows {
			out = append(out, r.OutPoint)
			if len(out) >= maxCandidates {
				break
			}
		}
		logOBTCDevf(g.chainParams,
			"REAP collect page nextHeight=%d rows=%d total=%d hasMore=%t lastExpiry=%d",
			nextBlockHeight, len(rows), len(out), hasMore, rows[len(rows)-1].ExpiryKey)
		if !hasMore {
			break
		}
		last := rows[len(rows)-1]
		fromKey = last.ExpiryKey
		op := last.OutPoint
		startAfter = &op
	}
	logOBTCDevf(g.chainParams,
		"REAP collect done nextHeight=%d collected=%d",
		nextBlockHeight, len(out))
	return out, nil
}

// mergeUtxoEntriesIfMissing copies UTXO entries from src into dst only when
// dst doesn't already have an entry for the outpoint (or has a nil placeholder).
// This preserves any in-template spends already reflected in dst.
func mergeUtxoEntriesIfMissing(dst, src *blockchain.UtxoViewpoint) {
	if dst == nil || src == nil {
		return
	}

	dstEntries := dst.Entries()
	for outpoint, srcEntry := range src.Entries() {
		cur, exists := dstEntries[outpoint]
		if !exists || cur == nil {
			dstEntries[outpoint] = srcEntry
		}
	}
}
