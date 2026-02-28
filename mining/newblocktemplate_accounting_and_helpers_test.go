package mining

import (
	"bytes"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/wire"
)

func TestNewBlockTemplateFeeAccountingConsistency(t *testing.T) {
	t.Run("with_reap_and_normal_tx", func(t *testing.T) {
		h := setupBoundaryHarness(t)
		defer h.cleanup()

		nextHeight := h.chain.BestSnapshot().Height + 1
		plannedREAPTx, plannedREAPFee, err := h.generator.maybeBuildREAPTx(nextHeight)
		if err != nil {
			t.Fatalf("maybeBuildREAPTx failed: %v", err)
		}
		if plannedREAPTx == nil {
			t.Fatalf("expected planned REAP tx")
		}

		normalFee := int64(10_000)
		normalTx := buildOPTrueSpendTx(h.spendable[120], h.values[120], normalFee, 0)
		baseWeight := initialTemplateWeight(t, h.params, nextHeight)
		normalWeight := uint32(blockchain.GetTransactionWeight(normalTx))
		reserve := h.generator.reservedREAPWeight(nextHeight)

		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.policy.BlockMaxWeight = baseWeight + normalWeight + reserve + 8_000
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(normalTx, normalFee, 10_000)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if !templateContainsTx(tmpl, normalTx.Hash()) {
			t.Fatalf("expected normal tx to be included")
		}
		if !templateHasREAPTx(tmpl) {
			t.Fatalf("expected REAP tx to be appended")
		}

		// Strong fee-accounting consistency: coinbase fee entry equals negative
		// sum of all non-coinbase entries.
		nonCoinbaseFeeSum := int64(0)
		for i := 1; i < len(tmpl.Fees); i++ {
			nonCoinbaseFeeSum += tmpl.Fees[i]
		}
		if tmpl.Fees[0] != -nonCoinbaseFeeSum {
			t.Fatalf("coinbase fee mismatch: got %d want %d", tmpl.Fees[0], -nonCoinbaseFeeSum)
		}

		normalIdx := templateTxIndex(tmpl, normalTx.Hash())
		if normalIdx < 0 {
			t.Fatalf("normal tx index missing")
		}
		if got := tmpl.Fees[normalIdx]; got != normalFee {
			t.Fatalf("normal tx fee mismatch: got %d want %d", got, normalFee)
		}

		reapIdx := -1
		for i := 1; i < len(tmpl.Block.Transactions); i++ {
			if reap.IsLikelyREAPTx(tmpl.Block.Transactions[i]) {
				reapIdx = i
				break
			}
		}
		if reapIdx < 0 {
			t.Fatalf("reap tx index missing")
		}
		if got := tmpl.Fees[reapIdx]; got != plannedREAPFee {
			t.Fatalf("reap fee mismatch: got %d want %d", got, plannedREAPFee)
		}

		subsidy := blockchain.CalcBlockSubsidy(tmpl.Height, h.params)
		coinbaseOut := tmpl.Block.Transactions[0].TxOut[0].Value
		if want := subsidy + nonCoinbaseFeeSum; coinbaseOut != want {
			t.Fatalf("coinbase value mismatch: got %d want %d", coinbaseOut, want)
		}
	})

	t.Run("without_reap", func(t *testing.T) {
		h := setupBoundaryHarnessAtHeight(t, 109, []int32{10})
		defer h.cleanup()

		nextHeight := h.chain.BestSnapshot().Height + 1
		plannedREAPTx, _, err := h.generator.maybeBuildREAPTx(nextHeight)
		if err != nil {
			t.Fatalf("maybeBuildREAPTx failed: %v", err)
		}
		if plannedREAPTx != nil {
			t.Fatalf("expected no planned REAP tx")
		}

		normalFee := int64(10_000)
		normalTx := buildOPTrueSpendTx(h.spendable[10], h.values[10], normalFee, 0)
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.policy.BlockMaxWeight = 1_000_000
		h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(normalTx, normalFee, 10_000)})

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}
		if templateHasREAPTx(tmpl) {
			t.Fatalf("did not expect REAP tx")
		}
		if !templateContainsTx(tmpl, normalTx.Hash()) {
			t.Fatalf("expected normal tx to be included")
		}

		nonCoinbaseFeeSum := int64(0)
		for i := 1; i < len(tmpl.Fees); i++ {
			nonCoinbaseFeeSum += tmpl.Fees[i]
		}
		if tmpl.Fees[0] != -nonCoinbaseFeeSum {
			t.Fatalf("coinbase fee mismatch: got %d want %d", tmpl.Fees[0], -nonCoinbaseFeeSum)
		}
		subsidy := blockchain.CalcBlockSubsidy(tmpl.Height, h.params)
		coinbaseOut := tmpl.Block.Transactions[0].TxOut[0].Value
		if want := subsidy + nonCoinbaseFeeSum; coinbaseOut != want {
			t.Fatalf("coinbase value mismatch: got %d want %d", coinbaseOut, want)
		}
	})
}

func TestNewBlockTemplateFinalConnectFailurePath(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	// Keep this path focused on final template connect checks.
	h.generator.reapIndex = nil

	invalid := wire.NewMsgTx(wire.TxVersion)
	invalid.AddTxIn(&wire.TxIn{PreviousOutPoint: h.spendable[120]})
	invalidTx := btcutil.NewTx(invalid)

	h.generator.policy.BlockPrioritySize = 0
	h.generator.policy.BlockMinWeight = 0
	h.generator.policy.TxMinFreeFee = 0
	h.generator.policy.BlockMaxWeight = 1_000_000
	h.generator.txSource = newStaticTxSource([]*TxDesc{makeTxDesc(invalidTx, h.values[120], 10_000)})

	_, err := h.generator.NewBlockTemplate(nil)
	if err == nil {
		t.Fatalf("expected NewBlockTemplate to fail when final CheckConnectBlockTemplate rejects invalid tx")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "output") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestNewBlockTemplateHelperFunctions(t *testing.T) {
	h := setupBoundaryHarness(t)
	defer h.cleanup()

	t.Run("UpdateBlockTime_no_reduce_min_difficulty", func(t *testing.T) {
		msgBlock := wire.NewMsgBlock(&wire.BlockHeader{Bits: 0x1d00ffff})
		oldBits := msgBlock.Header.Bits
		if err := h.generator.UpdateBlockTime(msgBlock); err != nil {
			t.Fatalf("UpdateBlockTime failed: %v", err)
		}
		if msgBlock.Header.Bits != oldBits {
			t.Fatalf("expected Bits unchanged when ReduceMinDifficulty=false: got %08x want %08x", msgBlock.Header.Bits, oldBits)
		}
		if minTs := MinimumMedianTime(h.chain.BestSnapshot()); msgBlock.Header.Timestamp.Before(minTs) {
			t.Fatalf("timestamp is below minimum median-adjusted time")
		}
	})

	t.Run("UpdateBlockTime_reduce_min_difficulty", func(t *testing.T) {
		paramsCopy := *h.params
		paramsCopy.ReduceMinDifficulty = true

		g2 := *h.generator
		g2.chainParams = &paramsCopy

		msgBlock := wire.NewMsgBlock(&wire.BlockHeader{Bits: 0})
		if err := g2.UpdateBlockTime(msgBlock); err != nil {
			t.Fatalf("UpdateBlockTime failed: %v", err)
		}
		expectedBits, err := h.chain.CalcNextRequiredDifficulty(msgBlock.Header.Timestamp)
		if err != nil {
			t.Fatalf("CalcNextRequiredDifficulty failed: %v", err)
		}
		if msgBlock.Header.Bits != expectedBits {
			t.Fatalf("difficulty mismatch: got %08x want %08x", msgBlock.Header.Bits, expectedBits)
		}
	})

	t.Run("UpdateExtraNonce_success_and_accessors", func(t *testing.T) {
		h.generator.reapIndex = nil
		h.generator.txSource = newStaticTxSource(nil)
		h.generator.policy.BlockPrioritySize = 0
		h.generator.policy.BlockMinWeight = 0
		h.generator.policy.TxMinFreeFee = 0
		h.generator.policy.BlockMaxWeight = 1_000_000

		tmpl, err := h.generator.NewBlockTemplate(nil)
		if err != nil {
			t.Fatalf("NewBlockTemplate failed: %v", err)
		}

		oldScript := append([]byte(nil), tmpl.Block.Transactions[0].TxIn[0].SignatureScript...)
		oldMerkle := tmpl.Block.Header.MerkleRoot

		if err := h.generator.UpdateExtraNonce(tmpl.Block, tmpl.Height, 42); err != nil {
			t.Fatalf("UpdateExtraNonce failed: %v", err)
		}

		newScript := tmpl.Block.Transactions[0].TxIn[0].SignatureScript
		if bytes.Equal(oldScript, newScript) {
			t.Fatalf("expected coinbase signature script to change after UpdateExtraNonce")
		}
		if tmpl.Block.Header.MerkleRoot == oldMerkle {
			t.Fatalf("expected merkle root to change after UpdateExtraNonce")
		}

		bestFromGen := h.generator.BestSnapshot()
		bestFromChain := h.chain.BestSnapshot()
		if bestFromGen.Height != bestFromChain.Height || bestFromGen.Hash != bestFromChain.Hash {
			t.Fatalf("BestSnapshot mismatch between generator and chain")
		}

		customSource := newStaticTxSource(nil)
		h.generator.txSource = customSource
		if got := h.generator.TxSource(); got != customSource {
			t.Fatalf("TxSource accessor mismatch")
		}
	})
}
