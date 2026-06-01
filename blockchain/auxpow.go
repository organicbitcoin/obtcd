// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

var auxPowMagic = []byte{0xfa, 0xbe, 0x6d, 0x6d}

const maxAuxPowChainMerkleBranchHashes = 30

func auxPowVersionChainID(version int32) uint32 {
	return uint32(version) >> 16
}

func auxPowCommitmentRootBytes(root *chainhash.Hash) []byte {
	commitment := root.CloneBytes()
	for i, j := 0, len(commitment)-1; i < j; i, j = i+1, j-1 {
		commitment[i], commitment[j] = commitment[j], commitment[i]
	}
	return commitment
}

func calcMerkleBranchRoot(leaf chainhash.Hash, branch []chainhash.Hash,
	branchMask uint32) chainhash.Hash {

	root := leaf
	for i := range branch {
		sibling := branch[i]
		if branchMask&(1<<uint(i)) != 0 {
			root = HashMerkleBranches(&sibling, &root)
		} else {
			root = HashMerkleBranches(&root, &sibling)
		}
	}
	return root
}

func expectedAuxPowIndex(nonce, chainID uint32, branchHeight int) uint32 {
	rand := nonce
	rand = rand*1103515245 + 12345
	rand += chainID
	rand = rand*1103515245 + 12345
	return rand % (uint32(1) << uint(branchHeight))
}

func validateAuxPowCommitment(aux *wire.AuxPow, childHash *chainhash.Hash,
	chainID uint32) error {

	if aux == nil {
		return ruleError(ErrInvalidAuxPow, "auxpow block is missing auxpow proof")
	}

	if auxPowVersionChainID(aux.ParentHeader.Version) == chainID {
		return ruleError(ErrInvalidAuxPow, "auxpow parent has our chain id")
	}

	if !IsCoinBaseTx(&aux.CoinbaseTx) {
		return ruleError(ErrInvalidAuxPow, "auxpow parent transaction is not coinbase")
	}

	if aux.CoinbaseBranchMask != 0 {
		return ruleError(ErrInvalidAuxPow, "auxpow coinbase merkle branch is not at index zero")
	}

	coinbaseHash := aux.CoinbaseTx.TxHash()
	parentRoot := calcMerkleBranchRoot(coinbaseHash, aux.CoinbaseMerkleBranch,
		aux.CoinbaseBranchMask)
	if !parentRoot.IsEqual(&aux.ParentHeader.MerkleRoot) {
		str := fmt.Sprintf("auxpow parent merkle root mismatch: got %s, want %s",
			parentRoot, aux.ParentHeader.MerkleRoot)
		return ruleError(ErrInvalidAuxPow, str)
	}

	if len(aux.ChainMerkleBranch) > maxAuxPowChainMerkleBranchHashes {
		return ruleError(ErrInvalidAuxPow, "auxpow chain merkle branch is too deep")
	}

	chainRoot := calcMerkleBranchRoot(*childHash, aux.ChainMerkleBranch,
		aux.ChainBranchMask)
	commitmentRoot := auxPowCommitmentRootBytes(&chainRoot)

	sigScript := aux.CoinbaseTx.TxIn[0].SignatureScript
	magicIndex := bytes.Index(sigScript, auxPowMagic)
	if magicIndex < 0 {
		return ruleError(ErrInvalidAuxPow, "auxpow coinbase missing merged mining header")
	}
	if bytes.LastIndex(sigScript, auxPowMagic) != magicIndex {
		return ruleError(ErrInvalidAuxPow, "auxpow coinbase has multiple merged mining headers")
	}

	commitmentStart := magicIndex + len(auxPowMagic)
	commitmentEnd := commitmentStart + chainhash.HashSize
	if len(sigScript) < commitmentEnd+8 {
		return ruleError(ErrInvalidAuxPow, "auxpow coinbase commitment is truncated")
	}
	if !bytes.Equal(sigScript[commitmentStart:commitmentEnd], commitmentRoot) {
		return ruleError(ErrInvalidAuxPow, "auxpow coinbase commitment root mismatch")
	}

	merkleSize := binary.LittleEndian.Uint32(sigScript[commitmentEnd : commitmentEnd+4])
	merkleNonce := binary.LittleEndian.Uint32(sigScript[commitmentEnd+4 : commitmentEnd+8])
	expectedSize := uint32(1) << uint(len(aux.ChainMerkleBranch))
	if merkleSize != expectedSize {
		str := fmt.Sprintf("auxpow merkle size %d does not match branch height %d",
			merkleSize, len(aux.ChainMerkleBranch))
		return ruleError(ErrInvalidAuxPow, str)
	}

	expectedIndex := expectedAuxPowIndex(merkleNonce, chainID,
		len(aux.ChainMerkleBranch))
	if aux.ChainBranchMask != expectedIndex {
		str := fmt.Sprintf("auxpow chain index %d does not match expected %d",
			aux.ChainBranchMask, expectedIndex)
		return ruleError(ErrInvalidAuxPow, str)
	}

	return nil
}

func validateAuxPowProofOfWork(aux *wire.AuxPow, target *big.Int,
	flags BehaviorFlags) error {

	if flags&BFNoPoWCheck == BFNoPoWCheck {
		return nil
	}

	parentHash := aux.ParentHeader.BlockHash()
	hashNum := HashToBig(&parentHash)
	if hashNum.Cmp(target) > 0 {
		str := fmt.Sprintf("auxpow parent hash of %064x is higher than "+
			"expected max of %064x", hashNum, target)
		return ruleError(ErrHighHash, str)
	}

	return nil
}

func validateAuxPow(block *btcutil.Block, chainParams *chaincfg.Params,
	blockHeight int32, flags BehaviorFlags) error {

	msgBlock := block.MsgBlock()
	header := &msgBlock.Header
	if !chaincfg.IsAuxPowEnabled(chainParams, blockHeight) {
		return ruleError(ErrInvalidAuxPow, "auxpow block before activation height")
	}
	if auxPowVersionChainID(header.Version) != chainParams.AuxPowChainID {
		str := fmt.Sprintf("auxpow block chain id %d does not match expected %d",
			auxPowVersionChainID(header.Version), chainParams.AuxPowChainID)
		return ruleError(ErrInvalidAuxPow, str)
	}

	target := CompactToBig(header.Bits)
	if err := validateAuxPowCommitment(msgBlock.AuxPow, block.Hash(),
		chainParams.AuxPowChainID); err != nil {
		return err
	}
	return validateAuxPowProofOfWork(msgBlock.AuxPow, target, flags)
}
