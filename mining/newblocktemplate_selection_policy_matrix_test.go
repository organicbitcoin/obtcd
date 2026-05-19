// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
)

func TestNewBlockTemplatePriorityFeeSwitchWithREAPBoundary(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx == nil {
		t.Fatalf("expected planned REAP tx")
	}

	fee := int64(10_000)
	txLowFeeHighPriority := buildOPTrueSpendTx(h.spendable[120], h.values[120], fee, 0)
	txHighFeeLowPriority := buildOPTrueSpendTx(h.spendable[121], h.values[121], fee, 6_000)
	weightLowFee := uint32(blockchain.GetTransactionWeight(txLowFeeHighPriority))
	weightHighFee := uint32(blockchain.GetTransactionWeight(txHighFeeLowPriority))
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	reserve := uint32(blockchain.GetTransactionWeight(plannedREAPTx))

	t.Run("switches-to-fee-order-under-reap-reserve", func(t *testing.T) {
		h.generator.policy.BlockPrioritySize = 1 // force immediate switch + requeue
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.BlockMaxWeight = baseWeight + weightLowFee + weightHighFee + reserve + 8_000
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(txLowFeeHighPriority, fee, 1_000),
			makeTxDesc(txHighFeeLowPriority, fee, 100_000),
		})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		idxHighFee := templateTxIndex(tmpl, txHighFeeLowPriority.Hash())
		idxLowFee := templateTxIndex(tmpl, txLowFeeHighPriority.Hash())
		if idxHighFee == -1 || idxLowFee == -1 {
			t.Fatalf("expected both transactions to be included")
		}
		if idxHighFee >= idxLowFee {
			t.Fatalf("expected high-fee tx to be ordered before low-fee tx after switch: high=%d low=%d", idxHighFee, idxLowFee)
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be appended under reserved area")
		}
	})

	t.Run("fee-ordered-first-tx-kept-second-rejected-at-normal-boundary", func(t *testing.T) {
		h.generator.policy.BlockPrioritySize = 1 // force immediate switch + requeue
		h.generator.policy.BlockMinWeight = 0
		normalLimit := baseWeight + weightHighFee + 1
		h.generator.policy.BlockMaxWeight = normalLimit + reserve
		h.generator.policy.TxMinFreeFee = 0
		h.generator.txSource = newStaticTxSource([]*TxDesc{
			makeTxDesc(txLowFeeHighPriority, fee, 1_000),
			makeTxDesc(txHighFeeLowPriority, fee, 100_000),
		})

		if got := h.generator.normalTxWeightLimit(nextHeight, true, reserve); got != normalLimit {
			t.Fatalf("unexpected normal limit: got %d want %d", got, normalLimit)
		}

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		if !templateContainsTx(tmpl, txHighFeeLowPriority.Hash()) {
			t.Fatalf("expected high-fee tx to be included first after switch")
		}
		if templateContainsTx(tmpl, txLowFeeHighPriority.Hash()) {
			t.Fatalf("expected second tx to be rejected when crossing normal-region boundary")
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be appended even when second normal tx is rejected")
		}
	})
}

func TestNewBlockTemplateLowFeePolicyWithREAPReserveMatrix(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx == nil {
		t.Fatalf("expected planned REAP tx")
	}

	fee := int64(1)
	lowFeeTx := buildOPTrueSpendTx(h.spendable[122], h.values[122], fee, 0)
	lowFeeWeight := uint32(blockchain.GetTransactionWeight(lowFeeTx))
	baseWeight := initialTemplateWeight(t, h.params, nextHeight)
	reserve := uint32(blockchain.GetTransactionWeight(plannedREAPTx))
	blockMax := baseWeight + lowFeeWeight + reserve + 8_000

	tests := []struct {
		name              string
		txMinFreeFee      btcutil.Amount
		blockMinWeight    uint32
		expectLowFeeInTpl bool
	}{
		{name: "minweight-zero-lowfee-rejected", txMinFreeFee: btcutil.Amount(100_000_000), blockMinWeight: 0, expectLowFeeInTpl: false},
		{name: "minweight-high-lowfee-allowed", txMinFreeFee: btcutil.Amount(100_000_000), blockMinWeight: 1_000_000, expectLowFeeInTpl: true},
		{name: "minfreefee-zero-lowfee-allowed", txMinFreeFee: 0, blockMinWeight: 0, expectLowFeeInTpl: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h.generator.policy.BlockPrioritySize = 0 // start sorted-by-fee
			h.generator.policy.BlockMaxWeight = blockMax
			h.generator.policy.BlockMinWeight = tc.blockMinWeight
			h.generator.policy.TxMinFreeFee = tc.txMinFreeFee
			h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(lowFeeTx, fee, 0)})

			tmpl, err := h.generator.NewBlockTemplate(nil)
			if err != nil {
				t.Fatalf("NewBlockTemplate failed: %v", err)
			}

			gotLowFee := templateContainsTx(tmpl, lowFeeTx.Hash())
			if gotLowFee != tc.expectLowFeeInTpl {
				t.Fatalf("low-fee inclusion mismatch: got %v want %v", gotLowFee, tc.expectLowFeeInTpl)
			}
			if !templateHasREAPTx(tmpl) {
				t.Fatalf("expected REAP tx to remain appendable while low-fee policy is applied")
			}
		})
	}
}

func TestNewBlockTemplateREAPAppendExceptionBranches(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	nextHeight := h.chain.BestSnapshot().Height + 1
	plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
	if err != nil {
		t.Fatalf("maybeBuildREAPTx failed: %v", err)
	}
	if plannedREAPTx == nil {
		t.Fatalf("expected planned REAP tx")
	}

	h.generator.policy.BlockPrioritySize = 0
	h.generator.policy.BlockMinWeight = 0
	h.generator.policy.TxMinFreeFee = 0
	h.generator.policy.BlockMaxWeight = 1_000_000
	h.generator.txSource = newStaticTxSource(nil)

	t.Run("sigop-calc-error", func(t *testing.T) {
		h.generator.reapSigOpCostFn = func(tx *btcutil.Tx, _ *blockchain.UtxoViewpoint, _ bool) (int, error) {
			if tx == nil {
				t.Fatalf("expected non-nil planned REAP tx")
			}
			return 0, errors.New("forced sigop failure")
		}
		h.generator.reapFetchInputViewFn = nil
		defer func() {
			h.generator.reapSigOpCostFn = nil
			h.generator.reapFetchInputViewFn = nil
		}()

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be skipped when sigop calculation errors")
		}
	})

	t.Run("input-fetch-error-after-sigop-success", func(t *testing.T) {
		h.generator.reapSigOpCostFn = func(tx *btcutil.Tx, _ *blockchain.UtxoViewpoint, _ bool) (int, error) {
			if tx == nil {
				t.Fatalf("expected non-nil planned REAP tx")
			}
			return 1, nil
		}
		h.generator.reapFetchInputViewFn = func(_ *btcutil.Tx) (*blockchain.UtxoViewpoint, error) {
			return nil, errors.New("forced fetch input view failure")
		}
		defer func() {
			h.generator.reapSigOpCostFn = nil
			h.generator.reapFetchInputViewFn = nil
		}()

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be skipped when input view fetch fails")
		}
	})

	t.Run("sigop-over-limit", func(t *testing.T) {
		h.generator.reapSigOpCostFn = func(tx *btcutil.Tx, _ *blockchain.UtxoViewpoint, _ bool) (int, error) {
			if tx == nil {
				t.Fatalf("expected non-nil planned REAP tx")
			}
			return int(blockchain.MaxBlockSigOpsCost) + 1, nil
		}
		h.generator.reapFetchInputViewFn = nil
		defer func() {
			h.generator.reapSigOpCostFn = nil
			h.generator.reapFetchInputViewFn = nil
		}()

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be skipped when sigop cost exceeds block limit")
		}
	})
}
