//go:build rpctest
// +build rpctest

package integration

import (
	"math"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestNetsyncSyncsWithPreexistingOrphan(t *testing.T) {
	leader := newNetsyncHarness(t)
	follower := newNetsyncHarness(t)

	parent := createHarnessBlock(t, leader, bestHarnessBlock(t, leader))
	child := createHarnessBlock(t, leader, parent)

	require.NoError(t, follower.Client.SubmitBlock(child, nil))
	assertBestBlock(t, follower, chaincfg.RegressionNetParams.GenesisHash, 0)

	tips, err := follower.Client.GetChainTips()
	require.NoError(t, err)
	require.Len(t, tips, 1)
	require.Equal(t, chaincfg.RegressionNetParams.GenesisHash.String(), tips[0].Hash)
	require.Equal(t, "active", tips[0].Status)

	require.NoError(t, leader.Client.SubmitBlock(parent, nil))
	require.NoError(t, leader.Client.SubmitBlock(child, nil))
	assertBestBlock(t, leader, child.Hash(), 2)

	require.NoError(t, rpctest.ConnectNode(follower, leader))
	joinBlocks(t, leader, follower)

	assertBestBlock(t, leader, child.Hash(), 2)
	assertBestBlock(t, follower, child.Hash(), 2)
}

func TestNetsyncRejectsInvalidBlockButLaterSyncsValidTip(t *testing.T) {
	leader := newNetsyncHarness(t)
	follower := newNetsyncHarness(t)

	valid := createHarnessBlock(t, leader, bestHarnessBlock(t, leader))
	invalid := createMerkleInvalidBlock(t, valid)

	err := follower.Client.SubmitBlock(invalid, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rejected:")
	require.Contains(t, err.Error(), "merkle root is invalid")

	assertBestBlock(t, follower, chaincfg.RegressionNetParams.GenesisHash, 0)

	tips, tipErr := follower.Client.GetChainTips()
	require.NoError(t, tipErr)
	require.Len(t, tips, 1)
	require.Equal(t, chaincfg.RegressionNetParams.GenesisHash.String(), tips[0].Hash)
	require.Equal(t, "active", tips[0].Status)

	require.NoError(t, leader.Client.SubmitBlock(valid, nil))
	assertBestBlock(t, leader, valid.Hash(), 1)

	require.NoError(t, rpctest.ConnectNode(follower, leader))
	joinBlocks(t, leader, follower)

	assertBestBlock(t, leader, valid.Hash(), 1)
	assertBestBlock(t, follower, valid.Hash(), 1)
}

func bestHarnessBlock(t *testing.T, h *rpctest.Harness) *btcutil.Block {
	t.Helper()

	bestHash, bestHeight, err := h.Client.GetBestBlock()
	require.NoError(t, err)

	msgBlock, err := h.Client.GetBlock(bestHash)
	require.NoError(t, err)

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(bestHeight)
	return block
}

func createHarnessBlock(t *testing.T, h *rpctest.Harness, prev *btcutil.Block) *btcutil.Block {
	t.Helper()

	addr, err := h.NewAddress()
	require.NoError(t, err)

	block, err := rpctest.CreateBlock(prev, nil, rpctest.BlockVersion,
		time.Time{}, addr, nil, h.ActiveNet)
	require.NoError(t, err)

	return block
}

func createMerkleInvalidBlock(t *testing.T, block *btcutil.Block) *btcutil.Block {
	t.Helper()

	msgBlock := block.MsgBlock().Copy()
	msgBlock.Header.MerkleRoot[0] ^= 0x01
	msgBlock.Header.Nonce = 0
	solveBlockHeader(t, &msgBlock.Header)

	invalid := btcutil.NewBlock(msgBlock)
	invalid.SetHeight(block.Height())
	return invalid
}

func solveBlockHeader(t *testing.T, header *wire.BlockHeader) {
	t.Helper()

	target := blockchain.CompactToBig(header.Bits)
	for nonce := uint32(0); ; nonce++ {
		header.Nonce = nonce

		hash := header.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			return
		}

		if nonce == math.MaxUint32 {
			t.Fatal("unable to solve block header")
		}
	}
}
