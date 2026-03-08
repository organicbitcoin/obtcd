//go:build rpctest
// +build rpctest

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/stretchr/testify/require"
)

const netsyncJoinTimeout = time.Minute

func TestNetsyncSyncsToLongerCompetingBranch(t *testing.T) {
	shorter := newNetsyncHarness(t)
	longer := newNetsyncHarness(t)

	shorterBlocks, err := shorter.Client.Generate(2)
	require.NoError(t, err)
	shorterTip := shorterBlocks[len(shorterBlocks)-1]

	longerBlocks, err := longer.Client.Generate(4)
	require.NoError(t, err)
	longerTip := longerBlocks[len(longerBlocks)-1]

	require.NoError(t, rpctest.ConnectNode(shorter, longer))
	joinBlocks(t, shorter, longer)

	assertBestBlock(t, shorter, longerTip, 4)
	assertBestBlock(t, longer, longerTip, 4)

	assertChainTipEventually(t, shorter, btcjson.GetChainTipsResult{
		Height:    4,
		Hash:      longerTip.String(),
		BranchLen: 0,
		Status:    "active",
	})
	assertChainTipEventually(t, shorter, btcjson.GetChainTipsResult{
		Height:    2,
		Hash:      shorterTip.String(),
		BranchLen: 2,
		Status:    "valid-fork",
	})
}

func TestNetsyncReconnectCatchesUpDelayedPeer(t *testing.T) {
	leader := newNetsyncHarness(t)
	follower := newNetsyncHarness(t)

	require.NoError(t, rpctest.ConnectNode(follower, leader))
	joinBlocks(t, leader, follower)

	initialBlocks, err := leader.Client.Generate(2)
	require.NoError(t, err)
	initialTip := initialBlocks[len(initialBlocks)-1]

	joinBlocks(t, leader, follower)
	assertBestBlock(t, leader, initialTip, 2)
	assertBestBlock(t, follower, initialTip, 2)

	disconnectPeer(t, follower)

	advancedBlocks, err := leader.Client.Generate(3)
	require.NoError(t, err)
	advancedTip := advancedBlocks[len(advancedBlocks)-1]

	assertBestBlock(t, leader, advancedTip, 5)
	assertBestBlock(t, follower, initialTip, 2)

	require.NoError(t, rpctest.ConnectNode(follower, leader))
	joinBlocks(t, leader, follower)

	assertBestBlock(t, leader, advancedTip, 5)
	assertBestBlock(t, follower, advancedTip, 5)
}

func newNetsyncHarness(t *testing.T) *rpctest.Harness {
	t.Helper()

	h, err := rpctest.New(&chaincfg.RegressionNetParams, nil, nil, "")
	require.NoError(t, err)
	require.NoError(t, h.SetUp(false, 0))

	t.Cleanup(func() {
		require.NoError(t, h.TearDown())
	})

	return h
}

func joinBlocks(t *testing.T, nodes ...*rpctest.Harness) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- rpctest.JoinNodes(nodes, rpctest.Blocks)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(netsyncJoinTimeout):
		t.Fatalf("nodes never synchronized their best chains")
	}
}

func disconnectPeer(t *testing.T, from *rpctest.Harness) {
	t.Helper()

	addr := singlePeerAddr(t, from)
	require.NoError(t, from.Client.AddNode(addr, rpcclient.ANRemove))

	require.Eventually(t, func() bool {
		count, err := from.Client.GetConnectionCount()
		return err == nil && count == 0
	}, 10*time.Second, 100*time.Millisecond)
}

func singlePeerAddr(t *testing.T, h *rpctest.Harness) string {
	t.Helper()

	peers, err := h.Client.GetPeerInfo()
	require.NoError(t, err)
	require.Len(t, peers, 1)

	return peers[0].Addr
}

func assertBestBlock(t *testing.T, h *rpctest.Harness, wantHash *chainhash.Hash,
	wantHeight int32) {
	t.Helper()

	bestHash, bestHeight, err := h.Client.GetBestBlock()
	require.NoError(t, err)
	require.Equal(t, wantHeight, bestHeight)
	require.Equal(t, *wantHash, *bestHash)
}

func assertChainTipEventually(t *testing.T, h *rpctest.Harness,
	want btcjson.GetChainTipsResult) {
	t.Helper()

	require.Eventually(t, func() bool {
		tips, err := h.Client.GetChainTips()
		if err != nil {
			return false
		}

		for _, tip := range tips {
			if tip.Hash != want.Hash {
				continue
			}

			return tip.Height == want.Height &&
				tip.BranchLen == want.BranchLen &&
				tip.Status == want.Status
		}

		return false
	}, 10*time.Second, 100*time.Millisecond, fmt.Sprintf(
		"missing expected chain tip %s with status %s", want.Hash,
		want.Status,
	))
}
