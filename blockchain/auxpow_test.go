// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain/internal/testhelper"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func auxPowTestParams(startHeight int32) *chaincfg.Params {
	params := chaincfg.ObtcRegTestParams
	params.AuxPowChainID = chaincfg.ObtcAuxPowChainID
	params.AuxPowStartHeight = startHeight
	params.AuxPowForkResetBits = params.PowLimitBits
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
