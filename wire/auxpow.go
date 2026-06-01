// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

const (
	// AuxPowVersionFlag is the legacy block-version bit used by Namecoin-style
	// AuxPoW blocks.
	AuxPowVersionFlag int32 = 1 << 8

	// MaxAuxPowMerkleBranchHashes bounds AuxPoW merkle branches to prevent
	// malicious allocations while leaving ample room for multi-aux commitments.
	MaxAuxPowMerkleBranchHashes = 32
)

// IsAuxPowVersion reports whether a block version declares an AuxPoW proof.
func IsAuxPowVersion(version int32) bool {
	return version&AuxPowVersionFlag != 0
}

// AuxPow carries the parent-chain proof required to validate a merged-mined
// block.  The layout follows the Namecoin-style serialization used by existing
// merged-mining pool infrastructure.
type AuxPow struct {
	CoinbaseTx MsgTx
	// ParentBlockHash is the legacy CMerkleTx hashBlock field.  It is retained
	// for Namecoin-style wire compatibility but normalized to zero during
	// decoding and encoding because it is redundant and never validated.
	ParentBlockHash      chainhash.Hash
	CoinbaseMerkleBranch []chainhash.Hash
	CoinbaseBranchMask   uint32
	ChainMerkleBranch    []chainhash.Hash
	ChainBranchMask      uint32
	ParentHeader         BlockHeader
}

// Copy creates a deep copy of the AuxPow proof.
func (aux *AuxPow) Copy() *AuxPow {
	if aux == nil {
		return nil
	}

	cp := &AuxPow{
		CoinbaseTx:           *aux.CoinbaseTx.Copy(),
		ParentBlockHash:      aux.ParentBlockHash,
		CoinbaseBranchMask:   aux.CoinbaseBranchMask,
		ChainBranchMask:      aux.ChainBranchMask,
		ParentHeader:         aux.ParentHeader,
		CoinbaseMerkleBranch: make([]chainhash.Hash, len(aux.CoinbaseMerkleBranch)),
		ChainMerkleBranch:    make([]chainhash.Hash, len(aux.ChainMerkleBranch)),
	}
	copy(cp.CoinbaseMerkleBranch, aux.CoinbaseMerkleBranch)
	copy(cp.ChainMerkleBranch, aux.ChainMerkleBranch)
	return cp
}

func readAuxPowBranch(r io.Reader, pver uint32, buf []byte,
	fieldName string) ([]chainhash.Hash, error) {

	count, err := ReadVarIntBuf(r, pver, buf)
	if err != nil {
		return nil, err
	}
	if count > MaxAuxPowMerkleBranchHashes {
		str := fmt.Sprintf("%s too long [count %d, max %d]",
			fieldName, count, MaxAuxPowMerkleBranchHashes)
		return nil, messageError("AuxPow.BtcDecode", str)
	}

	branch := make([]chainhash.Hash, count)
	for i := range branch {
		if err := readElement(r, &branch[i]); err != nil {
			return nil, err
		}
	}
	return branch, nil
}

func writeAuxPowBranch(w io.Writer, pver uint32, buf []byte,
	branch []chainhash.Hash, fieldName string) error {

	if len(branch) > MaxAuxPowMerkleBranchHashes {
		str := fmt.Sprintf("%s too long [count %d, max %d]",
			fieldName, len(branch), MaxAuxPowMerkleBranchHashes)
		return messageError("AuxPow.BtcEncode", str)
	}

	if err := WriteVarIntBuf(w, pver, uint64(len(branch)), buf); err != nil {
		return err
	}
	for i := range branch {
		if err := writeElement(w, &branch[i]); err != nil {
			return err
		}
	}
	return nil
}

// BtcDecode decodes an AuxPow proof.
func (aux *AuxPow) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	buf := binarySerializer.Borrow()
	defer binarySerializer.Return(buf)

	scriptBuf := scriptPool.Borrow()
	defer scriptPool.Return(scriptBuf)

	if err := aux.CoinbaseTx.btcDecode(r, pver, enc, buf, scriptBuf[:]); err != nil {
		return err
	}
	var ignoredParentBlockHash chainhash.Hash
	if err := readElement(r, &ignoredParentBlockHash); err != nil {
		return err
	}
	aux.ParentBlockHash = chainhash.Hash{}

	var err error
	aux.CoinbaseMerkleBranch, err = readAuxPowBranch(r, pver, buf,
		"coinbase merkle branch")
	if err != nil {
		return err
	}
	if err := readElement(r, &aux.CoinbaseBranchMask); err != nil {
		return err
	}

	aux.ChainMerkleBranch, err = readAuxPowBranch(r, pver, buf,
		"chain merkle branch")
	if err != nil {
		return err
	}
	if err := readElement(r, &aux.ChainBranchMask); err != nil {
		return err
	}

	return readBlockHeaderBuf(r, pver, &aux.ParentHeader, buf)
}

// BtcEncode encodes an AuxPow proof.
func (aux *AuxPow) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	buf := binarySerializer.Borrow()
	defer binarySerializer.Return(buf)

	if err := aux.CoinbaseTx.btcEncode(w, pver, enc, buf); err != nil {
		return err
	}
	var zeroParentBlockHash chainhash.Hash
	if err := writeElement(w, &zeroParentBlockHash); err != nil {
		return err
	}
	if err := writeAuxPowBranch(w, pver, buf, aux.CoinbaseMerkleBranch,
		"coinbase merkle branch"); err != nil {
		return err
	}
	if err := writeElement(w, aux.CoinbaseBranchMask); err != nil {
		return err
	}
	if err := writeAuxPowBranch(w, pver, buf, aux.ChainMerkleBranch,
		"chain merkle branch"); err != nil {
		return err
	}
	if err := writeElement(w, aux.ChainBranchMask); err != nil {
		return err
	}
	return writeBlockHeaderBuf(w, pver, &aux.ParentHeader, buf)
}

// SerializeSize returns the number of bytes it would take to serialize the
// AuxPow proof.
func (aux *AuxPow) SerializeSize() int {
	if aux == nil {
		return 0
	}

	n := aux.CoinbaseTx.SerializeSize()
	n += aux.serializeSizeNoCoinbase()
	return n
}

// SerializeSizeStripped returns the number of bytes it would take to serialize
// the AuxPow proof without witness data in the parent coinbase transaction.
func (aux *AuxPow) SerializeSizeStripped() int {
	if aux == nil {
		return 0
	}

	n := aux.CoinbaseTx.SerializeSizeStripped()
	n += aux.serializeSizeNoCoinbase()
	return n
}

func (aux *AuxPow) serializeSizeNoCoinbase() int {
	n := chainhash.HashSize
	n += VarIntSerializeSize(uint64(len(aux.CoinbaseMerkleBranch)))
	n += chainhash.HashSize * len(aux.CoinbaseMerkleBranch)
	n += 4
	n += VarIntSerializeSize(uint64(len(aux.ChainMerkleBranch)))
	n += chainhash.HashSize * len(aux.ChainMerkleBranch)
	n += 4
	n += blockHeaderLen
	return n
}
