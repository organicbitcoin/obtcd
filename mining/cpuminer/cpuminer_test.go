package cpuminer

import (
	"bytes"
	"errors"
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
	"github.com/btcsuite/btcd/mining"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type staticTxSource struct {
	updated time.Time
}

func newStaticTxSource() *staticTxSource {
	return &staticTxSource{updated: time.Now()}
}

func (s *staticTxSource) LastUpdated() time.Time {
	return s.updated
}

func (s *staticTxSource) MiningDescs() []*mining.TxDesc {
	return nil
}

func (s *staticTxSource) HaveTransaction(hash *chainhash.Hash) bool {
	return false
}

type minerHarness struct {
	params    *chaincfg.Params
	chain     *blockchain.BlockChain
	db        database.DB
	reapIndex *expiryindex.ExpiryIndex
	generator *mining.BlkTmplGenerator
	minerAddr btcutil.Address
	tip       *btcutil.Block
	cleanup   func()
}

func TestSubmitBlockStateChecksAndErrors(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		h := setupMinerHarness(t, 1)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}

		var (
			called    bool
			gotBlock  *btcutil.Block
			gotFlags  blockchain.BehaviorFlags
			coinbaseV int64
		)
		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				called = true
				gotBlock = block
				gotFlags = flags
				coinbaseV = block.MsgBlock().Transactions[0].TxOut[0].Value
				return false, nil
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		block := btcutil.NewBlock(template.Block)
		if !miner.submitBlock(block) {
			t.Fatalf("expected submitBlock to accept non-stale block")
		}
		if !called {
			t.Fatalf("expected ProcessBlock to be called")
		}
		if gotBlock == nil || !gotBlock.Hash().IsEqual(block.Hash()) {
			t.Fatalf("unexpected block passed to ProcessBlock")
		}
		if gotFlags != blockchain.BFNone {
			t.Fatalf("unexpected behavior flags: got %v want %v", gotFlags, blockchain.BFNone)
		}
		if coinbaseV <= 0 {
			t.Fatalf("expected positive coinbase amount, got %d", coinbaseV)
		}
	})

	t.Run("stale", func(t *testing.T) {
		h := setupMinerHarness(t, 1)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}

		prevHash := template.Block.BlockHash()
		h.tip = advanceChainNoPoW(t, h, h.tip)
		best := h.chain.BestSnapshot()
		if best.Hash == prevHash {
			t.Fatalf("expected chain tip to advance")
		}

		called := false
		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				called = true
				return false, nil
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		if miner.submitBlock(btcutil.NewBlock(template.Block)) {
			t.Fatalf("expected stale block submission to fail")
		}
		if called {
			t.Fatalf("expected stale block to be rejected before ProcessBlock")
		}
	})

	t.Run("orphan", func(t *testing.T) {
		h := setupMinerHarness(t, 1)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}

		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock:           func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) { return true, nil },
			ConnectedCount:         func() int32 { return 1 },
			IsCurrent:              func() bool { return true },
		})

		if miner.submitBlock(btcutil.NewBlock(template.Block)) {
			t.Fatalf("expected orphan block submission to fail")
		}
	})

	t.Run("rule_error", func(t *testing.T) {
		h := setupMinerHarness(t, 1)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}

		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				return false, blockchain.RuleError{
					ErrorCode:   blockchain.ErrHighHash,
					Description: "synthetic rule rejection",
				}
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		if miner.submitBlock(btcutil.NewBlock(template.Block)) {
			t.Fatalf("expected rule-rejected block submission to fail")
		}
	})

	t.Run("unexpected_error", func(t *testing.T) {
		h := setupMinerHarness(t, 1)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}

		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				return false, errors.New("boom")
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		if miner.submitBlock(btcutil.NewBlock(template.Block)) {
			t.Fatalf("expected unexpected ProcessBlock error to fail submission")
		}
	})
}

func TestGenerateNBlocksREAPActivationBoundary(t *testing.T) {
	t.Run("pre_activation_has_no_reap", func(t *testing.T) {
		h := setupMinerHarness(t, 109)
		defer h.cleanup()

		var submitted *btcutil.Block
		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				submitted = block
				_, isOrphan, err := h.chain.ProcessBlock(block, flags)
				return isOrphan, err
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		hashes, err := miner.GenerateNBlocks(1)
		if err != nil {
			t.Fatalf("GenerateNBlocks: %v", err)
		}
		if len(hashes) != 1 {
			t.Fatalf("expected 1 block hash, got %d", len(hashes))
		}
		if submitted == nil {
			t.Fatalf("expected ProcessBlock to receive the solved block")
		}
		if !hashes[0].IsEqual(submitted.Hash()) {
			t.Fatalf("returned hash mismatch: got %s want %s", hashes[0], submitted.Hash())
		}
		if blockHasREAP(submitted.MsgBlock()) {
			t.Fatalf("did not expect REAP tx before activation")
		}
		if miner.started || miner.discreteMining {
			t.Fatalf("expected miner discrete state to reset after GenerateNBlocks")
		}
	})

	t.Run("post_activation_includes_reap", func(t *testing.T) {
		h := setupMinerHarness(t, 243)
		defer h.cleanup()

		template, err := h.generator.NewBlockTemplate(h.minerAddr)
		if err != nil {
			t.Fatalf("NewBlockTemplate: %v", err)
		}
		if !blockHasREAP(template.Block) {
			t.Fatalf("expected template to contain REAP tx once active")
		}

		var submitted *btcutil.Block
		miner := New(&Config{
			ChainParams:            h.params,
			BlockTemplateGenerator: h.generator,
			MiningAddrs:            []btcutil.Address{h.minerAddr},
			ProcessBlock: func(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
				submitted = block
				_, isOrphan, err := h.chain.ProcessBlock(block, flags)
				return isOrphan, err
			},
			ConnectedCount: func() int32 { return 1 },
			IsCurrent:      func() bool { return true },
		})

		hashes, err := miner.GenerateNBlocks(1)
		if err != nil {
			t.Fatalf("GenerateNBlocks: %v", err)
		}
		if len(hashes) != 1 {
			t.Fatalf("expected 1 block hash, got %d", len(hashes))
		}
		if submitted == nil {
			t.Fatalf("expected ProcessBlock to receive the solved block")
		}
		if !hashes[0].IsEqual(submitted.Hash()) {
			t.Fatalf("returned hash mismatch: got %s want %s", hashes[0], submitted.Hash())
		}
		if !blockHasREAP(submitted.MsgBlock()) {
			t.Fatalf("expected solved block to retain REAP tx")
		}
		if miner.started || miner.discreteMining {
			t.Fatalf("expected miner discrete state to reset after GenerateNBlocks")
		}
	})
}

func setupMinerHarness(t *testing.T, targetHeight int32) *minerHarness {
	t.Helper()

	params := chaincfg.ObtcRegTestParams
	timeSource := blockchain.NewMedianTime()

	tmpDir, err := os.MkdirTemp("", "cpuminer_")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "cpuminer.db")
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

	var prev *btcutil.Block
	for height := int32(1); height <= targetHeight; height++ {
		prev = advanceChainNoPoW(t, &minerHarness{
			params:    &params,
			chain:     chain,
			db:        db,
			reapIndex: idx,
		}, prev)
	}

	policy := &mining.Policy{
		BlockMinWeight:    0,
		BlockMaxWeight:    1_000_000,
		BlockPrioritySize: 0,
		TxMinFreeFee:      0,
	}
	generator := mining.NewBlkTmplGenerator(
		policy,
		&params,
		newStaticTxSource(),
		chain,
		timeSource,
		txscript.NewSigCache(1000),
		txscript.NewHashCache(1000),
	)
	generator.SetExpiryCommitmentSource(idx)
	generator.SetREAPIndex(idx)

	addr := newMiningAddress(t, &params)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return &minerHarness{
		params:    &params,
		chain:     chain,
		db:        db,
		reapIndex: idx,
		generator: generator,
		minerAddr: addr,
		tip:       prev,
		cleanup:   cleanup,
	}
}

func advanceChainNoPoW(t *testing.T, h *minerHarness, prev *btcutil.Block) *btcutil.Block {
	t.Helper()

	var (
		nextHeight int32
		prevHash   chainhash.Hash
		prevTime   time.Time
	)

	if prev == nil {
		nextHeight = 1
		prevHash = *h.params.GenesisHash
		prevTime = h.params.GenesisBlock.Header.Timestamp
	} else {
		nextHeight = prev.Height() + 1
		prevHash = *prev.Hash()
		prevTime = prev.MsgBlock().Header.Timestamp
	}

	coinbaseScript, err := standardCoinbaseScript(nextHeight, 0)
	if err != nil {
		t.Fatalf("coinbase script: %v", err)
	}
	coinbaseTx, err := createCoinbaseTx(h.params, coinbaseScript, nextHeight, nil)
	if err != nil {
		t.Fatalf("coinbase tx: %v", err)
	}

	ep := chaincfg.GetExpiryParams(h.params)
	if ep != nil && nextHeight >= ep.ExpiryCommitmentEnableAtHeight {
		root, err := h.reapIndex.GetAccumulatorDigest()
		if err != nil {
			t.Fatalf("GetAccumulatorDigest: %v", err)
		}
		mining.AddExpiryCommitment(coinbaseTx, root)
	}

	ts := prevTime.Add(time.Second)
	bits, err := h.chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		t.Fatalf("CalcNextRequiredDifficulty: %v", err)
	}
	version, err := h.chain.CalcNextBlockVersion()
	if err != nil {
		t.Fatalf("CalcNextBlockVersion: %v", err)
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
		t.Fatalf("AddTransaction: %v", err)
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(nextHeight)
	isMainChain, isOrphan, err := h.chain.ProcessBlock(block, blockchain.BFNoPoWCheck)
	if err != nil {
		t.Fatalf("ProcessBlock(%d): %v", nextHeight, err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected chain result at height %d: main=%v orphan=%v", nextHeight, isMainChain, isOrphan)
	}
	if err := h.db.Update(func(dbTx database.Tx) error {
		return h.reapIndex.ConnectBlock(dbTx, block, nil)
	}); err != nil {
		t.Fatalf("ExpiryIndex.ConnectBlock(%d): %v", nextHeight, err)
	}

	return block
}

func standardCoinbaseScript(nextBlockHeight int32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(nextBlockHeight)).
		AddInt64(int64(extraNonce)).AddData([]byte(mining.CoinbaseFlags)).
		Script()
}

func createCoinbaseTx(params *chaincfg.Params, coinbaseScript []byte, nextBlockHeight int32,
	addr btcutil.Address) (*btcutil.Tx, error) {
	var pkScript []byte
	if addr != nil {
		var err error
		pkScript, err = txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		pkScript, err = txscript.NewScriptBuilder().AddOp(txscript.OP_TRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		SignatureScript:  coinbaseScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    blockchain.CalcBlockSubsidy(nextBlockHeight, params),
		PkScript: pkScript,
	})
	return btcutil.NewTx(tx), nil
}

func newMiningAddress(t *testing.T, params *chaincfg.Params) btcutil.Address {
	t.Helper()

	addrHash := bytes.Repeat([]byte{0x01}, 20)
	addr, err := btcutil.NewAddressPubKeyHash(addrHash, params)
	if err != nil {
		t.Fatalf("NewAddressPubKeyHash: %v", err)
	}
	return addr
}

func blockHasREAP(msgBlock *wire.MsgBlock) bool {
	for i, tx := range msgBlock.Transactions {
		if i == 0 {
			continue
		}
		if reap.IsLikelyREAPTx(tx) {
			return true
		}
	}
	return false
}
