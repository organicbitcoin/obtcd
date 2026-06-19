// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

type config struct {
	SourceDBPath    string
	IndexDBPath     string
	DBType          string
	DBNet           string
	ForkHeight      int
	ForkHash        string
	BatchSize       int
	ProgressEvery   int64
	ResetIndex      bool
	OutPath         string
	ReapLimit       int
	ListLimit       int
	BenchmarkHeight int
}

type result struct {
	ForkHeight       int32                         `json:"fork_height"`
	ForkHash         string                        `json:"fork_hash"`
	SourceBestHeight int32                         `json:"source_best_height"`
	SourceBestHash   string                        `json:"source_best_hash"`
	Build            *expiryindex.ShadowBuildStats `json:"build"`
	IndexStats       *expiryindex.ExpiryIndexStats `json:"index_stats"`
	QueryBenchmarks  []queryBenchmark              `json:"query_benchmarks"`
	IndexDBPath      string                        `json:"index_db_path"`
	IndexDBBytes     int64                         `json:"index_db_bytes"`
	GeneratedAt      time.Time                     `json:"generated_at"`
}

type queryBenchmark struct {
	Name            string  `json:"name"`
	Height          int32   `json:"height"`
	Limit           int     `json:"limit"`
	Results         int     `json:"results"`
	HasMore         bool    `json:"has_more,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
	RatePerSecond   float64 `json:"rate_per_second,omitempty"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	res, err := run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "obtc-expiryindex-shadow: %v\n", err)
		os.Exit(1)
	}
	body, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal result: %v\n", err)
		os.Exit(1)
	}
	body = append(body, '\n')
	if cfg.OutPath != "" {
		if err := os.WriteFile(filepath.Clean(cfg.OutPath), body, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "write result: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Print(string(body))
}

func parseConfig(args []string) (*config, error) {
	cfg := &config{
		DBType:        "ffldb",
		DBNet:         "mainnet",
		ForkHeight:    -1,
		BatchSize:     5000,
		ProgressEvery: 500000,
		ReapLimit:     1024,
		ListLimit:     10000,
	}
	fs := flag.NewFlagSet("obtc-expiryindex-shadow", flag.ContinueOnError)
	fs.StringVar(&cfg.SourceDBPath, "source-dbpath", "", "path to synced BTC blocks database directory")
	fs.StringVar(&cfg.IndexDBPath, "index-dbpath", "", "path for the independent shadow expiryindex database")
	fs.StringVar(&cfg.DBType, "dbtype", cfg.DBType, "database backend")
	fs.StringVar(&cfg.DBNet, "dbnet", cfg.DBNet, "database network: mainnet|testnet3|testnet4|regtest|simnet")
	fs.IntVar(&cfg.ForkHeight, "fork-height", cfg.ForkHeight, "BTC fork anchor height")
	fs.StringVar(&cfg.ForkHash, "fork-hash", "", "BTC fork anchor hash")
	fs.IntVar(&cfg.BatchSize, "batch-size", cfg.BatchSize, "index write batch size")
	fs.Int64Var(&cfg.ProgressEvery, "progress-every", cfg.ProgressEvery, "stderr progress interval in seen UTXOs; 0 disables")
	fs.BoolVar(&cfg.ResetIndex, "reset-index", false, "delete --index-dbpath before building")
	fs.StringVar(&cfg.OutPath, "out", "", "optional JSON result path")
	fs.IntVar(&cfg.ReapLimit, "reap-limit", cfg.ReapLimit, "REAP strict prefix benchmark limit")
	fs.IntVar(&cfg.ListLimit, "list-limit", cfg.ListLimit, "expiry range scan benchmark limit")
	fs.IntVar(&cfg.BenchmarkHeight, "benchmark-height", 0, "benchmark height; defaults to fork-height + 2016")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.SourceDBPath == "" {
		return nil, errors.New("--source-dbpath is required")
	}
	if cfg.IndexDBPath == "" {
		return nil, errors.New("--index-dbpath is required")
	}
	if cfg.ForkHeight < 0 {
		return nil, errors.New("--fork-height is required")
	}
	if strings.TrimSpace(cfg.ForkHash) == "" {
		return nil, errors.New("--fork-hash is required")
	}
	if cfg.BatchSize <= 0 {
		return nil, errors.New("--batch-size must be > 0")
	}
	if cfg.ReapLimit <= 0 || cfg.ListLimit <= 0 {
		return nil, errors.New("--reap-limit and --list-limit must be > 0")
	}
	return cfg, nil
}

func run(cfg *config) (*result, error) {
	dbNet, err := resolveDBNet(cfg.DBNet)
	if err != nil {
		return nil, err
	}

	sourceDB, err := database.Open(cfg.DBType, filepath.Clean(cfg.SourceDBPath), dbNet)
	if err != nil {
		return nil, fmt.Errorf("open source db: %w", err)
	}
	defer sourceDB.Close()

	expectedHash, err := chainhash.NewHashFromStr(strings.TrimSpace(cfg.ForkHash))
	if err != nil {
		return nil, fmt.Errorf("parse fork hash: %w", err)
	}
	actualHash, err := blockchain.HashByHeightFromDB(sourceDB, int32(cfg.ForkHeight))
	if err != nil {
		return nil, fmt.Errorf("read fork anchor hash: %w", err)
	}
	if !actualHash.IsEqual(expectedHash) {
		return nil, fmt.Errorf("fork anchor mismatch at height %d: local=%s expected=%s",
			cfg.ForkHeight, actualHash, expectedHash)
	}
	snapshot, err := blockchain.BestSnapshotFromDB(sourceDB)
	if err != nil {
		return nil, fmt.Errorf("read source best snapshot: %w", err)
	}

	indexPath := filepath.Clean(cfg.IndexDBPath)
	if cfg.ResetIndex {
		if err := os.RemoveAll(indexPath); err != nil {
			return nil, fmt.Errorf("reset index db: %w", err)
		}
	}
	if err := os.MkdirAll(indexPath, 0700); err != nil {
		return nil, err
	}
	indexDB, err := openOrCreateDB(cfg.DBType, indexPath, dbNet)
	if err != nil {
		return nil, fmt.Errorf("open index db: %w", err)
	}
	defer indexDB.Close()

	buildStats, err := expiryindex.BuildShadowIndexFromUTXO(indexDB, &chaincfg.ObtcMainNetParams,
		func(fn func(expiryindex.ShadowUTXO) error) error {
			return blockchain.ForEachUTXOInDB(sourceDB, func(entry blockchain.UTXOSnapshotEntry) error {
				return fn(expiryindex.ShadowUTXO{
					OutPoint:     entry.OutPoint,
					CreateHeight: entry.BlockHeight,
					Amount:       entry.Amount,
					IsCoinBase:   entry.IsCoinBase,
				})
			})
		},
		expiryindex.ShadowBuildOptions{
			ChainTipHeight:   snapshot.Height,
			ChainTipHash:     &snapshot.Hash,
			BatchSize:        cfg.BatchSize,
			ProgressInterval: cfg.ProgressEvery,
			OnProgress: func(p expiryindex.ShadowBuildProgress) {
				fmt.Fprintf(os.Stderr, "progress seen=%d indexed=%d elapsed=%s rate=%.0f/s\n",
					p.SeenUTXOs, p.IndexedUTXOs, p.Elapsed.Round(time.Second), p.RatePerSec)
			},
		})
	if err != nil {
		return nil, fmt.Errorf("build shadow index: %w", err)
	}

	idx, err := expiryindex.NewExpiryIndex(indexDB, &chaincfg.ObtcMainNetParams)
	if err != nil {
		return nil, err
	}
	indexStats, err := idx.GetStats()
	if err != nil {
		return nil, fmt.Errorf("get index stats: %w", err)
	}
	benchHeight := int32(cfg.BenchmarkHeight)
	if benchHeight == 0 {
		benchHeight = int32(cfg.ForkHeight) + 2016
	}
	benchmarks, err := runBenchmarks(idx, benchHeight, cfg.ReapLimit, cfg.ListLimit)
	if err != nil {
		return nil, err
	}
	size, err := dirSize(indexPath)
	if err != nil {
		return nil, err
	}

	return &result{
		ForkHeight:       int32(cfg.ForkHeight),
		ForkHash:         expectedHash.String(),
		SourceBestHeight: snapshot.Height,
		SourceBestHash:   snapshot.Hash.String(),
		Build:            buildStats,
		IndexStats:       indexStats,
		QueryBenchmarks:  benchmarks,
		IndexDBPath:      indexPath,
		IndexDBBytes:     size,
		GeneratedAt:      time.Now().UTC(),
	}, nil
}

func runBenchmarks(idx *expiryindex.ExpiryIndex, height int32, reapLimit, listLimit int) ([]queryBenchmark, error) {
	var results []queryBenchmark
	start := time.Now()
	reapCandidates, err := idx.ReapPrefixCandidates(height, reapLimit)
	if err != nil {
		return nil, fmt.Errorf("reap prefix benchmark: %w", err)
	}
	elapsed := time.Since(start)
	results = append(results, queryBenchmark{
		Name:            "reap_prefix_candidates",
		Height:          height,
		Limit:           reapLimit,
		Results:         len(reapCandidates),
		DurationSeconds: elapsed.Seconds(),
		RatePerSecond:   rate(len(reapCandidates), elapsed),
	})

	start = time.Now()
	expiring, hasMore, err := idx.ScanExpiringUTXOs(0, uint64(height), listLimit, nil)
	if err != nil {
		return nil, fmt.Errorf("scan expiring benchmark: %w", err)
	}
	elapsed = time.Since(start)
	results = append(results, queryBenchmark{
		Name:            "scan_expiring_utxos",
		Height:          height,
		Limit:           listLimit,
		Results:         len(expiring),
		HasMore:         hasMore,
		DurationSeconds: elapsed.Seconds(),
		RatePerSecond:   rate(len(expiring), elapsed),
	})
	return results, nil
}

func rate(n int, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(n) / d.Seconds()
}

func openOrCreateDB(dbType, path string, net wire.BitcoinNet) (database.DB, error) {
	db, err := database.Open(dbType, path, net)
	if err == nil {
		return db, nil
	}
	return database.Create(dbType, path, net)
}

func resolveDBNet(network string) (wire.BitcoinNet, error) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet", "main":
		return wire.MainNet, nil
	case "testnet3", "testnet":
		return wire.TestNet3, nil
	case "testnet4":
		return wire.TestNet4, nil
	case "regtest", "regression":
		return wire.TestNet, nil
	case "simnet":
		return wire.SimNet, nil
	default:
		return 0, fmt.Errorf("unsupported --dbnet %q", network)
	}
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}
