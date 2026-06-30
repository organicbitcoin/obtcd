// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type safetyTxSpec struct {
	prevOut       wire.OutPoint
	stxoHeight    int32
	stxoAmount    int64
	outputAmounts []int64
	version       int32
	addMarker     bool
}

type safetyUTXO struct {
	height int32
	amount int64
}

type safetyMockChain struct {
	bestHeight        int32
	blocks            map[int32]*btcutil.Block
	utxos             map[wire.OutPoint]safetyUTXO
	spendJournalCalls int
}

func (m *safetyMockChain) BestHeight() int32 {
	return m.bestHeight
}

func (m *safetyMockChain) BlockByHeight(height int32) (*btcutil.Block, error) {
	block, ok := m.blocks[height]
	if !ok {
		return nil, database.Error{
			ErrorCode:   database.ErrBlockNotFound,
			Description: "safety mock block not found",
		}
	}
	return block, nil
}

func (m *safetyMockChain) FetchSpendJournal(block *btcutil.Block) ([]blockchain.SpentTxOut, error) {
	m.spendJournalCalls++
	return nil, fmt.Errorf("safety mock spend journal unavailable")
}

func (m *safetyMockChain) ForEachUTXO(fn func(wire.OutPoint, int32) error) error {
	for outpoint, utxo := range m.utxos {
		if err := fn(outpoint, utxo.height); err != nil {
			return err
		}
	}
	return nil
}

func (m *safetyMockChain) ForEachUTXOWithAmount(fn func(wire.OutPoint, int32, int64) error) error {
	for outpoint, utxo := range m.utxos {
		if err := fn(outpoint, utxo.height, utxo.amount); err != nil {
			return err
		}
	}
	return nil
}

func setupSafetyIndex(t *testing.T) (database.DB, *ExpiryIndex, func()) {
	t.Helper()

	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		teardown()
		t.Fatalf("new expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	}); err != nil {
		teardown()
		t.Fatalf("create expiry index: %v", err)
	}
	if err := idx.Init(); err != nil {
		teardown()
		t.Fatalf("init expiry index: %v", err)
	}

	idx.expiryParams.StartScanHeight = 1
	idx.expiryParams.EnableAtHeight = 0
	idx.expiryParams.ExpiryCommitmentEnableAtHeight = 0
	return db, idx, teardown
}

func seedSafetyTip(t *testing.T, db database.DB, idx *ExpiryIndex, height int32,
	hash chainhash.Hash) {

	t.Helper()
	if err := db.Update(func(dbTx database.Tx) error {
		if err := dbPutAccumulatorState(dbTx, NewMuHash()); err != nil {
			return err
		}
		if err := dbPutAccumulatorTipHash(dbTx, &hash); err != nil {
			return err
		}
		return dbPutTipHeightIndexed(dbTx, height)
	}); err != nil {
		t.Fatalf("seed safety tip: %v", err)
	}
	idx.curTipHeight = height
}

func buildSafetyBlock(t *testing.T, height int32, prevHash chainhash.Hash,
	coinbaseSpendable bool, specs ...safetyTxSpec) (*btcutil.Block,
	[]blockchain.SpentTxOut, []wire.OutPoint) {

	t.Helper()

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
		SignatureScript: []byte{
			byte(height),
			byte(height >> 8),
			byte(len(specs)),
		},
		Sequence: 0xffffffff,
	})
	coinbaseScript := []byte{txscript.OP_RETURN, 0x01}
	if coinbaseSpendable {
		coinbaseScript = []byte{txscript.OP_TRUE}
	}
	coinbase.AddTxOut(&wire.TxOut{
		Value:    50_0000_0000,
		PkScript: coinbaseScript,
	})

	txs := []*wire.MsgTx{coinbase}
	stxos := make([]blockchain.SpentTxOut, 0, len(specs))
	created := make([]wire.OutPoint, 0)
	if coinbaseSpendable {
		created = append(created, wire.OutPoint{
			Hash:  *btcutil.NewTx(coinbase).Hash(),
			Index: 0,
		})
	}

	for specIdx, spec := range specs {
		version := spec.version
		if version == 0 {
			version = 1
		}

		tx := wire.NewMsgTx(version)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: spec.prevOut,
			SignatureScript:  []byte{txscript.OP_TRUE},
			Sequence:         0xffffffff,
		})
		for _, amount := range spec.outputAmounts {
			tx.AddTxOut(&wire.TxOut{
				Value:    amount,
				PkScript: []byte{txscript.OP_TRUE},
			})
		}
		if spec.addMarker {
			tx.AddTxOut(&wire.TxOut{
				Value:    0,
				PkScript: []byte{txscript.OP_RETURN, byte(specIdx), byte(height)},
			})
		}

		txHash := *btcutil.NewTx(tx).Hash()
		for outIdx, txOut := range tx.TxOut {
			if txscript.IsUnspendable(txOut.PkScript) {
				continue
			}
			created = append(created, wire.OutPoint{
				Hash:  txHash,
				Index: uint32(outIdx),
			})
		}
		stxos = append(stxos, blockchain.SpentTxOut{
			Amount:     spec.stxoAmount,
			PkScript:   []byte{txscript.OP_TRUE},
			Height:     spec.stxoHeight,
			IsCoinBase: spec.stxoHeight > 0,
		})
		txs = append(txs, tx)
	}

	rootMaterial := []byte(fmt.Sprintf("expiry-index-safety-%d-%x", height, prevHash[:]))
	for _, tx := range txs {
		txHash := *btcutil.NewTx(tx).Hash()
		rootMaterial = append(rootMaterial, txHash[:]...)
	}
	merkleRoot := chainhash.DoubleHashH(rootMaterial)
	block := btcutil.NewBlock(&wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  prevHash,
			MerkleRoot: merkleRoot,
			Timestamp:  time.Unix(int64(height), 0),
			Bits:       0x207fffff,
		},
		Transactions: txs,
	})
	block.SetHeight(height)
	return block, stxos, created
}

func scanSafetyRows(t *testing.T, idx *ExpiryIndex) []ExpiringUTXO {
	t.Helper()

	rows, hasMore, err := idx.ScanExpiringUTXOs(0, ^uint64(0), 10_000, nil)
	if err != nil {
		t.Fatalf("scan expiry index: %v", err)
	}
	if hasMore {
		t.Fatal("expected safety scan to fit in one page")
	}
	out := make([]ExpiringUTXO, len(rows))
	for i := range rows {
		out[i] = *rows[i]
	}
	return out
}

func scanSafetyRowsPaged(t *testing.T, idx *ExpiryIndex, pageSize int) []ExpiringUTXO {
	t.Helper()

	var (
		all        []ExpiringUTXO
		fromKey    uint64
		startAfter *wire.OutPoint
	)
	for {
		page, hasMore, err := idx.ScanExpiringUTXOs(fromKey, ^uint64(0), pageSize, startAfter)
		if err != nil {
			t.Fatalf("paged scan: %v", err)
		}
		for _, row := range page {
			all = append(all, *row)
		}
		if !hasMore {
			break
		}
		last := page[len(page)-1]
		fromKey = last.ExpiryKey
		startAfter = &last.OutPoint
	}
	return all
}

func requireSafetyContains(t *testing.T, rows []ExpiringUTXO, op wire.OutPoint) {
	t.Helper()
	for _, row := range rows {
		if row.OutPoint == op {
			return
		}
	}
	t.Fatalf("expected scan to contain %s, got %+v", op, rows)
}

func requireSafetyNotContains(t *testing.T, rows []ExpiringUTXO, op wire.OutPoint) {
	t.Helper()
	for _, row := range rows {
		if row.OutPoint == op {
			t.Fatalf("scan contains stale outpoint %s: %+v", op, rows)
		}
	}
}

func TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan(t *testing.T) {
	db, idx, teardown := setupSafetyIndex(t)
	defer teardown()

	var genesisHash chainhash.Hash
	seedSafetyTip(t, db, idx, 0, genesisHash)
	before, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("initial snapshot: %v", err)
	}

	fakePrev := makeOutPoint(0x10, 0)
	block, stxos, created := buildSafetyBlock(t, 1, genesisHash, true, safetyTxSpec{
		prevOut:       fakePrev,
		stxoHeight:    0,
		stxoAmount:    50_0000_0000,
		outputAmounts: []int64{2000, 1000},
	})
	if len(created) != 3 {
		t.Fatalf("test setup expected coinbase plus two tx outputs, got %d", len(created))
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, stxos)
	}); err != nil {
		t.Fatalf("connect block: %v", err)
	}

	connectedSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("connected snapshot: %v", err)
	}
	if connectedSnapshot.TipHeight != 1 || connectedSnapshot.TipHash != *block.Hash() {
		t.Fatalf("unexpected connected snapshot: %+v", connectedSnapshot)
	}
	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("stats after connect: %v", err)
	}
	if stats.TotalUTXOs != 3 || stats.TotalExpiryKeys != 1 {
		t.Fatalf("unexpected stats after connect: %+v", stats)
	}

	rows := scanSafetyRows(t, idx)
	if len(rows) != 3 {
		t.Fatalf("expected 3 scan rows after connect, got %d", len(rows))
	}
	wantExpiry := idx.expiryParams.CalculateExpiryKey(1)
	for _, row := range rows {
		if row.ExpiryKey != wantExpiry {
			t.Fatalf("unexpected expiry key: got %d want %d", row.ExpiryKey, wantExpiry)
		}
	}
	for _, op := range created {
		requireSafetyContains(t, rows, op)
	}

	candidates, err := idx.ReapPrefixCandidates(int32(wantExpiry), 10)
	if err != nil {
		t.Fatalf("REAP prefix candidates: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 prefix candidates, got %d", len(candidates))
	}
	if candidates[0].OutPoint != created[2] || candidates[1].OutPoint != created[1] ||
		candidates[2].OutPoint != created[0] {

		t.Fatalf("REAP strict order should be amount then outpoint, got %+v", candidates)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, block, stxos)
	}); err != nil {
		t.Fatalf("disconnect block: %v", err)
	}
	disconnectedSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("disconnected snapshot: %v", err)
	}
	if disconnectedSnapshot != before {
		t.Fatalf("disconnect should restore snapshot: got %+v want %+v",
			disconnectedSnapshot, before)
	}
	if rows := scanSafetyRows(t, idx); len(rows) != 0 {
		t.Fatalf("disconnect should remove created UTXOs, got %+v", rows)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, block, stxos)
	}); err != nil {
		t.Fatalf("reconnect block: %v", err)
	}
	reconnectedSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("reconnected snapshot: %v", err)
	}
	if reconnectedSnapshot != connectedSnapshot {
		t.Fatalf("reconnect snapshot mismatch: got %+v want %+v",
			reconnectedSnapshot, connectedSnapshot)
	}
	if got, want := scanSafetyRows(t, idx), rows; !reflect.DeepEqual(got, want) {
		t.Fatalf("reconnect scan mismatch: got %+v want %+v", got, want)
	}
}

func TestExpiryIndexSpendRemoveReapRefundAndRepeatedDelete(t *testing.T) {
	db, idx, teardown := setupSafetyIndex(t)
	defer teardown()

	var genesisHash chainhash.Hash
	seedSafetyTip(t, db, idx, 0, genesisHash)

	baseBlock, baseStxos, baseCreated := buildSafetyBlock(t, 1, genesisHash, false, safetyTxSpec{
		prevOut:       makeOutPoint(0x20, 0),
		stxoHeight:    0,
		stxoAmount:    50_0000_0000,
		outputAmounts: []int64{1000},
	})
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, baseBlock, baseStxos)
	}); err != nil {
		t.Fatalf("connect base block: %v", err)
	}
	if len(baseCreated) != 1 {
		t.Fatalf("test setup expected one base UTXO, got %d", len(baseCreated))
	}
	oldOut := baseCreated[0]
	baseSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("base snapshot: %v", err)
	}

	reapBlock, reapStxos, reapCreated := buildSafetyBlock(t, 2, *baseBlock.Hash(), false, safetyTxSpec{
		prevOut:       oldOut,
		stxoHeight:    1,
		stxoAmount:    1000,
		outputAmounts: []int64{700},
		version:       3,
		addMarker:     true,
	})
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, reapBlock, reapStxos)
	}); err != nil {
		t.Fatalf("connect REAP-like block: %v", err)
	}
	if len(reapCreated) != 1 {
		t.Fatalf("REAP-like marker must be unspendable and skipped, created=%d", len(reapCreated))
	}
	refundOut := reapCreated[0]
	rows := scanSafetyRows(t, idx)
	requireSafetyNotContains(t, rows, oldOut)
	requireSafetyContains(t, rows, refundOut)

	beforeRepeatedDelete, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("snapshot before repeated delete: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.disconnectTxOut(dbTx, &oldOut)
	}); err != nil {
		t.Fatalf("repeated delete of absent outpoint should be a no-op: %v", err)
	}
	afterRepeatedDelete, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("snapshot after repeated delete: %v", err)
	}
	if afterRepeatedDelete != beforeRepeatedDelete {
		t.Fatalf("repeated delete changed snapshot: got %+v want %+v",
			afterRepeatedDelete, beforeRepeatedDelete)
	}

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, reapBlock, reapStxos)
	}); err != nil {
		t.Fatalf("disconnect REAP-like block: %v", err)
	}
	rollbackRows := scanSafetyRows(t, idx)
	requireSafetyContains(t, rollbackRows, oldOut)
	requireSafetyNotContains(t, rollbackRows, refundOut)
	rollbackSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("rollback snapshot: %v", err)
	}
	if rollbackSnapshot != baseSnapshot {
		t.Fatalf("disconnect REAP-like block should restore base snapshot: got %+v want %+v",
			rollbackSnapshot, baseSnapshot)
	}
}

func TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild(t *testing.T) {
	db, idx, teardown := setupSafetyIndex(t)
	defer teardown()

	var genesisHash chainhash.Hash
	seedSafetyTip(t, db, idx, 0, genesisHash)

	baseBlock, baseStxos, baseCreated := buildSafetyBlock(t, 1, genesisHash, false, safetyTxSpec{
		prevOut:       makeOutPoint(0x30, 0),
		stxoHeight:    0,
		stxoAmount:    50_0000_0000,
		outputAmounts: []int64{1000},
	})
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, baseBlock, baseStxos)
	}); err != nil {
		t.Fatalf("connect base block: %v", err)
	}
	baseOut := baseCreated[0]

	blockA, stxosA, createdA := buildSafetyBlock(t, 2, *baseBlock.Hash(), false, safetyTxSpec{
		prevOut:       baseOut,
		stxoHeight:    1,
		stxoAmount:    1000,
		outputAmounts: []int64{777},
	})
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, blockA, stxosA)
	}); err != nil {
		t.Fatalf("connect branch A: %v", err)
	}
	staleAOut := createdA[0]
	aRows := scanSafetyRows(t, idx)
	requireSafetyContains(t, aRows, staleAOut)
	requireSafetyNotContains(t, aRows, baseOut)

	if err := db.Update(func(dbTx database.Tx) error {
		return idx.DisconnectBlock(dbTx, blockA, stxosA)
	}); err != nil {
		t.Fatalf("disconnect branch A: %v", err)
	}

	blockB, stxosB, createdB := buildSafetyBlock(t, 2, *baseBlock.Hash(), false, safetyTxSpec{
		prevOut:       makeOutPoint(0x31, 0),
		stxoHeight:    0,
		stxoAmount:    50_0000_0000,
		outputAmounts: []int64{777},
	})
	if *blockA.Hash() == *blockB.Hash() {
		t.Fatalf("test setup requires distinct branch tips")
	}
	if err := db.Update(func(dbTx database.Tx) error {
		return idx.ConnectBlock(dbTx, blockB, stxosB)
	}); err != nil {
		t.Fatalf("connect branch B: %v", err)
	}
	activeBOut := createdB[0]

	reorgRows := scanSafetyRows(t, idx)
	requireSafetyContains(t, reorgRows, baseOut)
	requireSafetyContains(t, reorgRows, activeBOut)
	requireSafetyNotContains(t, reorgRows, staleAOut)
	if paged := scanSafetyRowsPaged(t, idx, 1); !reflect.DeepEqual(paged, reorgRows) {
		t.Fatalf("paged scan differs from full scan after reorg: paged=%+v full=%+v",
			paged, reorgRows)
	}

	reorgSnapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("reorg snapshot: %v", err)
	}
	reorgStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("reorg stats: %v", err)
	}
	reorgCandidates, err := idx.ReapPrefixCandidates(int32(idx.expiryParams.CalculateExpiryKey(2)), 10)
	if err != nil {
		t.Fatalf("reorg REAP candidates: %v", err)
	}

	rebuildDB, rebuildTeardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create rebuild db: %v", err)
	}
	defer rebuildTeardown()
	rebuildIdx, err := NewExpiryIndex(rebuildDB, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new rebuild index: %v", err)
	}
	if err := rebuildDB.Update(func(dbTx database.Tx) error {
		return rebuildIdx.Create(dbTx)
	}); err != nil {
		t.Fatalf("create rebuild index: %v", err)
	}
	if err := rebuildIdx.Init(); err != nil {
		t.Fatalf("init rebuild index: %v", err)
	}
	rebuildIdx.expiryParams.StartScanHeight = 1
	rebuildIdx.expiryParams.EnableAtHeight = 0
	rebuildIdx.expiryParams.ExpiryCommitmentEnableAtHeight = 0
	rebuildIdx.chain = &safetyMockChain{
		bestHeight: 2,
		blocks: map[int32]*btcutil.Block{
			2: blockB,
		},
		utxos: map[wire.OutPoint]safetyUTXO{
			baseOut:    {height: 1, amount: 1000},
			activeBOut: {height: 2, amount: 777},
		},
	}
	if err := rebuildIdx.fastRebuildFromUTXO(2); err != nil {
		t.Fatalf("fast rebuild after reorg: %v", err)
	}

	rebuildSnapshot, err := rebuildIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("rebuild snapshot: %v", err)
	}
	if rebuildSnapshot != reorgSnapshot {
		t.Fatalf("rebuild snapshot mismatch after reorg: live=%+v rebuild=%+v",
			reorgSnapshot, rebuildSnapshot)
	}
	rebuildStats, err := rebuildIdx.GetStats()
	if err != nil {
		t.Fatalf("rebuild stats: %v", err)
	}
	if !reflect.DeepEqual(rebuildStats, reorgStats) {
		t.Fatalf("rebuild stats mismatch after reorg: live=%+v rebuild=%+v",
			reorgStats, rebuildStats)
	}
	rebuildRows := scanSafetyRows(t, rebuildIdx)
	if !reflect.DeepEqual(rebuildRows, reorgRows) {
		t.Fatalf("rebuild scan mismatch after reorg: live=%+v rebuild=%+v",
			reorgRows, rebuildRows)
	}
	rebuildCandidates, err := rebuildIdx.ReapPrefixCandidates(int32(idx.expiryParams.CalculateExpiryKey(2)), 10)
	if err != nil {
		t.Fatalf("rebuild REAP candidates: %v", err)
	}
	if !reflect.DeepEqual(rebuildCandidates, reorgCandidates) {
		t.Fatalf("rebuild REAP candidates mismatch: live=%+v rebuild=%+v",
			reorgCandidates, rebuildCandidates)
	}
}

func TestExpiryIndexFastRebuildUsesUTXOSetWithoutSpendJournals(t *testing.T) {
	_, idx, teardown := setupSafetyIndex(t)
	defer teardown()

	tipBlock, _, _ := buildSafetyBlock(t, 5, chainhash.Hash{}, false)
	mock := &safetyMockChain{
		bestHeight: 5,
		blocks: map[int32]*btcutil.Block{
			5: tipBlock,
		},
		utxos: map[wire.OutPoint]safetyUTXO{
			makeOutPoint(0x40, 0): {height: 2, amount: 400},
			makeOutPoint(0x41, 0): {height: 4, amount: 100},
		},
	}
	idx.chain = mock

	if err := idx.fastRebuildFromUTXO(5); err != nil {
		t.Fatalf("fast rebuild: %v", err)
	}
	if mock.spendJournalCalls != 0 {
		t.Fatalf("fast rebuild should not fetch historical spend journals, got %d calls",
			mock.spendJournalCalls)
	}

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("snapshot after fast rebuild: %v", err)
	}
	if snapshot.TipHeight != 5 || snapshot.TipHash != *tipBlock.Hash() {
		t.Fatalf("unexpected fast rebuild snapshot: %+v", snapshot)
	}
	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("stats after fast rebuild: %v", err)
	}
	if stats.TotalUTXOs != 2 || stats.TotalExpiryKeys != 2 {
		t.Fatalf("unexpected stats after fast rebuild: %+v", stats)
	}
	candidates, err := idx.ReapPrefixCandidates(1_000, 10)
	if err != nil {
		t.Fatalf("REAP candidates after fast rebuild: %v", err)
	}
	if len(candidates) != 2 || candidates[0].Amount != 400 || candidates[1].Amount != 100 {
		t.Fatalf("unexpected candidate amounts after amount-aware rebuild: %+v", candidates)
	}
}
