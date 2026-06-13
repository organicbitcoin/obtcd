// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type forkDAAChain struct {
	params *chaincfg.Params
}

func (c forkDAAChain) ChainParams() *chaincfg.Params {
	return c.params
}

func (c forkDAAChain) BlocksPerRetarget() int32 {
	return int32(c.params.TargetTimespan / c.params.TargetTimePerBlock)
}

func (c forkDAAChain) MinRetargetTimespan() int64 {
	targetTimespan := int64(c.params.TargetTimespan / time.Second)
	return targetTimespan / c.params.RetargetAdjustmentFactor
}

func (c forkDAAChain) MaxRetargetTimespan() int64 {
	targetTimespan := int64(c.params.TargetTimespan / time.Second)
	return targetTimespan * c.params.RetargetAdjustmentFactor
}

func (c forkDAAChain) VerifyCheckpoint(height int32, hash *chainhash.Hash) bool {
	return true
}

func (c forkDAAChain) FindPreviousCheckpoint() (HeaderCtx, error) {
	return nil, nil
}

type forkDAANode struct {
	height    int32
	bits      uint32
	timestamp int64
	parent    *forkDAANode
}

func (n *forkDAANode) Height() int32 {
	return n.height
}

func (n *forkDAANode) Bits() uint32 {
	return n.bits
}

func (n *forkDAANode) Timestamp() int64 {
	return n.timestamp
}

func (n *forkDAANode) Parent() HeaderCtx {
	if n == nil || n.parent == nil {
		return nil
	}
	return n.parent
}

func (n *forkDAANode) RelativeAncestorCtx(distance int32) HeaderCtx {
	node := n
	for i := int32(0); node != nil && i < distance; i++ {
		node = node.parent
	}
	if node == nil {
		return nil
	}
	return node
}

func forkDAAParams() *chaincfg.Params {
	params := chaincfg.ObtcMainNetParams
	params.PowLimit = new(big.Int).Set(chaincfg.ObtcMainNetParams.PowLimit)
	return &params
}

func TestOBTCForkFirstBlockResetsDifficulty(t *testing.T) {
	params := forkDAAParams()
	prev := &forkDAANode{
		height:    chaincfg.ObtcMainNetForkHeight,
		bits:      0x1b0404cb,
		timestamp: time.Unix(1_700_000_000, 0).Unix(),
	}

	bits, err := calcNextRequiredDifficulty(prev,
		time.Unix(prev.timestamp+600, 0), forkDAAChain{params})
	if err != nil {
		t.Fatalf("calcNextRequiredDifficulty: %v", err)
	}
	if bits != 0x1d00ffff {
		t.Fatalf("expected first OBTC block difficulty 0x1d00ffff, got %08x", bits)
	}
}

func TestOBTCForkBootstrapASERTAdjusts(t *testing.T) {
	params := forkDAAParams()
	anchor := &forkDAANode{
		height:    chaincfg.ObtcMainNetFirstBlockHeight,
		bits:      0x1c0ffff0,
		timestamp: time.Unix(1_700_000_000, 0).Unix(),
	}

	bits, err := calcNextRequiredDifficulty(anchor,
		time.Unix(anchor.timestamp+300, 0), forkDAAChain{params})
	if err != nil {
		t.Fatalf("calcNextRequiredDifficulty: %v", err)
	}
	if bits == anchor.bits {
		t.Fatalf("expected bootstrap ASERT to adjust difficulty")
	}
	if CompactToBig(bits).Cmp(CompactToBig(anchor.bits)) >= 0 {
		t.Fatalf("expected early bootstrap block to lower target, got %08x", bits)
	}
}

func TestOBTCForkNormalASERTUsesActivationAnchor(t *testing.T) {
	params := forkDAAParams()
	anchorTime := time.Unix(1_700_000_000, 0)
	anchor := &forkDAANode{
		height:    chaincfg.ObtcMainNetActivationHeight,
		bits:      0x1c0ffff0,
		timestamp: anchorTime.Unix(),
	}

	last := anchor
	for i := int32(1); i <= 10; i++ {
		last = &forkDAANode{
			height:    anchor.height + i,
			bits:      0x1b0ffff0,
			timestamp: anchor.timestamp + int64(i*600),
			parent:    last,
		}
	}

	bits, err := calcNextRequiredDifficulty(last,
		anchorTime.Add(11*10*time.Minute), forkDAAChain{params})
	if err != nil {
		t.Fatalf("calcNextRequiredDifficulty: %v", err)
	}
	if bits != anchor.bits {
		t.Fatalf("expected normal ASERT to use activation anchor bits %08x, got %08x",
			anchor.bits, bits)
	}
}

func TestOBTCForkDAALongChainBoundary(t *testing.T) {
	params := forkDAAParams()
	chain := forkDAAChain{params}
	startTime := time.Unix(1_700_000_000, 0)

	prev := &forkDAANode{
		height:    chaincfg.ObtcMainNetForkHeight,
		bits:      0x1b0404cb,
		timestamp: startTime.Add(-10 * time.Minute).Unix(),
	}

	var first *forkDAANode
	last := prev
	for height := chaincfg.ObtcMainNetFirstBlockHeight; height <= chaincfg.ObtcMainNetActivationHeight; height++ {
		blockTime := startTime.Add(time.Duration(height-chaincfg.ObtcMainNetFirstBlockHeight) *
			10 * time.Minute)
		bits, err := calcNextRequiredDifficulty(last, blockTime, chain)
		if err != nil {
			t.Fatalf("height %d calcNextRequiredDifficulty: %v", height, err)
		}
		node := &forkDAANode{
			height:    height,
			bits:      bits,
			timestamp: blockTime.Unix(),
			parent:    last,
		}
		if height == chaincfg.ObtcMainNetFirstBlockHeight {
			first = node
		}
		last = node
	}

	if first.bits != params.ForkDAAForkResetBits {
		t.Fatalf("first block bits got %08x want %08x",
			first.bits, params.ForkDAAForkResetBits)
	}
	if last.height != chaincfg.ObtcMainNetActivationHeight {
		t.Fatalf("unexpected activation node height %d", last.height)
	}

	nextTime := startTime.Add(time.Duration(chaincfg.ObtcMainNetActivationHeight-
		chaincfg.ObtcMainNetFirstBlockHeight+1) * 10 * time.Minute)
	bits, err := calcNextRequiredDifficulty(last, nextTime, chain)
	if err != nil {
		t.Fatalf("post-activation calcNextRequiredDifficulty: %v", err)
	}
	if bits != last.bits {
		t.Fatalf("expected exact-schedule normal ASERT to keep activation bits %08x, got %08x",
			last.bits, bits)
	}

	fastBits, err := calcNextRequiredDifficulty(last, nextTime.Add(-5*time.Minute), chain)
	if err != nil {
		t.Fatalf("fast post-activation calcNextRequiredDifficulty: %v", err)
	}
	if CompactToBig(fastBits).Cmp(CompactToBig(last.bits)) >= 0 {
		t.Fatalf("expected fast post-activation block to lower target, got %08x from %08x",
			fastBits, last.bits)
	}
}

func TestOBTCForkHeaderAcceptsResetDifficulty(t *testing.T) {
	params := forkDAAParams()
	prev := &forkDAANode{
		height:    chaincfg.ObtcMainNetForkHeight,
		bits:      0x1b0404cb,
		timestamp: time.Unix(1_700_000_000, 0).Unix(),
	}
	header := &wire.BlockHeader{
		Version:   4,
		Bits:      params.ForkDAAForkResetBits,
		Timestamp: time.Unix(prev.timestamp+600, 0),
	}

	if err := CheckBlockHeaderContext(header, prev, 0, forkDAAChain{params}, true); err != nil {
		t.Fatalf("expected OBTC H+1 header context to pass: %v", err)
	}
}

func TestOBTCForkHeaderRejectsBitcoinHPlusOneDifficulty(t *testing.T) {
	params := forkDAAParams()
	prev := &forkDAANode{
		height:    chaincfg.ObtcMainNetForkHeight,
		bits:      0x1b0404cb,
		timestamp: time.Unix(1_700_000_000, 0).Unix(),
	}
	header := &wire.BlockHeader{
		Version:   4,
		Bits:      prev.bits,
		Timestamp: time.Unix(prev.timestamp+600, 0),
	}

	err := CheckBlockHeaderContext(header, prev, 0, forkDAAChain{params}, true)
	if err == nil {
		t.Fatal("expected BTC H+1 difficulty to be rejected")
	}
	ruleErr, ok := err.(RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T: %v", err, err)
	}
	if ruleErr.ErrorCode != ErrUnexpectedDifficulty {
		t.Fatalf("unexpected error code: got %v want %v",
			ruleErr.ErrorCode, ErrUnexpectedDifficulty)
	}
}
