package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/blockchain"
)

type DryRunSummary struct {
	Picked     int
	TaxTotal   int64
	BurnTotal  int64
	EstWeight  int64
	MarkerHash string
}

func BuildDryRunSummary(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (DryRunSummary, error) {
	if view == nil {
		return DryRunSummary{}, ErrNilView
	}

	var inTotal int64
	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			return DryRunSummary{}, fmt.Errorf("missing utxo in view: %s", op.String())
		}
		inTotal += entry.Amount()
	}

	taxTotal := int64(0)
	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		taxTotal += taxForValue(entry.Amount(), p)
	}

	burnTotal := inTotal - taxTotal
	if burnTotal < 0 {
		return DryRunSummary{}, fmt.Errorf("negative burn total")
	}

	return DryRunSummary{
		Picked:     len(plan.Inputs),
		TaxTotal:   taxTotal,
		BurnTotal:  burnTotal,
		EstWeight:  EstimateBlueprintWeight(len(plan.Inputs)),
		MarkerHash: MarkerDigest(plan.Inputs),
	}, nil
}
