// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain/internal/testhelper"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestOBTCFullBlockAcceptsValidREAPSpend(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-fullblock-valid-reap",
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	chain.TstSetCoinbaseMaturity(1)

	blocks, spends := buildOBTCBlocks(t, chain, 144)
	expired := spends[1][0]

	view := loadReapView(t, chain, expired.PrevOut)
	reapTx := makeValidReapTx(t, view, 145, expired.PrevOut)

	block := newOBTCBlock(t, chain, blocks[143], reapTx)
	isMainChain, isOrphan, err := chain.ProcessBlock(block, BFNoPoWCheck)
	if err != nil {
		t.Fatalf("expected valid REAP block to connect: %v", err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected chain/orphan flags main=%v orphan=%v", isMainChain, isOrphan)
	}
}

func TestOBTCFullBlockRejectsNonREAPExpiredSpend(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-fullblock-expired-nonreap",
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	chain.TstSetCoinbaseMaturity(1)

	blocks, spends := buildOBTCBlocks(t, chain, 144)
	expired := spends[1][0]

	normalTx := wire.NewMsgTx(1)
	normalTx.AddTxIn(&wire.TxIn{PreviousOutPoint: expired.PrevOut})
	normalTx.AddTxOut(&wire.TxOut{Value: int64(expired.Amount - 1), PkScript: []byte{txscript.OP_TRUE}})

	block := newOBTCBlock(t, chain, blocks[143], normalTx)
	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	if err == nil {
		t.Fatal("expected expired non-REAP spend to be rejected")
	}

	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T", err)
	}
	if ruleErr.ErrorCode != ErrBadTxInput {
		t.Fatalf("unexpected error code: got %v want %v", ruleErr.ErrorCode, ErrBadTxInput)
	}
}

func TestOBTCFullBlockRejectsMultipleREAPTransactions(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-fullblock-multi-reap",
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	chain.TstSetCoinbaseMaturity(1)

	blocks, spends := buildOBTCBlocks(t, chain, 145)
	firstExpired := spends[1][0]
	secondExpired := spends[2][0]

	viewA := loadReapView(t, chain, firstExpired.PrevOut)
	viewB := loadReapView(t, chain, secondExpired.PrevOut)
	reapTxA := makeValidReapTx(t, viewA, 146, firstExpired.PrevOut)
	reapTxB := makeValidReapTx(t, viewB, 146, secondExpired.PrevOut)

	block := newOBTCBlock(t, chain, blocks[144], reapTxA, reapTxB)
	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	if err == nil {
		t.Fatal("expected block with multiple REAP txs to be rejected")
	}

	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T", err)
	}
	if ruleErr.ErrorCode != ErrMultipleReapTx {
		t.Fatalf("unexpected error code: got %v want %v", ruleErr.ErrorCode, ErrMultipleReapTx)
	}
}

func buildOBTCBlocks(t *testing.T, chain *BlockChain,
	targetHeight int32) ([]*btcutil.Block, map[int32][]*testhelper.SpendableOut) {

	t.Helper()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)

	blocks := make([]*btcutil.Block, 0, targetHeight)
	spends := make(map[int32][]*testhelper.SpendableOut, targetHeight)
	for height := int32(1); height <= targetHeight; height++ {
		block, blockSpends := newOBTCBlockWithSpends(t, chain, prev)
		_, _, err := chain.ProcessBlock(block, BFNoPoWCheck)
		if err != nil {
			t.Fatalf("unable to process block %d: %v", height, err)
		}

		blocks = append(blocks, block)
		spends[height] = blockSpends
		prev = block
	}

	return blocks, spends
}

func newOBTCBlock(t *testing.T, chain *BlockChain, prev *btcutil.Block,
	extraTxs ...*wire.MsgTx) *btcutil.Block {

	t.Helper()

	block, _ := newOBTCBlockWithSpends(t, chain, prev, extraTxs...)
	return block
}

func newOBTCBlockWithSpends(t *testing.T, chain *BlockChain, prev *btcutil.Block,
	extraTxs ...*wire.MsgTx) (*btcutil.Block, []*testhelper.SpendableOut) {

	t.Helper()

	blockHeight := prev.Height() + 1
	txns := make([]*wire.MsgTx, 0, 1+len(extraTxs))
	txns = append(txns, testhelper.CreateCoinbaseTx(
		blockHeight, CalcBlockSubsidy(blockHeight, chain.chainParams),
	))
	txns = append(txns, extraTxs...)

	block := btcutil.NewBlock(&wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  *prev.Hash(),
			MerkleRoot: calcMerkleRoot(txns),
			Bits:       chain.chainParams.PowLimitBits,
			Timestamp:  nextOBTCBlockTimestamp(prev, blockHeight),
		},
		Transactions: txns,
	})
	block.SetHeight(blockHeight)

	spends := make([]*testhelper.SpendableOut, 0, len(txns))
	for _, tx := range txns {
		if len(tx.TxOut) == 0 {
			continue
		}

		out := testhelper.MakeSpendableOutForTx(tx, 0)
		spends = append(spends, &out)
	}

	return block, spends
}

func nextOBTCBlockTimestamp(prev *btcutil.Block, nextHeight int32) time.Time {
	if nextHeight == 1 {
		return prev.MsgBlock().Header.Timestamp.Add(time.Second)
	}
	return prev.MsgBlock().Header.Timestamp.Add(time.Second)
}

func loadReapView(t *testing.T, chain *BlockChain,
	outpoints ...wire.OutPoint) *UtxoViewpoint {

	t.Helper()

	view := NewUtxoViewpoint()
	for _, outpoint := range outpoints {
		entry, err := chain.FetchUtxoEntry(outpoint)
		if err != nil {
			t.Fatalf("unable to fetch utxo %v: %v", outpoint, err)
		}
		if entry == nil {
			t.Fatalf("missing utxo %v", outpoint)
		}

		view.Entries()[outpoint] = entry.Clone()
	}

	return view
}
