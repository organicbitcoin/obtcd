// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mining/reap"
)

func loadUTXORows(path string) ([]utxoRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	dec := json.NewDecoder(gz)
	var rows []utxoRow
	for {
		var row utxoRow
		if err := dec.Decode(&row); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func writePreviewFiles(rows []utxoRow, network string, params *chaincfg.Params,
	snapshotHeight int32, snapshotHash, detailPath, summaryPath string) (*reapPreviewSummary, error) {

	detailFile, err := os.Create(detailPath)
	if err != nil {
		return nil, err
	}
	defer detailFile.Close()

	detailRawHash := sha256.New()
	detailFileHash := sha256.New()
	cw := &countingWriter{w: detailFile, hash: detailFileHash}
	gz := gzip.NewWriter(cw)
	gz.Name = detailPath
	gz.ModTime = time.Unix(0, 0)

	summary, err := buildPreview(rows, network, params, snapshotHeight, snapshotHash, func(detail reapPreviewDetail) error {
		line, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		line = append(line, '\n')
		if _, err := detailRawHash.Write(line); err != nil {
			return err
		}
		_, err = gz.Write(line)
		return err
	})
	if closeErr := gz.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}

	summary.DetailSHA256 = hex.EncodeToString(detailRawHash.Sum(nil))
	summary.DetailFileSHA256 = hex.EncodeToString(detailFileHash.Sum(nil))

	summaryBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, err
	}
	summaryBytes = append(summaryBytes, '\n')
	if err := os.WriteFile(summaryPath, summaryBytes, 0600); err != nil {
		return nil, err
	}

	return summary, nil
}

func buildPreview(rows []utxoRow, network string, params *chaincfg.Params,
	snapshotHeight int32, snapshotHash string, emit func(reapPreviewDetail) error) (*reapPreviewSummary, error) {

	if params == nil {
		params = &chaincfg.ObtcMainNetParams
	}
	p := reap.DefaultREAPParamsForNet(params, reap.SortModeStrict)
	if p.TaxDen <= 0 {
		return nil, fmt.Errorf("invalid REAP tax denominator: %d", p.TaxDen)
	}

	candidates, err := orderedPreviewCandidates(rows)
	if err != nil {
		return nil, err
	}

	summary := &reapPreviewSummary{
		Network:        network,
		SnapshotHeight: snapshotHeight,
		SnapshotHash:   snapshotHash,
		GeneratedAt:    time.Now().UTC(),
		UTXORowCount:   len(rows),
		Params: reapPreviewParams{
			TaxNumerator:     p.TaxNum,
			TaxDenominator:   p.TaxDen,
			DustThresholdSat: p.DustThresholdSat,
			MaxInputs:        p.MaxInputs,
			DustMaxInputs:    p.DustMaxInputs,
			WeightBudget:     p.WeightBudget,
		},
	}

	if len(rows) == 0 {
		return summary, nil
	}

	next := 0
	height := candidates[0].ExpiryKey
	for first := 0; first < len(candidates); {
		height = max(height, candidates[first].ExpiryKey)
		for next < len(candidates) && candidates[next].ExpiryKey <= height {
			next++
		}
		backlog := candidates[first:next]
		detail, picked := selectPreviewInputs(height, backlog, p)
		if picked == 0 {
			return nil, fmt.Errorf("unable to select REAP inputs at height %d with backlog=%d", height, len(backlog))
		}
		first += picked
		detail.RemainingBacklog = next - first

		if err := emit(detail); err != nil {
			return nil, err
		}
		appendSummary(summary, detail)
		height++
	}

	buildBuckets(summary)
	return summary, nil
}

func selectPreviewInputs(height uint64, backlog []previewCandidate, p reap.REAPParams) (reapPreviewDetail, int) {
	detail := reapPreviewDetail{ReapHeight: height}
	dustCount := 0
	normalCount := 0

	for i, row := range backlog {
		dust := isDust(row.Amount, p.DustThresholdSat)
		if !canSelectPreviewCandidate(dust, dustCount, normalCount, p) {
			return detail, i
		}
		nextWeight := estimatePreviewNextWeight(dust, dustCount, normalCount, len(detail.SelectedInputs), p)
		if p.WeightBudget > 0 && nextWeight > p.WeightBudget && len(detail.SelectedInputs) > 0 {
			return detail, i
		}

		tax := taxForAmount(row.Amount, p)
		refund := row.Amount - tax
		if dust {
			tax = row.Amount
			refund = 0
			dustCount++
			detail.DustTaxSat += tax
			detail.DustInputs++
		} else {
			normalCount++
			detail.NormalTaxSat += tax
			detail.NormalInputs++
		}
		detail.TaxTotalSat += tax
		detail.RefundTotalSat += refund
		detail.SelectedInputs = append(detail.SelectedInputs, reapPreviewInput{
			TxID:         row.txID,
			Vout:         row.OutPoint.Index,
			AmountSat:    row.Amount,
			ExpiryHeight: row.ExpiryKey,
			TaxSat:       tax,
			RefundSat:    refund,
			Dust:         dust,
		})
		detail.SelectedInputCount = len(detail.SelectedInputs)
		detail.EstWeight = estimatePreviewPlanWeight(dustCount, normalCount, len(detail.SelectedInputs), p)
		if previewTierLimitReached(dustCount, normalCount, p) {
			return detail, i + 1
		}
	}
	return detail, len(backlog)
}

func appendSummary(summary *reapPreviewSummary, detail reapPreviewDetail) {
	summary.ReapBlockCount++
	summary.SelectedInputs += detail.SelectedInputCount
	summary.TaxTotalSat += detail.TaxTotalSat
	summary.RefundTotalSat += detail.RefundTotalSat
	summary.DustTaxSat += detail.DustTaxSat
	summary.NormalTaxSat += detail.NormalTaxSat
	if detail.RemainingBacklog > summary.MaxRemainingBacklog {
		summary.MaxRemainingBacklog = detail.RemainingBacklog
	}
	summary.ByHeight = append(summary.ByHeight, reapHeightSummary{
		ReapHeight:       detail.ReapHeight,
		SelectedInputs:   detail.SelectedInputCount,
		TaxTotalSat:      detail.TaxTotalSat,
		RefundTotalSat:   detail.RefundTotalSat,
		DustTaxSat:       detail.DustTaxSat,
		NormalTaxSat:     detail.NormalTaxSat,
		DustInputs:       detail.DustInputs,
		NormalInputs:     detail.NormalInputs,
		RemainingBacklog: detail.RemainingBacklog,
	})
}

func buildBuckets(summary *reapPreviewSummary) {
	summary.ByDay = buildBucket(summary.ByHeight, 144)
	summary.ByWeek = buildBucket(summary.ByHeight, 1008)
}

func buildBucket(heights []reapHeightSummary, width uint64) []reapBucketSummary {
	if len(heights) == 0 {
		return nil
	}
	buckets := make([]reapBucketSummary, 0)
	var cur *reapBucketSummary
	for _, h := range heights {
		start := (h.ReapHeight / width) * width
		end := start + width - 1
		if cur == nil || cur.StartHeight != start {
			buckets = append(buckets, reapBucketSummary{StartHeight: start, EndHeight: end})
			cur = &buckets[len(buckets)-1]
		}
		cur.SelectedInputs += h.SelectedInputs
		cur.TaxTotalSat += h.TaxTotalSat
		cur.RefundTotalSat += h.RefundTotalSat
		cur.DustTaxSat += h.DustTaxSat
		cur.NormalTaxSat += h.NormalTaxSat
	}
	return buckets
}

type previewCandidate struct {
	blockchain.ReapPrefixCandidate
	txID string
}

func orderedPreviewCandidates(rows []utxoRow) ([]previewCandidate, error) {
	candidates := make([]previewCandidate, len(rows))
	for i, row := range rows {
		c := &candidates[i]
		if row.TxID == "" {
			return nil, fmt.Errorf("missing txid in UTXO row %d", i+1)
		}
		if err := chainhash.Decode(&c.OutPoint.Hash, row.TxID); err != nil {
			return nil, fmt.Errorf("invalid txid in UTXO row %d: %w", i+1, err)
		}
		c.txID, c.OutPoint.Index = row.TxID, row.Vout
		c.ExpiryKey, c.Amount = row.ExpiryHeight, row.AmountSat
	}
	slices.SortFunc(candidates, func(a, b previewCandidate) int {
		return blockchain.CompareReapCandidates(a.ReapPrefixCandidate, b.ReapPrefixCandidate)
	})
	return candidates, nil
}

func compareUTXORowsExportOrder(a, b utxoRow) int {
	if a.ExpiryHeight != b.ExpiryHeight {
		if a.ExpiryHeight < b.ExpiryHeight {
			return -1
		}
		return 1
	}
	if a.TxID != b.TxID {
		if a.TxID < b.TxID {
			return -1
		}
		return 1
	}
	switch {
	case a.Vout < b.Vout:
		return -1
	case a.Vout > b.Vout:
		return 1
	default:
		return 0
	}
}

func isDust(amount, threshold int64) bool {
	return threshold > 0 && amount > 0 && amount < threshold
}

func taxForAmount(amount int64, p reap.REAPParams) int64 {
	if amount <= 0 || p.TaxNum <= 0 || p.TaxDen <= 0 {
		return 0
	}
	return (amount * p.TaxNum) / p.TaxDen
}

func canSelectPreviewCandidate(dust bool, dustCount, normalCount int, p reap.REAPParams) bool {
	if p.DustMaxInputs <= 0 {
		return dustCount+normalCount < p.MaxInputs
	}
	if dustCount >= p.DustMaxInputs || normalCount >= p.MaxInputs {
		return false
	}
	if dust {
		return dustCount < p.DustMaxInputs
	}
	return normalCount < p.MaxInputs
}

func previewTierLimitReached(dustCount, normalCount int, p reap.REAPParams) bool {
	if p.DustMaxInputs <= 0 {
		return dustCount+normalCount >= p.MaxInputs
	}
	return dustCount >= p.DustMaxInputs || normalCount >= p.MaxInputs
}

func estimatePreviewNextWeight(dust bool, dustCount, normalCount, inputCount int, p reap.REAPParams) int64 {
	if p.DustMaxInputs <= 0 {
		return reap.EstimateBlueprintWeight(inputCount + 1)
	}
	nextDust := dustCount
	nextNormal := normalCount
	if dust {
		nextDust++
	} else {
		nextNormal++
	}
	return reap.EstimateTieredBlueprintWeight(nextDust, nextNormal)
}

func estimatePreviewPlanWeight(dustCount, normalCount, inputCount int, p reap.REAPParams) int64 {
	if p.DustMaxInputs <= 0 {
		return reap.EstimateBlueprintWeight(inputCount)
	}
	return reap.EstimateTieredBlueprintWeight(dustCount, normalCount)
}
