//go:build rpctest
// +build rpctest

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
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

type replayTxBuilder func(prevHash chainhash.Hash, prevIndex uint32,
	prevValue int64, destPkScript []byte,
	hashType txscript.SigHashType) (*wire.MsgTx, error)

type replayActivationCase struct {
	name    string
	prepare func(t *testing.T) ([]byte, replayTxBuilder)
}

func TestOBTCReplayProtectedSegWitTxActivation(t *testing.T) {
	t.Parallel()

	testCases := []replayActivationCase{
		{
			name: "p2wpkh",
			prepare: func(t *testing.T) ([]byte, replayTxBuilder) {
				t.Helper()

				privKey, fundingPkScript, subScript, err := newFixedP2WPKHSpendData()
				require.NoError(t, err)

				builder := func(prevHash chainhash.Hash, prevIndex uint32,
					prevValue int64, destPkScript []byte,
					hashType txscript.SigHashType) (*wire.MsgTx, error) {

					return makeWitnessPubKeyHashSpendWithHashType(
						prevHash, prevIndex, prevValue, fundingPkScript,
						subScript, destPkScript, privKey, hashType,
					)
				}

				return fundingPkScript, builder
			},
		},
		{
			name: "p2wsh",
			prepare: func(t *testing.T) ([]byte, replayTxBuilder) {
				t.Helper()

				privKey, fundingPkScript, witnessScript, err := newFixedP2WSHSpendData()
				require.NoError(t, err)

				builder := func(prevHash chainhash.Hash, prevIndex uint32,
					prevValue int64, destPkScript []byte,
					hashType txscript.SigHashType) (*wire.MsgTx, error) {

					return makeWitnessScriptHashSpendWithHashType(
						prevHash, prevIndex, prevValue, fundingPkScript,
						witnessScript, destPkScript, privKey, hashType,
					)
				}

				return fundingPkScript, builder
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			runOBTCReplayProtectionActivationCase(t, testCase)
		})
	}
}

func TestOBTCReplayProtectedTaprootTxActivation(t *testing.T) {
	t.Parallel()

	testCases := []replayActivationCase{
		{
			name: "key_path",
			prepare: func(t *testing.T) ([]byte, replayTxBuilder) {
				t.Helper()

				privKey, fundingPkScript, err := newFixedTaprootKeySpendData()
				require.NoError(t, err)

				builder := func(prevHash chainhash.Hash, prevIndex uint32,
					prevValue int64, destPkScript []byte,
					hashType txscript.SigHashType) (*wire.MsgTx, error) {

					return makeTaprootKeySpendWithHashType(
						prevHash, prevIndex, prevValue, fundingPkScript,
						destPkScript, privKey, hashType,
					)
				}

				return fundingPkScript, builder
			},
		},
		{
			name: "tapscript",
			prepare: func(t *testing.T) ([]byte, replayTxBuilder) {
				t.Helper()

				spendData, err := newFixedTapscriptSpendData()
				require.NoError(t, err)

				builder := func(prevHash chainhash.Hash, prevIndex uint32,
					prevValue int64, destPkScript []byte,
					hashType txscript.SigHashType) (*wire.MsgTx, error) {

					return makeTapscriptSpendWithHashType(
						prevHash, prevIndex, prevValue, spendData,
						destPkScript, hashType,
					)
				}

				return spendData.pkScript, builder
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			runOBTCReplayProtectionActivationCase(t, testCase)
		})
	}
}

func runOBTCReplayProtectionActivationCase(t *testing.T, testCase replayActivationCase) {
	t.Helper()

	r, err := rpctest.New(&chaincfg.ObtcRegTestParams, nil, nil, "")
	require.NoError(t, err)
	require.NoError(t, r.SetUp(true, 11))
	t.Cleanup(func() {
		require.NoError(t, r.TearDown())
	})

	height, err := r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 111, height)

	fundingPkScript, buildTx := testCase.prepare(t)
	fundingHash, err := r.SendOutputs([]*wire.TxOut{{
		Value:    1_000_000,
		PkScript: fundingPkScript,
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

	standardTx, err := buildTx(
		*fundingHash, 0, 1_000_000, destPkScript, txscript.SigHashAll,
	)
	require.NoError(t, err)

	replayTx, err := buildTx(
		*fundingHash, 0, 1_000_000, destPkScript,
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

func newFixedPrivateKey() (*btcec.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(
		"1f1e1d1c1b1a191817161514131211101f1e1d1c1b1a19181716151413121110",
	)
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(keyBytes)
	return privKey, nil
}

func newPubKeyHashScript(pubKey *btcec.PublicKey) ([]byte, error) {
	addr, err := btcutil.NewAddressPubKey(pubKey.SerializeCompressed(),
		&chaincfg.ObtcRegTestParams)
	if err != nil {
		return nil, err
	}

	return txscript.PayToAddrScript(addr.AddressPubKeyHash())
}

func newFixedP2PKHScript() (*btcec.PrivateKey, []byte, error) {
	privKey, err := newFixedPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	pkScript, err := newPubKeyHashScript(privKey.PubKey())
	if err != nil {
		return nil, nil, err
	}

	return privKey, pkScript, nil
}

func newFixedP2WPKHSpendData() (*btcec.PrivateKey, []byte, []byte, error) {
	privKey, err := newFixedPrivateKey()
	if err != nil {
		return nil, nil, nil, err
	}

	pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
	witnessAddr, err := btcutil.NewAddressWitnessPubKeyHash(
		pubKeyHash, &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	fundingPkScript, err := txscript.PayToAddrScript(witnessAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	subScript, err := newPubKeyHashScript(privKey.PubKey())
	if err != nil {
		return nil, nil, nil, err
	}

	return privKey, fundingPkScript, subScript, nil
}

func newFixedP2WSHSpendData() (*btcec.PrivateKey, []byte, []byte, error) {
	privKey, err := newFixedPrivateKey()
	if err != nil {
		return nil, nil, nil, err
	}

	witnessScript, err := txscript.NewScriptBuilder().
		AddData(privKey.PubKey().SerializeCompressed()).
		AddOp(txscript.OP_CHECKSIG).
		Script()
	if err != nil {
		return nil, nil, nil, err
	}

	scriptHash := sha256.Sum256(witnessScript)
	witnessAddr, err := btcutil.NewAddressWitnessScriptHash(
		scriptHash[:], &chaincfg.ObtcRegTestParams,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	fundingPkScript, err := txscript.PayToAddrScript(witnessAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	return privKey, fundingPkScript, witnessScript, nil
}

func newFixedTaprootKeySpendData() (*btcec.PrivateKey, []byte, error) {
	privKey, err := newFixedPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	outputKey := txscript.ComputeTaprootKeyNoScript(privKey.PubKey())
	pkScript, err := txscript.PayToTaprootScript(outputKey)
	if err != nil {
		return nil, nil, err
	}

	return privKey, pkScript, nil
}

type fixedTapscriptSpendData struct {
	privKey      *btcec.PrivateKey
	pkScript     []byte
	tapLeaf      txscript.TapLeaf
	tapScript    []byte
	controlBlock []byte
}

func newFixedTapscriptSpendData() (*fixedTapscriptSpendData, error) {
	privKey, err := newFixedPrivateKey()
	if err != nil {
		return nil, err
	}

	internalKey := privKey.PubKey()
	tapScript, err := txscript.NewScriptBuilder().
		AddData(schnorr.SerializePubKey(internalKey)).
		AddOp(txscript.OP_CHECKSIG).
		Script()
	if err != nil {
		return nil, err
	}

	tapLeaf := txscript.NewBaseTapLeaf(tapScript)
	tapScriptTree := txscript.AssembleTaprootScriptTree(tapLeaf)
	ctrlBlock := tapScriptTree.LeafMerkleProofs[0].ToControlBlock(
		internalKey,
	)
	ctrlBlockBytes, err := ctrlBlock.ToBytes()
	if err != nil {
		return nil, err
	}

	tapScriptRootHash := tapScriptTree.RootNode.TapHash()
	outputKey := txscript.ComputeTaprootOutputKey(
		internalKey, tapScriptRootHash[:],
	)
	pkScript, err := txscript.PayToTaprootScript(outputKey)
	if err != nil {
		return nil, err
	}

	return &fixedTapscriptSpendData{
		privKey:      privKey,
		pkScript:     pkScript,
		tapLeaf:      tapLeaf,
		tapScript:    tapScript,
		controlBlock: ctrlBlockBytes,
	}, nil
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

func makeWitnessPubKeyHashSpendWithHashType(prevHash chainhash.Hash,
	prevIndex uint32, prevValue int64, prevPkScript, subScript,
	destPkScript []byte, privKey *btcec.PrivateKey,
	hashType txscript.SigHashType) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 1_000,
		PkScript: destPkScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, prevValue)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	witness, err := txscript.WitnessSignature(
		tx, sigHashes, 0, prevValue, subScript, hashType, privKey, true,
	)
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].Witness = witness
	return tx, nil
}

func makeWitnessScriptHashSpendWithHashType(prevHash chainhash.Hash,
	prevIndex uint32, prevValue int64, prevPkScript, witnessScript,
	destPkScript []byte, privKey *btcec.PrivateKey,
	hashType txscript.SigHashType) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 1_000,
		PkScript: destPkScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, prevValue)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	sig, err := txscript.RawTxInWitnessSignature(
		tx, sigHashes, 0, prevValue, witnessScript, hashType, privKey,
	)
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].Witness = wire.TxWitness{sig, witnessScript}
	return tx, nil
}

func makeTaprootKeySpendWithHashType(prevHash chainhash.Hash, prevIndex uint32,
	prevValue int64, prevPkScript, destPkScript []byte,
	privKey *btcec.PrivateKey, hashType txscript.SigHashType) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 1_000,
		PkScript: destPkScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevPkScript, prevValue)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	witness, err := txscript.TaprootWitnessSignature(
		tx, sigHashes, 0, prevValue, prevPkScript, hashType, privKey,
		taprootReplaySigHashOptions(hashType)...,
	)
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].Witness = witness
	return tx, nil
}

func makeTapscriptSpendWithHashType(prevHash chainhash.Hash, prevIndex uint32,
	prevValue int64, spendData *fixedTapscriptSpendData,
	destPkScript []byte, hashType txscript.SigHashType) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    prevValue - 1_000,
		PkScript: destPkScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(
		spendData.pkScript, prevValue,
	)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, prevValue, spendData.pkScript,
		spendData.tapLeaf, hashType, spendData.privKey,
		taprootReplaySigHashOptions(hashType)...,
	)
	if err != nil {
		return nil, err
	}

	tx.TxIn[0].Witness = wire.TxWitness{
		sig, spendData.tapScript, spendData.controlBlock,
	}
	return tx, nil
}

func taprootReplaySigHashOptions(hashType txscript.SigHashType) []txscript.TaprootSigHashOption {
	if hashType&txscript.SigHashOBTCReplayProtection != txscript.SigHashOBTCReplayProtection {
		return nil
	}

	return []txscript.TaprootSigHashOption{
		txscript.WithOBTCReplayProtectionSighash(),
	}
}

// ---------------------------------------------------------------------------
// TestOBTCRPCObservabilityAndFilters covers:
//   - getreapplan before and after REAP activation
//   - getexpirycommitment (enabled state)
//   - listexpiring with min_amount_sat filter and improved pagination
// ---------------------------------------------------------------------------

func TestOBTCRPCObservabilityAndFilters(t *testing.T) {
	t.Parallel()

	// Start node with expiry index enabled.  ObtcRegTest activates REAP at
	// height ObtcRegTestForkHeight+10 = 110.  We mine 44 extra blocks (total
	// 144) so the expiry window is populated.
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

	// --- getreapplan ---

	reapPlan, err := callGetReapPlan(r.Client)
	require.NoError(t, err)
	require.True(t, reapPlan.Enabled, "getreapplan: expected Enabled=true")
	require.True(t, reapPlan.Active, "getreapplan: expected Active=true at height 145")
	require.EqualValues(t, 145, reapPlan.Height)
	// The genesis coinbase outputs from blocks 1..100 all expire at block 101+window.
	// With the default REAP params there should be at least one picked input.
	require.Greater(t, reapPlan.Picked, 0, "getreapplan: expected at least one picked UTXO")
	require.Greater(t, reapPlan.TaxTotal, int64(0), "getreapplan: expected positive tax_total")
	require.Greater(t, reapPlan.RefundTotal, int64(0), "getreapplan: expected positive refund_total")
	require.Greater(t, reapPlan.EstWeight, int64(0), "getreapplan: expected positive est_weight")
	require.NotEmpty(t, reapPlan.MarkerHash, "getreapplan: expected non-empty marker_hash")

	// --- getexpirycommitment ---

	commitment, err := callGetExpiryCommitment(r.Client)
	require.NoError(t, err)
	require.True(t, commitment.Enabled, "getexpirycommitment: expected Enabled=true")
	require.NotEmpty(t, commitment.Root, "getexpirycommitment: expected non-empty root")
	require.EqualValues(t, 144, commitment.TipHeight)
	require.NotEmpty(t, commitment.TipHash)
	// ObtcRegTest ExpiryCommitmentEnableAtHeight = ObtcRegTestForkHeight+10 = 110.
	require.EqualValues(t, chaincfg.ObtcRegTestForkHeight+10, commitment.EnableAtHeight)
	require.True(t, commitment.Active, "getexpirycommitment: expected Active=true at height 144")
	require.True(t, commitment.ActiveAtNextHeight)

	// --- listexpiring with amount_sat populated ---

	startHeight := int32(145)
	endHeight := int32(145)
	maxResults := 10
	expiring, err := callListExpiring(r.Client, &startHeight, &endHeight, &maxResults)
	require.NoError(t, err)
	require.NotEmpty(t, expiring.ExpiringUTXOs, "expected at least one expiring UTXO")
	// Every returned UTXO should have amount_sat > 0 (they are coinbase rewards).
	for _, utxo := range expiring.ExpiringUTXOs {
		require.Greater(t, utxo.AmountSat, int64(0),
			"expected positive amount_sat for UTXO %s:%d", utxo.TxID, utxo.Vout)
	}

	// --- listexpiring with min_amount_sat filter ---

	// Filter by a very large amount: should return an empty list.
	hugeAmt := int64(1_000_000_000_000)
	filtered, err := callListExpiringWithMinAmount(r.Client, &startHeight, &endHeight, &maxResults, &hugeAmt)
	require.NoError(t, err)
	require.Empty(t, filtered.ExpiringUTXOs,
		"expected empty result with impossibly large min_amount_sat filter")

	// Filter by zero: should behave same as no filter.
	zeroAmt := int64(0)
	unfiltered, err := callListExpiringWithMinAmount(r.Client, &startHeight, &endHeight, &maxResults, &zeroAmt)
	require.NoError(t, err)
	require.Equal(t, len(expiring.ExpiringUTXOs), len(unfiltered.ExpiringUTXOs),
		"expected same results with min_amount_sat=0 as without filter")

	// --- pagination cursor advances even when all items are filtered ---

	// Scan with a small page size (1) but a filter that passes everything,
	// and verify we can paginate through all items.
	pageSize := 1
	var allPaginated []btcjson.ExpiringUTXOResult
	var cursor *string
	scanStart := startHeight
	scanEnd := endHeight
	for {
		page, err := callListExpiringWithCursor(r.Client, &scanStart, &scanEnd, &pageSize, cursor)
		require.NoError(t, err)
		allPaginated = append(allPaginated, page.ExpiringUTXOs...)
		if page.NextOutpoint == nil {
			break
		}
		cursor = page.NextOutpoint
		if page.NextHeight != nil {
			scanStart = *page.NextHeight
		}
		if len(allPaginated) > 1000 {
			t.Fatal("pagination did not terminate within 1000 pages")
		}
	}
	// We should have paginated through at least as many items as the first
	// full-page scan returned.
	require.GreaterOrEqual(t, len(allPaginated), len(expiring.ExpiringUTXOs))
}

func TestOBTCListExpiringOrderingAndCursorContract(t *testing.T) {
	t.Parallel()

	r, err := rpctest.New(
		&chaincfg.ObtcRegTestParams, nil, []string{"--expiryindex"}, "",
	)
	require.NoError(t, err)
	require.NoError(t, r.SetUp(true, 44))
	t.Cleanup(func() {
		require.NoError(t, r.TearDown())
	})

	outputs := make([]*wire.TxOut, 0, 3)
	for _, btc := range []int64{90, 91, 92} {
		addr, err := r.NewAddress()
		require.NoError(t, err)
		pkScript, err := txscript.PayToAddrScript(addr)
		require.NoError(t, err)
		outputs = append(outputs, &wire.TxOut{
			Value:    btc * btcutil.SatoshiPerBitcoin,
			PkScript: pkScript,
		})
	}

	fundingHash, err := r.SendOutputs(outputs, 10)
	require.NoError(t, err)
	_, err = r.Client.Generate(1)
	require.NoError(t, err)
	_, err = r.Client.Generate(143)
	require.NoError(t, err)
	height, err := r.Client.GetBlockCount()
	require.NoError(t, err)
	require.EqualValues(t, 288, height)

	startHeight := int32(289)
	endHeight := int32(289)
	maxResults := 100
	minAmount := int64(90 * btcutil.SatoshiPerBitcoin)

	fullPage, err := callListExpiringWithMinAmount(r.Client, &startHeight, &endHeight, &maxResults, &minAmount)
	require.NoError(t, err)
	require.Len(t, fullPage.ExpiringUTXOs, 3, "expected only the high-value outputs from the crafted transaction")

	for i, utxo := range fullPage.ExpiringUTXOs {
		require.Equal(t, fundingHash.String(), utxo.TxID)
		require.EqualValues(t, i, utxo.Vout)
	}

	pageSize := 1
	scanStart := startHeight
	scanEnd := endHeight
	var cursor *string
	var paginated []btcjson.ExpiringUTXOResult
	for pages := 0; ; pages++ {
		require.Less(t, pages, 200, "pagination should terminate")
		page, err := callListExpiringWithCursorAndMinAmount(
			r.Client, &scanStart, &scanEnd, &pageSize, cursor, &minAmount,
		)
		require.NoError(t, err)
		paginated = append(paginated, page.ExpiringUTXOs...)
		if page.NextOutpoint == nil {
			break
		}
		cursor = page.NextOutpoint
		if page.NextHeight != nil {
			scanStart = *page.NextHeight
		}
	}

	require.Equal(t, fullPage.ExpiringUTXOs, paginated,
		"cursor pagination under min_amount_sat filter should preserve ordering and membership")
}

func callGetReapPlan(client *rpcclient.Client) (*btcjson.GetReapPlanResult, error) {
	result, err := client.RawRequest("getreapplan", nil)
	if err != nil {
		return nil, err
	}
	var plan btcjson.GetReapPlanResult
	if err := json.Unmarshal(result, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func callGetExpiryCommitment(client *rpcclient.Client) (*btcjson.GetExpiryCommitmentResult, error) {
	result, err := client.RawRequest("getexpirycommitment", nil)
	if err != nil {
		return nil, err
	}
	var c btcjson.GetExpiryCommitmentResult
	if err := json.Unmarshal(result, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func callListExpiringWithMinAmount(client *rpcclient.Client, start, end *int32,
	max *int, minAmt *int64) (*btcjson.ListExpiringResult, error) {

	params := buildListExpiringParamsFull(start, end, max, nil, minAmt)
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

func callListExpiringWithCursor(client *rpcclient.Client, start, end *int32,
	max *int, cursor *string) (*btcjson.ListExpiringResult, error) {

	params := buildListExpiringParamsFull(start, end, max, cursor, nil)
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

func callListExpiringWithCursorAndMinAmount(client *rpcclient.Client, start, end *int32,
	max *int, cursor *string, minAmt *int64) (*btcjson.ListExpiringResult, error) {

	params := buildListExpiringParamsFull(start, end, max, cursor, minAmt)
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

// buildListExpiringParamsFull builds the JSON-RPC params array for listexpiring,
// supporting the extended parameters (start_after, min_amount_sat).
func buildListExpiringParamsFull(start, end *int32, max *int, cursor *string, minAmt *int64) []json.RawMessage {
	// Determine how many positional params we need.
	needCursor := cursor != nil
	needMinAmt := minAmt != nil
	if !needCursor && !needMinAmt && start == nil && end == nil && max == nil {
		return nil
	}

	params := make([]json.RawMessage, 0, 5)
	appendOrNull := func(v interface{}, include bool) {
		if include {
			params = append(params, mustMarshal(v))
		} else {
			params = append(params, json.RawMessage("null"))
		}
	}

	appendOrNull(start, start != nil)
	appendOrNull(end, end != nil)
	appendOrNull(max, max != nil)
	appendOrNull(cursor, needCursor)
	if needMinAmt {
		appendOrNull(minAmt, true)
	}
	return params
}
