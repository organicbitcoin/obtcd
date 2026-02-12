package reap

import (
	"strings"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const REAPTxVersion = reapTxVersion

// IsLikelyREAPTx identifies REAP system transactions by version and marker
// output shape. It is intentionally policy-level (not full consensus).
func IsLikelyREAPTx(tx *wire.MsgTx) bool {
	if tx == nil {
		return false
	}
	if tx.Version != REAPTxVersion {
		return false
	}
	if len(tx.TxOut) < 1 {
		return false
	}
	markerOut := tx.TxOut[len(tx.TxOut)-1]
	if markerOut.Value != 0 {
		return false
	}
	marker, ok := ExtractMarkerPayload(markerOut.PkScript)
	if !ok {
		return false
	}
	return strings.HasPrefix(marker, "REAP:")
}

// ExtractMarkerPayload parses OP_RETURN marker payload from script.
func ExtractMarkerPayload(pkScript []byte) (string, bool) {
	data, err := txscript.PushedData(pkScript)
	if err != nil || len(data) < 1 {
		return "", false
	}
	return string(data[0]), true
}
