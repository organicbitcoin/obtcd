package rpcclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func setBackendVersion(client *Client, version BackendVersion) {
	client.backendVersionMu.Lock()
	client.backendVersion = version
	client.backendVersionMu.Unlock()
}

func marshalExpectedCmd(t *testing.T, cmd interface{}) string {
	t.Helper()

	payload, err := btcjson.MarshalCmd(btcjson.RpcVersion1, 1, cmd)
	require.NoError(t, err)

	return string(payload)
}

func encodeTxHex(t *testing.T, tx *wire.MsgTx) string {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, tx.Serialize(&buf))

	return hex.EncodeToString(buf.Bytes())
}

func TestInvalidateBlockAsyncRequest(t *testing.T) {
	t.Parallel()

	hash := &chainhash.Hash{1, 2, 3}

	tests := []struct {
		name string
		hash *chainhash.Hash
	}{
		{
			name: "with hash",
			hash: hash,
		},
		{
			name: "nil hash",
			hash: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, serverReceivedChannel, cleanup := makeClient(t)
			defer cleanup()

			client.InvalidateBlockAsync(test.hash)

			expected := marshalExpectedCmd(
				t, btcjson.NewInvalidateBlockCmd(func() string {
					if test.hash == nil {
						return ""
					}
					return test.hash.String()
				}()),
			)
			require.JSONEq(t, expected, <-serverReceivedChannel)
		})
	}
}

func TestReconsiderBlockAsyncRequest(t *testing.T) {
	t.Parallel()

	hash := &chainhash.Hash{4, 5, 6}

	tests := []struct {
		name string
		hash *chainhash.Hash
	}{
		{
			name: "with hash",
			hash: hash,
		},
		{
			name: "nil hash",
			hash: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, serverReceivedChannel, cleanup := makeClient(t)
			defer cleanup()

			client.ReconsiderBlockAsync(test.hash)

			expected := marshalExpectedCmd(
				t, btcjson.NewReconsiderBlockCmd(func() string {
					if test.hash == nil {
						return ""
					}
					return test.hash.String()
				}()),
			)
			require.JSONEq(t, expected, <-serverReceivedChannel)
		})
	}
}

func TestTestMempoolAcceptAsyncValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		version       BackendVersion
		txns          []*wire.MsgTx
		maxFeeRate    btcjson.BTCPerkvB
		expectedError error
	}{
		{
			name:          "unsupported backend version",
			version:       BtcdPre2401,
			txns:          []*wire.MsgTx{wire.NewMsgTx(1)},
			expectedError: ErrBackendVersion,
		},
		{
			name:          "empty tx list",
			version:       BtcdPost2401,
			txns:          nil,
			expectedError: ErrInvalidParam,
		},
		{
			name:          "too many txs",
			version:       BtcdPost2401,
			txns:          make([]*wire.MsgTx, 26),
			expectedError: ErrInvalidParam,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{}
			setBackendVersion(client, test.version)

			_, err := client.TestMempoolAcceptAsync(
				test.txns, test.maxFeeRate,
			).Receive()
			require.ErrorIs(t, err, test.expectedError)
		})
	}
}

func TestGetTxSpendingPrevOutAsyncValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		version       BackendVersion
		outpoints     []wire.OutPoint
		expectedError error
	}{
		{
			name:          "unsupported backend version",
			version:       BtcdPre2401,
			outpoints:     []wire.OutPoint{{}},
			expectedError: ErrBackendVersion,
		},
		{
			name:          "empty outpoints",
			version:       BtcdPost2401,
			outpoints:     nil,
			expectedError: ErrInvalidParam,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{}
			setBackendVersion(client, test.version)

			_, err := client.GetTxSpendingPrevOutAsync(
				test.outpoints,
			).Receive()
			require.ErrorIs(t, err, test.expectedError)
		})
	}
}

func TestTestMempoolAcceptAsyncRequest(t *testing.T) {
	t.Parallel()

	client, serverReceivedChannel, cleanup := makeClient(t)
	defer cleanup()
	setBackendVersion(client, BtcdPost2401)

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 2},
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		PkScript: []byte{0x51},
	})

	maxFeeRate := btcjson.BTCPerkvB(0.01)
	client.TestMempoolAcceptAsync([]*wire.MsgTx{tx}, maxFeeRate)

	expected := marshalExpectedCmd(
		t, btcjson.NewTestMempoolAcceptCmd(
			[]string{encodeTxHex(t, tx)}, maxFeeRate,
		),
	)
	require.JSONEq(t, expected, <-serverReceivedChannel)
}

func TestGetTxSpendingPrevOutAsyncRequest(t *testing.T) {
	t.Parallel()

	client, serverReceivedChannel, cleanup := makeClient(t)
	defer cleanup()
	setBackendVersion(client, BtcdPost2401)

	outpoint := wire.OutPoint{Hash: chainhash.Hash{9, 8, 7}, Index: 3}
	client.GetTxSpendingPrevOutAsync([]wire.OutPoint{outpoint})

	expected := marshalExpectedCmd(
		t, btcjson.NewGetTxSpendingPrevOutCmd([]wire.OutPoint{outpoint}),
	)
	require.JSONEq(t, expected, <-serverReceivedChannel)
}

func TestFutureTestMempoolAcceptResultReceive(t *testing.T) {
	t.Parallel()

	validResults := []*btcjson.TestMempoolAcceptResult{{
		Txid:    (&chainhash.Hash{1}).String(),
		Wtxid:   (&chainhash.Hash{2}).String(),
		Allowed: true,
		Vsize:   123,
	}}
	validPayload, err := json.Marshal(validResults)
	require.NoError(t, err)

	tests := []struct {
		name          string
		response      *Response
		expected      []*btcjson.TestMempoolAcceptResult
		expectedError string
	}{
		{
			name:     "valid json",
			response: &Response{result: validPayload},
			expected: validResults,
		},
		{
			name:          "invalid json",
			response:      &Response{result: []byte("{")},
			expectedError: "unexpected end of JSON input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			future := make(chan *Response, 1)
			future <- test.response

			results, err := FutureTestMempoolAcceptResult(future).Receive()
			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)
				require.Nil(t, results)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected, results)
		})
	}
}

func TestFutureGetTxSpendingPrevOutReceive(t *testing.T) {
	t.Parallel()

	validResults := []*btcjson.GetTxSpendingPrevOutResult{{
		Txid:         (&chainhash.Hash{3}).String(),
		Vout:         1,
		SpendingTxid: (&chainhash.Hash{4}).String(),
	}}
	validPayload, err := json.Marshal(validResults)
	require.NoError(t, err)

	tests := []struct {
		name          string
		response      *Response
		expected      []*btcjson.GetTxSpendingPrevOutResult
		expectedError string
	}{
		{
			name:     "valid json",
			response: &Response{result: validPayload},
			expected: validResults,
		},
		{
			name:          "invalid json",
			response:      &Response{result: []byte("{")},
			expectedError: "unexpected end of JSON input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			future := make(chan *Response, 1)
			future <- test.response

			results, err := FutureGetTxSpendingPrevOut(future).Receive()
			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)
				require.Nil(t, results)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expected, results)
		})
	}
}
