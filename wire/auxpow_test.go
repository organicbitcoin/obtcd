// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/davecgh/go-spew/spew"
)

func testAuxPowTx(script []byte) MsgTx {
	tx := NewMsgTx(1)
	tx.AddTxIn(&TxIn{
		PreviousOutPoint: OutPoint{Index: MaxPrevOutIndex},
		SignatureScript:  script,
		Sequence:         MaxTxInSequenceNum,
	})
	tx.AddTxOut(&TxOut{Value: 50, PkScript: []byte{0x51}})
	return *tx
}

func testAuxPowProof() *AuxPow {
	coinbaseBranch := chainhash.DoubleHashH([]byte("coinbase-branch"))
	chainBranch := chainhash.DoubleHashH([]byte("chain-branch"))
	return &AuxPow{
		CoinbaseTx:           testAuxPowTx([]byte{0xfa, 0xbe, 0x6d, 0x6d, 1, 2, 3}),
		CoinbaseMerkleBranch: []chainhash.Hash{coinbaseBranch},
		CoinbaseBranchMask:   0,
		ChainMerkleBranch:    []chainhash.Hash{chainBranch},
		ChainBranchMask:      1,
		ParentHeader: BlockHeader{
			Version:   1,
			PrevBlock: chainhash.DoubleHashH([]byte("parent")),
			Bits:      0x1d00ffff,
			Timestamp: time.Unix(1_700_000_000, 0),
			Nonce:     42,
		},
	}
}

func TestAuxPowWireRoundTrip(t *testing.T) {
	auxPow := testAuxPowProof()

	var buf bytes.Buffer
	if err := auxPow.BtcEncode(&buf, ProtocolVersion, WitnessEncoding); err != nil {
		t.Fatalf("BtcEncode: %v", err)
	}
	if got, want := buf.Len(), auxPow.SerializeSize(); got != want {
		t.Fatalf("SerializeSize mismatch: got %d want %d", got, want)
	}

	var decoded AuxPow
	if err := decoded.BtcDecode(&buf, ProtocolVersion, WitnessEncoding); err != nil {
		t.Fatalf("BtcDecode: %v", err)
	}
	if !reflect.DeepEqual(&decoded, auxPow) {
		t.Fatalf("decoded AuxPoW mismatch:\ngot  %s\nwant %s",
			spew.Sdump(&decoded), spew.Sdump(auxPow))
	}
}

func TestAuxPowNormalizesLegacyParentHash(t *testing.T) {
	auxPow := testAuxPowProof()
	var encoded bytes.Buffer
	if err := auxPow.BtcEncode(&encoded, ProtocolVersion, WitnessEncoding); err != nil {
		t.Fatalf("BtcEncode: %v", err)
	}

	payload := encoded.Bytes()
	parentHashStart := auxPow.CoinbaseTx.SerializeSize()
	for i := parentHashStart; i < parentHashStart+chainhash.HashSize; i++ {
		payload[i] = 0x42
	}

	var decoded AuxPow
	if err := decoded.BtcDecode(bytes.NewReader(payload), ProtocolVersion,
		WitnessEncoding); err != nil {

		t.Fatalf("BtcDecode: %v", err)
	}
	if decoded.ParentBlockHash != (chainhash.Hash{}) {
		t.Fatalf("legacy parent hash was retained: %s", decoded.ParentBlockHash)
	}

	encoded.Reset()
	if err := decoded.BtcEncode(&encoded, ProtocolVersion, WitnessEncoding); err != nil {
		t.Fatalf("normalized BtcEncode: %v", err)
	}
	payload = encoded.Bytes()
	for _, b := range payload[parentHashStart : parentHashStart+chainhash.HashSize] {
		if b != 0 {
			t.Fatal("legacy parent hash was not serialized as zero")
		}
	}
}

func TestAuxPowMsgBlockRoundTripAndTxLoc(t *testing.T) {
	tx := testAuxPowTx([]byte{0x01, 0x02})
	msgBlock := &MsgBlock{
		Header: BlockHeader{
			Version:    AuxPowVersionFlag | 4,
			MerkleRoot: tx.TxHash(),
			Bits:       0x1d00ffff,
			Timestamp:  time.Unix(1_700_000_100, 0),
		},
		AuxPow:       testAuxPowProof(),
		Transactions: []*MsgTx{&tx},
	}

	var buf bytes.Buffer
	if err := msgBlock.Serialize(&buf); err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	serialized := append([]byte(nil), buf.Bytes()...)
	if got, want := len(serialized), msgBlock.SerializeSize(); got != want {
		t.Fatalf("SerializeSize mismatch: got %d want %d", got, want)
	}

	var decoded MsgBlock
	if err := decoded.Deserialize(bytes.NewReader(serialized)); err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if decoded.AuxPow == nil {
		t.Fatal("expected decoded AuxPoW proof")
	}
	if !reflect.DeepEqual(&decoded, msgBlock) {
		t.Fatalf("decoded block mismatch:\ngot  %s\nwant %s",
			spew.Sdump(&decoded), spew.Sdump(msgBlock))
	}

	var txLocBlock MsgBlock
	txLocs, err := txLocBlock.DeserializeTxLoc(bytes.NewBuffer(serialized))
	if err != nil {
		t.Fatalf("DeserializeTxLoc: %v", err)
	}
	if len(txLocs) != 1 {
		t.Fatalf("expected one tx loc, got %d", len(txLocs))
	}
	if txLocs[0].TxStart <= MaxBlockHeaderPayload {
		t.Fatalf("tx loc starts before auxpow payload: %d", txLocs[0].TxStart)
	}
}

func TestAuxPowMsgBlockEncodeErrors(t *testing.T) {
	tx := testAuxPowTx([]byte{0x01})
	tests := []struct {
		name string
		msg  *MsgBlock
	}{
		{
			name: "version bit without proof",
			msg: &MsgBlock{
				Header:       BlockHeader{Version: AuxPowVersionFlag | 4},
				Transactions: []*MsgTx{&tx},
			},
		},
		{
			name: "proof without version bit",
			msg: &MsgBlock{
				Header:       BlockHeader{Version: 4},
				AuxPow:       testAuxPowProof(),
				Transactions: []*MsgTx{&tx},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := test.msg.Serialize(&buf); err == nil {
				t.Fatal("expected serialize error")
			}
		})
	}
}

func TestAuxPowMsgBlockWitnessSizes(t *testing.T) {
	tx := testAuxPowTx([]byte{0x01})
	msgBlock := &MsgBlock{
		Header: BlockHeader{
			Version: AuxPowVersionFlag | 4,
		},
		AuxPow:       testAuxPowProof(),
		Transactions: []*MsgTx{&tx},
	}
	msgBlock.AuxPow.CoinbaseTx.TxIn[0].Witness = TxWitness{[]byte{0x01, 0x02}}

	var serialized bytes.Buffer
	if err := msgBlock.Serialize(&serialized); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if got, want := serialized.Len(), msgBlock.SerializeSize(); got != want {
		t.Fatalf("SerializeSize mismatch: got %d want %d", got, want)
	}

	var stripped bytes.Buffer
	if err := msgBlock.SerializeNoWitness(&stripped); err != nil {
		t.Fatalf("SerializeNoWitness: %v", err)
	}
	if got, want := stripped.Len(), msgBlock.SerializeSizeStripped(); got != want {
		t.Fatalf("SerializeSizeStripped mismatch: got %d want %d", got, want)
	}
	if serialized.Len() <= stripped.Len() {
		t.Fatalf("witness encoding should be larger: full %d stripped %d",
			serialized.Len(), stripped.Len())
	}
}

func TestAuxPowCopyDeepCopy(t *testing.T) {
	auxPow := testAuxPowProof()
	cp := auxPow.Copy()
	cp.CoinbaseTx.TxIn[0].SignatureScript[0] ^= 0xff
	cp.CoinbaseMerkleBranch[0][0] ^= 0xff
	cp.ChainMerkleBranch[0][0] ^= 0xff

	if reflect.DeepEqual(cp, auxPow) {
		t.Fatal("mutated AuxPoW copy still equals original")
	}
	if auxPow.CoinbaseTx.TxIn[0].SignatureScript[0] != 0xfa {
		t.Fatal("mutating AuxPoW copy changed original coinbase")
	}
	if cp.CoinbaseMerkleBranch[0] == auxPow.CoinbaseMerkleBranch[0] {
		t.Fatal("mutating AuxPoW copy changed original coinbase branch")
	}
	if cp.ChainMerkleBranch[0] == auxPow.ChainMerkleBranch[0] {
		t.Fatal("mutating AuxPoW copy changed original chain branch")
	}
}

func TestAuxPowBranchLimits(t *testing.T) {
	auxPow := testAuxPowProof()
	auxPow.ChainMerkleBranch = make([]chainhash.Hash, MaxAuxPowMerkleBranchHashes+1)

	var encoded bytes.Buffer
	if err := auxPow.BtcEncode(&encoded, ProtocolVersion, WitnessEncoding); err == nil {
		t.Fatal("expected oversized branch encode error")
	}

	encoded.Reset()
	buf := binarySerializer.Borrow()
	defer binarySerializer.Return(buf)
	if err := WriteVarIntBuf(&encoded, ProtocolVersion,
		MaxAuxPowMerkleBranchHashes+1, buf); err != nil {

		t.Fatalf("WriteVarIntBuf: %v", err)
	}
	if _, err := readAuxPowBranch(&encoded, ProtocolVersion, buf,
		"test branch"); err == nil {

		t.Fatal("expected oversized branch decode error")
	}
}

func FuzzAuxPowDecode(f *testing.F) {
	var seed bytes.Buffer
	if err := testAuxPowProof().BtcEncode(&seed, ProtocolVersion,
		WitnessEncoding); err != nil {

		f.Fatalf("BtcEncode seed: %v", err)
	}
	f.Add(seed.Bytes())
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, payload []byte) {
		var auxPow AuxPow
		_ = auxPow.BtcDecode(bytes.NewReader(payload), ProtocolVersion,
			WitnessEncoding)
	})
}
