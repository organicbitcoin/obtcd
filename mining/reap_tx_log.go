package mining

import (
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/mining/reap"
	"github.com/btcsuite/btcd/txscript"
)

func formatREAPTxForLog(tx *btcutil.Tx) string {
	if tx == nil {
		return "<nil>"
	}

	msgTx := tx.MsgTx()
	var b strings.Builder
	fmt.Fprintf(&b, "txid=%s version=%d locktime=%d inputs=[",
		tx.Hash(), msgTx.Version, msgTx.LockTime)

	for i, txIn := range msgTx.TxIn {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "{idx=%d prevout=%s sequence=%d}",
			i, txIn.PreviousOutPoint, txIn.Sequence)
	}

	b.WriteString("] outputs=[")
	for i, txOut := range msgTx.TxOut {
		if i > 0 {
			b.WriteString(", ")
		}

		scriptAsm, err := txscript.DisasmString(txOut.PkScript)
		if err != nil {
			scriptAsm = fmt.Sprintf("%x", txOut.PkScript)
		}

		payload, ok := reap.ExtractMarkerPayload(txOut.PkScript)
		if ok && strings.HasPrefix(payload, "REAP:") {
			fmt.Fprintf(&b,
				"{idx=%d value=%d type=marker payload=%q script=%q}",
				i, txOut.Value, payload, scriptAsm)
			continue
		}

		fmt.Fprintf(&b, "{idx=%d value=%d script=%q}",
			i, txOut.Value, scriptAsm)
	}

	b.WriteString("]")
	return b.String()
}
