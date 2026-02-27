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
		return nil, 0, nil
	}
	expiryParams := chaincfg.GetExpiryParams(g.chainParams)
	if expiryParams == nil || nextBlockHeight < expiryParams.EnableAtHeight {
		return nil, 0, nil
	}

	p := reap.DefaultREAPParamsForNet(g.chainParams, reap.SortModeStrict)
	if err := p.Validate(); err != nil {
		return nil, 0, err
	}

	opSet, err := g.collectExpiredOutpoints(nextBlockHeight, p)
	if err != nil || len(opSet) == 0 {
		return nil, 0, err
	}

	dummy := wire.NewMsgTx(1)
	for _, op := range opSet {
		dummy.AddTxIn(&wire.TxIn{PreviousOutPoint: op})
	}
	view, err := g.chain.FetchUtxoView(btcutil.NewTx(dummy))
	if err != nil {
		return nil, 0, err
	}

	plan, err := reap.SelectCandidates(context.Background(), nextBlockHeight, g.reapIndex, view, p)
	if err != nil || len(plan.Inputs) == 0 {
		return nil, 0, err
	}

	tx, err := reap.BuildBlueprint(plan, view, p)
	if err != nil {
		return nil, 0, err
	}
	return btcutil.NewTx(tx), plan.TaxTotal, nil
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
	for len(out) < maxCandidates {
		rows, hasMore, err := g.reapIndex.ScanExpiringUTXOs(fromKey, toKey, p.ScanBatch, startAfter)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			out = append(out, r.OutPoint)
			if len(out) >= maxCandidates {
				break
			}
		}
		if !hasMore {
			break
		}
		last := rows[len(rows)-1]
		fromKey = last.ExpiryKey
		op := last.OutPoint
		startAfter = &op
	}
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
