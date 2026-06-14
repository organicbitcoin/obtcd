// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mining/reap"
)

const aggregateShardRecordSize = 16

type aggregatePreviewOptions struct {
	WorkDir   string
	ShardSpan uint64
}

type aggregateShardInfo struct {
	Path string
	Rows int64
}

type aggregateHeightStats struct {
	Count     int64
	AmountSat int64
}

type aggregateGroup struct {
	ExpiryHeight uint64
	AmountSat    int64
	Count        int64
}

func snapshotFromFile(path string) (int32, string, error) {
	var (
		seen   bool
		height int32
		hash   string
		index  int64
	)
	err := forEachUTXORow(path, func(row utxoRow) error {
		if !seen {
			seen = true
			height = row.SnapshotHeight
			hash = row.SnapshotHash
			return nil
		}
		index++
		if row.SnapshotHeight != height || row.SnapshotHash != hash {
			return fmt.Errorf("input contains mixed snapshots: row 0 is %d/%s, row %d is %d/%s",
				height, hash, index, row.SnapshotHeight, row.SnapshotHash)
		}
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	if !seen {
		return 0, "empty", nil
	}
	return height, hash, nil
}

func forEachUTXORow(path string, fn func(utxoRow) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	dec := json.NewDecoder(gz)
	for {
		var row utxoRow
		if err := dec.Decode(&row); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := fn(row); err != nil {
			return err
		}
	}
}

func writeAggregatePreviewFiles(inputPath, network string, params *chaincfg.Params,
	snapshotHeight int32, snapshotHash, blocksPath, summaryPath string,
	opts aggregatePreviewOptions) (*reapAggregateSummary, error) {

	if opts.ShardSpan == 0 {
		opts.ShardSpan = 4096
	}
	if opts.WorkDir == "" {
		opts.WorkDir = "."
	}

	workDir, err := os.MkdirTemp(opts.WorkDir, "obtc-reap-aggregate-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	shards, rowCount, err := shardAggregateRows(inputPath, workDir, opts.ShardSpan)
	if err != nil {
		return nil, err
	}

	summary, err := replayAggregateShards(shards, network, params, snapshotHeight,
		snapshotHash, blocksPath, opts.ShardSpan, rowCount)
	if err != nil {
		return nil, err
	}

	body, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if err := os.WriteFile(summaryPath, body, 0600); err != nil {
		return nil, err
	}

	return summary, nil
}

func shardAggregateRows(inputPath, workDir string, shardSpan uint64) (map[uint64]aggregateShardInfo, int64, error) {
	writers := newAggregateShardWriters(workDir)
	defer writers.closeAll()

	shards := make(map[uint64]aggregateShardInfo)
	var rowCount int64
	err := forEachUTXORow(inputPath, func(row utxoRow) error {
		shardID := row.ExpiryHeight / shardSpan
		w, err := writers.writer(shardID)
		if err != nil {
			return err
		}
		var record [aggregateShardRecordSize]byte
		binary.BigEndian.PutUint64(record[0:8], row.ExpiryHeight)
		binary.BigEndian.PutUint64(record[8:16], uint64(row.AmountSat))
		if _, err := w.Write(record[:]); err != nil {
			return err
		}
		info := shards[shardID]
		info.Path = writers.path(shardID)
		info.Rows++
		shards[shardID] = info
		rowCount++
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if err := writers.closeAll(); err != nil {
		return nil, 0, err
	}
	return shards, rowCount, nil
}

func replayAggregateShards(shards map[uint64]aggregateShardInfo, network string,
	params *chaincfg.Params, snapshotHeight int32, snapshotHash, blocksPath string,
	shardSpan uint64, rowCount int64) (*reapAggregateSummary, error) {

	if params == nil {
		params = &chaincfg.ObtcMainNetParams
	}
	p := reap.DefaultREAPParamsForNet(params, reap.SortModeStrict)
	if p.TaxDen <= 0 {
		return nil, fmt.Errorf("invalid REAP tax denominator: %d", p.TaxDen)
	}

	writer, err := newJSONLGzipWriter(blocksPath)
	if err != nil {
		return nil, err
	}

	summary := &reapAggregateSummary{
		Network:        network,
		SnapshotHeight: snapshotHeight,
		SnapshotHash:   snapshotHash,
		GeneratedAt:    time.Now().UTC(),
		UTXORowCount:   rowCount,
		BlocksFile:     blocksPath,
		ShardSpan:      shardSpan,
		Params: reapPreviewParams{
			TaxNumerator:     p.TaxNum,
			TaxDenominator:   p.TaxDen,
			DustThresholdSat: p.DustThresholdSat,
			MaxInputs:        p.MaxInputs,
			DustMaxInputs:    p.DustMaxInputs,
			WeightBudget:     p.WeightBudget,
		},
	}

	shardIDs := make([]uint64, 0, len(shards))
	for id := range shards {
		shardIDs = append(shardIDs, id)
	}
	sort.Slice(shardIDs, func(i, j int) bool { return shardIDs[i] < shardIDs[j] })

	var (
		backlog       []aggregateGroup
		backlogCount  int64
		currentHeight uint64
		haveHeight    bool
	)

	emit := func(block reapAggregateBlock) error {
		if block.SelectedInputs == 0 && block.ExpiredInputs == 0 && block.RemainingBacklog == 0 {
			return nil
		}
		if summary.ReapBlockCount == 0 {
			summary.FirstReapHeight = block.ReapHeight
		}
		summary.LastReapHeight = block.ReapHeight
		summary.ReapBlockCount++
		summary.ExpiredInputs += block.ExpiredInputs
		summary.ExpiredAmountSat += block.ExpiredAmountSat
		summary.SelectedInputs += block.SelectedInputs
		summary.TaxTotalSat += block.TaxTotalSat
		summary.RefundTotalSat += block.RefundTotalSat
		summary.DustTaxSat += block.DustTaxSat
		summary.NormalTaxSat += block.NormalTaxSat
		if block.RemainingBacklog > summary.MaxRemainingBacklog {
			summary.MaxRemainingBacklog = block.RemainingBacklog
		}
		return writer.WriteJSON(block)
	}

	processHeight := func(height uint64, expired aggregateHeightStats) error {
		block := selectAggregateBlock(height, expired, &backlog, &backlogCount, p)
		return emit(block)
	}

	for _, shardID := range shardIDs {
		groupsByHeight, statsByHeight, err := loadAggregateShard(shards[shardID].Path)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		heights := make([]uint64, 0, len(groupsByHeight))
		for height := range groupsByHeight {
			heights = append(heights, height)
		}
		sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })

		for _, height := range heights {
			if !haveHeight {
				currentHeight = height
				haveHeight = true
			}
			for backlogCount > 0 && currentHeight < height {
				if err := processHeight(currentHeight, aggregateHeightStats{}); err != nil {
					_ = writer.Close()
					return nil, err
				}
				currentHeight++
			}
			if currentHeight < height {
				currentHeight = height
			}
			for _, group := range groupsByHeight[height] {
				backlog = append(backlog, group)
				backlogCount += group.Count
			}
			if err := processHeight(currentHeight, statsByHeight[height]); err != nil {
				_ = writer.Close()
				return nil, err
			}
			currentHeight++
		}
	}
	for backlogCount > 0 {
		if err := processHeight(currentHeight, aggregateHeightStats{}); err != nil {
			_ = writer.Close()
			return nil, err
		}
		currentHeight++
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	summary.BlocksSHA256 = writer.SHA256()
	summary.BlocksFileSHA256 = writer.FileSHA256()
	return summary, nil
}

func loadAggregateShard(path string) (map[uint64][]aggregateGroup, map[uint64]aggregateHeightStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	byHeightAmount := make(map[uint64]map[int64]int64)
	statsByHeight := make(map[uint64]aggregateHeightStats)
	var record [aggregateShardRecordSize]byte
	for {
		_, err := io.ReadFull(f, record[:])
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				return nil, nil, fmt.Errorf("corrupt aggregate shard %s", path)
			}
			return nil, nil, err
		}
		height := binary.BigEndian.Uint64(record[0:8])
		amount := int64(binary.BigEndian.Uint64(record[8:16]))
		amounts := byHeightAmount[height]
		if amounts == nil {
			amounts = make(map[int64]int64)
			byHeightAmount[height] = amounts
		}
		amounts[amount]++
		stats := statsByHeight[height]
		stats.Count++
		stats.AmountSat += amount
		statsByHeight[height] = stats
	}

	groupsByHeight := make(map[uint64][]aggregateGroup, len(byHeightAmount))
	for height, amounts := range byHeightAmount {
		keys := make([]int64, 0, len(amounts))
		for amount := range amounts {
			keys = append(keys, amount)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		groups := make([]aggregateGroup, 0, len(keys))
		for _, amount := range keys {
			groups = append(groups, aggregateGroup{
				ExpiryHeight: height,
				AmountSat:    amount,
				Count:        amounts[amount],
			})
		}
		groupsByHeight[height] = groups
	}
	return groupsByHeight, statsByHeight, nil
}

func selectAggregateBlock(height uint64, expired aggregateHeightStats,
	backlog *[]aggregateGroup, backlogCount *int64, p reap.REAPParams) reapAggregateBlock {

	block := reapAggregateBlock{
		ReapHeight:       height,
		ExpiredInputs:    expired.Count,
		ExpiredAmountSat: expired.AmountSat,
	}
	dustCount := int64(0)
	normalCount := int64(0)

	for len(*backlog) > 0 {
		group := &(*backlog)[0]
		dust := isDust(group.AmountSat, p.DustThresholdSat)
		pick := aggregatePickCount(dust, dustCount, normalCount,
			block.SelectedInputs, group.Count, p)
		if pick <= 0 {
			break
		}
		taxEach := taxForAmount(group.AmountSat, p)
		refundEach := group.AmountSat - taxEach
		if dust {
			taxEach = group.AmountSat
			refundEach = 0
			dustCount += pick
			block.DustInputs += pick
			block.DustTaxSat += taxEach * pick
		} else {
			normalCount += pick
			block.NormalInputs += pick
			block.NormalTaxSat += taxEach * pick
		}
		block.SelectedInputs += pick
		block.TaxTotalSat += taxEach * pick
		block.RefundTotalSat += refundEach * pick
		group.Count -= pick
		*backlogCount -= pick
		if group.Count == 0 {
			*backlog = (*backlog)[1:]
		}
		if aggregateTierLimitReached(dustCount, normalCount, p) {
			break
		}
	}
	block.RemainingBacklog = *backlogCount
	return block
}

func aggregatePickCount(dust bool, dustCount, normalCount, selected, available int64,
	p reap.REAPParams) int64 {

	if available <= 0 {
		return 0
	}
	capacity := aggregateCandidateCapacity(dust, dustCount, normalCount, p)
	if capacity <= 0 {
		return 0
	}
	if capacity > available {
		capacity = available
	}
	if p.WeightBudget <= 0 {
		return capacity
	}
	if aggregateWeightWith(dust, dustCount, normalCount, selected, capacity, p) <= p.WeightBudget {
		return capacity
	}
	low := int64(0)
	high := capacity
	for low < high {
		mid := (low + high + 1) / 2
		if aggregateWeightWith(dust, dustCount, normalCount, selected, mid, p) <= p.WeightBudget {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low == 0 && selected == 0 {
		return 1
	}
	return low
}

func aggregateCandidateCapacity(dust bool, dustCount, normalCount int64, p reap.REAPParams) int64 {
	if p.DustMaxInputs <= 0 {
		return int64(p.MaxInputs) - dustCount - normalCount
	}
	if dustCount >= int64(p.DustMaxInputs) || normalCount >= int64(p.MaxInputs) {
		return 0
	}
	if dust {
		return int64(p.DustMaxInputs) - dustCount
	}
	return int64(p.MaxInputs) - normalCount
}

func aggregateWeightWith(dust bool, dustCount, normalCount, selected, add int64,
	p reap.REAPParams) int64 {

	if add <= 0 {
		return estimatePreviewPlanWeight(int(dustCount), int(normalCount), int(selected), p)
	}
	nextDust := dustCount
	nextNormal := normalCount
	if dust {
		nextDust += add
	} else {
		nextNormal += add
	}
	return estimatePreviewPlanWeight(int(nextDust), int(nextNormal), int(selected+add), p)
}

func aggregateTierLimitReached(dustCount, normalCount int64, p reap.REAPParams) bool {
	if p.DustMaxInputs <= 0 {
		return dustCount+normalCount >= int64(p.MaxInputs)
	}
	return dustCount >= int64(p.DustMaxInputs) || normalCount >= int64(p.MaxInputs)
}

type aggregateShardWriters struct {
	workDir string
	open    map[uint64]*os.File
	order   []uint64
}

func newAggregateShardWriters(workDir string) *aggregateShardWriters {
	return &aggregateShardWriters{
		workDir: workDir,
		open:    make(map[uint64]*os.File),
	}
}

func (w *aggregateShardWriters) path(shardID uint64) string {
	return filepath.Join(w.workDir, fmt.Sprintf("shard-%012d.bin", shardID))
}

func (w *aggregateShardWriters) writer(shardID uint64) (*os.File, error) {
	if f := w.open[shardID]; f != nil {
		return f, nil
	}
	if len(w.open) >= 64 {
		oldest := w.order[0]
		w.order = w.order[1:]
		if err := w.open[oldest].Close(); err != nil {
			return nil, err
		}
		delete(w.open, oldest)
	}
	f, err := os.OpenFile(w.path(shardID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	w.open[shardID] = f
	w.order = append(w.order, shardID)
	return f, nil
}

func (w *aggregateShardWriters) closeAll() error {
	var firstErr error
	for id, f := range w.open {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(w.open, id)
	}
	w.order = nil
	return firstErr
}
