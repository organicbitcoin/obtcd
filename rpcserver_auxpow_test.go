// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func TestAuxPowRPCHandlersRegistered(t *testing.T) {
	for _, method := range []string{
		"createauxblock",
		"submitauxblock",
		"getauxblock",
	} {
		if rpcHandlers[method] == nil {
			t.Fatalf("expected RPC handler for %s", method)
		}
	}
}

func testRPCAuxPow() *wire.AuxPow {
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		SignatureScript:  []byte{0xfa, 0xbe, 0x6d, 0x6d},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: []byte{0x51}})

	parentHash := chainhash.DoubleHashH([]byte("rpc-parent"))
	return &wire.AuxPow{
		CoinbaseTx:      *tx,
		ParentBlockHash: parentHash,
		ParentHeader: wire.BlockHeader{
			Version:   1,
			PrevBlock: parentHash,
			Bits:      0x1d00ffff,
			Timestamp: time.Unix(1_700_000_000, 0),
		},
	}
}

func encodeRPCAuxPow(t *testing.T, auxPow *wire.AuxPow) string {
	t.Helper()

	var buf bytes.Buffer
	if err := auxPow.BtcEncode(&buf, 0, wire.WitnessEncoding); err != nil {
		t.Fatalf("BtcEncode: %v", err)
	}
	return hex.EncodeToString(buf.Bytes())
}

func TestDecodeAuxPowHex(t *testing.T) {
	encoded := encodeRPCAuxPow(t, testRPCAuxPow())
	if _, err := decodeAuxPowHex(encoded); err != nil {
		t.Fatalf("decodeAuxPowHex valid proof: %v", err)
	}
	if _, err := decodeAuxPowHex(encoded + "00"); err == nil {
		t.Fatal("expected trailing data error")
	}
}

func TestAuxWorkStateCopiesTemplates(t *testing.T) {
	state := newAuxWorkState()
	block := wire.NewMsgBlock(&wire.BlockHeader{
		Version:   chaincfg.ObtcBlockVersion(true),
		Bits:      0x1d00ffff,
		Timestamp: time.Unix(1_700_000_000, 0),
	})
	block.AuxPow = testRPCAuxPow()
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex}})
	tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	block.Transactions = []*wire.MsgTx{tx}

	hash := block.BlockHash()
	state.store(&hash, block)

	block.Header.Nonce = 99
	firstLookup := state.lookup(&hash)
	if firstLookup == nil {
		t.Fatal("expected stored template")
	}
	if firstLookup.Header.Nonce == 99 {
		t.Fatal("stored template was mutated by caller after store")
	}

	firstLookup.Header.Nonce = 100
	secondLookup := state.lookup(&hash)
	if secondLookup.Header.Nonce == 100 {
		t.Fatal("stored template was mutated by lookup result")
	}
}

type auxPowSyncManagerStub struct {
	submitted *btcutil.Block
	err       error
}

func (s *auxPowSyncManagerStub) IsCurrent() bool {
	return true
}

func (s *auxPowSyncManagerStub) SubmitBlock(block *btcutil.Block,
	flags blockchain.BehaviorFlags) (bool, error) {

	s.submitted = block
	return false, s.err
}

func (s *auxPowSyncManagerStub) Pause() chan<- struct{} {
	return make(chan struct{})
}

func (s *auxPowSyncManagerStub) SyncPeerID() int32 {
	return 0
}

func (s *auxPowSyncManagerStub) LocateHeaders(locators []*chainhash.Hash,
	hashStop *chainhash.Hash) []wire.BlockHeader {

	return nil
}

func testAuxWorkBlock() *wire.MsgBlock {
	block := wire.NewMsgBlock(&wire.BlockHeader{
		Version:   chaincfg.ObtcBlockVersion(true),
		Bits:      0x1d00ffff,
		Timestamp: time.Unix(1_700_000_000, 0),
	})
	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: wire.MaxPrevOutIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{0x51}})
	block.Transactions = []*wire.MsgTx{tx}
	return block
}

func TestSubmitAuxBlockRoutesCachedTemplate(t *testing.T) {
	syncMgr := &auxPowSyncManagerStub{}
	server := &rpcServer{
		cfg:          rpcserverConfig{SyncMgr: syncMgr},
		auxWorkState: newAuxWorkState(),
	}
	block := testAuxWorkBlock()
	hash := block.BlockHash()
	server.auxWorkState.store(&hash, block)

	result, err := submitAuxBlock(server, hash.String(),
		encodeRPCAuxPow(t, testRPCAuxPow()))
	if err != nil {
		t.Fatalf("submitAuxBlock: %v", err)
	}
	if result != true {
		t.Fatalf("unexpected submit result: got %v want true", result)
	}
	if syncMgr.submitted == nil {
		t.Fatal("expected block submission")
	}
	if !syncMgr.submitted.Hash().IsEqual(&hash) {
		t.Fatalf("submitted hash mismatch: got %s want %s", syncMgr.submitted.Hash(), hash)
	}
	if syncMgr.submitted.MsgBlock().AuxPow == nil {
		t.Fatal("submitted block is missing AuxPoW proof")
	}
}

func TestSubmitAuxBlockRejectsUnknownTemplate(t *testing.T) {
	server := &rpcServer{auxWorkState: newAuxWorkState()}
	hash := chainhash.DoubleHashH([]byte("unknown"))

	if _, err := submitAuxBlock(server, hash.String(),
		encodeRPCAuxPow(t, testRPCAuxPow())); err == nil {

		t.Fatal("expected unknown template error")
	}
}

func TestSubmitAuxBlockReturnsFalseOnRejectedBlock(t *testing.T) {
	syncMgr := &auxPowSyncManagerStub{err: errors.New("rejected")}
	server := &rpcServer{
		cfg:          rpcserverConfig{SyncMgr: syncMgr},
		auxWorkState: newAuxWorkState(),
	}
	block := testAuxWorkBlock()
	hash := block.BlockHash()
	server.auxWorkState.store(&hash, block)

	result, err := submitAuxBlock(server, hash.String(),
		encodeRPCAuxPow(t, testRPCAuxPow()))
	if err != nil {
		t.Fatalf("submitAuxBlock: %v", err)
	}
	if result != false {
		t.Fatalf("unexpected submit result: got %v want false", result)
	}
}

func TestAuxWorkStateBoundsTemplates(t *testing.T) {
	state := newAuxWorkState()
	for nonce := uint32(0); nonce < maxAuxWorkItems+1; nonce++ {
		block := testAuxWorkBlock()
		block.Header.Nonce = nonce
		hash := block.BlockHash()
		state.store(&hash, block)
	}
	if got := len(state.templates); got != maxAuxWorkItems {
		t.Fatalf("unexpected template count: got %d want %d", got, maxAuxWorkItems)
	}
}

func TestAuxBlockTargetUsesPoolByteOrder(t *testing.T) {
	const want = "0000000000000000000000000000000000000000000000000000ffff00000000"
	if got := auxBlockTarget(0x1d00ffff); got != want {
		t.Fatalf("unexpected AuxPoW target: got %s want %s", got, want)
	}
}
