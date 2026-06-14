// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const reapTxVersion = 3

// unminedInputHeight matches mining.UnminedHeight and marks UTXOs that come
// from an unconfirmed mempool parent rather than a mined block.
const unminedInputHeight = int32(0x7fffffff)

// ReapPrefixCandidate identifies one expired UTXO in canonical REAP order.
type ReapPrefixCandidate struct {
	OutPoint  wire.OutPoint
	ExpiryKey uint64
	Amount    int64
}

// ReapPrefixSource provides the canonical prefix of the globally expired UTXO
// set for REAP consensus validation.
type ReapPrefixSource interface {
	// ReapPrefixTipHeight returns the chain height represented by the source.
	ReapPrefixTipHeight() (int32, error)

	// ReapPrefixCandidates returns at most limit expired live UTXOs at
	// blockHeight, ordered by the REAP canonical key.
	ReapPrefixCandidates(blockHeight int32, limit int) ([]ReapPrefixCandidate, error)
}

func isLikelyReapTx(tx *wire.MsgTx) bool {
	if tx == nil || tx.Version != reapTxVersion || len(tx.TxOut) < 1 {
		return false
	}
	markerOut := tx.TxOut[len(tx.TxOut)-1]
	if markerOut.Value != 0 {
		return false
	}
	payload, ok := extractMarkerPayload(markerOut.PkScript)
	if !ok {
		return false
	}
	return strings.HasPrefix(payload, "REAP:")
}

func findReapTx(block *btcutil.Block) (*wire.MsgTx, int) {
	if block == nil {
		return nil, -1
	}
	for i, tx := range block.Transactions()[1:] {
		msgTx := tx.MsgTx()
		if isLikelyReapTx(msgTx) {
			return msgTx, i + 1
		}
	}
	return nil, -1
}

func extractMarkerPayload(pkScript []byte) (string, bool) {
	data, err := txscript.PushedData(pkScript)
	if err != nil || len(data) < 1 {
		return "", false
	}
	return string(data[0]), true
}

func parseReapMarkerPayload(payload string) (height int32, count int, digest string, err error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 4 || parts[0] != "REAP" {
		return 0, 0, "", fmt.Errorf("invalid REAP marker payload")
	}
	h, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker height: %w", err)
	}
	c, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker count: %w", err)
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return 0, 0, "", fmt.Errorf("invalid REAP marker digest: %w", err)
	}
	return int32(h), c, parts[3], nil
}

func reapInputDigest(tx *wire.MsgTx) string {
	h := sha256.New()
	var idx [4]byte
	for _, in := range tx.TxIn {
		op := in.PreviousOutPoint
		h.Write(op.Hash[:])
		binary.LittleEndian.PutUint32(idx[:], op.Index)
		h.Write(idx[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func checkReapMarker(tx *wire.MsgTx, txHeight int32,
	chainParams *chaincfg.Params) error {

	if !isLikelyReapTx(tx) {
		return nil
	}
	txHash := tx.TxHash()
	logOBTCDevf(chainParams,
		"REAP marker check start tx=%s blockHeight=%d inputs=%d outputs=%d",
		txHash, txHeight, len(tx.TxIn), len(tx.TxOut))
	payload, ok := extractMarkerPayload(tx.TxOut[len(tx.TxOut)-1].PkScript)
	if !ok {
		logOBTCDevf(chainParams,
			"REAP marker check failed tx=%s reason=unparseable-marker", txHash)
		return ruleError(ErrBadTxInput, "reap transaction has unparseable marker output")
	}
	h, count, digest, err := parseReapMarkerPayload(payload)
	if err != nil {
		logOBTCDevf(chainParams,
			"REAP marker check failed tx=%s reason=parse-error err=%v", txHash, err)
		return ruleError(ErrBadTxInput, err.Error())
	}
	if h != txHeight {
		logOBTCDevf(chainParams,
			"REAP marker check failed tx=%s reason=height-mismatch markerHeight=%d blockHeight=%d",
			txHash, h, txHeight)
		return ruleError(ErrBadTxInput, "reap marker height mismatch")
	}
	if count != len(tx.TxIn) {
		logOBTCDevf(chainParams,
			"REAP marker check failed tx=%s reason=count-mismatch markerCount=%d inputCount=%d",
			txHash, count, len(tx.TxIn))
		return ruleError(ErrBadTxInput, "reap marker count mismatch")
	}
	if digest != reapInputDigest(tx) {
		logOBTCDevf(chainParams,
			"REAP marker check failed tx=%s reason=digest-mismatch markerDigest=%s computedDigest=%s",
			txHash, digest, reapInputDigest(tx))
		return ruleError(ErrBadTxInput, "reap marker digest mismatch")
	}
	logOBTCDevf(chainParams,
		"REAP marker check ok tx=%s markerHeight=%d inputCount=%d digest=%s",
		txHash, h, count, digest)
	return nil
}

func checkReapBlockHardening(block *btcutil.Block, blockHeight int32,
	chainParams *chaincfg.Params) error {

	if block == nil || chainParams == nil {
		return nil
	}

	expiryParams := chaincfg.GetExpiryParams(chainParams)
	if expiryParams == nil || blockHeight < expiryParams.ReapConsensusAtHeight {
		return nil
	}
	logOBTCDevf(chainParams,
		"REAP block hardening check start height=%d txCount=%d activationHeight=%d",
		blockHeight, len(block.Transactions()), expiryParams.ReapConsensusAtHeight)

	reapCount := 0
	for i, tx := range block.Transactions()[1:] {
		if !isLikelyReapTx(tx.MsgTx()) {
			continue
		}

		reapCount++
		if reapCount > 1 {
			logOBTCDevf(chainParams,
				"REAP block hardening failed height=%d secondReapIndex=%d",
				blockHeight, i+1)
			return ruleError(ErrMultipleReapTx, fmt.Sprintf(
				"block contains multiple REAP transactions (second at index %d)",
				i+1,
			))
		}
	}

	logOBTCDevf(chainParams,
		"REAP block hardening ok height=%d reapTxCount=%d", blockHeight, reapCount)

	return nil
}

func checkReapGlobalPrefix(block *btcutil.Block, blockHeight int32,
	source ReapPrefixSource, chainParams *chaincfg.Params) error {

	if block == nil || chainParams == nil {
		return nil
	}

	expiryParams := chaincfg.GetExpiryParams(chainParams)
	if expiryParams == nil || blockHeight < expiryParams.ReapConsensusAtHeight {
		return nil
	}

	reapTx, txIndex := findReapTx(block)
	if reapTx == nil {
		return nil
	}

	txHash := reapTx.TxHash()
	inputCount := len(reapTx.TxIn)
	logOBTCDevf(chainParams,
		"REAP global prefix check start tx=%s blockHeight=%d txIndex=%d inputCount=%d",
		txHash, blockHeight, txIndex, inputCount)

	if source == nil {
		logOBTCDevf(chainParams,
			"REAP global prefix check failed tx=%s reason=no-source", txHash)
		return ruleError(ErrBadReapPrefix,
			"reap prefix source is not configured")
	}

	tipHeight, err := source.ReapPrefixTipHeight()
	if err != nil {
		return err
	}
	parentHeight := blockHeight - 1
	if tipHeight != parentHeight {
		logOBTCDevf(chainParams,
			"REAP global prefix check failed tx=%s reason=tip-mismatch sourceTip=%d parentHeight=%d",
			txHash, tipHeight, parentHeight)
		return ruleError(ErrBadReapPrefix, fmt.Sprintf(
			"reap prefix source tip %d does not match parent height %d",
			tipHeight, parentHeight,
		))
	}

	if inputCount == 0 {
		return nil
	}

	expected, err := source.ReapPrefixCandidates(blockHeight, inputCount)
	if err != nil {
		return err
	}
	if len(expected) < inputCount {
		logOBTCDevf(chainParams,
			"REAP global prefix check failed tx=%s reason=short-prefix got=%d want=%d",
			txHash, len(expected), inputCount)
		return ruleError(ErrBadReapPrefix, fmt.Sprintf(
			"reap transaction has %d inputs but only %d global expired candidates exist",
			inputCount, len(expected),
		))
	}

	for i, txIn := range reapTx.TxIn {
		got := txIn.PreviousOutPoint
		want := expected[i].OutPoint
		if got != want {
			logOBTCDevf(chainParams,
				"REAP global prefix check failed tx=%s reason=prefix-mismatch input=%d got=%s want=%s",
				txHash, i, got.String(), want.String())
			return ruleError(ErrBadReapPrefix, fmt.Sprintf(
				"reap input %d is not the canonical global prefix: got %s want %s",
				i, got.String(), want.String(),
			))
		}
	}

	logOBTCDevf(chainParams,
		"REAP global prefix check ok tx=%s blockHeight=%d inputCount=%d",
		txHash, blockHeight, inputCount)

	return nil
}

type reapInputOrderKey struct {
	expiry uint64
	amount int64
	op     wire.OutPoint
}

func compareReapInputOrderKey(a, b reapInputOrderKey) int {
	if a.expiry != b.expiry {
		if a.expiry < b.expiry {
			return -1
		}
		return 1
	}

	if a.amount != b.amount {
		if a.amount < b.amount {
			return -1
		}
		return 1
	}

	hcmp := bytes.Compare(a.op.Hash[:], b.op.Hash[:])
	if hcmp != 0 {
		return hcmp
	}

	switch {
	case a.op.Index < b.op.Index:
		return -1
	case a.op.Index > b.op.Index:
		return 1
	default:
		return 0
	}
}

func makeReapInputOrderKey(outpoint wire.OutPoint, utxoView *UtxoViewpoint,
	expiryParams *chaincfg.ExpiryParams) (reapInputOrderKey, error) {

	utxo := utxoView.LookupEntry(outpoint)
	if utxo == nil || utxo.IsSpent() {
		return reapInputOrderKey{}, fmt.Errorf("missing utxo for reap input %s", outpoint.String())
	}

	return reapInputOrderKey{
		expiry: expiryParams.CalculateExpiryKey(utxo.BlockHeight()),
		amount: utxo.Amount(),
		op:     outpoint,
	}, nil
}

func checkReapConsensusHardening(tx *wire.MsgTx, txHeight int32,
	utxoView *UtxoViewpoint, chainParams *chaincfg.Params) error {

	if chainParams == nil || utxoView == nil || !isLikelyReapTx(tx) {
		return nil
	}

	expiryParams := chaincfg.GetExpiryParams(chainParams)
	if expiryParams == nil || txHeight < expiryParams.ReapConsensusAtHeight {
		return nil
	}
	txHash := tx.TxHash()
	logOBTCDevf(chainParams,
		"REAP consensus hardening start tx=%s blockHeight=%d inputCount=%d maxInputs=%d dustMaxInputs=%d maxWeight=%d",
		txHash, txHeight, len(tx.TxIn), expiryParams.ReapMaxInputs,
		expiryParams.ReapDustMaxInputs, expiryParams.ReapMaxWeight)

	if expiryParams.ReapMaxWeight > 0 {
		txWeight := GetTransactionWeight(btcutil.NewTx(tx))
		if txWeight > expiryParams.ReapMaxWeight {
			logOBTCDevf(chainParams,
				"REAP consensus hardening failed tx=%s reason=max-weight weight=%d maxWeight=%d",
				txHash, txWeight, expiryParams.ReapMaxWeight)
			return ruleError(ErrBadTxInput, fmt.Sprintf(
				"reap transaction weight %d exceeds consensus limit %d",
				txWeight, expiryParams.ReapMaxWeight,
			))
		}
	}

	twoTierLimits := expiryParams.ReapDustMaxInputs > 0
	if !twoTierLimits && expiryParams.ReapMaxInputs > 0 &&
		len(tx.TxIn) > expiryParams.ReapMaxInputs {

		logOBTCDevf(chainParams,
			"REAP consensus hardening failed tx=%s reason=max-inputs inputCount=%d maxInputs=%d",
			txHash, len(tx.TxIn), expiryParams.ReapMaxInputs)
		return ruleError(ErrBadTxInput, fmt.Sprintf(
			"reap transaction input count %d exceeds consensus limit %d",
			len(tx.TxIn), expiryParams.ReapMaxInputs,
		))
	}

	var prevKey reapInputOrderKey
	dustCount := 0
	normalCount := 0

	for i, txIn := range tx.TxIn {
		curKey, err := makeReapInputOrderKey(txIn.PreviousOutPoint, utxoView, expiryParams)
		if err != nil {
			return ruleError(ErrMissingTxOut, err.Error())
		}
		if i > 0 && compareReapInputOrderKey(prevKey, curKey) > 0 {
			logOBTCDevf(chainParams,
				"REAP consensus hardening failed tx=%s reason=canonical-order prev=%s cur=%s",
				txHash, tx.TxIn[i-1].PreviousOutPoint, tx.TxIn[i].PreviousOutPoint)
			return ruleError(ErrBadTxInput, "reap transaction inputs out of canonical order")
		}

		if twoTierLimits {
			dustTier := isReapDustTierAmount(curKey.amount, expiryParams)
			if dustTier {
				if dustCount >= expiryParams.ReapDustMaxInputs {
					logOBTCDevf(chainParams,
						"REAP consensus hardening failed tx=%s reason=dust-max-inputs dustCount=%d max=%d",
						txHash, dustCount+1, expiryParams.ReapDustMaxInputs)
					return ruleError(ErrBadTxInput, fmt.Sprintf(
						"reap dust input count %d exceeds consensus limit %d",
						dustCount+1, expiryParams.ReapDustMaxInputs,
					))
				}
			} else if normalCount >= expiryParams.ReapMaxInputs {
				logOBTCDevf(chainParams,
					"REAP consensus hardening failed tx=%s reason=normal-max-inputs normalCount=%d max=%d",
					txHash, normalCount+1, expiryParams.ReapMaxInputs)
				return ruleError(ErrBadTxInput, fmt.Sprintf(
					"reap normal input count %d exceeds consensus limit %d",
					normalCount+1, expiryParams.ReapMaxInputs,
				))
			}

			if dustCount >= expiryParams.ReapDustMaxInputs ||
				normalCount >= expiryParams.ReapMaxInputs {

				logOBTCDevf(chainParams,
					"REAP consensus hardening failed tx=%s reason=input-after-tier-cap input=%d dust=%d normal=%d",
					txHash, i, dustCount, normalCount)
				return ruleError(ErrBadTxInput,
					"reap transaction includes inputs after a tier cap was reached")
			}

			if dustTier {
				dustCount++
			} else {
				normalCount++
			}
		}

		prevKey = curKey
	}

	logOBTCDevf(chainParams,
		"REAP consensus hardening ok tx=%s blockHeight=%d inputCount=%d",
		txHash, txHeight, len(tx.TxIn))

	return nil
}

func isReapDustTierAmount(value int64, expiryParams *chaincfg.ExpiryParams) bool {
	return expiryParams != nil && expiryParams.ReapDustThresholdSat > 0 &&
		value > 0 && value < expiryParams.ReapDustThresholdSat
}

func reapTaxForValue(v int64, expiryParams *chaincfg.ExpiryParams) int64 {
	if expiryParams == nil || v <= 0 || expiryParams.ReapTaxDenominator <= 0 ||
		expiryParams.ReapTaxNumerator <= 0 {
		return 0
	}
	return (v * expiryParams.ReapTaxNumerator) / expiryParams.ReapTaxDenominator
}

func applyReapDustRule(value, refund, tax int64,
	expiryParams *chaincfg.ExpiryParams) (int64, int64) {

	if expiryParams == nil || expiryParams.ReapDustThresholdSat <= 0 {
		return refund, tax
	}
	if value > 0 && value < expiryParams.ReapDustThresholdSat {
		return 0, value
	}
	return refund, tax
}

func checkReapTaxRules(tx *wire.MsgTx, txHeight int32, utxoView *UtxoViewpoint,
	chainParams *chaincfg.Params) error {

	if chainParams == nil || utxoView == nil || !isLikelyReapTx(tx) {
		return nil
	}

	expiryParams := chaincfg.GetExpiryParams(chainParams)
	if expiryParams == nil || txHeight < expiryParams.EnableAtHeight {
		return nil
	}
	txHash := tx.TxHash()

	expectedRefundByScript := make(map[string]int64)
	var expectedRefundTotal int64

	for _, txIn := range tx.TxIn {
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil || utxo.IsSpent() {
			return ruleError(ErrMissingTxOut, fmt.Sprintf(
				"utxo %v missing from view during reap tax check",
				txIn.PreviousOutPoint))
		}

		value := utxo.Amount()
		tax := reapTaxForValue(value, expiryParams)
		refund := value - tax
		if refund < 0 {
			return ruleError(ErrBadTxOutValue, "reap refund amount is negative")
		}
		refund, _ = applyReapDustRule(value, refund, tax, expiryParams)
		if refund > 0 {
			expectedRefundByScript[string(utxo.PkScript())] += refund
		}
		expectedRefundTotal += refund
	}

	actualRefundByScript := make(map[string]int64)
	var actualRefundTotal int64
	for _, txOut := range tx.TxOut[:len(tx.TxOut)-1] {
		if txOut.Value <= 0 {
			return ruleError(ErrBadTxOutValue,
				"reap refund outputs must be positive")
		}
		actualRefundByScript[string(txOut.PkScript)] += txOut.Value
		actualRefundTotal += txOut.Value
	}

	if actualRefundTotal != expectedRefundTotal {
		logOBTCDevf(chainParams,
			"REAP tax check failed tx=%s reason=refund-total got=%d want=%d",
			txHash, actualRefundTotal, expectedRefundTotal)
		return ruleError(ErrBadTxOutValue, fmt.Sprintf(
			"reap refund total mismatch: got %d want %d",
			actualRefundTotal, expectedRefundTotal))
	}
	if len(actualRefundByScript) != len(expectedRefundByScript) {
		logOBTCDevf(chainParams,
			"REAP tax check failed tx=%s reason=refund-set-size got=%d want=%d",
			txHash, len(actualRefundByScript), len(expectedRefundByScript))
		return ruleError(ErrBadTxOutValue,
			"reap refund output set does not match expected distribution")
	}

	for script, expected := range expectedRefundByScript {
		if got, ok := actualRefundByScript[script]; !ok || got != expected {
			logOBTCDevf(chainParams,
				"REAP tax check failed tx=%s reason=refund-distribution scriptLen=%d got=%d want=%d",
				txHash, len(script), got, expected)
			return ruleError(ErrBadTxOutValue,
				"reap refund output set does not match expected distribution")
		}
	}

	logOBTCDevf(chainParams,
		"REAP tax check ok tx=%s refundTotal=%d outputGroups=%d",
		txHash, actualRefundTotal, len(actualRefundByScript))

	return nil
}

func checkExpirySpendRules(tx *wire.MsgTx, txHeight int32, utxoView *UtxoViewpoint,
	chainParams *chaincfg.Params) error {

	expiryParams := chaincfg.GetExpiryParams(chainParams)
	if expiryParams == nil {
		return nil
	}
	if txHeight < expiryParams.EnableAtHeight {
		return nil
	}

	isReap := isLikelyReapTx(tx)
	txHash := tx.TxHash()
	expiredInputs := 0
	for _, txIn := range tx.TxIn {
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil || utxo.IsSpent() {
			logOBTCDevf(chainParams,
				"expiry spend check failed tx=%s reason=missing-utxo outpoint=%s",
				txHash, txIn.PreviousOutPoint)
			return ruleError(ErrMissingTxOut, fmt.Sprintf(
				"utxo %v missing from view during expiry check",
				txIn.PreviousOutPoint))
		}
		if utxo.BlockHeight() == unminedInputHeight {
			if isReap {
				logOBTCDevf(chainParams,
					"expiry spend check failed tx=%s reason=reap-spends-unconfirmed outpoint=%s",
					txHash, txIn.PreviousOutPoint)
				return ruleError(ErrBadTxInput,
					"reap transaction spends unconfirmed utxo")
			}
			continue
		}

		expiryHeight := int32(expiryParams.CalculateExpiryKey(utxo.BlockHeight()))
		expired := txHeight >= expiryHeight
		if expired {
			expiredInputs++
		}

		if isReap && !expired {
			logOBTCDevf(chainParams,
				"expiry spend check failed tx=%s reason=reap-spends-live outpoint=%s createHeight=%d expiryHeight=%d blockHeight=%d",
				txHash, txIn.PreviousOutPoint, utxo.BlockHeight(), expiryHeight, txHeight)
			return ruleError(ErrBadTxInput,
				"reap transaction spends non-expired utxo")
		}
		if !isReap && expired {
			logOBTCDevf(chainParams,
				"expiry spend check failed tx=%s reason=nonreap-spends-expired outpoint=%s createHeight=%d expiryHeight=%d blockHeight=%d",
				txHash, txIn.PreviousOutPoint, utxo.BlockHeight(), expiryHeight, txHeight)
			return ruleError(ErrBadTxInput,
				"non-reap transaction spends expired utxo")
		}
	}
	logOBTCDevf(chainParams,
		"expiry spend check ok tx=%s blockHeight=%d isReap=%t inputs=%d expiredInputs=%d",
		txHash, txHeight, isReap, len(tx.TxIn), expiredInputs)
	return nil
}
