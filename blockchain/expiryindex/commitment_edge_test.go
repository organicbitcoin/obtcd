// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func TestValidateExpiryCommitmentRejectsUnsupportedVersionAfterActivation(t *testing.T) {
	idx := &ExpiryIndex{
		expiryParams: &ExpiryParams{ExpiryCommitmentEnableAtHeight: 10},
	}

	var root [AccumulatorDigestSize]byte
	script := BuildExpiryCommitmentScript(root)
	script[6] = ExpiryCommitmentVersion + 1

	coinbase := wire.NewMsgTx(1)
	coinbase.AddTxIn(&wire.TxIn{})
	coinbase.AddTxOut(&wire.TxOut{Value: 50 * btcutil.SatoshiPerBitcoin})
	coinbase.AddTxOut(&wire.TxOut{Value: 0, PkScript: script})

	block := btcutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{coinbase},
	})
	block.SetHeight(10)

	err := idx.validateExpiryCommitment(block, root)
	if err == nil {
		t.Fatal("expected unsupported commitment version to be rejected")
	}
	ruleErr, ok := err.(blockchain.RuleError)
	if !ok {
		t.Fatalf("expected RuleError, got %T: %v", err, err)
	}
	if ruleErr.ErrorCode != blockchain.ErrBadExpiryCommitmentFormat {
		t.Fatalf("unexpected error code: got %v want %v",
			ruleErr.ErrorCode, blockchain.ErrBadExpiryCommitmentFormat)
	}
}
