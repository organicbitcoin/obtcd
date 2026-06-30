// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func exportBTCDDB(cfg *config, params *chaincfg.Params) (string, *exportManifest, error) {
	expiryParams := chaincfg.GetExpiryParams(params)
	if expiryParams == nil || expiryParams.WindowBlocks == 0 {
		return "", nil, fmt.Errorf("network %q does not define OBTC expiry params", cfg.Network)
	}

	dbNet, err := resolveDBNet(cfg.DBNet)
	if err != nil {
		return "", nil, err
	}
	dbPath := filepath.Clean(cfg.DBPath)
	db, err := database.Open(cfg.DBType, dbPath, dbNet)
	if err != nil {
		return "", nil, fmt.Errorf("open %s database %s: %w", cfg.DBType, dbPath, err)
	}
	defer db.Close()

	expectedAnchor, err := chainhash.NewHashFromStr(strings.TrimSpace(cfg.ForkHash))
	if err != nil {
		return "", nil, fmt.Errorf("parse --fork-hash: %w", err)
	}
	forkHeight := int32(cfg.ForkHeight)
	if err := verifyForkAnchor(db, forkHeight, expectedAnchor); err != nil {
		return "", nil, err
	}

	snapshot, err := blockchain.BestSnapshotFromDB(db)
	if err != nil {
		return "", nil, fmt.Errorf("read best chain snapshot: %w", err)
	}
	if snapshot.Height != forkHeight {
		return "", nil, fmt.Errorf("direct DB export requires the best UTXO snapshot to equal fork anchor height: best=%d fork=%d",
			snapshot.Height, forkHeight)
	}
	if !snapshot.Hash.IsEqual(expectedAnchor) {
		return "", nil, fmt.Errorf("best chain hash %s does not match fork anchor %s",
			snapshot.Hash, expectedAnchor)
	}

	utxoStateHash := ""
	utxoStateConsistent := false
	if snapshot.UTXOStateHash != nil {
		utxoStateHash = snapshot.UTXOStateHash.String()
		utxoStateConsistent = snapshot.UTXOStateHash.IsEqual(&snapshot.Hash)
	}
	if !utxoStateConsistent && !cfg.AllowStaleUTXO {
		return "", nil, fmt.Errorf("on-disk UTXO state is not confirmed at best hash %s (utxo_state_hash=%q); stop the node cleanly or rerun with --allow-stale-utxo",
			snapshot.Hash, utxoStateHash)
	}

	endHeight, err := resolveDirectEndHeight(cfg, snapshot.Height, expiryParams.WindowBlocks)
	if err != nil {
		return "", nil, err
	}

	startedAt := time.Now().UTC()
	snapshotHash := snapshot.Hash.String()
	utxoPath := utxoPathFor(cfg.OutDir, snapshot.Height, snapshotHash)
	writer, err := newJSONLGzipWriter(utxoPath)
	if err != nil {
		return "", nil, err
	}

	manifest := &exportManifest{
		Network:             cfg.Network,
		Source:              "btcd-db",
		SnapshotHeight:      snapshot.Height,
		SnapshotHash:        snapshotHash,
		SnapshotStable:      true,
		FinalHeight:         snapshot.Height,
		FinalHash:           snapshotHash,
		ForkHeight:          forkHeight,
		ForkHash:            expectedAnchor.String(),
		DBPath:              dbPath,
		UTXOStateHash:       utxoStateHash,
		UTXOStateConsistent: utxoStateConsistent,
		StatsTotalUTXOs:     -1,
		StartHeight:         int32(cfg.StartHeight),
		EndHeight:           int32(endHeight),
		PageSize:            0,
		ExportStartedAt:     startedAt,
		OutputFile:          utxoPath,
	}

	err = blockchain.ForEachUTXOInDB(db, func(entry blockchain.UTXOSnapshotEntry) error {
		row, include, immatureCoinbase, err := rowFromDirectUTXO(entry, snapshot.Height, snapshotHash,
			expiryParams.WindowBlocks, int32(params.CoinbaseMaturity), cfg.StartHeight, endHeight)
		if err != nil || !include {
			if immatureCoinbase {
				manifest.SkippedImmatureCoinbaseCount++
			}
			return err
		}
		updateManifestForRow(manifest, nil, row, true)
		if err := writer.WriteJSON(row); err != nil {
			return err
		}
		return nil
	})
	closeErr := writer.Close()
	if err != nil {
		return "", nil, err
	}
	if closeErr != nil {
		return "", nil, closeErr
	}

	finishedAt := time.Now().UTC()
	manifest.ExportFinishedAt = finishedAt
	manifest.DurationSeconds = finishedAt.Sub(startedAt).Seconds()
	manifest.SHA256 = writer.SHA256()
	manifest.FileSHA256 = writer.FileSHA256()
	return utxoPath, manifest, nil
}

func verifyForkAnchor(db database.DB, height int32, expected *chainhash.Hash) error {
	if expected == nil {
		return errors.New("missing expected fork anchor hash")
	}
	actual, err := blockchain.HashByHeightFromDB(db, height)
	if err != nil {
		return fmt.Errorf("read fork anchor height %d: %w", height, err)
	}
	if !actual.IsEqual(expected) {
		return fmt.Errorf("fork anchor mismatch at height %d: local=%s expected=%s",
			height, actual, expected)
	}
	return nil
}

func rowFromDirectUTXO(entry blockchain.UTXOSnapshotEntry, snapshotHeight int32, snapshotHash string,
	windowBlocks uint64, coinbaseMaturity int32, startHeight, endHeight int) (utxoRow, bool, bool, error) {

	if entry.BlockHeight < 0 {
		return utxoRow{}, false, false, fmt.Errorf("negative UTXO create height for %s: %d",
			entry.OutPoint, entry.BlockHeight)
	}
	if entry.IsCoinBase && snapshotHeight-entry.BlockHeight < coinbaseMaturity {
		return utxoRow{}, false, true, nil
	}
	createHeight := uint64(entry.BlockHeight)
	expiryHeight := createHeight + windowBlocks
	if expiryHeight < uint64(startHeight) || expiryHeight > uint64(endHeight) {
		return utxoRow{}, false, false, nil
	}

	scriptHash := sha256.Sum256(entry.PkScript)

	return utxoRow{
		TxID:                  entry.OutPoint.Hash.String(),
		Vout:                  entry.OutPoint.Index,
		Outpoint:              entry.OutPoint.String(),
		AmountSat:             entry.Amount,
		CreateHeight:          createHeight,
		ExpiryHeight:          expiryHeight,
		BlocksToExpiry:        int64(expiryHeight) - int64(snapshotHeight),
		SnapshotHeight:        snapshotHeight,
		SnapshotHash:          snapshotHash,
		IsCoinbase:            entry.IsCoinBase,
		ScriptType:            txscript.GetScriptClass(entry.PkScript).String(),
		ScriptPubKeyLength:    len(entry.PkScript),
		ScriptPubKeySHA256Hex: hex.EncodeToString(scriptHash[:]),
	}, true, false, nil
}

func resolveDirectEndHeight(cfg *config, snapshotHeight int32, windowBlocks uint64) (int, error) {
	if cfg.EndHeightSet {
		if cfg.EndHeight < cfg.StartHeight {
			return 0, fmt.Errorf("end height %d is below start height %d",
				cfg.EndHeight, cfg.StartHeight)
		}
		return cfg.EndHeight, nil
	}
	end := int64(snapshotHeight) + int64(windowBlocks)
	if end > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("computed end height %d overflows int", end)
	}
	return int(end), nil
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
