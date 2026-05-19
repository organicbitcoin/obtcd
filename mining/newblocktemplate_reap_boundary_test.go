// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type staticTxSource struct {
	descs   []*TxDesc
	have    map[chainhash.Hash]struct{}
	updated time.Time
}

func newStaticTxSource(descs []*TxDesc) *staticTxSource {
	have := make(map[chainhash.Hash]struct{}, len(descs))
	for _, d := range descs {
		have[*d.Tx.Hash()] = struct{}{}
	}
	return &staticTxSource{
		descs:   descs,
		have:    have,
		updated: time.Now(),
	}
}

func (s *staticTxSource) LastUpdated() time.Time {
	return s.updated
}

func (s *staticTxSource) MiningDescs() []*TxDesc {
	return s.descs
}

func (s *staticTxSource) HaveTransaction(hash *chainhash.Hash) bool {
	_, ok := s.have[*hash]
	return ok
}

type boundaryHarness struct {
	params    *chaincfg.Params
	chain     *blockchain.BlockChain
	db        database.DB
	reapIndex *expiryindex.ExpiryIndex
	generator *BlkTmplGenerator
	spendable map[int32]wire.OutPoint
	values    map[int32]int64
	cleanup   func()
}

func setupBoundaryHarness(t *testing.T) *boundaryHarness {
	t.Helper()

	params := chaincfg.ObtcRegTestParams
	timeSource := blockchain.NewMedianTime()

	tmpDir, err := os.MkdirTemp("", "newblocktemplate_boundary_")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "boundary.db")
	db, err := database.Create("ffldb", dbPath, params.Net)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create db: %v", err)
	}

	chain, err := blockchain.New(&blockchain.Config{
		DB:               db,
		UtxoCacheMaxSize: 64 * 1024 * 1024,
		ChainParams:      &params,
		TimeSource:       timeSource,
		SigCache:         txscript.NewSigCache(1000),
		HashCache:        txscript.NewHashCache(1000),
	})
	if err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("new chain: %v", err)
	}

	idx, err := expiryindex.NewExpiryIndex(db, &params)
	if err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("new expiry index: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("create expiry index buckets: %v", err)
	}

	spendable := make(map[int32]wire.OutPoint)
	values := make(map[int32]int64)
	needHeights := map[int32]struct{}{100: {}, 120: {}, 121: {}, 122: {}}

	var prev *btcutil.Block
	for h := int32(1); h <= 243; h++ {
		blk := mineCoinbaseBlockNoPoWWithIdx(t, chain, &params, prev, idx)
		if err := db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, blk, nil)
		}); err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("connect block to expiry index at height %d: %v", h, err)
		}

		if _, ok := needHeights[h]; ok {
			coinbase := blk.Transactions()[0]
			spendable[h] = wire.OutPoint{Hash: *coinbase.Hash(), Index: 0}
			values[h] = coinbase.MsgTx().TxOut[0].Value
		}
		prev = blk
	}

	for h := range needHeights {
		if _, ok := spendable[h]; !ok {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("missing spendable output for height %d", h)
		}
	}

	policy := &Policy{
		BlockMinWeight:    0,
		BlockMaxWeight:    1_000_000,
		BlockPrioritySize: 0,
		TxMinFreeFee:      0,
	}
	generator := NewBlkTmplGenerator(
		policy,
		&params,
		newStaticTxSource(nil),
		chain,
		timeSource,
		txscript.NewSigCache(1000),
		txscript.NewHashCache(1000),
	)
	generator.SetExpiryCommitmentSource(idx)
	generator.SetREAPIndex(idx)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return &boundaryHarness{
		params:    &params,
		chain:     chain,
		db:        db,
		reapIndex: idx,
		generator: generator,
		spendable: spendable,
		values:    values,
		cleanup:   cleanup,
	}
}

func mineCoinbaseBlockNoPoW(t *testing.T, chain *blockchain.BlockChain,
	params *chaincfg.Params, prev *btcutil.Block) *btcutil.Block {
	return mineCoinbaseBlockNoPoWWithIdx(t, chain, params, prev, nil)
}

func mineCoinbaseBlockNoPoWWithIdx(t *testing.T, chain *blockchain.BlockChain,
	params *chaincfg.Params, prev *btcutil.Block,
	idx *expiryindex.ExpiryIndex) *btcutil.Block {
	t.Helper()

	var (
		nextHeight int32
		prevHash   chainhash.Hash
		prevTime   time.Time
	)

	if prev == nil {
		nextHeight = 1
		prevHash = *params.GenesisHash
		prevTime = params.GenesisBlock.Header.Timestamp
	} else {
		nextHeight = prev.Height() + 1
		prevHash = *prev.Hash()
		prevTime = prev.MsgBlock().Header.Timestamp
	}

	coinbaseScript, err := standardCoinbaseScript(nextHeight, 0)
	if err != nil {
		t.Fatalf("coinbase script: %v", err)
	}
	coinbaseTx, err := createCoinbaseTx(params, coinbaseScript, nextHeight, nil)
	if err != nil {
		t.Fatalf("coinbase tx: %v", err)
	}

	// Add expiry commitment if the ExpiryIndex is provided and the
	// commitment is active at this height.
	ep := chaincfg.GetExpiryParams(params)
	if idx != nil && ep != nil && nextHeight >= ep.ExpiryCommitmentEnableAtHeight {
		root, err := idx.GetAccumulatorDigest()
		if err == nil {
			AddExpiryCommitment(coinbaseTx, root)
		}
	}

	ts := prevTime.Add(time.Second)
	bits, err := chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		t.Fatalf("calc next required difficulty: %v", err)
	}
	version, err := chain.CalcNextBlockVersion()
	if err != nil {
		t.Fatalf("calc next block version: %v", err)
	}

	txs := []*btcutil.Tx{coinbaseTx}
	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{
		Version:    version,
		PrevBlock:  prevHash,
		MerkleRoot: blockchain.CalcMerkleRoot(txs, false),
		Timestamp:  ts,
		Bits:       bits,
	})
	if err := msgBlock.AddTransaction(coinbaseTx.MsgTx()); err != nil {
		t.Fatalf("add coinbase tx: %v", err)
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(nextHeight)
	isMainChain, isOrphan, err := chain.ProcessBlock(block, blockchain.BFNoPoWCheck)
	if err != nil {
		t.Fatalf("process block height %d: %v", nextHeight, err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected process result at height %d: main=%v orphan=%v", nextHeight, isMainChain, isOrphan)
	}

	return block
}

func buildOPTrueSpendTx(prevOut wire.OutPoint, prevValue, fee int64, padding int) *btcutil.Tx {
	pkScriptLen := padding + 1
	if pkScriptLen < 1 {
		pkScriptLen = 1
	}

	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut})
	msgTx.AddTxOut(&wire.TxOut{
		Value:    prevValue - fee,
		PkScript: bytes.Repeat([]byte{txscript.OP_TRUE}, pkScriptLen),
	})
	return btcutil.NewTx(msgTx)
}

func makeTxDesc(tx *btcutil.Tx, fee int64, feePerKB int64) *TxDesc {
	return &TxDesc{
		Tx:       tx,
		Added:    time.Now(),
		Height:   UnminedHeight,
		Fee:      fee,
		FeePerKB: feePerKB,
	}
}

func initialTemplateWeight(t *testing.T, params *chaincfg.Params, nextBlockHeight int32) uint32 {
	t.Helper()

	coinbaseScript, err := standardCoinbaseScript(nextBlockHeight, 0)
	if err != nil {
		t.Fatalf("coinbase script: %v", err)
	}
	coinbaseTx, err := createCoinbaseTx(params, coinbaseScript, nextBlockHeight, nil)
	if err != nil {
		t.Fatalf("coinbase tx: %v", err)
	}

	return uint32((blockHeaderOverhead * blockchain.WitnessScaleFactor) +
		blockchain.GetTransactionWeight(coinbaseTx))
}

func templateContainsTx(tmpl *BlockTemplate, hash *chainhash.Hash) bool {
	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		h := tx.TxHash()
		if h == *hash {
			return true
		}
	}
	return false
}

func templateHasREAPTx(tmpl *BlockTemplate) bool {
	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		if reap.IsLikelyREAPTx(tx) {
			return true
		}
	}
	return false
}

func templateInternalWeight(t *testing.T, params *chaincfg.Params, nextBlockHeight int32,
	tmpl *BlockTemplate) uint32 {
	t.Helper()

	weight := initialTemplateWeight(t, params, nextBlockHeight)
	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		weight += uint32(blockchain.GetTransactionWeight(btcutil.NewTx(tx)))
	}
	return weight
}

func TestNewBlockTemplateNormalAndREAPRegionBoundaries(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx == nil {
		t.Fatalf("expected a planned REAP tx at height %d", nextHeight)
	}

	reserve := uint32(blockchain.GetTransactionWeight(plannedREAPTx))
	if reserve == 0 {
		t.Fatalf("expected non-zero REAP reserve at height %d", nextHeight)
	}

	fee := int64(10_000)
	txA := buildOPTrueSpendTx(h.spendable[120], h.values[120], fee, 0)
	txB := buildOPTrueSpendTx(h.spendable[121], h.values[121], fee, 0)
	weightA := uint32(blockchain.GetTransactionWeight(txA))
	weightB := uint32(blockchain.GetTransactionWeight(txB))
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)

	t.Run("normal-below-limit-include-and-reap-append", func(t *testing.T) {
		normalLimit := baseWeight + weightA + 1
		h.generator.policy.BlockMaxWeight = normalLimit + reserve
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(txA, fee, 10_000)})

		if got := h.generator.normalTxWeightLimit(nextHeight, true, reserve); got != normalLimit {
			t.Fatalf("normal limit mismatch: got %d want %d", got, normalLimit)
		}

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		if !templateContainsTx(tmpl, txA.Hash()) {
			t.Fatalf("expected txA to be included below normal boundary")
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be appended")
		}

		internalWeight := templateInternalWeight(t, h.params, nextHeight, tmpl)
		if internalWeight <= normalLimit {
			t.Fatalf("expected template to extend into REAP region: internal=%d normalLimit=%d",
				internalWeight, normalLimit)
		}
	})

	t.Run("normal-at-limit-skip-and-keep-reap-room", func(t *testing.T) {
		normalLimit := baseWeight + weightA
		h.generator.policy.BlockMaxWeight = normalLimit + reserve
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(txA, fee, 10_000)})

		if got := h.generator.normalTxWeightLimit(nextHeight, true, reserve); got != normalLimit {
			t.Fatalf("normal limit mismatch: got %d want %d", got, normalLimit)
		}

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		if templateContainsTx(tmpl, txA.Hash()) {
			t.Fatalf("expected txA to be skipped when blockPlusTxWeight == normal limit")
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be appended even when txA hits boundary")
		}
	})

	t.Run("normal-spillover-into-reap-region-is-rejected", func(t *testing.T) {
		normalLimit := baseWeight + weightA + weightB
		h.generator.policy.BlockMaxWeight = normalLimit + reserve
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(txA, fee, 10_000),
			makeTxDesc(txB, fee, 9_000),
		})

		if got := h.generator.normalTxWeightLimit(nextHeight, true, reserve); got != normalLimit {
			t.Fatalf("normal limit mismatch: got %d want %d", got, normalLimit)
		}

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		if !templateContainsTx(tmpl, txA.Hash()) {
			t.Fatalf("expected txA (below limit) to be included")
		}
		if templateContainsTx(tmpl, txB.Hash()) {
			t.Fatalf("expected txB to be rejected at exact normal-region boundary")
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to occupy reserved region")
		}

		internalWeight := templateInternalWeight(t, h.params, nextHeight, tmpl)
		if internalWeight <= normalLimit {
			t.Fatalf("expected final template to extend into REAP region: internal=%d normalLimit=%d",
				internalWeight, normalLimit)
		}
	})
}
