// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

// ShadowUTXO is the minimal UTXO data needed to build an expiry index outside
// a live node.
type ShadowUTXO struct {
	OutPoint     wire.OutPoint
	CreateHeight int32
	Amount       int64
	IsCoinBase   bool
}

// ShadowBuildOptions controls a private shadow expiry index build.
type ShadowBuildOptions struct {
	ChainTipHeight       int32
	ChainTipHash         *chainhash.Hash
	BatchSize            int
	ProgressInterval     int64
	SkipImmatureCoinbase bool
	CoinbaseMaturity     int32
	OnProgress           func(ShadowBuildProgress)
}

// ShadowBuildProgress reports periodic build progress.
type ShadowBuildProgress struct {
	SeenUTXOs    int64         `json:"seen_utxos"`
	IndexedUTXOs int64         `json:"indexed_utxos"`
	Elapsed      time.Duration `json:"elapsed"`
	RatePerSec   float64       `json:"rate_per_sec"`
}

// ShadowBuildStats summarizes a private shadow expiry index build.
type ShadowBuildStats struct {
	ChainTipHeight          int32     `json:"chain_tip_height"`
	ChainTipHash            string    `json:"chain_tip_hash"`
	SeenUTXOs               int64     `json:"seen_utxos"`
	IndexedUTXOs            int64     `json:"indexed_utxos"`
	SkippedGenesisCreated   int64     `json:"skipped_genesis_created"`
	SkippedPreStart         int64     `json:"skipped_pre_start"`
	SkippedImmatureCoinbase int64     `json:"skipped_immature_coinbase"`
	BatchSize               int       `json:"batch_size"`
	StartedAt               time.Time `json:"started_at"`
	FinishedAt              time.Time `json:"finished_at"`
	DurationSeconds         float64   `json:"duration_seconds"`
	RatePerSec              float64   `json:"rate_per_sec"`
}

// BuildShadowIndexFromUTXO rebuilds the expiry index buckets in db from a UTXO
// source callback. It uses the production expiry index key schema, but is meant
// for private rehearsal databases rather than mutating a synced BTC block DB.
func BuildShadowIndexFromUTXO(db database.DB, params *chaincfg.Params,
	source func(func(ShadowUTXO) error) error, opts ShadowBuildOptions) (*ShadowBuildStats, error) {

	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if params == nil {
		return nil, fmt.Errorf("chain params is nil")
	}
	expiryParams := GetExpiryParams(params)
	if expiryParams == nil {
		return nil, fmt.Errorf("expiry index not supported for network %s", params.Name)
	}
	if source == nil {
		return nil, fmt.Errorf("UTXO source is nil")
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	startedAt := time.Now().UTC()
	stats := &ShadowBuildStats{
		ChainTipHeight: opts.ChainTipHeight,
		BatchSize:      batchSize,
		StartedAt:      startedAt,
	}
	if opts.ChainTipHash != nil {
		stats.ChainTipHash = opts.ChainTipHash.String()
	}

	err := db.Update(func(dbTx database.Tx) error {
		if err := createExpiryIndexBuckets(dbTx); err != nil {
			return err
		}
		return dbPutIndexVersion(dbTx, CurrentIndexVersion)
	})
	if err != nil {
		return nil, err
	}
	if err := clearExpiryIndexBucketsBatched(db, nil); err != nil {
		return nil, err
	}

	mh := NewMuHash()
	type batchEntry struct {
		outpoint  wire.OutPoint
		expiryKey uint64
		amount    int64
	}
	batch := make([]batchEntry, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := db.Update(func(dbTx database.Tx) error {
			for i := range batch {
				entry := batch[i]
				if err := putTxOutMapping(dbTx, &entry.outpoint, entry.expiryKey,
					entry.amount); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	process := func(utxo ShadowUTXO) error {
		stats.SeenUTXOs++
		if utxo.CreateHeight <= 0 {
			stats.SkippedGenesisCreated++
			return nil
		}
		if utxo.CreateHeight < expiryParams.StartScanHeight {
			stats.SkippedPreStart++
			return nil
		}
		if opts.SkipImmatureCoinbase && utxo.IsCoinBase &&
			opts.ChainTipHeight-utxo.CreateHeight < opts.CoinbaseMaturity {

			stats.SkippedImmatureCoinbase++
			return nil
		}

		expiryKey := expiryParams.CalculateExpiryKey(utxo.CreateHeight)
		mh.Add(computeEntryData(&utxo.OutPoint, expiryKey))
		batch = append(batch, batchEntry{
			outpoint:  utxo.OutPoint,
			expiryKey: expiryKey,
			amount:    utxo.Amount,
		})
		stats.IndexedUTXOs++
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
		if opts.OnProgress != nil && opts.ProgressInterval > 0 &&
			stats.SeenUTXOs%opts.ProgressInterval == 0 {

			elapsed := time.Since(startedAt)
			opts.OnProgress(ShadowBuildProgress{
				SeenUTXOs:    stats.SeenUTXOs,
				IndexedUTXOs: stats.IndexedUTXOs,
				Elapsed:      elapsed,
				RatePerSec:   float64(stats.SeenUTXOs) / elapsed.Seconds(),
			})
		}
		return nil
	}

	if err := source(process); err != nil {
		_ = resetShadowIndex(db)
		return nil, err
	}
	if err := flush(); err != nil {
		_ = resetShadowIndex(db)
		return nil, err
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := dbPutAccumulatorState(dbTx, mh); err != nil {
			return err
		}
		hash := opts.ChainTipHash
		if hash == nil {
			hash = &chainhash.Hash{}
		}
		if err := dbPutAccumulatorTipHash(dbTx, hash); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, opts.ChainTipHeight); err != nil {
			return err
		}
		return dbPutIndexVersion(dbTx, CurrentIndexVersion)
	})
	if err != nil {
		_ = resetShadowIndex(db)
		return nil, err
	}

	finishedAt := time.Now().UTC()
	stats.FinishedAt = finishedAt
	stats.DurationSeconds = finishedAt.Sub(startedAt).Seconds()
	if stats.DurationSeconds > 0 {
		stats.RatePerSec = float64(stats.SeenUTXOs) / stats.DurationSeconds
	}
	return stats, nil
}

func resetShadowIndex(db database.DB) error {
	if err := db.Update(func(dbTx database.Tx) error {
		if err := createExpiryIndexBuckets(dbTx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return clearExpiryIndexBucketsBatched(db, nil)
}
