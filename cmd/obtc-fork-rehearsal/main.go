// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

type config struct {
	DBType      string
	DBPath      string
	DBNet       string
	ForkHeight  int
	ForkHash    string
	Blocks      int
	BTCNextBits uint
	OutPath     string
}

type result struct {
	ForkHeight               int32     `json:"fork_height"`
	ForkHash                 string    `json:"fork_hash"`
	AnchorBits               uint32    `json:"anchor_bits"`
	AnchorTime               time.Time `json:"anchor_time"`
	FirstOBTCHeight          int32     `json:"first_obtc_height"`
	FirstOBTCBits            uint32    `json:"first_obtc_bits"`
	BTCNextBitsChecked       uint32    `json:"btc_next_bits_checked"`
	BTCNextRejected          bool      `json:"btc_next_rejected"`
	BTCNextRejectError       string    `json:"btc_next_reject_error,omitempty"`
	SimulatedBlocks          int       `json:"simulated_blocks"`
	SimulationTipHeight      int32     `json:"simulation_tip_height"`
	SimulationTipBits        uint32    `json:"simulation_tip_bits"`
	ForkDAABootstrapEnd      int32     `json:"fork_daa_bootstrap_end"`
	ForkDAABootstrapHalfLife string    `json:"fork_daa_bootstrap_half_life"`
	ForkDAANormalHalfLife    string    `json:"fork_daa_normal_half_life"`
	GeneratedAt              time.Time `json:"generated_at"`
}

type rehearsalChain struct {
	params *chaincfg.Params
}

func (c rehearsalChain) ChainParams() *chaincfg.Params {
	return c.params
}

func (c rehearsalChain) BlocksPerRetarget() int32 {
	return int32(c.params.TargetTimespan / c.params.TargetTimePerBlock)
}

func (c rehearsalChain) MinRetargetTimespan() int64 {
	targetTimespan := int64(c.params.TargetTimespan / time.Second)
	return targetTimespan / c.params.RetargetAdjustmentFactor
}

func (c rehearsalChain) MaxRetargetTimespan() int64 {
	targetTimespan := int64(c.params.TargetTimespan / time.Second)
	return targetTimespan * c.params.RetargetAdjustmentFactor
}

func (c rehearsalChain) VerifyCheckpoint(height int32, hash *chainhash.Hash) bool {
	return true
}

func (c rehearsalChain) FindPreviousCheckpoint() (blockchain.HeaderCtx, error) {
	return nil, nil
}

type rehearsalNode struct {
	height    int32
	bits      uint32
	timestamp int64
	parent    *rehearsalNode
}

func (n *rehearsalNode) Height() int32 {
	return n.height
}

func (n *rehearsalNode) Bits() uint32 {
	return n.bits
}

func (n *rehearsalNode) Timestamp() int64 {
	return n.timestamp
}

func (n *rehearsalNode) Parent() blockchain.HeaderCtx {
	if n == nil || n.parent == nil {
		return nil
	}
	return n.parent
}

func (n *rehearsalNode) RelativeAncestorCtx(distance int32) blockchain.HeaderCtx {
	node := n
	for i := int32(0); node != nil && i < distance; i++ {
		node = node.parent
	}
	if node == nil {
		return nil
	}
	return node
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	res, err := run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "obtc-fork-rehearsal: %v\n", err)
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
		DBType:     "ffldb",
		DBNet:      "mainnet",
		ForkHeight: -1,
		Blocks:     32,
	}
	fs := flag.NewFlagSet("obtc-fork-rehearsal", flag.ContinueOnError)
	fs.StringVar(&cfg.DBType, "dbtype", cfg.DBType, "database backend")
	fs.StringVar(&cfg.DBPath, "dbpath", "", "path to blocks database directory")
	fs.StringVar(&cfg.DBNet, "dbnet", cfg.DBNet, "database network: mainnet|testnet3|testnet4|regtest|simnet")
	fs.IntVar(&cfg.ForkHeight, "fork-height", cfg.ForkHeight, "BTC fork anchor height")
	fs.StringVar(&cfg.ForkHash, "fork-hash", "", "BTC fork anchor hash")
	fs.IntVar(&cfg.Blocks, "blocks", cfg.Blocks, "number of post-fork OBTC headers to simulate")
	fs.UintVar(&cfg.BTCNextBits, "btc-next-bits", 0, "optional BTC H+1 bits to verify rejection; defaults to anchor bits")
	fs.StringVar(&cfg.OutPath, "out", "", "optional JSON result path")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.DBPath == "" {
		return nil, errors.New("--dbpath is required")
	}
	if cfg.ForkHeight < 0 {
		return nil, errors.New("--fork-height is required")
	}
	if strings.TrimSpace(cfg.ForkHash) == "" {
		return nil, errors.New("--fork-hash is required")
	}
	if cfg.Blocks < 1 {
		return nil, errors.New("--blocks must be >= 1")
	}
	if cfg.BTCNextBits > uint(^uint32(0)) {
		return nil, errors.New("--btc-next-bits exceeds uint32")
	}
	return cfg, nil
}

func run(cfg *config) (*result, error) {
	dbNet, err := resolveDBNet(cfg.DBNet)
	if err != nil {
		return nil, err
	}
	db, err := database.Open(cfg.DBType, filepath.Clean(cfg.DBPath), dbNet)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	expectedHash, err := chainhash.NewHashFromStr(strings.TrimSpace(cfg.ForkHash))
	if err != nil {
		return nil, fmt.Errorf("parse fork hash: %w", err)
	}
	actualHash, err := blockchain.HashByHeightFromDB(db, int32(cfg.ForkHeight))
	if err != nil {
		return nil, fmt.Errorf("read fork anchor hash: %w", err)
	}
	if !actualHash.IsEqual(expectedHash) {
		return nil, fmt.Errorf("fork anchor mismatch at height %d: local=%s expected=%s",
			cfg.ForkHeight, actualHash, expectedHash)
	}
	anchorHeader, err := blockchain.HeaderByHeightFromDB(db, int32(cfg.ForkHeight))
	if err != nil {
		return nil, fmt.Errorf("read fork anchor header: %w", err)
	}

	params := rehearsalParams(int32(cfg.ForkHeight))
	chain := rehearsalChain{params: params}
	anchorNode := &rehearsalNode{
		height:    int32(cfg.ForkHeight),
		bits:      anchorHeader.Bits,
		timestamp: anchorHeader.Timestamp.Unix(),
	}

	last := anchorNode
	for i := 1; i <= cfg.Blocks; i++ {
		nextTime := anchorHeader.Timestamp.Add(time.Duration(i) * params.TargetTimePerBlock)
		bits, err := blockchain.CalcNextRequiredDifficultyForHeader(last, nextTime, chain)
		if err != nil {
			return nil, fmt.Errorf("calculate difficulty at height %d: %w", last.height+1, err)
		}
		header := &wire.BlockHeader{
			Version:   4,
			Bits:      bits,
			Timestamp: nextTime,
		}
		if err := blockchain.CheckBlockHeaderContext(header, last, 0, chain, true); err != nil {
			return nil, fmt.Errorf("validate OBTC header at height %d: %w", last.height+1, err)
		}
		last = &rehearsalNode{
			height:    last.height + 1,
			bits:      bits,
			timestamp: nextTime.Unix(),
			parent:    last,
		}
	}

	btcNextBits := uint32(cfg.BTCNextBits)
	if btcNextBits == 0 {
		btcNextBits = anchorHeader.Bits
	}
	badHeader := &wire.BlockHeader{
		Version:   4,
		Bits:      btcNextBits,
		Timestamp: anchorHeader.Timestamp.Add(params.TargetTimePerBlock),
	}
	rejectErr := blockchain.CheckBlockHeaderContext(badHeader, anchorNode, 0, chain, true)
	if rejectErr == nil {
		return nil, fmt.Errorf("BTC H+1 bits %08x were not rejected by fork DAA", btcNextBits)
	}

	return &result{
		ForkHeight:               int32(cfg.ForkHeight),
		ForkHash:                 expectedHash.String(),
		AnchorBits:               anchorHeader.Bits,
		AnchorTime:               anchorHeader.Timestamp,
		FirstOBTCHeight:          int32(cfg.ForkHeight) + 1,
		FirstOBTCBits:            params.ForkDAAForkResetBits,
		BTCNextBitsChecked:       btcNextBits,
		BTCNextRejected:          true,
		BTCNextRejectError:       rejectErr.Error(),
		SimulatedBlocks:          cfg.Blocks,
		SimulationTipHeight:      last.height,
		SimulationTipBits:        last.bits,
		ForkDAABootstrapEnd:      params.ForkDAABootstrapEndHeight,
		ForkDAABootstrapHalfLife: params.ForkDAABootstrapHalfLife.String(),
		ForkDAANormalHalfLife:    params.ForkDAANormalHalfLife.String(),
		GeneratedAt:              time.Now().UTC(),
	}, nil
}

func rehearsalParams(forkHeight int32) *chaincfg.Params {
	params := chaincfg.ObtcMainNetParams
	params.PowLimit = new(big.Int).Set(chaincfg.ObtcMainNetParams.PowLimit)
	params.ForkDAAStartHeight = forkHeight + 1
	params.ForkDAABootstrapEndHeight = forkHeight + 2016
	params.ForkDAAForkResetBits = params.PowLimitBits
	params.ForkDAABootstrapHalfLife = time.Hour
	params.ForkDAANormalHalfLife = 48 * time.Hour
	return &params
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
