// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
)

type config struct {
	Network          string
	Source           string
	RPCServer        string
	RPCUser          string
	RPCPass          string
	RPCCert          string
	NoTLS            bool
	DBType           string
	DBPath           string
	DBNet            string
	ForkHeight       int
	ForkHash         string
	OutDir           string
	InputPath        string
	PageSize         int
	StartHeight      int
	EndHeight        int
	EndHeightSet     bool
	AllowMovingTip   bool
	AllowStaleUTXO   bool
	AggregatePreview bool
	PreviewWorkDir   string
	PreviewShardSpan int
	ReapStartHeight  int
	NoPreview        bool
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "obtc-utxo-export: %v\n", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (*config, error) {
	cfg := &config{
		Network:     "obtcmainnet",
		Source:      "rpc",
		DBType:      "ffldb",
		DBNet:       "mainnet",
		ForkHeight:  -1,
		OutDir:      ".",
		StartHeight: 0,
	}
	fs := flag.NewFlagSet("obtc-utxo-export", flag.ContinueOnError)
	fs.StringVar(&cfg.Network, "network", cfg.Network, "network: obtcmainnet|obtctestnet|obtcregtest")
	fs.StringVar(&cfg.Source, "source", cfg.Source, "export source: rpc|btcd-db")
	fs.StringVar(&cfg.RPCServer, "rpcserver", "", "RPC server host:port")
	fs.StringVar(&cfg.RPCUser, "rpcuser", "", "RPC username")
	fs.StringVar(&cfg.RPCPass, "rpcpass", "", "RPC password")
	fs.StringVar(&cfg.RPCCert, "rpccert", "", "RPC TLS certificate")
	fs.BoolVar(&cfg.NoTLS, "notls", false, "disable RPC TLS")
	fs.StringVar(&cfg.DBType, "dbtype", cfg.DBType, "database backend for --source=btcd-db")
	fs.StringVar(&cfg.DBPath, "dbpath", "", "path to blocks database directory for --source=btcd-db")
	fs.StringVar(&cfg.DBNet, "dbnet", cfg.DBNet, "database network for --source=btcd-db: mainnet|testnet3|testnet4|regtest|simnet")
	fs.IntVar(&cfg.ForkHeight, "fork-height", cfg.ForkHeight, "required BTC fork anchor height for --source=btcd-db")
	fs.StringVar(&cfg.ForkHash, "fork-hash", "", "required BTC fork anchor hash for --source=btcd-db")
	fs.StringVar(&cfg.OutDir, "outdir", cfg.OutDir, "output directory")
	fs.StringVar(&cfg.InputPath, "input", "", "existing utxo-expiry-snapshot JSONL gzip file for offline preview")
	fs.IntVar(&cfg.PageSize, "page-size", 0, "listexpiring page size; defaults to network batch limit")
	fs.IntVar(&cfg.StartHeight, "start-height", cfg.StartHeight, "expiry height scan start")
	fs.IntVar(&cfg.EndHeight, "end-height", -1, "expiry height scan end; defaults to snapshot height + expiry window")
	fs.BoolVar(&cfg.AllowMovingTip, "allow-moving-tip", false, "allow export even if chain tip changes while scanning")
	fs.BoolVar(&cfg.AllowStaleUTXO, "allow-stale-utxo", false, "allow direct DB export when the flushed UTXO state hash does not match the best chain hash")
	fs.BoolVar(&cfg.AggregatePreview, "aggregate-preview", false, "write memory-bounded aggregate REAP preview instead of private txid/vout detail")
	fs.StringVar(&cfg.PreviewWorkDir, "preview-workdir", "", "temporary directory for aggregate preview shards; defaults to --outdir")
	fs.IntVar(&cfg.PreviewShardSpan, "preview-shard-span", 4096, "expiry-height span per aggregate preview shard")
	fs.IntVar(&cfg.ReapStartHeight, "reap-start-height", -1, "first chain height allowed to include REAP; defaults to fork-height+2016 when --fork-height is set")
	fs.BoolVar(&cfg.NoPreview, "no-preview", false, "export UTXO snapshot only")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.EndHeight >= 0 {
		cfg.EndHeightSet = true
	}
	cfg.Source = strings.ToLower(strings.TrimSpace(cfg.Source))
	if cfg.Source == "direct-db" {
		cfg.Source = "btcd-db"
	}
	if cfg.InputPath == "" {
		switch cfg.Source {
		case "rpc":
			if cfg.RPCUser == "" || cfg.RPCPass == "" {
				return nil, errors.New("--rpcuser and --rpcpass are required for RPC export")
			}
		case "btcd-db":
			if cfg.DBPath == "" {
				return nil, errors.New("--dbpath is required for --source=btcd-db")
			}
			if cfg.ForkHeight < 0 {
				return nil, errors.New("--fork-height is required for --source=btcd-db")
			}
			if strings.TrimSpace(cfg.ForkHash) == "" {
				return nil, errors.New("--fork-hash is required for --source=btcd-db")
			}
		default:
			return nil, fmt.Errorf("unsupported --source %q", cfg.Source)
		}
	}
	if cfg.RPCServer == "" {
		cfg.RPCServer = defaultRPCServer(cfg.Network)
	}
	if cfg.PageSize < 0 {
		return nil, errors.New("--page-size must be >= 0")
	}
	if cfg.StartHeight < 0 {
		return nil, errors.New("--start-height must be >= 0")
	}
	if cfg.PreviewShardSpan <= 0 {
		return nil, errors.New("--preview-shard-span must be > 0")
	}
	if cfg.ReapStartHeight < -1 {
		return nil, errors.New("--reap-start-height must be >= 0 or -1")
	}
	return cfg, nil
}

func run(cfg *config) error {
	params, err := resolveParams(cfg.Network)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutDir, 0700); err != nil {
		return err
	}

	if cfg.InputPath != "" {
		snapshotHeight, snapshotHash, err := snapshotFromFile(cfg.InputPath)
		if err != nil {
			return err
		}
		if cfg.AggregatePreview {
			blocksPath, summaryPath := aggregatePreviewPaths(cfg.OutDir, snapshotHeight, snapshotHash)
			_, err := writeAggregatePreviewFiles(cfg.InputPath, cfg.Network, params, snapshotHeight,
				snapshotHash, blocksPath, summaryPath, aggregatePreviewOptions{
					WorkDir:         aggregateWorkDir(cfg),
					ShardSpan:       uint64(cfg.PreviewShardSpan),
					ReapStartHeight: aggregateReapStartHeight(cfg),
				})
			if err != nil {
				return fmt.Errorf("write aggregate preview: %w", err)
			}
			fmt.Printf("preview_blocks=%s\npreview_summary=%s\n", blocksPath, summaryPath)
			return nil
		}
		rows, err := loadUTXORows(cfg.InputPath)
		if err != nil {
			return fmt.Errorf("load input: %w", err)
		}
		detailPath, summaryPath := previewPaths(cfg.OutDir, snapshotHeight, snapshotHash)
		_, err = writePreviewFiles(rows, cfg.Network, params, snapshotHeight, snapshotHash, detailPath, summaryPath)
		if err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
		fmt.Printf("preview_detail=%s\npreview_summary=%s\n", detailPath, summaryPath)
		return nil
	}

	var utxoPath string
	var manifest *exportManifest
	switch cfg.Source {
	case "rpc":
		utxoPath, manifest, err = exportOnline(cfg)
	case "btcd-db":
		utxoPath, manifest, err = exportBTCDDB(cfg, params)
	default:
		err = fmt.Errorf("unsupported --source %q", cfg.Source)
	}
	if err != nil {
		return err
	}
	manifestPath := manifestPathFor(cfg.OutDir, manifest.SnapshotHeight, manifest.SnapshotHash)
	if err := writeManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	fmt.Printf("utxo_snapshot=%s\nmanifest=%s\n", utxoPath, manifestPath)

	if err := validateManifest(manifest); err != nil {
		return err
	}
	if !manifest.SnapshotStable && !cfg.AllowMovingTip {
		return fmt.Errorf("chain tip changed during export: started %d/%s ended %d/%s",
			manifest.SnapshotHeight, manifest.SnapshotHash, manifest.FinalHeight, manifest.FinalHash)
	}
	if cfg.NoPreview {
		return nil
	}
	if cfg.AggregatePreview || cfg.Source == "btcd-db" {
		blocksPath, summaryPath := aggregatePreviewPaths(cfg.OutDir, manifest.SnapshotHeight, manifest.SnapshotHash)
		_, err := writeAggregatePreviewFiles(utxoPath, cfg.Network, params, manifest.SnapshotHeight,
			manifest.SnapshotHash, blocksPath, summaryPath, aggregatePreviewOptions{
				WorkDir:         aggregateWorkDir(cfg),
				ShardSpan:       uint64(cfg.PreviewShardSpan),
				ReapStartHeight: aggregateReapStartHeight(cfg),
			})
		if err != nil {
			return fmt.Errorf("write aggregate preview: %w", err)
		}
		fmt.Printf("preview_blocks=%s\npreview_summary=%s\n", blocksPath, summaryPath)
		return nil
	}

	rows, err := loadUTXORows(utxoPath)
	if err != nil {
		return fmt.Errorf("reload exported UTXO snapshot: %w", err)
	}
	detailPath, summaryPath := previewPaths(cfg.OutDir, manifest.SnapshotHeight, manifest.SnapshotHash)
	_, err = writePreviewFiles(rows, cfg.Network, params, manifest.SnapshotHeight, manifest.SnapshotHash, detailPath, summaryPath)
	if err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	fmt.Printf("preview_detail=%s\npreview_summary=%s\n", detailPath, summaryPath)
	return nil
}

func exportOnline(cfg *config) (string, *exportManifest, error) {
	client, err := rpcClient(cfg)
	if err != nil {
		return "", nil, err
	}
	defer client.Shutdown()

	startedAt := time.Now().UTC()
	chainInfo, err := client.GetBlockChainInfo()
	if err != nil {
		return "", nil, fmt.Errorf("getblockchaininfo: %w", err)
	}
	stats, err := client.GetExpiryIndexStats()
	if err != nil {
		return "", nil, fmt.Errorf("getexpiryindexstats: %w", err)
	}
	if stats.Disabled {
		return "", nil, errors.New("expiry index is disabled")
	}
	if stats.TipHeight != chainInfo.Blocks {
		return "", nil, fmt.Errorf("expiry index tip %d does not match chain height %d", stats.TipHeight, chainInfo.Blocks)
	}

	pageSize := cfg.PageSize
	if pageSize == 0 {
		pageSize = 10000
		if stats.NetworkParams != nil && stats.NetworkParams.ListBatchLimit > 0 {
			pageSize = stats.NetworkParams.ListBatchLimit
		}
	}

	endHeight, err := resolveEndHeight(cfg, chainInfo, stats)
	if err != nil {
		return "", nil, err
	}
	if endHeight < cfg.StartHeight {
		return "", nil, fmt.Errorf("end height %d is below start height %d", endHeight, cfg.StartHeight)
	}

	utxoPath := utxoPathFor(cfg.OutDir, chainInfo.Blocks, chainInfo.BestBlockHash)
	writer, err := newJSONLGzipWriter(utxoPath)
	if err != nil {
		return "", nil, err
	}

	manifest := &exportManifest{
		Network:         cfg.Network,
		Source:          "rpc-expiryindex",
		SnapshotHeight:  chainInfo.Blocks,
		SnapshotHash:    chainInfo.BestBlockHash,
		StatsTotalUTXOs: stats.TotalUTXOs,
		StartHeight:     int32(cfg.StartHeight),
		EndHeight:       int32(endHeight),
		PageSize:        pageSize,
		ExportStartedAt: startedAt,
		OutputFile:      utxoPath,
	}

	var previous *utxoRow
	scanHeight := int32(cfg.StartHeight)
	end := int32(endHeight)
	var cursor *string
	for {
		result, err := client.ListExpiring(&scanHeight, &end, &pageSize, cursor)
		if err != nil {
			_ = writer.Close()
			return "", nil, fmt.Errorf("listexpiring height=%d cursor=%v: %w", scanHeight, cursorValue(cursor), err)
		}
		manifest.Pages++
		for _, item := range result.ExpiringUTXOs {
			row := utxoRow{
				TxID:           item.TxID,
				Vout:           item.Vout,
				AmountSat:      item.AmountSat,
				CreateHeight:   item.CreateHeight,
				ExpiryHeight:   item.ExpiryHeight,
				BlocksToExpiry: item.BlocksToExpiry,
				SnapshotHeight: chainInfo.Blocks,
				SnapshotHash:   chainInfo.BestBlockHash,
			}
			updateManifestForRow(manifest, previous, row, true)
			if err := writer.WriteJSON(row); err != nil {
				_ = writer.Close()
				return "", nil, err
			}
			previous = &row
		}
		if result.NextHeight == nil || result.NextOutpoint == nil {
			break
		}
		scanHeight = *result.NextHeight
		cursor = result.NextOutpoint
	}

	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	finishedAt := time.Now().UTC()
	finalInfo, err := client.GetBlockChainInfo()
	if err != nil {
		return "", nil, fmt.Errorf("final getblockchaininfo: %w", err)
	}
	manifest.FinalHeight = finalInfo.Blocks
	manifest.FinalHash = finalInfo.BestBlockHash
	manifest.SnapshotStable = finalInfo.Blocks == chainInfo.Blocks &&
		finalInfo.BestBlockHash == chainInfo.BestBlockHash
	manifest.ExportFinishedAt = finishedAt
	manifest.DurationSeconds = finishedAt.Sub(startedAt).Seconds()
	manifest.SHA256 = writer.SHA256()
	manifest.FileSHA256 = writer.FileSHA256()

	return utxoPath, manifest, nil
}

func updateManifestForRow(manifest *exportManifest, previous *utxoRow, row utxoRow, amountKnown bool) {
	manifest.RowCount++
	manifest.SumAmountSat += row.AmountSat
	if !amountKnown {
		manifest.MissingAmountCount++
	}
	if manifest.RowCount == 1 || row.ExpiryHeight < manifest.FirstExpiryHeight {
		manifest.FirstExpiryHeight = row.ExpiryHeight
	}
	if row.ExpiryHeight > manifest.LastExpiryHeight {
		manifest.LastExpiryHeight = row.ExpiryHeight
	}
	if previous == nil {
		return
	}
	cmp := compareUTXORowsExportOrder(*previous, row)
	if cmp == 0 {
		manifest.DuplicateOutpointCount++
	}
	if cmp > 0 {
		manifest.OrderViolationCount++
	}
}

func resolveEndHeight(cfg *config, chainInfo *btcjson.GetBlockChainInfoResult,
	stats *btcjson.ExpiryIndexStatsResult) (int, error) {
	if cfg.EndHeightSet {
		return cfg.EndHeight, nil
	}
	if stats.NetworkParams == nil || stats.NetworkParams.WindowBlocks == 0 {
		return 0, errors.New("--end-height is required when expiry network params are unavailable")
	}
	end := int64(chainInfo.Blocks) + int64(stats.NetworkParams.WindowBlocks)
	if end > math.MaxInt32 {
		return 0, fmt.Errorf("computed end height %d exceeds int32 RPC parameter range", end)
	}
	return int(end), nil
}

func rpcClient(cfg *config) (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.RPCServer,
		User:         cfg.RPCUser,
		Pass:         cfg.RPCPass,
		HTTPPostMode: true,
		DisableTLS:   cfg.NoTLS,
	}
	if cfg.RPCCert != "" {
		cert, err := os.ReadFile(filepath.Clean(cfg.RPCCert))
		if err != nil {
			return nil, fmt.Errorf("read RPC cert: %w", err)
		}
		connCfg.DisableTLS = false
		connCfg.Certificates = cert
	}
	return rpcclient.New(connCfg, nil)
}

func writeManifest(path string, manifest *exportManifest) error {
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0600)
}

func validateManifest(manifest *exportManifest) error {
	var problems []string
	if manifest.StatsTotalUTXOs >= 0 && manifest.RowCount != int64(manifest.StatsTotalUTXOs) {
		problems = append(problems, fmt.Sprintf("row_count=%d stats_total_utxos=%d",
			manifest.RowCount, manifest.StatsTotalUTXOs))
	}
	if manifest.DuplicateOutpointCount != 0 {
		problems = append(problems, fmt.Sprintf("duplicate_outpoint_count=%d", manifest.DuplicateOutpointCount))
	}
	if manifest.OrderViolationCount != 0 {
		problems = append(problems, fmt.Sprintf("order_violation_count=%d", manifest.OrderViolationCount))
	}
	if manifest.MissingAmountCount != 0 {
		problems = append(problems, fmt.Sprintf("missing_amount_count=%d", manifest.MissingAmountCount))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("export integrity check failed: %s", strings.Join(problems, "; "))
}

func snapshotFromRows(rows []utxoRow) (int32, string, error) {
	if len(rows) == 0 {
		return 0, "empty", nil
	}
	height := rows[0].SnapshotHeight
	hash := rows[0].SnapshotHash
	for i, row := range rows[1:] {
		if row.SnapshotHeight != height || row.SnapshotHash != hash {
			return 0, "", fmt.Errorf("input contains mixed snapshots: row 0 is %d/%s, row %d is %d/%s",
				height, hash, i+1, row.SnapshotHeight, row.SnapshotHash)
		}
	}
	return height, hash, nil
}

func utxoPathFor(outDir string, height int32, hash string) string {
	return filepath.Join(outDir, fmt.Sprintf("utxo-expiry-snapshot-%d-%s.jsonl.gz", height, cleanHash(hash)))
}

func manifestPathFor(outDir string, height int32, hash string) string {
	return filepath.Join(outDir, fmt.Sprintf("utxo-expiry-snapshot-%d-%s.manifest.json", height, cleanHash(hash)))
}

func previewPaths(outDir string, height int32, hash string) (string, string) {
	clean := cleanHash(hash)
	return filepath.Join(outDir, fmt.Sprintf("reap-preview-detail-%d-%s.jsonl.gz", height, clean)),
		filepath.Join(outDir, fmt.Sprintf("reap-preview-summary-%d-%s.json", height, clean))
}

func aggregatePreviewPaths(outDir string, height int32, hash string) (string, string) {
	clean := cleanHash(hash)
	return filepath.Join(outDir, fmt.Sprintf("reap-preview-blocks-%d-%s.jsonl.gz", height, clean)),
		filepath.Join(outDir, fmt.Sprintf("reap-preview-aggregate-summary-%d-%s.json", height, clean))
}

func aggregateWorkDir(cfg *config) string {
	if cfg.PreviewWorkDir != "" {
		return cfg.PreviewWorkDir
	}
	return cfg.OutDir
}

func aggregateReapStartHeight(cfg *config) uint64 {
	if cfg.ReapStartHeight >= 0 {
		return uint64(cfg.ReapStartHeight)
	}
	if cfg.ForkHeight >= 0 {
		return uint64(cfg.ForkHeight + 2016)
	}
	return 0
}

func cleanHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, hash)
}

func cursorValue(cursor *string) string {
	if cursor == nil {
		return ""
	}
	return *cursor
}

func defaultRPCServer(network string) string {
	switch strings.ToLower(network) {
	case "obtcmainnet", "obtc-mainnet":
		return "127.0.0.1:9528"
	case "obtctestnet", "obtc-testnet":
		return "127.0.0.1:19528"
	case "obtcregtest", "obtc-regtest":
		return "127.0.0.1:29528"
	default:
		return "127.0.0.1:9528"
	}
}

func resolveParams(network string) (*chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "obtcmainnet", "obtc-mainnet":
		return &chaincfg.ObtcMainNetParams, nil
	case "obtctestnet", "obtc-testnet":
		return &chaincfg.ObtcTestNetParams, nil
	case "obtcregtest", "obtc-regtest":
		return &chaincfg.ObtcRegTestParams, nil
	default:
		return nil, fmt.Errorf("unsupported network %q", network)
	}
}
