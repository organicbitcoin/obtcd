// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildShadowIndexFromUTXO(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	tipHash := chainhash.Hash{0x01}
	source := func(fn func(ShadowUTXO) error) error {
		for _, utxo := range []ShadowUTXO{
			{OutPoint: wire.OutPoint{Hash: chainhash.Hash{0x02}, Index: 0}, CreateHeight: 0, Amount: 1000},
			{OutPoint: wire.OutPoint{Hash: chainhash.Hash{0x03}, Index: 1}, CreateHeight: 10, Amount: 2000},
			{OutPoint: wire.OutPoint{Hash: chainhash.Hash{0x04}, Index: 2}, CreateHeight: 11, Amount: 3000},
		} {
			if err := fn(utxo); err != nil {
				return err
			}
		}
		return nil
	}

	stats, err := BuildShadowIndexFromUTXO(db, &chaincfg.ObtcMainNetParams, source, ShadowBuildOptions{
		ChainTipHeight: 20,
		ChainTipHash:   &tipHash,
		BatchSize:      1,
	})
	if err != nil {
		t.Fatalf("build shadow index: %v", err)
	}
	if stats.SeenUTXOs != 3 || stats.IndexedUTXOs != 2 || stats.SkippedGenesisCreated != 1 {
		t.Fatalf("unexpected build stats: %+v", stats)
	}

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcMainNetParams)
	if err != nil {
		t.Fatalf("new expiry index: %v", err)
	}
	got, err := idx.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if got.TipHeight != 20 || got.TotalUTXOs != 2 {
		t.Fatalf("unexpected index stats: %+v", got)
	}
}
