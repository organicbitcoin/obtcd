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
