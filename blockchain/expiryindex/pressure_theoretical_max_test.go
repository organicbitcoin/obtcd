// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func TestSingleExpiryKeyTheoreticalMaxPressure(t *testing.T) {
	const blockHeight = 150
	const scanPageSize = 1024

	outputsText := os.Getenv("OBTC_EXPIRY_PRESSURE_OUTPUTS")
	if outputsText == "" {
		t.Skip("set OBTC_EXPIRY_PRESSURE_OUTPUTS to run the manual pressure test")
	}

	theoreticalMaxOutputs, err := strconv.Atoi(outputsText)
	if err != nil || theoreticalMaxOutputs <= 0 {
		t.Fatalf("invalid OBTC_EXPIRY_PRESSURE_OUTPUTS=%q", outputsText)
	}

	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if err := idx.Init(); err != nil {
		t.Fatalf("init index: %v", err)
	}

	identityRoot := NewMuHash().Digest()
	block := createPressureBlockWithSpendableOutputs(t, blockHeight, theoreticalMaxOutputs, &identityRoot)
	expiryKey := idx.expiryParams.CalculateExpiryKey(blockHeight)

	t.Logf("pressure case: connect one block with %d spendable outputs under expiry_key=%d",
		theoreticalMaxOutputs, expiryKey)

	connectStart := time.Now()
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("connect block: %v", err)
	}
	connectElapsed := time.Since(connectStart)
	t.Logf("connect completed in %s", connectElapsed)

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("get stats after connect: %v", err)
	}
	if stats.TotalUTXOs != theoreticalMaxOutputs {
		t.Fatalf("unexpected utxo count after connect: got %d want %d",
			stats.TotalUTXOs, theoreticalMaxOutputs)
	}
	if stats.TotalExpiryKeys != 1 {
		t.Fatalf("unexpected expiry key count after connect: got %d want 1", stats.TotalExpiryKeys)
	}

	var (
		storedCount  int
		storedValues int
	)
	if err := db.View(func(dbTx database.Tx) error {
		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket == nil {
			t.Fatal("expiry-to-outpoints bucket missing")
		}

		cursor := expiryBucket.Cursor()
		prefix := expiryCompositePrefix(expiryKey)
		for ok := cursor.Seek(prefix); ok; ok = cursor.Next() {
			key := cursor.Key()
			if key == nil {
				break
			}

			rowExpiryKey, _, err := decodeExpiryOutpointCompositeKey(key)
			if err != nil {
				return err
			}
			if rowExpiryKey != expiryKey {
				break
			}

			storedCount++
			storedValues += len(cursor.Value())
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect bucket: %v", err)
	}
	if storedCount != theoreticalMaxOutputs {
		t.Fatalf("unexpected stored outpoint count: got %d want %d",
			storedCount, theoreticalMaxOutputs)
	}

	scanStart := time.Now()
	rows, err := scanAllExpiringPages(idx, expiryKey, scanPageSize)
	if err != nil {
		t.Fatalf("scan all pages: %v", err)
	}
	scanElapsed := time.Since(scanStart)
	t.Logf("scan completed in %s", scanElapsed)
	if len(rows) != theoreticalMaxOutputs {
		t.Fatalf("unexpected scan result count: got %d want %d",
			len(rows), theoreticalMaxOutputs)
	}

	expectedHash := *block.Transactions()[0].Hash()
	for i, row := range rows {
		if row.ExpiryKey != expiryKey {
			t.Fatalf("row[%d] expiry key mismatch: got %d want %d", i, row.ExpiryKey, expiryKey)
		}
		if row.OutPoint.Hash != expectedHash {
			t.Fatalf("row[%d] txid mismatch: got %s want %s",
				i, row.OutPoint.Hash, expectedHash)
		}
		if row.OutPoint.Index != uint32(i) {
			t.Fatalf("row[%d] vout mismatch: got %d want %d",
				i, row.OutPoint.Index, i)
		}
	}

	disconnectStart := time.Now()
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("disconnect block: %v", err)
	}
	disconnectElapsed := time.Since(disconnectStart)

	finalStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("get stats after disconnect: %v", err)
	}
	if finalStats.TotalUTXOs != 0 {
		t.Fatalf("unexpected utxo count after disconnect: got %d want 0", finalStats.TotalUTXOs)
	}
	if finalStats.TotalExpiryKeys != 0 {
		t.Fatalf("unexpected expiry key count after disconnect: got %d want 0", finalStats.TotalExpiryKeys)
	}

	t.Logf("connect=%s scan=%s disconnect=%s stored_entries=%d total_value_bytes=%d composite_key_bytes=%d page_size=%d",
		connectElapsed, scanElapsed, disconnectElapsed, storedCount, storedValues,
		expiryOutpointCompositeKeySize, scanPageSize)
}

func createPressureBlockWithSpendableOutputs(t *testing.T, height int32, spendableOutputs int,
	root *[AccumulatorDigestSize]byte) *btcutil.Block {
	t.Helper()

	var prevHash chainhash.Hash
	var merkleRoot chainhash.Hash

	header := &wire.BlockHeader{
		Version:    1,
		PrevBlock:  prevHash,
		MerkleRoot: merkleRoot,
		Timestamp:  time.Now(),
		Bits:       0x207fffff,
		Nonce:      0,
	}

	coinbaseTx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
				SignatureScript:  []byte{0x00, 0x00},
				Sequence:         0xffffffff,
			},
		},
		TxOut:    make([]*wire.TxOut, 0, spendableOutputs+1),
		LockTime: 0,
	}

	for i := 0; i < spendableOutputs; i++ {
		coinbaseTx.TxOut = append(coinbaseTx.TxOut, &wire.TxOut{
			Value:    1,
			PkScript: []byte{0x51}, // OP_TRUE
		})
	}

	if root != nil {
		coinbaseTx.TxOut = append(coinbaseTx.TxOut, &wire.TxOut{
			Value:    0,
			PkScript: BuildExpiryCommitmentScript(*root),
		})
	}

	msgBlock := &wire.MsgBlock{
		Header:       *header,
		Transactions: []*wire.MsgTx{coinbaseTx},
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)
	return block
}
