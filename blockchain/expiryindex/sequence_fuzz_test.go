// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"math/rand"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func TestConnectDisconnectLongSequenceRollback(t *testing.T) {
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

	// Seed one spendable outpoint in the index and accumulator.
	seedOut := makeOutPoint(200, 0)
	seedHeight := int32(110)
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.connectTxOut(dbTx, &seedOut, seedHeight); err != nil {
			return err
		}
		// Update accumulator to include the seeded UTXO.
		mh, err := dbGetAccumulatorState(dbTx)
		if err != nil {
			return err
		}
		expiryKey := idx.expiryParams.CalculateExpiryKey(seedHeight)
		mh.Add(computeEntryData(&seedOut, expiryKey))
		if err := dbPutAccumulatorState(dbTx, mh); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, seedHeight); err != nil {
			return err
		}
		idx.curTipHeight = seedHeight
		return nil
	}); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	type item struct {
		block  *btcutil.Block
		stxos  []blockchain.SpentTxOut
		newOut wire.OutPoint
		height int32
	}

	var seq []item
	prevOut := seedOut
	prevCreateHeight := seedHeight
	expectedUTXOs := 1

	for h := int32(120); h < 145; h++ {
		// Read current accumulator digest for the commitment.
		var commitRoot *[AccumulatorDigestSize]byte
		if idx.isCommitmentActive(h) {
			r := [AccumulatorDigestSize]byte{}
			db.View(func(dbTx database.Tx) error {
				mh, err := dbGetAccumulatorState(dbTx)
				if err == nil {
					r = mh.Digest()
				}
				return nil
			})
			commitRoot = &r
		}
		b, newOut := buildSpendBlock(h, prevOut, commitRoot)
		stxos := []blockchain.SpentTxOut{{Height: prevCreateHeight}}

		err := db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, b, stxos)
		})
		if err != nil {
			t.Fatalf("connect block %d: %v", h, err)
		}

		expectedUTXOs += 1 // +2 outputs (coinbase+new) -1 spent input
		stats, err := idx.GetStats()
		if err != nil {
			t.Fatalf("stats after connect %d: %v", h, err)
		}
		if stats.TotalUTXOs != expectedUTXOs {
			t.Fatalf("utxo count mismatch at %d: got %d want %d", h, stats.TotalUTXOs, expectedUTXOs)
		}
		if stats.TipHeight != h {
			t.Fatalf("tip mismatch at %d: got %d", h, stats.TipHeight)
		}

		seq = append(seq, item{block: b, stxos: stxos, newOut: newOut, height: h})
		prevOut = newOut
		prevCreateHeight = h
	}

	for i := len(seq) - 1; i >= 0; i-- {
		it := seq[i]
		err := db.Update(func(dbTx database.Tx) error {
			return idx.DisconnectBlock(dbTx, it.block, it.stxos)
		})
		if err != nil {
			t.Fatalf("disconnect block %d: %v", it.height, err)
		}
		expectedUTXOs -= 1
		stats, err := idx.GetStats()
		if err != nil {
			t.Fatalf("stats after disconnect %d: %v", it.height, err)
		}
		if stats.TotalUTXOs != expectedUTXOs {
			t.Fatalf("utxo count mismatch after disconnect %d: got %d want %d", it.height, stats.TotalUTXOs, expectedUTXOs)
		}
	}

	// Back to seeded state.
	finalStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("final stats: %v", err)
	}
	if finalStats.TotalUTXOs != 1 || finalStats.TipHeight != 119 {
		t.Fatalf("final state mismatch: %+v", finalStats)
	}
}

func FuzzScanExpiringUTXOsProperties(f *testing.F) {
	f.Add(int64(1), 40, 7)
	f.Add(int64(42), 60, 11)
	f.Add(int64(99), 25, 5)

	f.Fuzz(func(t *testing.T, seed int64, n int, pageSize int) {
		if n <= 0 {
			n = 1
		}
		if n > 120 {
			n = 120
		}
		if pageSize <= 0 {
			pageSize = 1
		}
		if pageSize > 30 {
			pageSize = 30
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

		r := rand.New(rand.NewSource(seed))
		if err := db.Update(func(dbTx database.Tx) error {
			for i := 0; i < n; i++ {
				op := makeOutPoint(byte(i%250+1), uint32(i))
				createHeight := int32(100 + r.Intn(40))
				if err := idx.connectTxOut(dbTx, &op, createHeight); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("seed index: %v", err)
		}

		fromKey := uint64(0)
		toKey := uint64(1000)
		all, _, err := idx.ScanExpiringUTXOs(fromKey, toKey, 5000, nil)
		if err != nil {
			t.Fatalf("full scan: %v", err)
		}

		var paged []*ExpiringUTXO
		nextFrom := fromKey
		var startAfter *wire.OutPoint
		for steps := 0; steps < 1000; steps++ {
			page, hasMore, err := idx.ScanExpiringUTXOs(nextFrom, toKey, pageSize, startAfter)
			if err != nil {
				t.Fatalf("paged scan: %v", err)
			}
			if len(page) == 0 {
				if hasMore {
					t.Fatalf("hasMore=true with empty page")
				}
				break
			}
			for i := 1; i < len(page); i++ {
				if page[i-1].ExpiryKey > page[i].ExpiryKey {
					t.Fatalf("expiry key order violation")
				}
			}
			paged = append(paged, page...)
			last := page[len(page)-1]
			nextFrom = last.ExpiryKey
			startAfter = &last.OutPoint
			if !hasMore {
				break
			}
		}

		if len(paged) != len(all) {
			t.Fatalf("paged/full length mismatch: %d vs %d", len(paged), len(all))
		}

		seen := make(map[wire.OutPoint]struct{}, len(paged))
		for _, it := range paged {
			if _, ok := seen[it.OutPoint]; ok {
				t.Fatalf("duplicate outpoint in paged results: %v", it.OutPoint)
			}
			seen[it.OutPoint] = struct{}{}
		}
	})
}

func buildSpendBlock(height int32, prevOut wire.OutPoint, commitRoot *[AccumulatorDigestSize]byte) (*btcutil.Block, wire.OutPoint) {
	prevHash := chainhash.Hash{}
	root := chainhash.Hash{1}
	header := wire.BlockHeader{
		Version:    1,
		PrevBlock:  prevHash,
		MerkleRoot: root,
		Timestamp:  time.Unix(int64(height), 0),
		Bits:       0x207fffff,
	}

	coinbase := &wire.MsgTx{Version: 1}
	coinbase.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: 0xffffffff}, SignatureScript: []byte{byte(height & 0xff), byte((height >> 8) & 0xff)}, Sequence: 0xffffffff})
	coinbase.AddTxOut(&wire.TxOut{Value: 50_0000_0000, PkScript: []byte{0x51}})

	// Add expiry commitment if provided.
	if commitRoot != nil {
		coinbase.AddTxOut(&wire.TxOut{
			Value:    0,
			PkScript: BuildExpiryCommitmentScript(*commitRoot),
		})
	}

	spend := &wire.MsgTx{Version: 1}
	spend.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut, SignatureScript: []byte{0x51}, Sequence: 0xffffffff})
	spend.AddTxOut(&wire.TxOut{Value: 1_0000_0000, PkScript: []byte{0x51}})

	msgBlock := &wire.MsgBlock{Header: header, Transactions: []*wire.MsgTx{coinbase, spend}}
	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)

	newOut := wire.OutPoint{Hash: *btcutil.NewTx(spend).Hash(), Index: 0}
	return block, newOut
}
