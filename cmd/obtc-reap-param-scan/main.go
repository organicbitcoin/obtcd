// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	recordSize    = 16
	maxOpenShards = 512
	baseWeight    = int64(40)
	inputWeight   = int64(164)
	refundWeight  = int64(172)
	markerWeight  = int64(120)
)

type config struct {
	InputPath       string
	OutPath         string
	WorkDir         string
	ShardSpan       uint64
	ReapStartHeight uint64
	Scenarios       string
}

type utxoRow struct {
	AmountSat      int64  `json:"amount_sat"`
	ExpiryHeight   uint64 `json:"expiry_height"`
	SnapshotHeight int32  `json:"snapshot_height"`
	SnapshotHash   string `json:"snapshot_hash"`
}

type scenario struct {
	Name             string `json:"name"`
	MaxInputs        int64  `json:"max_inputs"`
	DustMaxInputs    int64  `json:"dust_max_inputs"`
	WeightBudget     int64  `json:"weight_budget"`
	TaxNumerator     int64  `json:"tax_numerator"`
	TaxDenominator   int64  `json:"tax_denominator"`
	DustThresholdSat int64  `json:"dust_threshold_sat"`
}

type group struct {
	ExpiryHeight uint64
	AmountSat    int64
	Count        int64
}

type heightStats struct {
	Count     int64
	AmountSat int64
}

type shardInfo struct {
	Path string
	Rows int64
}

type scanOutput struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	InputPath       string            `json:"input_path"`
	SnapshotHeight  int32             `json:"snapshot_height"`
	SnapshotHash    string            `json:"snapshot_hash"`
	ReapStartHeight uint64            `json:"reap_start_height"`
	RowCount        int64             `json:"row_count"`
	ShardSpan       uint64            `json:"shard_span"`
	Scenarios       []scenarioSummary `json:"scenarios"`
}

type scenarioSummary struct {
	Name                          string  `json:"name"`
	MaxInputs                     int64   `json:"max_inputs"`
	DustMaxInputs                 int64   `json:"dust_max_inputs"`
	WeightBudget                  int64   `json:"weight_budget"`
	ReservedBlockWeightPercent    float64 `json:"reserved_block_weight_percent"`
	FirstReapHeight               uint64  `json:"first_reap_height"`
	LastReapHeight                uint64  `json:"last_reap_height"`
	ReapBlockCount                int64   `json:"reap_block_count"`
	CalendarYearsAt10Min          float64 `json:"calendar_years_at_10min"`
	InitialExpiredInputs          int64   `json:"initial_expired_inputs"`
	InitialBacklogClearedHeight   uint64  `json:"initial_backlog_cleared_height"`
	InitialBacklogClearBlocks     int64   `json:"initial_backlog_clear_blocks"`
	InitialBacklogClearYears      float64 `json:"initial_backlog_clear_years"`
	ExpiredInputs                 int64   `json:"expired_inputs"`
	SelectedInputs                int64   `json:"selected_inputs"`
	TaxTotalSat                   int64   `json:"tax_total_sat"`
	RefundTotalSat                int64   `json:"refund_total_sat"`
	DustTaxSat                    int64   `json:"dust_tax_sat"`
	NormalTaxSat                  int64   `json:"normal_tax_sat"`
	MaxRemainingBacklog           int64   `json:"max_remaining_backlog"`
	MaxSelectedInputs             int64   `json:"max_selected_inputs"`
	MaxNormalInputs               int64   `json:"max_normal_inputs"`
	MaxDustInputs                 int64   `json:"max_dust_inputs"`
	MaxTaxSat                     int64   `json:"max_tax_sat"`
	MaxEstimatedWeight            int64   `json:"max_estimated_weight"`
	AverageSelectedInputsPerBlock float64 `json:"average_selected_inputs_per_block"`
}

type blockStats struct {
	Height           uint64
	ExpiredInputs    int64
	ExpiredAmountSat int64
	SelectedInputs   int64
	TaxTotalSat      int64
	RefundTotalSat   int64
	DustTaxSat       int64
	NormalTaxSat     int64
	DustInputs       int64
	NormalInputs     int64
	RemainingBacklog int64
	EstimatedWeight  int64
	SelectedInitial  int64
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "obtc-reap-param-scan: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (*config, error) {
	cfg := &config{
		ShardSpan:       4096,
		Scenarios:       "current:256:1024:200000,normal512:512:1024:200000,normal1024:1024:2048:400000,normal2048:2048:4096:800000",
		ReapStartHeight: 0,
	}
	fs := flag.NewFlagSet("obtc-reap-param-scan", flag.ContinueOnError)
	fs.StringVar(&cfg.InputPath, "input", "", "private utxo-expiry-snapshot JSONL gzip path")
	fs.StringVar(&cfg.OutPath, "out", "", "output JSON summary path")
	fs.StringVar(&cfg.WorkDir, "workdir", "", "temporary shard directory; defaults to output directory or current directory")
	fs.Uint64Var(&cfg.ShardSpan, "shard-span", cfg.ShardSpan, "expiry-height span per temporary shard")
	fs.Uint64Var(&cfg.ReapStartHeight, "reap-start-height", cfg.ReapStartHeight, "first height allowed to include REAP")
	fs.StringVar(&cfg.Scenarios, "scenarios", cfg.Scenarios, "comma-separated name:max_inputs:dust_max_inputs:weight_budget")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.InputPath == "" {
		return nil, errors.New("--input is required")
	}
	if cfg.OutPath == "" {
		return nil, errors.New("--out is required")
	}
	if cfg.ShardSpan == 0 {
		return nil, errors.New("--shard-span must be > 0")
	}
	if cfg.ReapStartHeight == 0 {
		return nil, errors.New("--reap-start-height is required")
	}
	return cfg, nil
}

func run(cfg *config) error {
	scenarios, err := parseScenarios(cfg.Scenarios)
	if err != nil {
		return err
	}
	workBase := cfg.WorkDir
	if workBase == "" {
		workBase = filepath.Dir(cfg.OutPath)
		if workBase == "" || workBase == "." {
			workBase = "."
		}
	}
	workDir, err := os.MkdirTemp(workBase, "obtc-reap-param-scan-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	fmt.Fprintf(os.Stderr, "sharding input %s into %s\n", cfg.InputPath, workDir)
	shards, rowCount, snapshotHeight, snapshotHash, err := shardRows(cfg.InputPath, workDir, cfg.ShardSpan)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "sharded rows=%d shards=%d snapshot=%d/%s\n", rowCount, len(shards), snapshotHeight, snapshotHash)

	output := scanOutput{
		GeneratedAt:     time.Now().UTC(),
		InputPath:       cfg.InputPath,
		SnapshotHeight:  snapshotHeight,
		SnapshotHash:    snapshotHash,
		ReapStartHeight: cfg.ReapStartHeight,
		RowCount:        rowCount,
		ShardSpan:       cfg.ShardSpan,
	}
	for _, sc := range scenarios {
		fmt.Fprintf(os.Stderr, "replaying scenario %s\n", sc.Name)
		summary, err := replayScenario(shards, cfg.ReapStartHeight, sc)
		if err != nil {
			return err
		}
		output.Scenarios = append(output.Scenarios, summary)
	}

	body, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.WriteFile(cfg.OutPath, body, 0600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", cfg.OutPath)
	return nil
}

func parseScenarios(raw string) ([]scenario, error) {
	var out []scenario
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) != 4 {
			return nil, fmt.Errorf("invalid scenario %q: want name:max_inputs:dust_max_inputs:weight_budget", part)
		}
		maxInputs, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || maxInputs <= 0 {
			return nil, fmt.Errorf("invalid max_inputs in %q", part)
		}
		dustMax, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || dustMax < 0 {
			return nil, fmt.Errorf("invalid dust_max_inputs in %q", part)
		}
		weightBudget, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || weightBudget < 0 {
			return nil, fmt.Errorf("invalid weight_budget in %q", part)
		}
		out = append(out, scenario{
			Name:             fields[0],
			MaxInputs:        maxInputs,
			DustMaxInputs:    dustMax,
			WeightBudget:     weightBudget,
			TaxNumerator:     30,
			TaxDenominator:   100,
			DustThresholdSat: 720,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("at least one scenario is required")
	}
	return out, nil
}

func shardRows(inputPath, workDir string, shardSpan uint64) (map[uint64]shardInfo, int64, int32, string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, 0, 0, "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, 0, 0, "", err
	}
	defer gz.Close()

	writers := newShardWriters(workDir)
	defer writers.closeAll()
	dec := json.NewDecoder(gz)
	shards := make(map[uint64]shardInfo)
	var (
		rowCount       int64
		snapshotHeight int32
		snapshotHash   string
		seenSnapshot   bool
	)
	for {
		var row utxoRow
		if err := dec.Decode(&row); err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, "", err
		}
		if row.AmountSat < 0 {
			return nil, 0, 0, "", fmt.Errorf("negative amount at row %d", rowCount)
		}
		if !seenSnapshot {
			seenSnapshot = true
			snapshotHeight = row.SnapshotHeight
			snapshotHash = row.SnapshotHash
		} else if row.SnapshotHeight != snapshotHeight || row.SnapshotHash != snapshotHash {
			return nil, 0, 0, "", fmt.Errorf("mixed snapshot at row %d", rowCount)
		}
		shardID := row.ExpiryHeight / shardSpan
		w, err := writers.writer(shardID)
		if err != nil {
			return nil, 0, 0, "", err
		}
		var rec [recordSize]byte
		binary.BigEndian.PutUint64(rec[0:8], row.ExpiryHeight)
		binary.BigEndian.PutUint64(rec[8:16], uint64(row.AmountSat))
		if _, err := w.Write(rec[:]); err != nil {
			return nil, 0, 0, "", err
		}
		info := shards[shardID]
		info.Path = writers.path(shardID)
		info.Rows++
		shards[shardID] = info
		rowCount++
		if rowCount%5_000_000 == 0 {
			fmt.Fprintf(os.Stderr, "sharded %d rows\n", rowCount)
		}
	}
	if err := writers.closeAll(); err != nil {
		return nil, 0, 0, "", err
	}
	return shards, rowCount, snapshotHeight, snapshotHash, nil
}

func replayScenario(shards map[uint64]shardInfo, reapStart uint64, sc scenario) (scenarioSummary, error) {
	ids := make([]uint64, 0, len(shards))
	for id := range shards {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	summary := scenarioSummary{
		Name:                       sc.Name,
		MaxInputs:                  sc.MaxInputs,
		DustMaxInputs:              sc.DustMaxInputs,
		WeightBudget:               sc.WeightBudget,
		ReservedBlockWeightPercent: float64(sc.WeightBudget) / 4_000_000 * 100,
	}
	var (
		backlog               []group
		backlogCount          int64
		currentHeight         uint64
		haveHeight            bool
		startStats            heightStats
		initialExpiredInputs  int64
		selectedInitialInputs int64
	)

	emit := func(block blockStats) {
		if block.SelectedInputs == 0 && block.ExpiredInputs == 0 && block.RemainingBacklog == 0 {
			return
		}
		if summary.ReapBlockCount == 0 {
			summary.FirstReapHeight = block.Height
		}
		summary.LastReapHeight = block.Height
		summary.ReapBlockCount++
		summary.ExpiredInputs += block.ExpiredInputs
		summary.SelectedInputs += block.SelectedInputs
		summary.TaxTotalSat += block.TaxTotalSat
		summary.RefundTotalSat += block.RefundTotalSat
		summary.DustTaxSat += block.DustTaxSat
		summary.NormalTaxSat += block.NormalTaxSat
		if block.RemainingBacklog > summary.MaxRemainingBacklog {
			summary.MaxRemainingBacklog = block.RemainingBacklog
		}
		if block.SelectedInputs > summary.MaxSelectedInputs {
			summary.MaxSelectedInputs = block.SelectedInputs
		}
		if block.NormalInputs > summary.MaxNormalInputs {
			summary.MaxNormalInputs = block.NormalInputs
		}
		if block.DustInputs > summary.MaxDustInputs {
			summary.MaxDustInputs = block.DustInputs
		}
		if block.TaxTotalSat > summary.MaxTaxSat {
			summary.MaxTaxSat = block.TaxTotalSat
		}
		if block.EstimatedWeight > summary.MaxEstimatedWeight {
			summary.MaxEstimatedWeight = block.EstimatedWeight
		}
		selectedInitialInputs += block.SelectedInitial
		if summary.InitialBacklogClearedHeight == 0 && initialExpiredInputs > 0 &&
			selectedInitialInputs >= initialExpiredInputs {
			summary.InitialBacklogClearedHeight = block.Height
			summary.InitialBacklogClearBlocks = int64(block.Height - reapStart + 1)
			summary.InitialBacklogClearYears = yearsAt10Min(summary.InitialBacklogClearBlocks)
		}
	}
	processHeight := func(height uint64, expired heightStats) {
		block := selectBlock(height, expired, &backlog, &backlogCount, sc, reapStart)
		emit(block)
	}

	for _, id := range ids {
		groupsByHeight, statsByHeight, err := loadShard(shards[id].Path)
		if err != nil {
			return summary, err
		}
		heights := make([]uint64, 0, len(groupsByHeight))
		for height := range groupsByHeight {
			heights = append(heights, height)
		}
		sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
		for _, height := range heights {
			if height < reapStart {
				for _, g := range groupsByHeight[height] {
					backlog = append(backlog, g)
					backlogCount += g.Count
				}
				stats := statsByHeight[height]
				startStats.Count += stats.Count
				startStats.AmountSat += stats.AmountSat
				initialExpiredInputs += stats.Count
				continue
			}
			if !haveHeight {
				currentHeight = reapStart
				haveHeight = true
			}
			for backlogCount > 0 && currentHeight < height {
				expired := heightStats{}
				if currentHeight == reapStart {
					expired = startStats
					startStats = heightStats{}
				}
				processHeight(currentHeight, expired)
				currentHeight++
			}
			if currentHeight < height {
				currentHeight = height
			}
			for _, g := range groupsByHeight[height] {
				backlog = append(backlog, g)
				backlogCount += g.Count
			}
			expired := statsByHeight[height]
			if currentHeight == reapStart {
				expired.Count += startStats.Count
				expired.AmountSat += startStats.AmountSat
				startStats = heightStats{}
			}
			processHeight(currentHeight, expired)
			currentHeight++
		}
	}
	if !haveHeight && (backlogCount > 0 || startStats.Count > 0) {
		currentHeight = reapStart
		haveHeight = true
		processHeight(currentHeight, startStats)
		currentHeight++
		startStats = heightStats{}
	}
	for backlogCount > 0 {
		processHeight(currentHeight, heightStats{})
		currentHeight++
	}

	summary.InitialExpiredInputs = initialExpiredInputs
	if summary.ReapBlockCount > 0 {
		summary.CalendarYearsAt10Min = yearsAt10Min(int64(summary.LastReapHeight - summary.FirstReapHeight + 1))
		summary.AverageSelectedInputsPerBlock = float64(summary.SelectedInputs) / float64(summary.ReapBlockCount)
	}
	return summary, nil
}

func selectBlock(height uint64, expired heightStats, backlog *[]group, backlogCount *int64,
	sc scenario, reapStart uint64) blockStats {

	block := blockStats{
		Height:           height,
		ExpiredInputs:    expired.Count,
		ExpiredAmountSat: expired.AmountSat,
	}
	var dustCount, normalCount int64
	for len(*backlog) > 0 {
		g := &(*backlog)[0]
		dust := g.AmountSat > 0 && g.AmountSat < sc.DustThresholdSat
		pick := pickCount(dust, dustCount, normalCount, block.SelectedInputs, g.Count, sc)
		if pick <= 0 {
			break
		}
		taxEach := taxForAmount(g.AmountSat, sc)
		refundEach := g.AmountSat - taxEach
		if dust {
			taxEach = g.AmountSat
			refundEach = 0
			dustCount += pick
			block.DustInputs += pick
			block.DustTaxSat += taxEach * pick
		} else {
			normalCount += pick
			block.NormalInputs += pick
			block.NormalTaxSat += taxEach * pick
		}
		if g.ExpiryHeight < reapStart {
			block.SelectedInitial += pick
		}
		block.SelectedInputs += pick
		block.TaxTotalSat += taxEach * pick
		block.RefundTotalSat += refundEach * pick
		g.Count -= pick
		*backlogCount -= pick
		if g.Count == 0 {
			*backlog = (*backlog)[1:]
		}
		if tierLimitReached(dustCount, normalCount, sc) {
			break
		}
	}
	block.RemainingBacklog = *backlogCount
	block.EstimatedWeight = estimateWeight(dustCount, normalCount)
	return block
}

func pickCount(dust bool, dustCount, normalCount, selected, available int64, sc scenario) int64 {
	capacity := candidateCapacity(dust, dustCount, normalCount, sc)
	if capacity <= 0 {
		return 0
	}
	if capacity > available {
		capacity = available
	}
	if sc.WeightBudget <= 0 {
		return capacity
	}
	if weightWith(dust, dustCount, normalCount, capacity) <= sc.WeightBudget {
		return capacity
	}
	low, high := int64(0), capacity
	for low < high {
		mid := (low + high + 1) / 2
		if weightWith(dust, dustCount, normalCount, mid) <= sc.WeightBudget {
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

func candidateCapacity(dust bool, dustCount, normalCount int64, sc scenario) int64 {
	if sc.DustMaxInputs <= 0 {
		return sc.MaxInputs - dustCount - normalCount
	}
	if dustCount >= sc.DustMaxInputs || normalCount >= sc.MaxInputs {
		return 0
	}
	if dust {
		return sc.DustMaxInputs - dustCount
	}
	return sc.MaxInputs - normalCount
}

func weightWith(dust bool, dustCount, normalCount, add int64) int64 {
	if dust {
		dustCount += add
	} else {
		normalCount += add
	}
	return estimateWeight(dustCount, normalCount)
}

func estimateWeight(dustCount, normalCount int64) int64 {
	return baseWeight + (dustCount+normalCount)*inputWeight + normalCount*refundWeight + markerWeight
}

func tierLimitReached(dustCount, normalCount int64, sc scenario) bool {
	if sc.DustMaxInputs <= 0 {
		return dustCount+normalCount >= sc.MaxInputs
	}
	return dustCount >= sc.DustMaxInputs || normalCount >= sc.MaxInputs
}

func taxForAmount(amount int64, sc scenario) int64 {
	if amount <= 0 || sc.TaxDenominator <= 0 || sc.TaxNumerator <= 0 {
		return 0
	}
	return amount * sc.TaxNumerator / sc.TaxDenominator
}

func loadShard(path string) (map[uint64][]group, map[uint64]heightStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	byHeightAmount := make(map[uint64]map[int64]int64)
	statsByHeight := make(map[uint64]heightStats)
	var rec [recordSize]byte
	for {
		_, err := io.ReadFull(f, rec[:])
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				return nil, nil, fmt.Errorf("corrupt shard %s", path)
			}
			return nil, nil, err
		}
		height := binary.BigEndian.Uint64(rec[0:8])
		amount := int64(binary.BigEndian.Uint64(rec[8:16]))
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

	groupsByHeight := make(map[uint64][]group, len(byHeightAmount))
	for height, amounts := range byHeightAmount {
		keys := make([]int64, 0, len(amounts))
		for amount := range amounts {
			keys = append(keys, amount)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		groups := make([]group, 0, len(keys))
		for _, amount := range keys {
			groups = append(groups, group{
				ExpiryHeight: height,
				AmountSat:    amount,
				Count:        amounts[amount],
			})
		}
		groupsByHeight[height] = groups
	}
	return groupsByHeight, statsByHeight, nil
}

func yearsAt10Min(blocks int64) float64 {
	return float64(blocks) * 10 / (60 * 24 * 365.25)
}

type shardWriters struct {
	workDir string
	open    map[uint64]*bufio.Writer
	files   map[uint64]*os.File
	order   []uint64
}

func newShardWriters(workDir string) *shardWriters {
	return &shardWriters{
		workDir: workDir,
		open:    make(map[uint64]*bufio.Writer),
		files:   make(map[uint64]*os.File),
	}
}

func (w *shardWriters) path(id uint64) string {
	return filepath.Join(w.workDir, fmt.Sprintf("shard-%012d.bin", id))
}

func (w *shardWriters) writer(id uint64) (*bufio.Writer, error) {
	if bw := w.open[id]; bw != nil {
		return bw, nil
	}
	if len(w.open) >= maxOpenShards {
		oldest := w.order[0]
		w.order = w.order[1:]
		if err := w.closeOne(oldest); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(w.path(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	w.open[id] = bw
	w.files[id] = f
	w.order = append(w.order, id)
	return bw, nil
}

func (w *shardWriters) closeOne(id uint64) error {
	bw := w.open[id]
	f := w.files[id]
	delete(w.open, id)
	delete(w.files, id)
	if bw != nil {
		if err := bw.Flush(); err != nil {
			if f != nil {
				_ = f.Close()
			}
			return err
		}
	}
	if f != nil {
		return f.Close()
	}
	return nil
}

func (w *shardWriters) closeAll() error {
	var firstErr error
	for id := range w.open {
		if err := w.closeOne(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	w.order = nil
	return firstErr
}
