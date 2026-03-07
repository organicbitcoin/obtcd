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

func checkReapMarker(tx *wire.MsgTx, txHeight int32) error {
	if !isLikelyReapTx(tx) {
		return nil
	}
	payload, ok := extractMarkerPayload(tx.TxOut[len(tx.TxOut)-1].PkScript)
	if !ok {
		return ruleError(ErrBadTxInput, "reap transaction has unparseable marker output")
	}
	h, count, digest, err := parseReapMarkerPayload(payload)
	if err != nil {
		return ruleError(ErrBadTxInput, err.Error())
	}
	if h != txHeight {
		return ruleError(ErrBadTxInput, "reap marker height mismatch")
	}
	if count != len(tx.TxIn) {
		return ruleError(ErrBadTxInput, "reap marker count mismatch")
	}
	if digest != reapInputDigest(tx) {
		return ruleError(ErrBadTxInput, "reap marker digest mismatch")
	}
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

	reapCount := 0
	for i, tx := range block.Transactions()[1:] {
		if !isLikelyReapTx(tx.MsgTx()) {
			continue
		}

		reapCount++
		if reapCount > 1 {
			return ruleError(ErrMultipleReapTx, fmt.Sprintf(
				"block contains multiple REAP transactions (second at index %d)",
				i+1,
			))
		}
	}

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

	if expiryParams.ReapMaxInputs > 0 && len(tx.TxIn) > expiryParams.ReapMaxInputs {
		return ruleError(ErrBadTxInput, fmt.Sprintf(
			"reap transaction input count %d exceeds consensus limit %d",
			len(tx.TxIn), expiryParams.ReapMaxInputs,
		))
	}

	if len(tx.TxIn) <= 1 {
		return nil
	}

	prevKey, err := makeReapInputOrderKey(tx.TxIn[0].PreviousOutPoint, utxoView, expiryParams)
	if err != nil {
		return ruleError(ErrMissingTxOut, err.Error())
	}

	for i := 1; i < len(tx.TxIn); i++ {
		curKey, err := makeReapInputOrderKey(tx.TxIn[i].PreviousOutPoint, utxoView, expiryParams)
		if err != nil {
			return ruleError(ErrMissingTxOut, err.Error())
		}
		if compareReapInputOrderKey(prevKey, curKey) > 0 {
			return ruleError(ErrBadTxInput, "reap transaction inputs out of canonical order")
		}
		prevKey = curKey
	}

	return nil
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
		return ruleError(ErrBadTxOutValue, fmt.Sprintf(
			"reap refund total mismatch: got %d want %d",
			actualRefundTotal, expectedRefundTotal))
	}
	if len(actualRefundByScript) != len(expectedRefundByScript) {
		return ruleError(ErrBadTxOutValue,
			"reap refund output set does not match expected distribution")
	}

	for script, expected := range expectedRefundByScript {
		if got, ok := actualRefundByScript[script]; !ok || got != expected {
			return ruleError(ErrBadTxOutValue,
				"reap refund output set does not match expected distribution")
		}
	}

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
	for _, txIn := range tx.TxIn {
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil || utxo.IsSpent() {
			return ruleError(ErrMissingTxOut, fmt.Sprintf(
				"utxo %v missing from view during expiry check",
				txIn.PreviousOutPoint))
		}
		expiryHeight := int32(expiryParams.CalculateExpiryKey(utxo.BlockHeight()))
		expired := txHeight >= expiryHeight

		if isReap && !expired {
			return ruleError(ErrBadTxInput,
				"reap transaction spends non-expired utxo")
		}
		if !isReap && expired {
			return ruleError(ErrBadTxInput,
				"non-reap transaction spends expired utxo")
		}
	}
	return nil
}
