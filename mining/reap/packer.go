// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const reapTxVersion = 3

type refundOutput struct {
	pkScript []byte
	value    int64
}

func BuildBlueprint(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (*wire.MsgTx, error) {
	if view == nil {
		return nil, ErrNilView
	}
	p.debugLogf("REAP blueprint build start height=%d inputs=%d expectedRefund=%d expectedTax=%d",
		plan.Height, len(plan.Inputs), plan.RefundTotal, plan.TaxTotal)

	tx := wire.NewMsgTx(reapTxVersion)
	// Keep the tx final for inclusion at plan.Height while preserving a
	// height-tied locktime hint.
	if plan.Height > 0 {
		tx.LockTime = uint32(plan.Height - 1)
	}

	var inTotal int64
	taxTotal := int64(0)
	refundByScript := make(map[string]*refundOutput)

	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			return nil, fmt.Errorf("missing utxo in view: %s", op.String())
		}
		amt := entry.Amount()
		inTotal += amt
		tax := taxForValue(amt, p)
		refund := amt - tax
		if refund < 0 {
			return nil, fmt.Errorf("negative refund amount")
		}
		refund, tax = applyDustRule(amt, refund, tax, p.DustThresholdSat)
		taxTotal += tax

		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: 0xFFFFFFFE})

		if refund == 0 {
			continue
		}
		key := string(entry.PkScript())
		if out, ok := refundByScript[key]; ok {
			out.value += refund
		} else {
			refundByScript[key] = &refundOutput{pkScript: append([]byte(nil), entry.PkScript()...), value: refund}
		}
	}

	refundOutputs := make([]refundOutput, 0, len(refundByScript))
	for _, out := range refundByScript {
		refundOutputs = append(refundOutputs, *out)
	}
	slices.SortFunc(refundOutputs, func(a, b refundOutput) int {
		return bytes.Compare(a.pkScript, b.pkScript)
	})

	refundTotal := int64(0)
	for _, out := range refundOutputs {
		refundTotal += out.value
		tx.AddTxOut(&wire.TxOut{Value: out.value, PkScript: out.pkScript})
	}

	markerScript, err := markerScript(plan.Height, len(plan.Inputs), plan.Inputs)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerScript})

	if plan.TaxTotal != 0 || plan.RefundTotal != 0 {
		if plan.TaxTotal != taxTotal || plan.RefundTotal != refundTotal {
			return nil, fmt.Errorf("plan totals mismatch: plan tax=%d refund=%d, computed tax=%d refund=%d",
				plan.TaxTotal, plan.RefundTotal, taxTotal, refundTotal)
		}
	}

	if inTotal != refundTotal+taxTotal {
		return nil, fmt.Errorf("input/output invariant broken")
	}

	p.debugLogf("REAP blueprint build done height=%d txid=%s inputs=%d refundOutputs=%d refund=%d tax=%d marker=%s",
		plan.Height, tx.TxHash(), len(tx.TxIn), len(refundOutputs), refundTotal, taxTotal,
		MarkerDigest(plan.Inputs))

	return tx, nil
}

func markerScript(height int32, count int, inputs []wire.OutPoint) ([]byte, error) {
	sum := MarkerDigest(inputs)
	payload := strings.Join([]string{"REAP", strconv.FormatInt(int64(height), 10), strconv.Itoa(count), sum}, ":")
	return txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte(payload)).Script()
}
