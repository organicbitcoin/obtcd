// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

// ExpiryChainAccessor wraps a BlockChain to satisfy the
// expiryindex.ChainAccessor interface. This adapter lives in the blockchain
// package because it needs access to package-private helpers (ForEachUTXO
// iterates the internal UTXO set).
type ExpiryChainAccessor struct {
	chain *BlockChain
}

// NewExpiryChainAccessor returns an adapter that bridges BlockChain to the
// expiryindex.ChainAccessor interface.
func NewExpiryChainAccessor(chain *BlockChain) *ExpiryChainAccessor {
	return &ExpiryChainAccessor{chain: chain}
}

// BestHeight returns the current best chain tip height.
func (a *ExpiryChainAccessor) BestHeight() int32 {
	return a.chain.BestSnapshot().Height
}

// BlockByHeight returns the block at the given height on the main chain.
func (a *ExpiryChainAccessor) BlockByHeight(height int32) (*btcutil.Block, error) {
	return a.chain.BlockByHeight(height)
}

// FetchSpendJournal returns spent transaction outputs for the given block.
func (a *ExpiryChainAccessor) FetchSpendJournal(block *btcutil.Block) ([]SpentTxOut, error) {
	return a.chain.FetchSpendJournal(block)
}

// ForEachUTXO iterates over all unspent outputs.
func (a *ExpiryChainAccessor) ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error {
	return a.chain.ForEachUTXO(fn)
}
