package reap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const reapTxVersion = 3

func BuildBlueprint(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (*wire.MsgTx, error) {
	if view == nil {
		return nil, ErrNilView
	}

	tx := wire.NewMsgTx(reapTxVersion)
	tx.LockTime = uint32(plan.Height)

	var inTotal int64
	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		if entry == nil || entry.IsSpent() {
			return nil, fmt.Errorf("missing utxo in view: %s", op.String())
		}
		inTotal += entry.Amount()
		tx.AddTxIn(&wire.TxIn{PreviousOutPoint: op, Sequence: 0xFFFFFFFE})
	}

	burnScript, err := burnScript(p.BurnPolicy)
	if err != nil {
		return nil, err
	}

	taxTotal := int64(0)
	for _, op := range plan.Inputs {
		entry := view.LookupEntry(op)
		taxTotal += taxForValue(entry.Amount(), p)
	}
	burnTotal := inTotal - taxTotal
	if burnTotal < 0 {
		return nil, fmt.Errorf("negative burn total")
	}

	tx.AddTxOut(&wire.TxOut{Value: burnTotal, PkScript: burnScript})

	markerScript, err := markerScript(plan.Height, len(plan.Inputs), plan.Inputs)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: markerScript})

	if plan.TaxTotal != 0 || plan.BurnTotal != 0 {
		if plan.TaxTotal != taxTotal || plan.BurnTotal != burnTotal {
			return nil, fmt.Errorf("plan totals mismatch: plan tax=%d burn=%d, computed tax=%d burn=%d",
				plan.TaxTotal, plan.BurnTotal, taxTotal, burnTotal)
		}
	}

	if inTotal != burnTotal+taxTotal {
		return nil, fmt.Errorf("input/output invariant broken")
	}

	return tx, nil
}

func burnScript(policy BurnPolicy) ([]byte, error) {
	sb := txscript.NewScriptBuilder()
	switch policy {
	case BurnPolicyOpReturn:
		return sb.AddOp(txscript.OP_RETURN).AddData([]byte("REAP_BURN")).Script()
	case BurnPolicyP2WSHZero:
		return sb.AddOp(txscript.OP_0).AddData(make([]byte, 32)).Script()
	case BurnPolicyP2TRNullKey:
		return sb.AddOp(txscript.OP_1).AddData(make([]byte, 32)).Script()
	default:
		return nil, fmt.Errorf("unknown burn policy %d", policy)
	}
}

func markerScript(height int32, count int, inputs []wire.OutPoint) ([]byte, error) {
	h := sha256.New()
	for _, op := range inputs {
		h.Write(op.Hash[:])
		var idx [4]byte
		idx[0] = byte(op.Index)
		idx[1] = byte(op.Index >> 8)
		idx[2] = byte(op.Index >> 16)
		idx[3] = byte(op.Index >> 24)
		h.Write(idx[:])
	}
	sum := hex.EncodeToString(h.Sum(nil))
	payload := strings.Join([]string{"REAP", strconv.FormatInt(int64(height), 10), strconv.Itoa(count), sum}, ":")
	return txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData([]byte(payload)).Script()
}
