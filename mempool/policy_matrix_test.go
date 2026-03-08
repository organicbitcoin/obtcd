package mempool

import (
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func TestValidateSegWitDeploymentMatrix(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	tx := btcutil.NewTx(wire.NewMsgTx(wire.TxVersion))
	tx.MsgTx().AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
		Sequence:         wire.MaxTxInSequenceNum,
		Witness:          wire.TxWitness{[]byte{0x01}},
	})
	tx.MsgTx().AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})

	harness.txPool.cfg.IsDeploymentActive = func(deploymentID uint32) (bool, error) {
		return false, nil
	}
	err = harness.txPool.validateSegWitDeployment(tx)
	if err == nil {
		t.Fatal("expected witness tx to be rejected before segwit activation")
	}

	harness.txPool.cfg.IsDeploymentActive = func(deploymentID uint32) (bool, error) {
		return true, nil
	}
	if err := harness.txPool.validateSegWitDeployment(tx); err != nil {
		t.Fatalf("expected witness tx to pass after segwit activation: %v", err)
	}

	nonWitness := btcutil.NewTx(wire.NewMsgTx(wire.TxVersion))
	nonWitness.MsgTx().AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 2},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	nonWitness.MsgTx().AddTxOut(&wire.TxOut{Value: 1, PkScript: []byte{txscript.OP_TRUE}})
	if err := harness.txPool.validateSegWitDeployment(nonWitness); err != nil {
		t.Fatalf("expected non-witness tx to bypass segwit activation checks: %v", err)
	}
}

func TestValidateStandardnessVersionMatrix(t *testing.T) {
	harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	tx, err := createSignedTxWithHashType(
		harness, outputs[0], 2, 1_000, txscript.SigHashAll,
	)
	if err != nil {
		t.Fatalf("unable to create tx: %v", err)
	}

	utxoView, err := harness.chain.FetchUtxoView(tx)
	if err != nil {
		t.Fatalf("unable to fetch utxo view: %v", err)
	}

	nextBlockHeight := harness.chain.BestHeight() + 1
	medianTimePast := harness.chain.MedianTimePast()

	harness.txPool.cfg.Policy.MaxTxVersion = 1
	err = harness.txPool.validateStandardness(tx, nextBlockHeight, medianTimePast, utxoView)
	if err == nil {
		t.Fatal("expected tx version above policy max to be rejected")
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 2
	if err := harness.txPool.validateStandardness(tx, nextBlockHeight, medianTimePast, utxoView); err != nil {
		t.Fatalf("expected tx version to pass after raising policy max: %v", err)
	}
}

func TestValidateSigCostMatrix(t *testing.T) {
	harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	tx, err := createSignedTxWithHashType(
		harness, outputs[0], wire.TxVersion, 1_000, txscript.SigHashAll,
	)
	if err != nil {
		t.Fatalf("unable to create tx: %v", err)
	}

	utxoView, err := harness.chain.FetchUtxoView(tx)
	if err != nil {
		t.Fatalf("unable to fetch utxo view: %v", err)
	}

	harness.txPool.cfg.Policy.MaxSigOpCostPerTx = 1
	err = harness.txPool.validateSigCost(tx, utxoView)
	if err == nil {
		t.Fatal("expected low sigop budget to reject tx")
	}

	harness.txPool.cfg.Policy.MaxSigOpCostPerTx = blockchain.MaxBlockSigOpsCost / 4
	if err := harness.txPool.validateSigCost(tx, utxoView); err != nil {
		t.Fatalf("expected tx to pass with default sigop budget: %v", err)
	}
}

func TestValidateRelayFeeMetRateLimitMatrix(t *testing.T) {
	harness, outputs, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	tx, err := createSignedTxWithHashType(
		harness, outputs[0], wire.TxVersion, 0, txscript.SigHashAll,
	)
	if err != nil {
		t.Fatalf("unable to create tx: %v", err)
	}

	utxoView, err := harness.chain.FetchUtxoView(tx)
	if err != nil {
		t.Fatalf("unable to fetch utxo view: %v", err)
	}

	nextBlockHeight := harness.chain.BestHeight() + 1
	txSize := int64(tx.MsgTx().SerializeSize())

	harness.txPool.cfg.Policy.DisableRelayPriority = true
	harness.txPool.cfg.Policy.FreeTxRelayLimit = 0
	harness.txPool.lastPennyUnix = time.Now().Unix()
	err = harness.txPool.validateRelayFeeMet(
		tx, 0, txSize, utxoView, nextBlockHeight, true, true,
	)
	if err == nil {
		t.Fatal("expected free tx to be rejected by the rate limiter")
	}
	if !strings.Contains(err.Error(), "rate limiter") {
		t.Fatalf("unexpected relay fee error: %v", err)
	}

	harness.txPool.cfg.Policy.FreeTxRelayLimit = 15
	minFee := calcMinRequiredTxRelayFee(txSize, harness.txPool.cfg.Policy.MinRelayTxFee)
	if err := harness.txPool.validateRelayFeeMet(
		tx, minFee, txSize, utxoView, nextBlockHeight, true, true,
	); err != nil {
		t.Fatalf("expected tx to pass once min relay fee is met: %v", err)
	}
}

func TestProcessTransactionOBTCReplayProtectionActivation(t *testing.T) {
	tests := []struct {
		name       string
		height     int32
		hashType   txscript.SigHashType
		shouldPass bool
	}{
		{
			name:       "pre-activation standard accepted",
			height:     112,
			hashType:   txscript.SigHashAll,
			shouldPass: true,
		},
		{
			name:       "pre-activation replay rejected",
			height:     112,
			hashType:   txscript.SigHashOBTCReplayProtection | txscript.SigHashAll,
			shouldPass: false,
		},
		{
			name:       "post-activation standard rejected",
			height:     113,
			hashType:   txscript.SigHashAll,
			shouldPass: false,
		},
		{
			name:       "post-activation replay accepted",
			height:     113,
			hashType:   txscript.SigHashOBTCReplayProtection | txscript.SigHashAll,
			shouldPass: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, outputs, err := newPoolHarness(&chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("unable to create pool harness: %v", err)
			}

			harness.chain.SetHeight(test.height)
			harness.txPool.cfg.Policy.MaxTxVersion = wire.TxVersion

			tx, err := createSignedTxWithHashType(
				harness, outputs[0], wire.TxVersion, 1_000, test.hashType,
			)
			if err != nil {
				t.Fatalf("unable to create tx: %v", err)
			}

			_, err = harness.txPool.ProcessTransaction(tx, false, false, 0)
			if test.shouldPass && err != nil {
				t.Fatalf("expected tx to be accepted at height %d: %v", test.height, err)
			}
			if !test.shouldPass && err == nil {
				t.Fatalf("expected tx to be rejected at height %d", test.height)
			}
		})
	}
}

func TestProcessTransactionOBTCReplayProtectionOrphanResolution(t *testing.T) {
	tests := []struct {
		name                  string
		height                int32
		parentHashType        txscript.SigHashType
		expectedAcceptedCount int
		expectChildInPool     bool
	}{
		{
			name:                  "pre-activation replay orphan removed",
			height:                112,
			parentHashType:        txscript.SigHashAll,
			expectedAcceptedCount: 1,
			expectChildInPool:     false,
		},
		{
			name:                  "post-activation replay orphan accepted",
			height:                113,
			parentHashType:        txscript.SigHashOBTCReplayProtection | txscript.SigHashAll,
			expectedAcceptedCount: 2,
			expectChildInPool:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, outputs, err := newPoolHarness(&chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("unable to create pool harness: %v", err)
			}

			harness.chain.SetHeight(test.height)

			parent, err := createSignedTxWithInputsHashType(
				harness, []spendableOutput{outputs[0]}, 1, 1_000, false,
				test.parentHashType,
			)
			if err != nil {
				t.Fatalf("unable to create parent tx: %v", err)
			}

			childInput := txOutToSpendableOut(parent, 0)
			child, err := createSignedTxWithInputsHashType(
				harness, []spendableOutput{childInput}, 1, 1_000, false,
				txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
			)
			if err != nil {
				t.Fatalf("unable to create child tx: %v", err)
			}

			accepted, err := harness.txPool.ProcessTransaction(child, true, false, 0)
			if err != nil {
				t.Fatalf("unable to add orphan child: %v", err)
			}
			if len(accepted) != 0 {
				t.Fatalf("expected orphan child to return no accepted txs, got %d", len(accepted))
			}
			if !harness.txPool.IsOrphanInPool(child.Hash()) {
				t.Fatalf("expected child to be tracked as orphan")
			}

			accepted, err = harness.txPool.ProcessTransaction(parent, false, false, 0)
			if err != nil {
				t.Fatalf("unable to add parent tx: %v", err)
			}

			childStillOrphan := harness.txPool.IsOrphanInPool(child.Hash())
			childInPool := harness.txPool.IsTransactionInPool(child.Hash())
			if childStillOrphan {
				t.Fatalf("expected child to be removed from orphan pool")
			}
			if childInPool != test.expectChildInPool {
				_, _, replayErr := harness.txPool.MaybeAcceptTransaction(child, true, true)
				t.Fatalf("unexpected child mempool membership: want %v, recheckErr=%v",
					test.expectChildInPool, replayErr)
			}
			if len(accepted) != test.expectedAcceptedCount {
				t.Fatalf("expected %d accepted txs, got %d (childInPool=%v)",
					test.expectedAcceptedCount, len(accepted), childInPool)
			}
		})
	}
}

func TestProcessTransactionOBTCReplayProtectionReplacementMatrix(t *testing.T) {
	const defaultFee = btcutil.SatoshiPerBitcoin

	tests := []struct {
		name           string
		setup          func(*testing.T, *poolHarness, spendableOutput) (*btcutil.Tx, *btcutil.Tx, *btcutil.Tx)
		shouldPass     bool
		originalInPool bool
		childInPool    bool
	}{
		{
			name: "non-replay replacement rejected",
			setup: func(t *testing.T, harness *poolHarness, baseOut spendableOutput) (*btcutil.Tx, *btcutil.Tx, *btcutil.Tx) {
				t.Helper()

				original, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{baseOut}, 1, defaultFee, true,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create original tx: %v", err)
				}
				if _, err := harness.txPool.ProcessTransaction(original, false, false, 0); err != nil {
					t.Fatalf("unable to add original tx: %v", err)
				}

				replacement, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{baseOut}, 1, defaultFee*3, false,
					txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create replacement tx: %v", err)
				}

				return original, nil, replacement
			},
			shouldPass:     false,
			originalInPool: true,
			childInPool:    false,
		},
		{
			name: "replay replacement accepted",
			setup: func(t *testing.T, harness *poolHarness, baseOut spendableOutput) (*btcutil.Tx, *btcutil.Tx, *btcutil.Tx) {
				t.Helper()

				original, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{baseOut}, 1, defaultFee, true,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create original tx: %v", err)
				}
				if _, err := harness.txPool.ProcessTransaction(original, false, false, 0); err != nil {
					t.Fatalf("unable to add original tx: %v", err)
				}

				replacement, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{baseOut}, 1, defaultFee*3, false,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create replacement tx: %v", err)
				}

				return original, nil, replacement
			},
			shouldPass:     true,
			originalInPool: false,
			childInPool:    false,
		},
		{
			name: "inherited replay replacement accepted",
			setup: func(t *testing.T, harness *poolHarness, baseOut spendableOutput) (*btcutil.Tx, *btcutil.Tx, *btcutil.Tx) {
				t.Helper()

				parent, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{baseOut}, 1, defaultFee, true,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create parent tx: %v", err)
				}
				if _, err := harness.txPool.ProcessTransaction(parent, false, false, 0); err != nil {
					t.Fatalf("unable to add parent tx: %v", err)
				}

				parentOut := txOutToSpendableOut(parent, 0)
				child, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{parentOut}, 1, defaultFee, false,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create child tx: %v", err)
				}
				if _, err := harness.txPool.ProcessTransaction(child, false, false, 0); err != nil {
					t.Fatalf("unable to add child tx: %v", err)
				}

				replacement, err := createSignedTxWithInputsHashType(
					harness, []spendableOutput{parentOut}, 1, defaultFee*3, false,
					txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
				)
				if err != nil {
					t.Fatalf("unable to create replacement tx: %v", err)
				}

				return parent, child, replacement
			},
			shouldPass:     true,
			originalInPool: true,
			childInPool:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness, outputs, err := newPoolHarness(&chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("unable to create pool harness: %v", err)
			}

			harness.chain.SetHeight(113)

			original, child, replacement := test.setup(t, harness, outputs[0])
			accepted, err := harness.txPool.ProcessTransaction(
				replacement, false, false, 0,
			)
			if test.shouldPass && err != nil {
				t.Fatalf("expected replacement to be accepted: %v", err)
			}
			if !test.shouldPass && err == nil {
				t.Fatalf("expected replacement to be rejected")
			}
			if test.shouldPass && len(accepted) != 1 {
				t.Fatalf("expected accepted replacement list size 1, got %d", len(accepted))
			}

			if harness.txPool.IsTransactionInPool(replacement.Hash()) != test.shouldPass {
				t.Fatalf("unexpected replacement mempool membership: want %v", test.shouldPass)
			}
			if harness.txPool.IsTransactionInPool(original.Hash()) != test.originalInPool {
				t.Fatalf("unexpected original mempool membership: want %v", test.originalInPool)
			}
			if child != nil && harness.txPool.IsTransactionInPool(child.Hash()) != test.childInPool {
				t.Fatalf("unexpected child mempool membership: want %v", test.childInPool)
			}
		})
	}
}

func TestProcessTransactionWitnessDeploymentRejectsOrphanBeforePooling(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.IsDeploymentActive = func(deploymentID uint32) (bool, error) {
		return false, nil
	}

	msg := wire.NewMsgTx(wire.TxVersion)
	msg.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
		Sequence:         wire.MaxTxInSequenceNum,
		Witness:          wire.TxWitness{[]byte{0x01}},
	})
	msg.AddTxOut(&wire.TxOut{Value: 1_000, PkScript: []byte{txscript.OP_TRUE}})

	tx := btcutil.NewTx(msg)
	_, err = harness.txPool.ProcessTransaction(tx, true, false, 0)
	if err == nil {
		t.Fatal("expected witness tx to be rejected before segwit activation")
	}
	if !strings.Contains(err.Error(), "segwit isn't active yet") {
		t.Fatalf("unexpected error: %v", err)
	}
	if harness.txPool.IsOrphanInPool(tx.Hash()) {
		t.Fatalf("witness tx should not be added to orphan pool")
	}
}

func TestProcessTransactionREAPRejectsOrphanBeforePooling(t *testing.T) {
	harness, _, err := newPoolHarness(&chaincfg.MainNetParams)
	if err != nil {
		t.Fatalf("unable to create pool harness: %v", err)
	}

	harness.txPool.cfg.Policy.MaxTxVersion = 3

	msg := wire.NewMsgTx(3)
	msg.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Index: 1},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	burn, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP_BURN")).Script()
	marker, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte("REAP:100:1:abcd")).Script()
	msg.AddTxOut(&wire.TxOut{Value: 1000, PkScript: burn})
	msg.AddTxOut(&wire.TxOut{Value: 0, PkScript: marker})

	tx := btcutil.NewTx(msg)
	_, err = harness.txPool.ProcessTransaction(tx, true, false, 0)
	if err == nil {
		t.Fatal("expected REAP tx to be rejected before orphan handling")
	}
	if !strings.Contains(err.Error(), "reap system transaction") {
		t.Fatalf("unexpected error: %v", err)
	}
	if harness.txPool.IsOrphanInPool(tx.Hash()) {
		t.Fatalf("REAP tx should not be added to orphan pool")
	}
}

func createSignedTxWithHashType(harness *poolHarness, input spendableOutput,
	version int32, fee btcutil.Amount,
	hashType txscript.SigHashType) (*btcutil.Tx, error) {

	tx := wire.NewMsgTx(version)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: input.outPoint,
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		PkScript: harness.payScript,
		Value:    int64(input.amount - fee),
	})

	sigScript, err := txscript.SignatureScript(
		tx, 0, harness.payScript, hashType, harness.signKey, true,
	)
	if err != nil {
		return nil, err
	}
	tx.TxIn[0].SignatureScript = sigScript

	return btcutil.NewTx(tx), nil
}

func createSignedTxWithInputsHashType(harness *poolHarness, inputs []spendableOutput,
	numOutputs uint32, fee btcutil.Amount, signalsReplacement bool,
	hashType txscript.SigHashType) (*btcutil.Tx, error) {

	var totalInput btcutil.Amount
	for _, input := range inputs {
		totalInput += input.amount
	}
	totalInput -= fee

	amountPerOutput := int64(totalInput) / int64(numOutputs)
	remainder := int64(totalInput) - amountPerOutput*int64(numOutputs)

	tx := wire.NewMsgTx(wire.TxVersion)
	sequence := wire.MaxTxInSequenceNum
	if signalsReplacement {
		sequence = MaxRBFSequence
	}

	for _, input := range inputs {
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: input.outPoint,
			Sequence:         sequence,
		})
	}
	for i := uint32(0); i < numOutputs; i++ {
		amount := amountPerOutput
		if i == numOutputs-1 {
			amount += remainder
		}
		tx.AddTxOut(&wire.TxOut{
			PkScript: harness.payScript,
			Value:    amount,
		})
	}

	for i := range tx.TxIn {
		sigScript, err := txscript.SignatureScript(
			tx, i, harness.payScript, hashType, harness.signKey, true,
		)
		if err != nil {
			return nil, err
		}
		tx.TxIn[i].SignatureScript = sigScript
	}

	return btcutil.NewTx(tx), nil
}
