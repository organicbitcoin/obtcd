package mining

import (
	"bytes"
	"crypto/sha256"
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

func setupBoundaryHarnessAtHeight(t *testing.T, tipHeight int32, needHeights []int32) *boundaryHarness {
	t.Helper()

	params := chaincfg.ObtcRegTestParams
	timeSource := blockchain.NewMedianTime()

	tmpDir, err := os.MkdirTemp("", "newblocktemplate_boundary_dynamic_")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "boundary_dynamic.db")
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

	needSet := make(map[int32]struct{}, len(needHeights))
	for _, h := range needHeights {
		needSet[h] = struct{}{}
	}

	spendable := make(map[int32]wire.OutPoint)
	values := make(map[int32]int64)
	var prev *btcutil.Block
	for h := int32(1); h <= tipHeight; h++ {
		blk := mineCoinbaseBlockNoPoWWithIdx(t, chain, &params, prev, idx)
		if err := db.Update(func(dbTx database.Tx) error {
			return idx.ConnectBlock(dbTx, blk, nil)
		}); err != nil {
			db.Close()
			os.RemoveAll(tmpDir)
			t.Fatalf("connect block to expiry index at height %d: %v", h, err)
		}

		if _, ok := needSet[h]; ok {
			coinbase := blk.Transactions()[0]
			spendable[h] = wire.OutPoint{Hash: *coinbase.Hash(), Index: 0}
			values[h] = coinbase.MsgTx().TxOut[0].Value
		}
		prev = blk
	}

	for _, h := range needHeights {
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

func setupTemplateHarnessWithParamsAtHeight(t *testing.T, params chaincfg.Params, tipHeight int32, needHeights []int32) *boundaryHarness {
	t.Helper()
	timeSource := blockchain.NewMedianTime()

	tmpDir, err := os.MkdirTemp("", "newblocktemplate_params_dynamic_")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "params_dynamic.db")
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

	needSet := make(map[int32]struct{}, len(needHeights))
	for _, h := range needHeights {
		needSet[h] = struct{}{}
	}

	spendable := make(map[int32]wire.OutPoint)
	values := make(map[int32]int64)
	var prev *btcutil.Block
	for h := int32(1); h <= tipHeight; h++ {
		blk := mineCoinbaseBlockNoPoW(t, chain, &params, prev)
		if _, ok := needSet[h]; ok {
			coinbase := blk.Transactions()[0]
			spendable[h] = wire.OutPoint{Hash: *coinbase.Hash(), Index: 0}
			values[h] = coinbase.MsgTx().TxOut[0].Value
		}
		prev = blk
	}

	for _, h := range needHeights {
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

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return &boundaryHarness{
		params:    &params,
		chain:     chain,
		db:        db,
		generator: generator,
		spendable: spendable,
		values:    values,
		cleanup:   cleanup,
	}
}

func buildSpendTxWithOutputScript(prevOut wire.OutPoint, prevValue, fee int64, pkScript []byte) *btcutil.Tx {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{PreviousOutPoint: prevOut})
	msgTx.AddTxOut(&wire.TxOut{
		Value:    prevValue - fee,
		PkScript: pkScript,
	})
	return btcutil.NewTx(msgTx)
}

func buildWitnessSpendTx(prevOut wire.OutPoint, prevValue, fee int64, padding int) *btcutil.Tx {
	pkScriptLen := padding + 1
	if pkScriptLen < 1 {
		pkScriptLen = 1
	}

	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: prevOut,
		Witness:          wire.TxWitness{[]byte{0x01}},
	})
	msgTx.AddTxOut(&wire.TxOut{
		Value:    prevValue - fee,
		PkScript: bytes.Repeat([]byte{txscript.OP_TRUE}, pkScriptLen),
	})
	return btcutil.NewTx(msgTx)
}

func templateTxIndex(tmpl *BlockTemplate, hash *chainhash.Hash) int {
	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		h := tx.TxHash()
		if h == *hash {
			return i
		}
	}
	return -1
}

func txHasInput(tx *btcutil.Tx, out wire.OutPoint) bool {
	for _, in := range tx.MsgTx().TxIn {
		if in.PreviousOutPoint == out {
			return true
		}
	}
	return false
}

func countREAPTxInTemplate(tmpl *BlockTemplate) int {
	count := 0
	for i, tx := range tmpl.Block.Transactions {
		if i == 0 {
			continue
		}
		if reap.IsLikelyREAPTx(tx) {
			count++
		}
	}
	return count
}

func buildP2WSHOpTrueScript(t *testing.T) []byte {
	t.Helper()
	witnessScript := []byte{txscript.OP_TRUE}
	digest := sha256.Sum256(witnessScript)
	script, err := txscript.NewScriptBuilder().
		AddOp(txscript.OP_0).
		AddData(digest[:]).
		Script()
	if err != nil {
		t.Fatalf("build p2wsh script: %v", err)
	}
	return script
}

func buildP2WSHOpTrueWitnessSpend(prevOut wire.OutPoint, prevValue, fee int64) *btcutil.Tx {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: prevOut,
		Witness:          wire.TxWitness{[]byte{txscript.OP_TRUE}},
	})
	msgTx.AddTxOut(&wire.TxOut{
		Value:    prevValue - fee,
		PkScript: []byte{txscript.OP_TRUE},
	})
	return btcutil.NewTx(msgTx)
}

func mineBlockWithTxsNoPoW(t *testing.T, chain *blockchain.BlockChain,
	params *chaincfg.Params, txs ...*btcutil.Tx) *btcutil.Block {
	t.Helper()

	best := chain.BestSnapshot()
	nextHeight := best.Height + 1

	coinbaseScript, err := standardCoinbaseScript(nextHeight, 0)
	if err != nil {
		t.Fatalf("coinbase script: %v", err)
	}
	coinbaseTx, err := createCoinbaseTx(params, coinbaseScript, nextHeight, nil)
	if err != nil {
		t.Fatalf("coinbase tx: %v", err)
	}

	ts := best.MedianTime.Add(time.Second)
	bits, err := chain.CalcNextRequiredDifficulty(ts)
	if err != nil {
		t.Fatalf("calc next required difficulty: %v", err)
	}
	version, err := chain.CalcNextBlockVersion()
	if err != nil {
		t.Fatalf("calc next block version: %v", err)
	}

	allTxs := make([]*btcutil.Tx, 0, 1+len(txs))
	allTxs = append(allTxs, coinbaseTx)
	allTxs = append(allTxs, txs...)

	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{
		Version:    version,
		PrevBlock:  best.Hash,
		MerkleRoot: blockchain.CalcMerkleRoot(allTxs, false),
		Timestamp:  ts,
		Bits:       bits,
	})
	for _, tx := range allTxs {
		if err := msgBlock.AddTransaction(tx.MsgTx()); err != nil {
			t.Fatalf("add tx: %v", err)
		}
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

func injectCoinbaseIntoExpiryIndexAtHeight(t *testing.T, h *boundaryHarness,
	sourceHeight int32, fakeCreateHeight int32) {
	t.Helper()

	if h == nil || h.db == nil || h.reapIndex == nil {
		t.Fatalf("harness is missing db/index for expiry injection")
	}

	sourceBlock, err := h.chain.BlockByHeight(sourceHeight)
	if err != nil {
		t.Fatalf("BlockByHeight(%d): %v", sourceHeight, err)
	}
	coinbase := sourceBlock.MsgBlock().Transactions[0]

	fakeMsg := wire.NewMsgBlock(&wire.BlockHeader{})
	if err := fakeMsg.AddTransaction(coinbase.Copy()); err != nil {
		t.Fatalf("add copied coinbase tx: %v", err)
	}
	fakeBlock := btcutil.NewBlock(fakeMsg)
	fakeBlock.SetHeight(fakeCreateHeight)

	if err := h.db.Update(func(dbTx database.Tx) error {
		return h.reapIndex.ConnectBlock(dbTx, fakeBlock, nil)
	}); err != nil {
		t.Fatalf("inject forged expiry mapping failed: %v", err)
	}
}

func TestNewBlockTemplateREAPConflictWithNormalSkipsREAP(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	// Forge one non-expired coinbase output (height 120) into an "expired" index row
	// so the REAP planner selects it. This allows us to validate that a regular
	// mempool tx consuming that outpoint can block REAP append in NewBlockTemplate.
	injectCoinbaseIntoExpiryIndexAtHeight(t, h, 120, 100)

	nextHeight := h.chain.BestSnapshot().Height + 1
	reapTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if reapTx == nil {
		t.Fatalf("expected planned REAP tx at height %d", nextHeight)
	}
	if !txHasInput(reapTx, h.spendable[120]) {
		t.Fatalf("expected planned REAP tx to include forged-expiry outpoint at height 120")
	}

	fee := int64(10_000)
	conflictNormalTx := buildOPTrueSpendTx(h.spendable[120], h.values[120], fee, 0)
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	conflictWeight := uint32(blockchain.GetTransactionWeight(conflictNormalTx))
	reserve := h.generator.reservedREAPWeight(nextHeight)

	h.generator.policy.BlockMaxWeight = baseWeight + conflictWeight + reserve + 1024
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(conflictNormalTx, fee, 10_000)})

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}

	if !templateContainsTx(tmpl, conflictNormalTx.Hash()) {
		t.Fatalf("expected conflicting normal tx to be included")
	}
	if templateHasREAPTx(tmpl) {
		t.Fatalf("expected REAP tx to be skipped after normal tx consumed a planned REAP input")
	}
}

func TestNewBlockTemplateNoPlannedREAPNoReserve(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 109, []int32{10})
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx != nil {
		t.Fatalf("expected no planned REAP tx at height %d", nextHeight)
	}

	fee := int64(10_000)
	largeTx := buildOPTrueSpendTx(h.spendable[10], h.values[10], fee, 18_000)
	largeWeight := uint32(blockchain.GetTransactionWeight(largeTx))
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)

	const blockMax = uint32(450_000)
	if baseWeight+largeWeight >= blockMax {
		t.Fatalf("test tx is too large for configured block max: base=%d tx=%d blockMax=%d", baseWeight, largeWeight, blockMax)
	}
	if largeWeight <= blockMax-400_000 {
		t.Fatalf("test tx should exceed hypothetical reserved normal region: txWeight=%d threshold=%d", largeWeight, blockMax-400_000)
	}

	h.generator.policy.BlockMaxWeight = blockMax
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(largeTx, fee, 10_000)})

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}

	if !templateContainsTx(tmpl, largeTx.Hash()) {
		t.Fatalf("expected large tx to be included when no REAP tx is planned")
	}
	if templateHasREAPTx(tmpl) {
		t.Fatalf("did not expect REAP tx when no expired candidates exist")
	}
}

func TestNewBlockTemplateDependencyBoundaryBehavior(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 109, []int32{9, 10})
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	fee := int64(10_000)

	t.Run("parent-included-child-unlocked", func(t *testing.T) {
		parent := buildOPTrueSpendTx(h.spendable[10], h.values[10], fee, 0)
		parentOutValue := h.values[10] - fee
		child := buildOPTrueSpendTx(
			wire.OutPoint{Hash: *parent.Hash(), Index: 0},
			parentOutValue,
			fee,
			0,
		)

		parentWeight := uint32(blockchain.GetTransactionWeight(parent))
		childWeight := uint32(blockchain.GetTransactionWeight(child))
		h.generator.policy.BlockMaxWeight = baseWeight + parentWeight + childWeight + 1024
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(child, fee, 9_000),
			makeTxDesc(parent, fee, 10_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		parentIdx := templateTxIndex(tmpl, parent.Hash())
		childIdx := templateTxIndex(tmpl, child.Hash())
		if parentIdx == -1 || childIdx == -1 {
			t.Fatalf("expected both parent and child to be included")
		}
		if parentIdx >= childIdx {
			t.Fatalf("dependency order broken: parent index=%d child index=%d", parentIdx, childIdx)
		}
	})

	t.Run("parent-at-boundary-skipped-child-never-enqueued", func(t *testing.T) {
		parent := buildOPTrueSpendTx(h.spendable[9], h.values[9], fee, 0)
		parentOutValue := h.values[9] - fee
		child := buildOPTrueSpendTx(
			wire.OutPoint{Hash: *parent.Hash(), Index: 0},
			parentOutValue,
			fee,
			0,
		)

		parentWeight := uint32(blockchain.GetTransactionWeight(parent))
		h.generator.policy.BlockMaxWeight = baseWeight + parentWeight
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(child, fee, 9_000),
			makeTxDesc(parent, fee, 10_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		if templateContainsTx(tmpl, parent.Hash()) {
			t.Fatalf("expected parent to be skipped at exact normal boundary")
		}
		if templateContainsTx(tmpl, child.Hash()) {
			t.Fatalf("expected child to remain excluded when parent is skipped")
		}
	})
}

func TestNewBlockTemplateCoverageBranches(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 220, []int32{100, 101, 102, 103, 104, 105, 106, 107, 108})
	defer h.cleanup()

	// Isolate mempool selection branches from REAP append behavior.
	h.generator.reapIndex = nil

	nextHeight := h.chain.BestSnapshot().Height + 1
	fee := int64(10_000)

	t.Run("skip-coinbase-nonfinal-and-missing-prev", func(t *testing.T) {
		coinbaseScript, err := standardCoinbaseScript(nextHeight, 7)
		if err != nil {
			t.Fatalf("standardCoinbaseScript: %v", err)
		}
		coinbaseCandidate, err := createCoinbaseTx(h.params, coinbaseScript, nextHeight, nil)
		if err != nil {
			t.Fatalf("createCoinbaseTx: %v", err)
		}

		nonFinal := buildOPTrueSpendTx(h.spendable[102], h.values[102], fee, 0)
		nonFinal.MsgTx().LockTime = uint32(nextHeight + 100)
		nonFinal.MsgTx().TxIn[0].Sequence = 0

		missingPrev := buildOPTrueSpendTx(
			wire.OutPoint{Hash: chainhash.Hash{0x42}, Index: 0},
			100_000,
			1_000,
			0,
		)

		good := buildOPTrueSpendTx(h.spendable[103], h.values[103], fee, 0)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(coinbaseCandidate, 0, 0),
			makeTxDesc(nonFinal, fee, 10_000),
			makeTxDesc(missingPrev, 1_000, 1_000),
			makeTxDesc(good, fee, 10_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if !templateContainsTx(tmpl, good.Hash()) {
			t.Fatalf("expected valid tx to be included")
		}
		if templateContainsTx(tmpl, nonFinal.Hash()) {
			t.Fatalf("did not expect non-final tx to be included")
		}
		if templateContainsTx(tmpl, missingPrev.Hash()) {
			t.Fatalf("did not expect tx with missing prevout to be included")
		}
	})

	t.Run("priority-switch-requeue-then-include", func(t *testing.T) {
		tx := buildOPTrueSpendTx(h.spendable[104], h.values[104], fee, 0)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 1
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(tx, fee, 10_000)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if !templateContainsTx(tmpl, tx.Hash()) {
			t.Fatalf("expected tx to be included after priority->fee switch requeue")
		}
	})

	t.Run("low-fee-skipped-by-policy", func(t *testing.T) {
		tx := buildOPTrueSpendTx(h.spendable[105], h.values[105], 1, 0)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = btcutil.Amount(100_000_000)
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(tx, 1, 0)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateContainsTx(tmpl, tx.Hash()) {
			t.Fatalf("expected low-fee tx to be skipped by free-fee policy")
		}
	})

	t.Run("sigopcost-overflow-skipped", func(t *testing.T) {
		tx := buildSpendTxWithOutputScript(
			h.spendable[101],
			h.values[101],
			fee,
			bytes.Repeat([]byte{txscript.OP_CHECKSIG}, 21_000),
		)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(tx, fee, 10_000)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateContainsTx(tmpl, tx.Hash()) {
			t.Fatalf("expected tx to be skipped when sigop cost exceeds block cap")
		}
	})

	t.Run("sigopcost-error-on-double-spend", func(t *testing.T) {
		tx1 := buildOPTrueSpendTx(h.spendable[106], h.values[106], fee, 0)
		tx2 := buildOPTrueSpendTx(h.spendable[106], h.values[106], fee, 3)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(tx1, fee, 10_000),
			makeTxDesc(tx2, fee, 9_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if !templateContainsTx(tmpl, tx1.Hash()) {
			t.Fatalf("expected first tx to be included")
		}
		if templateContainsTx(tmpl, tx2.Hash()) {
			t.Fatalf("expected double-spend tx to be skipped")
		}
	})

	t.Run("check-inputs-overspend-skipped", func(t *testing.T) {
		tx := wire.NewMsgTx(wire.TxVersion)
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: h.spendable[107]})
		tx.AddTxOut(&wire.TxOut{Value: h.values[107] + 1, PkScript: []byte{txscript.OP_TRUE}})
		overSpend := btcutil.NewTx(tx)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(overSpend, 0, 10_000)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateContainsTx(tmpl, overSpend.Hash()) {
			t.Fatalf("expected overspend tx to be skipped by input checks")
		}
	})

	t.Run("validate-scripts-error-skipped", func(t *testing.T) {
		parent := buildSpendTxWithOutputScript(
			h.spendable[108],
			h.values[108],
			fee,
			[]byte{txscript.OP_FALSE},
		)
		parentOutValue := h.values[108] - fee
		child := buildOPTrueSpendTx(
			wire.OutPoint{Hash: *parent.Hash(), Index: 0},
			parentOutValue,
			fee,
			0,
		)

		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(child, fee, 9_000),
			makeTxDesc(parent, fee, 10_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if !templateContainsTx(tmpl, parent.Hash()) {
			t.Fatalf("expected parent tx to be included")
		}
		if templateContainsTx(tmpl, child.Hash()) {
			t.Fatalf("expected child tx to be skipped due to script validation failure")
		}
	})
}

func TestNewBlockTemplateWitnessIncludesCommitment(t *testing.T) {
	h := setupBoundaryHarnessAtHeight(t, 220, []int32{100})
	defer h.cleanup()

	// Keep this test focused on segwit/witness flow.
	h.generator.reapIndex = nil

	fee := int64(10_000)
	witnessTx := buildWitnessSpendTx(h.spendable[100], h.values[100], fee, 0)

	h.generator.policy.BlockMaxWeight = 1_000_000
	h.generator.policy.BlockPrioritySize = 0
	h.generator.policy.BlockMinWeight = 0
	h.generator.policy.TxMinFreeFee = 0
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(witnessTx, fee, 10_000)})

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	if len(tmpl.WitnessCommitment) == 0 {
		t.Fatalf("expected witness commitment to be present when witness tx is seen during assembly")
	}
}

func TestNewBlockTemplateREAPBuildErrorFallsBackToNormal(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	brokenIndex, brokenTeardown := createMiningTestExpiryIndexWithOutputs(t, false, 0)
	defer brokenTeardown()
	h.generator.reapIndex = brokenIndex

	fee := int64(10_000)
	tx := buildOPTrueSpendTx(h.spendable[120], h.values[120], fee, 0)
	nextHeight := h.chain.BestSnapshot().Height + 1
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	txWeight := uint32(blockchain.GetTransactionWeight(tx))
	h.generator.policy.BlockMaxWeight = baseWeight + txWeight + 1024
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(tx, fee, 10_000)})

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	if !templateContainsTx(tmpl, tx.Hash()) {
		t.Fatalf("expected normal tx to be included when REAP build fails")
	}
	if templateHasREAPTx(tmpl) {
		t.Fatalf("did not expect REAP tx when REAP build path errors")
	}
}

func TestNewBlockTemplateCoverageREAPSkippedAtBlockMaxBoundary(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	reapTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if reapTx == nil {
		t.Fatalf("expected planned REAP tx")
	}

	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	reapWeight := uint32(blockchain.GetTransactionWeight(reapTx))
	h.generator.policy.BlockMaxWeight = baseWeight + reapWeight
	h.generator.txSource = newStaticTxSource(nil)

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	if templateHasREAPTx(tmpl) {
		t.Fatalf("expected REAP tx to be skipped when blockPlusTxWeight == BlockMaxWeight")
	}
}

func TestNewBlockTemplateCoverageSegwitInactiveSkipsWitnessTx(t *testing.T) {
	params := chaincfg.ObtcRegTestParams
	params.Deployments[chaincfg.DeploymentSegwit] = chaincfg.ConsensusDeployment{
		BitNumber:         1,
		DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(time.Unix(4_102_444_800, 0)),
		DeploymentEnder:   chaincfg.NewMedianTimeDeploymentEnder(time.Unix(4_102_531_200, 0)),
	}

	h := setupTemplateHarnessWithParamsAtHeight(t, params, 220, []int32{100})
	defer h.cleanup()

	state, err := h.chain.ThresholdState(chaincfg.DeploymentSegwit)
	if err != nil {
		t.Fatalf("threshold state: %v", err)
	}
	if state == blockchain.ThresholdActive {
		t.Fatalf("expected segwit to be inactive in this harness")
	}

	fee := int64(10_000)
	witnessTx := buildWitnessSpendTx(h.spendable[100], h.values[100], fee, 0)
	h.generator.policy.BlockMaxWeight = 1_000_000
	h.generator.policy.BlockPrioritySize = 0
	h.generator.policy.BlockMinWeight = 0
	h.generator.policy.TxMinFreeFee = 0
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(witnessTx, fee, 10_000)})

	tmpl, err := h.generator.NewBlockTemplate(nil)
	if err != nil {
		t.Fatalf("NewBlockTemplate failed: %v", err)
	}
	if templateContainsTx(tmpl, witnessTx.Hash()) {
		t.Fatalf("expected witness tx to be skipped when segwit is inactive")
	}
	if len(tmpl.WitnessCommitment) != 0 {
		t.Fatalf("did not expect witness commitment when segwit is inactive")
	}
}
