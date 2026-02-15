package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

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
	for _, in := range tx.TxIn {
		op := in.PreviousOutPoint
		h.Write(op.Hash[:])
		var idx [4]byte
		idx[0] = byte(op.Index)
		idx[1] = byte(op.Index >> 8)
		idx[2] = byte(op.Index >> 16)
		idx[3] = byte(op.Index >> 24)
		h.Write(idx[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func checkReapMarker(tx *wire.MsgTx, txHeight int32) error {
	if !isLikelyReapTx(tx) {
		return nil
	}
	payload, _ := extractMarkerPayload(tx.TxOut[len(tx.TxOut)-1].PkScript)
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
			continue
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
