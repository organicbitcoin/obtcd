// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC license.

package blockchain

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
)

// Replay real Bitcoin mainnet blocks with ordinary proof-of-work, transaction
// and script checks, starting from an empty OBTC chain database.
func TestOBTCSharedHistoryColdSync(t *testing.T) {
	chain, teardown, err := chainSetup("obtc-shared-history", &chaincfg.ObtcMainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	blocks, err := loadBlocks("blk_0_to_14131.dat")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 14132 || *blocks[0].Hash() != *chaincfg.MainNetParams.GenesisHash {
		t.Fatal("incomplete Bitcoin history fixture")
	}
	for h, block := range blocks[1:] {
		main, orphan, err := chain.ProcessBlock(block, BFNone)
		if err != nil || !main || orphan {
			t.Fatalf("Bitcoin block %d: main=%v orphan=%v err=%v", h+1, main, orphan, err)
		}
	}
	tip := chain.BestSnapshot()
	if tip.Height != int32(len(blocks)-1) || tip.Hash.String() != "00000000b3e750f37fdb42e1018799a9f44b546d393b130b369590a072430a1c" {
		t.Fatalf("unexpected tip %v", tip)
	}
	for _, id := range []uint32{chaincfg.DeploymentCSV, chaincfg.DeploymentSegwit, chaincfg.DeploymentTaproot} {
		state, err := chain.ThresholdState(id)
		if err != nil || state != ThresholdDefined {
			t.Errorf("early deployment %d: state=%v err=%v", id, state, err)
		}
	}
}

func TestOBTCSharedHistoryBoundary(t *testing.T) {
	obtc, btc := &chaincfg.ObtcMainNetParams, &chaincfg.MainNetParams
	h := chaincfg.ObtcMainNetForkHeight
	for _, test := range []struct {
		network *chaincfg.Params
		height  int32
		rules   *chaincfg.Params
	}{
		{obtc, h - 1, btc}, {obtc, h, btc}, {obtc, h + 1, obtc},
		{&chaincfg.ObtcTestNetParams, 1, &chaincfg.ObtcTestNetParams},
		{&chaincfg.ObtcRegTestParams, 1, &chaincfg.ObtcRegTestParams}, {btc, h + 1, btc},
	} {
		if got := chaincfg.ConsensusParamsAtHeight(test.network, test.height); got != test.rules {
			t.Errorf("%s at %d uses %s", test.network.Name, test.height, got.Name)
		}
	}
	chain := newFakeChain(obtc)
	for _, id := range []uint32{chaincfg.DeploymentCSV, chaincfg.DeploymentSegwit, chaincfg.DeploymentTaproot} {
		before, oldCache := chain.deploymentForNextBlock(&blockNode{height: h - 1}, id)
		after, newCache := chain.deploymentForNextBlock(&blockNode{height: h}, id)
		if before.params != btc || after.params != obtc || oldCache == newCache ||
			before.deployment.DeploymentStarter == btc.Deployments[id].DeploymentStarter ||
			!after.ForceActive(&blockNode{height: h}) {
			t.Fatalf("wrong fork transition for deployment %d", id)
		}
	}
	for _, height := range []int32{h, h + 1, chaincfg.ObtcMainNetActivationHeight - 1} {
		if reapRulesActive(obtc, height) {
			t.Errorf("unexpected early REAP activation at %d", height)
		}
	}
	if !reapRulesActive(obtc, chaincfg.ObtcMainNetActivationHeight) {
		t.Fatal("missing REAP activation")
	}
}

// Exercise Bitcoin's real voting windows on a synthetic signalling branch.
// At this median time CSV and SegWit can be voted in, while Taproot is dormant.
func TestOBTCSharedHistoryVotingWindows(t *testing.T) {
	chain := newFakeChain(&chaincfg.ObtcMainNetParams)
	node := chain.bestChain.Tip()
	when := time.Date(2017, time.March, 1, 0, 0, 0, 0, time.UTC)
	window := int32(chaincfg.MainNetParams.MinerConfirmationWindow)
	states := []ThresholdState{ThresholdStarted, ThresholdLockedIn, ThresholdActive}
	for i, want := range states {
		for node.height+1 < int32(i+1)*window {
			when = when.Add(time.Second)
			node = newFakeNode(node, vbTopBits|3, 0, when)
			chain.index.AddNode(node)
		}
		for _, id := range []uint32{chaincfg.DeploymentCSV, chaincfg.DeploymentSegwit, chaincfg.DeploymentTaproot} {
			expected := want
			if id == chaincfg.DeploymentTaproot {
				expected = ThresholdDefined
			}
			got, err := chain.deploymentState(node, id)
			if err != nil || got != expected {
				t.Fatalf("window %d deployment %d: got %v want %v, err %v", i+1, id, got, expected, err)
			}
		}
	}
}
