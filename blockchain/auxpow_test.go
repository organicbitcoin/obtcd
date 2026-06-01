// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain/internal/testhelper"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func auxPowTestParams(startHeight int32) *chaincfg.Params {
	params := chaincfg.ObtcRegTestParams
	params.AuxPowChainID = chaincfg.ObtcAuxPowChainID
	params.AuxPowStartHeight = startHeight
	params.AuxPowBootstrapEndHeight = startHeight + 2016
	params.AuxPowForkResetBits = params.PowLimitBits
	params.AuxPowBootstrapHalfLife = time.Hour
	params.AuxPowNormalHalfLife = 48 * time.Hour
	return &params
}

func makeAuxPowCoinbase(script []byte) *wire.MsgTx {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Index: wire.MaxPrevOutIndex,
		},
		SignatureScript: script,
		Sequence:        wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: []byte{0x51}})
	return tx
}

func makeTestAuxPow(childHash *chainhash.Hash, chainID uint32) *wire.AuxPow {
	return makeTestAuxPowWithBranches(childHash, chainID, nil, 0, nil)
}

func makeTestAuxPowWithBranches(childHash *chainhash.Hash, chainID uint32,
	chainBranch []chainhash.Hash, merkleNonce uint32,
	coinbaseBranch []chainhash.Hash) *wire.AuxPow {

	chainIndex := expectedAuxPowIndex(merkleNonce, chainID, len(chainBranch))
	chainRoot := calcMerkleBranchRoot(*childHash, chainBranch, chainIndex)
	script := make([]byte, 0, len(auxPowMagic)+chainhash.HashSize+8)
	script = append(script, auxPowMagic...)
	script = append(script, auxPowCommitmentRootBytes(&chainRoot)...)
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(1)<<uint(len(chainBranch)))
	binary.LittleEndian.PutUint32(buf[4:8], merkleNonce)
	script = append(script, buf[:]...)

	coinbaseTx := makeAuxPowCoinbase(script)
	coinbaseHash := coinbaseTx.TxHash()
	parentHeader := wire.BlockHeader{
		Version:    1,
		MerkleRoot: calcMerkleBranchRoot(coinbaseHash, coinbaseBranch, 0),
		Bits:       0x1d00ffff,
		Timestamp:  time.Unix(1_700_000_000, 0),
	}

	return &wire.AuxPow{
		CoinbaseTx:           *coinbaseTx,
		CoinbaseMerkleBranch: coinbaseBranch,
		ChainMerkleBranch:    chainBranch,
		ChainBranchMask:      chainIndex,
		ParentHeader:         parentHeader,
	}
}

func makeAuxPowChildBlock(t *testing.T, chain *BlockChain,
	prev *btcutil.Block) *btcutil.Block {

	t.Helper()

	blockHeight := prev.Height() + 1
	coinbase := testhelper.CreateCoinbaseTx(
		blockHeight, CalcBlockSubsidy(blockHeight, chain.chainParams),
	)
	txns := []*wire.MsgTx{coinbase}
	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    chaincfg.ObtcBlockVersion(true),
			PrevBlock:  *prev.Hash(),
			MerkleRoot: calcMerkleRoot(txns),
			Bits:       chain.chainParams.PowLimitBits,
			Timestamp:  prev.MsgBlock().Header.Timestamp.Add(time.Second),
		},
		Transactions: txns,
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(blockHeight)
	msgBlock.AuxPow = makeTestAuxPow(block.Hash(), chain.chainParams.AuxPowChainID)
	return block
}

func makeTimedAuxPowChildBlock(t *testing.T, chain *BlockChain,
	prev *btcutil.Block) *btcutil.Block {

	t.Helper()

	block := makeAuxPowChildBlock(t, chain, prev)
	block.MsgBlock().Header.Timestamp = prev.MsgBlock().Header.Timestamp.Add(
		chain.chainParams.TargetTimePerBlock)
	block = btcutil.NewBlock(block.MsgBlock())
	block.SetHeight(prev.Height() + 1)
	block.MsgBlock().AuxPow = makeTestAuxPow(block.Hash(),
		chain.chainParams.AuxPowChainID)
	return block
}

func makeTimedOrdinaryPowChildBlock(t *testing.T, chain *BlockChain,
	prev *btcutil.Block) *btcutil.Block {

	t.Helper()

	block, _ := newOBTCBlockWithSpends(t, chain, prev)
	block.MsgBlock().Header.Version = chaincfg.ObtcBlockVersion(false)
	block.MsgBlock().Header.Timestamp = prev.MsgBlock().Header.Timestamp.Add(
		chain.chainParams.TargetTimePerBlock)
	return block
}

func tagAuxPowTestBlock(t *testing.T, chain *BlockChain, block *btcutil.Block,
	tag byte) *btcutil.Block {

	t.Helper()

	msgBlock := block.MsgBlock()
	coinbaseScript := msgBlock.Transactions[0].TxIn[0].SignatureScript
	msgBlock.Transactions[0].TxIn[0].SignatureScript =
		append(coinbaseScript, tag)
	msgBlock.Header.MerkleRoot = calcMerkleRoot(msgBlock.Transactions)

	taggedBlock := btcutil.NewBlock(msgBlock)
	taggedBlock.SetHeight(block.Height())
	if wire.IsAuxPowVersion(msgBlock.Header.Version) {
		msgBlock.AuxPow = makeTestAuxPow(taggedBlock.Hash(),
			chain.chainParams.AuxPowChainID)
	}
	return taggedBlock
}

func processAuxPowTestBlock(t *testing.T, chain *BlockChain, block *btcutil.Block,
	wantMainChain, wantOrphan bool) {

	t.Helper()

	isMainChain, isOrphan, err := chain.ProcessBlock(block, BFNoPoWCheck)
	if err != nil {
		t.Fatalf("unable to process block %s: %v", block.Hash(), err)
	}
	if isMainChain != wantMainChain || isOrphan != wantOrphan {
		t.Fatalf("unexpected process result for %s: main=%v orphan=%v, "+
			"want main=%v orphan=%v", block.Hash(), isMainChain, isOrphan,
			wantMainChain, wantOrphan)
	}
}

func TestOBTCAuxPowOrdinaryPoWAcceptedPostFork(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-ordinary-pow",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block, _ := newOBTCBlockWithSpends(t, chain, prev)
	block.MsgBlock().Header.Version = chaincfg.ObtcBlockVersion(false)

	isMainChain, isOrphan, err := chain.ProcessBlock(block, BFNoPoWCheck)
	if err != nil {
		t.Fatalf("ordinary post-fork PoW block rejected: %v", err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected process result main=%v orphan=%v", isMainChain, isOrphan)
	}
}

func TestOBTCAuxPowAcceptedPostFork(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-valid",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block := makeAuxPowChildBlock(t, chain, prev)

	isMainChain, isOrphan, err := chain.ProcessBlock(block, BFNoPoWCheck)
	if err != nil {
		t.Fatalf("valid AuxPoW block rejected: %v", err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected process result main=%v orphan=%v", isMainChain, isOrphan)
	}
}

func TestOBTCAuxPowAcceptedWithCheckedParentWork(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-checked-parent-work",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block := makeAuxPowChildBlock(t, chain, prev)
	target := CompactToBig(block.MsgBlock().Header.Bits)
	for {
		parentHash := block.MsgBlock().AuxPow.ParentHeader.BlockHash()
		if HashToBig(&parentHash).Cmp(target) <= 0 {
			break
		}
		block.MsgBlock().AuxPow.ParentHeader.Nonce++
	}

	isMainChain, isOrphan, err := chain.ProcessBlock(block, BFNone)
	if err != nil {
		t.Fatalf("valid AuxPoW block with checked parent work rejected: %v", err)
	}
	if !isMainChain || isOrphan {
		t.Fatalf("unexpected process result main=%v orphan=%v", isMainChain, isOrphan)
	}
}

func TestOBTCAuxPowRejectedPreFork(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-prefork",
		auxPowTestParams(2))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block := makeAuxPowChildBlock(t, chain, prev)

	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	if err == nil {
		t.Fatal("expected pre-fork AuxPoW block to be rejected")
	}
	ruleErr, ok := err.(RuleError)
	if !ok || ruleErr.ErrorCode != ErrInvalidAuxPow {
		t.Fatalf("unexpected error: got %T %[1]v", err)
	}
}

func TestOBTCAuxPowRejectsInvalidProof(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-invalid",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block := makeAuxPowChildBlock(t, chain, prev)
	block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript[len(auxPowMagic)] ^= 0x01

	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	if err == nil {
		t.Fatal("expected invalid AuxPoW proof to be rejected")
	}
	ruleErr, ok := err.(RuleError)
	if !ok || ruleErr.ErrorCode != ErrInvalidAuxPow {
		t.Fatalf("unexpected error: got %T %[1]v", err)
	}
}

func assertAuxPowRuleError(t *testing.T, err error, code ErrorCode) {
	t.Helper()

	if err == nil {
		t.Fatal("expected rule error")
	}
	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T %[1]v", err)
	}
	if ruleErr.ErrorCode != code {
		t.Fatalf("unexpected error code: got %v want %v: %v",
			ruleErr.ErrorCode, code, ruleErr)
	}
}

func TestOBTCAuxPowRejectsMalformedProofCases(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*btcutil.Block)
	}{
		{
			name: "missing proof",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow = nil
			},
		},
		{
			name: "wrong chain id",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().Header.Version = wire.AuxPowVersionFlag |
					int32(999<<16) | 4
			},
		},
		{
			name: "missing merged mining header",
			mutate: func(block *btcutil.Block) {
				script := block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript
				block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript =
					script[len(auxPowMagic):]
			},
		},
		{
			name: "multiple merged mining headers",
			mutate: func(block *btcutil.Block) {
				script := block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript
				script = append(script, auxPowMagic...)
				block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript = script
			},
		},
		{
			name: "truncated commitment",
			mutate: func(block *btcutil.Block) {
				script := block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript
				block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript =
					script[:len(script)-1]
			},
		},
		{
			name: "wrong merkle size",
			mutate: func(block *btcutil.Block) {
				script := block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].SignatureScript
				offset := len(auxPowMagic) + chainhash.HashSize
				binary.LittleEndian.PutUint32(script[offset:offset+4], 2)
			},
		},
		{
			name: "wrong chain index",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.ChainBranchMask = 1
			},
		},
		{
			name: "parent coinbase branch index is not zero",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.CoinbaseBranchMask = 1
			},
		},
		{
			name: "parent has child chain id",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.ParentHeader.Version =
					int32(chaincfg.ObtcAuxPowChainID<<16) | 4
			},
		},
		{
			name: "chain merkle branch too deep",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.ChainMerkleBranch =
					make([]chainhash.Hash, maxAuxPowChainMerkleBranchHashes+1)
			},
		},
		{
			name: "parent transaction is not coinbase",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.CoinbaseTx.TxIn[0].
					PreviousOutPoint.Index = 0
			},
		},
		{
			name: "parent merkle root mismatch",
			mutate: func(block *btcutil.Block) {
				block.MsgBlock().AuxPow.ParentHeader.MerkleRoot[0] ^= 0x01
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chain, teardown, err := chainSetup("obtc-auxpow-"+test.name,
				auxPowTestParams(1))
			if err != nil {
				t.Fatalf("unable to setup chain: %v", err)
			}
			defer teardown()

			prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
			prev.SetHeight(0)
			block := makeAuxPowChildBlock(t, chain, prev)
			test.mutate(block)

			_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
			assertAuxPowRuleError(t, err, ErrInvalidAuxPow)
		})
	}
}

func TestOBTCAuxPowAcceptsMerkleBranches(t *testing.T) {
	childHash := chainhash.DoubleHashH([]byte("child"))
	chainBranch := []chainhash.Hash{
		chainhash.DoubleHashH([]byte("chain sibling 0")),
		chainhash.DoubleHashH([]byte("chain sibling 1")),
	}
	coinbaseBranch := []chainhash.Hash{
		chainhash.DoubleHashH([]byte("parent sibling")),
	}

	for _, nonce := range []uint32{0, 1, 7, 42} {
		auxPow := makeTestAuxPowWithBranches(&childHash,
			chaincfg.ObtcAuxPowChainID, chainBranch, nonce, coinbaseBranch)
		if err := validateAuxPowCommitment(auxPow, &childHash,
			chaincfg.ObtcAuxPowChainID); err != nil {
			t.Fatalf("valid branched AuxPoW rejected for nonce %d: %v", nonce, err)
		}
	}
}

func TestOBTCAuxPowCommitmentUsesReversedRootBytes(t *testing.T) {
	childHash := chainhash.DoubleHashH([]byte("child byte order"))
	auxPow := makeTestAuxPow(&childHash, chaincfg.ObtcAuxPowChainID)
	script := auxPow.CoinbaseTx.TxIn[0].SignatureScript
	rootStart := len(auxPowMagic)
	rootEnd := rootStart + chainhash.HashSize

	if got, want := script[rootStart:rootEnd],
		auxPowCommitmentRootBytes(&childHash); string(got) != string(want) {

		t.Fatalf("unexpected commitment root bytes: got %x want %x", got, want)
	}

	copy(script[rootStart:rootEnd], childHash[:])
	err := validateAuxPowCommitment(auxPow, &childHash, chaincfg.ObtcAuxPowChainID)
	assertAuxPowRuleError(t, err, ErrInvalidAuxPow)
}

func TestOBTCAuxPowIgnoresLegacyParentHashField(t *testing.T) {
	childHash := chainhash.DoubleHashH([]byte("child"))
	auxPow := makeTestAuxPow(&childHash, chaincfg.ObtcAuxPowChainID)
	auxPow.ParentBlockHash = chainhash.DoubleHashH([]byte("ignored compatibility field"))

	if err := validateAuxPowCommitment(auxPow, &childHash,
		chaincfg.ObtcAuxPowChainID); err != nil {

		t.Fatalf("legacy parent hash compatibility field was validated: %v", err)
	}
}

func TestOBTCAuxPowNamecoinGoldenVector(t *testing.T) {
	// Generated by Namecoin Core's test/functional/test_framework/auxpow.py
	// constructAuxpow and finishAuxpow helpers for the child hash below.
	const childHashHex = "000102030405060708090a0b0c0d0e0f" +
		"101112131415161718191a1b1c1d1e1f"
	const auxPowHex = "0100000001000000000000000000000000000000000000000000000000000000" +
		"0000000000ffffffff2cfabe6d6d000102030405060708090a0b0c0d0e0f101112" +
		"131415161718191a1b1c1d1e1f0100000000000000ffffffff0000000000364812" +
		"e85905fb931eaecad2552c663ae3a7b5836137187cdb44f8c444c36c5a00000000" +
		"000000000000010000000000000000000000000000000000000000000000000000" +
		"0000000000000000000cf91523a85267c4ed19655df6ca9ffaea77f6f554bc9a79" +
		"da8664bacf8bd11e000000000000000000000000"

	payload, err := hex.DecodeString(auxPowHex)
	if err != nil {
		t.Fatalf("unable to decode golden AuxPoW: %v", err)
	}
	reader := bytes.NewReader(payload)
	var auxPow wire.AuxPow
	if err := auxPow.BtcDecode(reader, wire.ProtocolVersion,
		wire.WitnessEncoding); err != nil {

		t.Fatalf("unable to deserialize golden AuxPoW: %v", err)
	}
	if reader.Len() != 0 {
		t.Fatalf("golden AuxPoW has %d trailing bytes", reader.Len())
	}
	if auxPow.ParentBlockHash != (chainhash.Hash{}) {
		t.Fatalf("golden AuxPoW retained legacy parent hash: %s",
			auxPow.ParentBlockHash)
	}

	childHash, err := chainhash.NewHashFromStr(childHashHex)
	if err != nil {
		t.Fatalf("unable to decode child hash: %v", err)
	}
	if err := validateAuxPowCommitment(&auxPow, childHash,
		chaincfg.ObtcAuxPowChainID); err != nil {

		t.Fatalf("Namecoin golden AuxPoW rejected: %v", err)
	}

	auxPow.ParentBlockHash = chainhash.Hash{}
	if err := validateAuxPowCommitment(&auxPow, childHash,
		chaincfg.ObtcAuxPowChainID); err != nil {

		t.Fatalf("zero legacy parent hash field rejected: %v", err)
	}
}

func TestOBTCAuxPowMixedMiningReorg(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-mixed-reorg",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	genesis := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	genesis.SetHeight(0)

	main1 := makeTimedAuxPowChildBlock(t, chain, genesis)
	processAuxPowTestBlock(t, chain, main1, true, false)
	main2 := makeTimedOrdinaryPowChildBlock(t, chain, main1)
	processAuxPowTestBlock(t, chain, main2, true, false)

	side1 := tagAuxPowTestBlock(t, chain,
		makeTimedOrdinaryPowChildBlock(t, chain, genesis), 0x01)
	processAuxPowTestBlock(t, chain, side1, false, false)
	side2 := tagAuxPowTestBlock(t, chain,
		makeTimedAuxPowChildBlock(t, chain, side1), 0x02)
	processAuxPowTestBlock(t, chain, side2, false, false)
	side3 := tagAuxPowTestBlock(t, chain,
		makeTimedOrdinaryPowChildBlock(t, chain, side2), 0x03)
	processAuxPowTestBlock(t, chain, side3, true, false)

	if got, want := chain.BestSnapshot().Hash, *side3.Hash(); got != want {
		t.Fatalf("unexpected tip after mixed mining reorg: got %s want %s",
			got, want)
	}
}

func TestOBTCAuxPowOrphanConnectsAfterParent(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-orphan",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	genesis := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	genesis.SetHeight(0)
	parent := makeTimedAuxPowChildBlock(t, chain, genesis)
	child := makeTimedAuxPowChildBlock(t, chain, parent)

	processAuxPowTestBlock(t, chain, child, false, true)
	processAuxPowTestBlock(t, chain, parent, true, false)

	if got, want := chain.BestSnapshot().Hash, *child.Hash(); got != want {
		t.Fatalf("unexpected tip after orphan connection: got %s want %s",
			got, want)
	}
}

func TestOBTCAuxPowReloadsFromDatabase(t *testing.T) {
	const dbName = "obtc-auxpow-reload"
	params := auxPowTestParams(1)
	chain, teardown, err := chainSetup(dbName, params)
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}

	genesis := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	genesis.SetHeight(0)
	block := makeTimedAuxPowChildBlock(t, chain, genesis)
	processAuxPowTestBlock(t, chain, block, true, false)
	if err := chain.FlushUtxoCache(FlushRequired); err != nil {
		t.Fatalf("unable to flush UTXO cache: %v", err)
	}
	if err := chain.db.Close(); err != nil {
		t.Fatalf("unable to close database: %v", err)
	}

	dbPath := filepath.Join(testDbRoot, dbName)
	reopenedDB, err := database.Open(testDbType, dbPath, blockDataNet)
	if err != nil {
		t.Fatalf("unable to reopen database: %v", err)
	}
	defer func() {
		_ = reopenedDB.Close()
		teardown()
	}()

	reloadedChain, err := New(&Config{
		DB:          reopenedDB,
		ChainParams: params,
		Checkpoints: nil,
		TimeSource:  NewMedianTime(),
		SigCache:    txscript.NewSigCache(1000),
	})
	if err != nil {
		t.Fatalf("unable to reload chain: %v", err)
	}
	if got, want := reloadedChain.BestSnapshot().Hash, *block.Hash(); got != want {
		t.Fatalf("unexpected reloaded tip: got %s want %s", got, want)
	}

	reloadedBlock, err := reloadedChain.BlockByHash(block.Hash())
	if err != nil {
		t.Fatalf("unable to fetch reloaded AuxPoW block: %v", err)
	}
	if reloadedBlock.MsgBlock().AuxPow == nil {
		t.Fatal("reloaded block is missing AuxPoW proof")
	}
	if reloadedBlock.MsgBlock().AuxPow.ParentBlockHash != (chainhash.Hash{}) {
		t.Fatal("reloaded AuxPoW retained legacy parent hash")
	}

	next := makeTimedAuxPowChildBlock(t, reloadedChain, reloadedBlock)
	processAuxPowTestBlock(t, reloadedChain, next, true, false)
}

func TestOBTCAuxPowRejectsProofOnOrdinaryPoWBlock(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-auxpow-proof-on-pow",
		auxPowTestParams(1))
	if err != nil {
		t.Fatalf("unable to setup chain: %v", err)
	}
	defer teardown()

	prev := btcutil.NewBlock(chain.chainParams.GenesisBlock)
	prev.SetHeight(0)
	block, _ := newOBTCBlockWithSpends(t, chain, prev)
	block.MsgBlock().Header.Version = chaincfg.ObtcBlockVersion(false)
	block.MsgBlock().AuxPow = makeTestAuxPow(block.Hash(), chain.chainParams.AuxPowChainID)

	_, _, err = chain.ProcessBlock(block, BFNoPoWCheck)
	assertAuxPowRuleError(t, err, ErrInvalidAuxPow)
}

func TestOBTCAuxPowRejectsHighParentHash(t *testing.T) {
	var childHash chainhash.Hash
	auxPow := makeTestAuxPow(&childHash, chaincfg.ObtcAuxPowChainID)

	err := validateAuxPowProofOfWork(auxPow, big.NewInt(1), BFNone)
	assertAuxPowRuleError(t, err, ErrHighHash)
}
