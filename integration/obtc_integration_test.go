//go:build rpctest
// +build rpctest

package integration

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestOBTCREAPEndToEnd(t *testing.T) {
	t.Parallel()

	r, err := rpctest.New(
		&chaincfg.ObtcRegTestParams, nil, []string{"--expiryindex"}, "",
	)
	require.NoError(t, err)
	require.NoError(t, r.SetUp(true, 44))
	t.Cleanup(func() {
		require.NoError(t, r.TearDown())
	})

	height, err := r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 144, height)

	stats, err := getExpiryStats(r.Client)
	require.NoError(t, err)
	require.False(t, stats.Disabled)
	require.EqualValues(t, 144, stats.TipHeight)

	startHeight := int32(145)
	endHeight := int32(145)
	maxResults := 10
	expiring, err := callListExpiring(
		r.Client, &startHeight, &endHeight, &maxResults,
	)
	require.NoError(t, err)
	require.NotEmpty(t, expiring.ExpiringUTXOs)

	generated, err := r.Client.Generate(1)
	require.NoError(t, err)
	require.Len(t, generated, 1)

	block, err := r.Client.GetBlock(generated[0])
	require.NoError(t, err)
	require.Len(t, block.Transactions, 2)

	reapTx := block.Transactions[1]
	require.EqualValues(t, 3, reapTx.Version)
	require.NotEmpty(t, reapTx.TxIn)
	require.True(t, isREAPMarkerOutput(reapTx.TxOut[len(reapTx.TxOut)-1]))

	expiringSet := make(map[wire.OutPoint]struct{}, len(expiring.ExpiringUTXOs))
	for _, utxo := range expiring.ExpiringUTXOs {
		hash, err := chainhash.NewHashFromStr(utxo.TxID)
		require.NoError(t, err)
		expiringSet[wire.OutPoint{Hash: *hash, Index: utxo.Vout}] = struct{}{}
	}

	var matched bool
	for _, txIn := range reapTx.TxIn {
		if _, ok := expiringSet[txIn.PreviousOutPoint]; ok {
			matched = true

			txOut, err := r.Client.GetTxOut(
				&txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index, false,
			)
			require.NoError(t, err)
			require.Nil(t, txOut)
			break
		}
	}
	require.True(t, matched, "expected REAP tx to consume a listed expiring UTXO")
}

func TestOBTCReplayProtectedLegacyTxActivation(t *testing.T) {
	t.Parallel()

	r, err := rpctest.New(&chaincfg.ObtcRegTestParams, nil, nil, "")
	require.NoError(t, err)
	require.NoError(t, r.SetUp(true, 11))
	t.Cleanup(func() {
		require.NoError(t, r.TearDown())
	})

	height, err := r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 111, height)

	privKey, prevPkScript, err := newFixedP2PKHScript()
	require.NoError(t, err)

	fundingHash, err := r.SendOutputs([]*wire.TxOut{{
		Value:    1_000_000,
		PkScript: prevPkScript,
	}}, 10)
	require.NoError(t, err)

	_, err = r.Client.Generate(1)
	require.NoError(t, err)

	height, err = r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 112, height)

	destAddr, err := r.NewAddress()
	require.NoError(t, err)
	destPkScript, err := txscript.PayToAddrScript(destAddr)
	require.NoError(t, err)

	standardTx, err := makeLegacySpendWithHashType(
		*fundingHash, 0, 1_000_000, prevPkScript, destPkScript, privKey,
		txscript.SigHashAll,
	)
	require.NoError(t, err)

	replayTx, err := makeLegacySpendWithHashType(
		*fundingHash, 0, 1_000_000, prevPkScript, destPkScript, privKey,
		txscript.SigHashOBTCReplayProtection|txscript.SigHashAll,
	)
	require.NoError(t, err)

	beforeStandard, err := r.Client.TestMempoolAccept([]*wire.MsgTx{standardTx}, 0)
	require.NoError(t, err)
	require.Len(t, beforeStandard, 1)
	require.True(t, beforeStandard[0].Allowed)

	beforeReplay, err := r.Client.TestMempoolAccept([]*wire.MsgTx{replayTx}, 0)
	require.NoError(t, err)
	require.Len(t, beforeReplay, 1)
	require.False(t, beforeReplay[0].Allowed)
	require.NotEmpty(t, beforeReplay[0].RejectReason)

	_, err = r.Client.SendRawTransaction(replayTx, true)
	require.Error(t, err)

	_, err = r.Client.Generate(1)
	require.NoError(t, err)

	height, err = r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 113, height)

	afterStandard, err := r.Client.TestMempoolAccept([]*wire.MsgTx{standardTx}, 0)
	require.NoError(t, err)
	require.Len(t, afterStandard, 1)
	require.False(t, afterStandard[0].Allowed)
	require.NotEmpty(t, afterStandard[0].RejectReason)

	afterReplay, err := r.Client.TestMempoolAccept([]*wire.MsgTx{replayTx}, 0)
	require.NoError(t, err)
	require.Len(t, afterReplay, 1)
	require.True(t, afterReplay[0].Allowed)

	replayHash, err := r.Client.SendRawTransaction(replayTx, true)
	require.NoError(t, err)

	mined, err := r.Client.Generate(1)
	require.NoError(t, err)
	require.Len(t, mined, 1)

	minedBlock, err := r.Client.GetBlock(mined[0])
	require.NoError(t, err)

	var found bool
	for _, tx := range minedBlock.Transactions {
		if tx.TxHash() == *replayHash {
			found = true
			break
		}
	}
	require.True(t, found, "expected replay-protected tx to be mined after activation")
}

func getExpiryStats(client *rpcclient.Client) (*btcjson.ExpiryIndexStatsResult, error) {
	result, err := client.RawRequest("getexpiryindexstats", nil)
	if err != nil {
		return nil, err
	}

	var stats btcjson.ExpiryIndexStatsResult
	if err := json.Unmarshal(result, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func callListExpiring(client *rpcclient.Client, start, end *int32,
	max *int) (*btcjson.ListExpiringResult, error) {

	params := buildListExpiringParams(start, end, max)
	result, err := client.RawRequest("listexpiring", params)
	if err != nil {
		return nil, err
	}

	var list btcjson.ListExpiringResult
	if err := json.Unmarshal(result, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func buildListExpiringParams(start, end *int32, max *int) []json.RawMessage {
	if start == nil && end == nil && max == nil {
		return nil
	}

	params := make([]json.RawMessage, 0, 3)
	if start != nil {
		params = append(params, mustMarshal(*start))
	} else if end != nil || max != nil {
		params = append(params, json.RawMessage("null"))
	}

	if end != nil {
		params = append(params, mustMarshal(*end))
	} else if max != nil {
		params = append(params, json.RawMessage("null"))
	}

	if max != nil {
		params = append(params, mustMarshal(*max))
	}

	return params
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}

func isREAPMarkerOutput(txOut *wire.TxOut) bool {
	if txOut == nil || txOut.Value != 0 {
		return false
	}

	pushed, err := txscript.PushedData(txOut.PkScript)
	if err != nil || len(pushed) == 0 {
		return false
	}

	return strings.HasPrefix(string(pushed[0]), "REAP:")
}

func newFixedP2PKHScript() (*btcec.PrivateKey, []byte, error) {
	keyBytes, err := hex.DecodeString(
		"1f1e1d1c1b1a191817161514131211101f1e1d1c1b1a19181716151413121110",
	)
	if err != nil {
		return nil, nil, err
	}

	privKey, pubKey := btcec.PrivKeyFromBytes(keyBytes)
	addr, err := btcutil.NewAddressPubKey(pubKey.SerializeCompressed(),
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		return nil, nil, err
	}

	pkScript, err := txscript.PayToAddrScript(addr.AddressPubKeyHash())
	if err != nil {
		return nil, nil, err
	}

	return privKey, pkScript, nil
}

func makeLegacySpendWithHashType(prevHash chainhash.Hash, prevIndex uint32,
	prevValue int64, prevPkScript, destPkScript []byte,
	privKey *btcec.PrivateKey, hashType txscript.SigHashType) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 1_000,
		PkScript: destPkScript,
	})

	sigScript, err := txscript.SignatureScript(
		tx, 0, prevPkScript, hashType, privKey, true,
	)
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].SignatureScript = sigScript
	return tx, nil
}
