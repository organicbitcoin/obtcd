// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/wire"
)

func TestJSONLGzipRoundTripAndStableHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utxo.jsonl.gz")
	w, err := newJSONLGzipWriter(path)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	rows := []utxoRow{
		{TxID: "00", Vout: 0, AmountSat: 1000, CreateHeight: 1, ExpiryHeight: 145, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "01", Vout: 1, AmountSat: 2000, CreateHeight: 2, ExpiryHeight: 146, SnapshotHeight: 7, SnapshotHash: "abc"},
	}
	for _, row := range rows {
		if err := w.WriteJSON(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if w.SHA256() == "" || w.FileSHA256() == "" {
		t.Fatal("expected content and file hashes")
	}

	got, err := loadUTXORows(path)
	if err != nil {
		t.Fatalf("load rows: %v", err)
	}
	if len(got) != len(rows) || got[1].TxID != rows[1].TxID || got[1].AmountSat != rows[1].AmountSat {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, rows)
	}
}

func TestUpdateManifestForRowCountsIntegrityIssues(t *testing.T) {
	manifest := &exportManifest{}
	row1 := utxoRow{TxID: "aa", Vout: 0, AmountSat: 10, ExpiryHeight: 100}
	row2 := utxoRow{TxID: "aa", Vout: 0, AmountSat: 10, ExpiryHeight: 100}
	row3 := utxoRow{TxID: "00", Vout: 0, AmountSat: 0, ExpiryHeight: 99}

	updateManifestForRow(manifest, nil, row1, true)
	updateManifestForRow(manifest, &row1, row2, true)
	updateManifestForRow(manifest, &row2, row3, false)

	if manifest.RowCount != 3 {
		t.Fatalf("row count got %d", manifest.RowCount)
	}
	if manifest.DuplicateOutpointCount != 1 {
		t.Fatalf("duplicate count got %d", manifest.DuplicateOutpointCount)
	}
	if manifest.OrderViolationCount != 1 {
		t.Fatalf("order violation count got %d", manifest.OrderViolationCount)
	}
	if manifest.MissingAmountCount != 1 {
		t.Fatalf("missing amount count got %d", manifest.MissingAmountCount)
	}
	if manifest.FirstExpiryHeight != 99 || manifest.LastExpiryHeight != 100 {
		t.Fatalf("expiry range got %d..%d", manifest.FirstExpiryHeight, manifest.LastExpiryHeight)
	}
}

func TestParseConfigDirectDBRequiresAnchor(t *testing.T) {
	_, err := parseConfig([]string{
		"--source=btcd-db",
		"--dbpath=/tmp/blocks_ffldb",
		"--fork-height=100",
	})
	if err == nil {
		t.Fatal("expected missing fork hash error")
	}

	cfg, err := parseConfig([]string{
		"--source=direct-db",
		"--dbpath=/tmp/blocks_ffldb",
		"--fork-height=100",
		"--fork-hash=0000000000000000000000000000000000000000000000000000000000000001",
		"--no-preview",
	})
	if err != nil {
		t.Fatalf("parse direct db config: %v", err)
	}
	if cfg.Source != "btcd-db" || cfg.DBType != "ffldb" || cfg.DBNet != "mainnet" {
		t.Fatalf("unexpected direct config: %+v", cfg)
	}
}

func TestRowFromDirectUTXOCalculatesExpiryFields(t *testing.T) {
	hash := chainhash.Hash{0x01}
	row, include, immatureCoinbase, err := rowFromDirectUTXO(blockchain.UTXOSnapshotEntry{
		OutPoint:    wire.OutPoint{Hash: hash, Index: 2},
		BlockHeight: 25,
		Amount:      5000,
	}, 140, "snapshot", 144, 100, 0, 200)
	if err != nil {
		t.Fatalf("row from direct utxo: %v", err)
	}
	if !include {
		t.Fatal("expected row to be included")
	}
	if immatureCoinbase {
		t.Fatal("did not expect immature coinbase marker")
	}
	if row.CreateHeight != 25 || row.ExpiryHeight != 169 || row.BlocksToExpiry != 29 {
		t.Fatalf("unexpected expiry fields: %+v", row)
	}
	if row.TxID != hash.String() || row.Vout != 2 || row.AmountSat != 5000 {
		t.Fatalf("unexpected row identity: %+v", row)
	}
}

func TestRowFromDirectUTXOHonorsExpiryRange(t *testing.T) {
	hash := chainhash.Hash{0x02}
	_, include, immatureCoinbase, err := rowFromDirectUTXO(blockchain.UTXOSnapshotEntry{
		OutPoint:    wire.OutPoint{Hash: hash},
		BlockHeight: 25,
		Amount:      100,
	}, 140, "snapshot", 144, 100, 170, 200)
	if err != nil {
		t.Fatalf("row from direct utxo: %v", err)
	}
	if include {
		t.Fatal("expected row outside expiry range to be skipped")
	}
	if immatureCoinbase {
		t.Fatal("did not expect immature coinbase marker")
	}
}

func TestRowFromDirectUTXOSkipsImmatureCoinbase(t *testing.T) {
	hash := chainhash.Hash{0x03}
	_, include, immatureCoinbase, err := rowFromDirectUTXO(blockchain.UTXOSnapshotEntry{
		OutPoint:    wire.OutPoint{Hash: hash},
		BlockHeight: 95,
		Amount:      50_0000_0000,
		IsCoinBase:  true,
	}, 140, "snapshot", 144, 100, 0, 300)
	if err != nil {
		t.Fatalf("row from direct utxo: %v", err)
	}
	if include {
		t.Fatal("expected immature coinbase to be skipped")
	}
	if !immatureCoinbase {
		t.Fatal("expected immature coinbase marker")
	}
}

func TestResolveDirectEndHeightRejectsInvertedRange(t *testing.T) {
	_, err := resolveDirectEndHeight(&config{
		StartHeight:  200,
		EndHeight:    199,
		EndHeightSet: true,
	}, 100, 144)
	if err == nil {
		t.Fatal("expected inverted range error")
	}
}

func TestSelectPreviewInputsAppliesDustTaxAndCaps(t *testing.T) {
	p := reap.REAPParams{
		MaxInputs:        2,
		DustMaxInputs:    0,
		TaxNum:           30,
		TaxDen:           100,
		DustThresholdSat: 720,
	}
	backlog := []utxoRow{
		{TxID: "00", Vout: 0, AmountSat: 100, ExpiryHeight: 10},
		{TxID: "01", Vout: 0, AmountSat: 1000, ExpiryHeight: 10},
		{TxID: "02", Vout: 0, AmountSat: 2000, ExpiryHeight: 10},
	}

	detail, picked := selectPreviewInputs(10, backlog, p)
	if picked != 2 || detail.SelectedInputCount != 2 {
		t.Fatalf("picked got %d detail=%d", picked, detail.SelectedInputCount)
	}
	if detail.DustTaxSat != 100 || detail.NormalTaxSat != 300 || detail.TaxTotalSat != 400 {
		t.Fatalf("unexpected tax split: %+v", detail)
	}
	if detail.RefundTotalSat != 700 {
		t.Fatalf("refund total got %d", detail.RefundTotalSat)
	}
}

func TestBuildPreviewSortsAndCarriesBacklog(t *testing.T) {
	rows := []utxoRow{
		{TxID: "02", Vout: 0, AmountSat: 2000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "01", Vout: 0, AmountSat: 1000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "00", Vout: 0, AmountSat: 100, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
	}
	var details []reapPreviewDetail
	summary, err := buildPreview(rows, "obtcmainnet", &chaincfg.ObtcMainNetParams, 7, "abc", func(d reapPreviewDetail) error {
		details = append(details, d)
		return nil
	})
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if summary.SelectedInputs != 3 || summary.TaxTotalSat != 1000 || summary.RefundTotalSat != 2100 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(details) != 1 {
		t.Fatalf("details count got %d", len(details))
	}
	if details[0].SelectedInputs[0].TxID != "00" {
		t.Fatalf("expected strict amount ordering, got %+v", details[0].SelectedInputs)
	}
	if len(summary.ByDay) != 1 || len(summary.ByWeek) != 1 {
		t.Fatalf("expected day/week buckets: %+v %+v", summary.ByDay, summary.ByWeek)
	}
}

func TestAggregatePreviewMatchesInMemoryPreview(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "utxo.jsonl.gz")
	w, err := newJSONLGzipWriter(input)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	rows := []utxoRow{
		{TxID: "04", Vout: 0, AmountSat: 5000, ExpiryHeight: 11, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "02", Vout: 0, AmountSat: 2000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "00", Vout: 0, AmountSat: 100, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "03", Vout: 0, AmountSat: 3000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "01", Vout: 0, AmountSat: 1000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
	}
	for _, row := range rows {
		if err := w.WriteJSON(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}

	var details []reapPreviewDetail
	inMemory, err := buildPreview(rows, "obtcmainnet", &chaincfg.ObtcMainNetParams, 7, "abc", func(d reapPreviewDetail) error {
		details = append(details, d)
		return nil
	})
	if err != nil {
		t.Fatalf("build in-memory preview: %v", err)
	}

	blocksPath := filepath.Join(dir, "blocks.jsonl.gz")
	summaryPath := filepath.Join(dir, "summary.json")
	aggregate, err := writeAggregatePreviewFiles(input, "obtcmainnet", &chaincfg.ObtcMainNetParams, 7, "abc",
		blocksPath, summaryPath, aggregatePreviewOptions{WorkDir: dir, ShardSpan: 1})
	if err != nil {
		t.Fatalf("write aggregate preview: %v", err)
	}
	if aggregate.UTXORowCount != int64(len(rows)) {
		t.Fatalf("row count got %d", aggregate.UTXORowCount)
	}
	if aggregate.SelectedInputs != int64(inMemory.SelectedInputs) ||
		aggregate.TaxTotalSat != inMemory.TaxTotalSat ||
		aggregate.RefundTotalSat != inMemory.RefundTotalSat ||
		aggregate.MaxRemainingBacklog != int64(inMemory.MaxRemainingBacklog) {
		t.Fatalf("aggregate mismatch got %+v want %+v", aggregate, inMemory)
	}
	if aggregate.BlocksSHA256 == "" || aggregate.BlocksFileSHA256 == "" {
		t.Fatal("expected aggregate block hashes")
	}
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("summary missing: %v", err)
	}
}

func TestAggregatePreviewHonorsReapStartHeight(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "utxo.jsonl.gz")
	w, err := newJSONLGzipWriter(input)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	rows := []utxoRow{
		{TxID: "00", Vout: 0, AmountSat: 1000, ExpiryHeight: 10, SnapshotHeight: 7, SnapshotHash: "abc"},
		{TxID: "01", Vout: 0, AmountSat: 2000, ExpiryHeight: 11, SnapshotHeight: 7, SnapshotHash: "abc"},
	}
	for _, row := range rows {
		if err := w.WriteJSON(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}

	blocksPath := filepath.Join(dir, "blocks.jsonl.gz")
	summaryPath := filepath.Join(dir, "summary.json")
	summary, err := writeAggregatePreviewFiles(input, "obtcmainnet", &chaincfg.ObtcMainNetParams, 7, "abc",
		blocksPath, summaryPath, aggregatePreviewOptions{WorkDir: dir, ShardSpan: 1, ReapStartHeight: 20})
	if err != nil {
		t.Fatalf("write aggregate preview: %v", err)
	}
	if summary.FirstReapHeight != 20 || summary.ReapStartHeight != 20 {
		t.Fatalf("expected first/start height 20, got first=%d start=%d",
			summary.FirstReapHeight, summary.ReapStartHeight)
	}
	gotBlocks := loadAggregateBlocks(t, blocksPath)
	if len(gotBlocks) == 0 || gotBlocks[0].ReapHeight != 20 {
		t.Fatalf("unexpected blocks: %+v", gotBlocks)
	}
	if gotBlocks[0].ExpiredInputs != 2 || gotBlocks[0].ExpiredAmountSat != 3000 {
		t.Fatalf("expected historical expiries to be counted at start: %+v", gotBlocks[0])
	}
}

func loadAggregateBlocks(t *testing.T, path string) []reapAggregateBlock {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open aggregate blocks: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()
	dec := json.NewDecoder(gz)
	var blocks []reapAggregateBlock
	for {
		var block reapAggregateBlock
		if err := dec.Decode(&block); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode block: %v", err)
		}
		blocks = append(blocks, block)
	}
	return blocks
}
