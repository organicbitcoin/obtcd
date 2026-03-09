//go:build rpctest
// +build rpctest

package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

var (
	btcctlBuildOnce sync.Once
	btcctlBinary    string
	btcctlBuildErr  error
)

func TestBTCCTLSmoke(t *testing.T) {
	btcctlPath := buildBTCCTLBinary(t)

	h, err := rpctest.New(&chaincfg.RegressionNetParams, nil, nil, "")
	require.NoError(t, err)
	require.NoError(t, h.SetUp(true, 100))
	t.Cleanup(func() {
		require.NoError(t, h.TearDown())
	})

	rpcCfg := h.RPCConfig()
	certFile := filepath.Join(t.TempDir(), "rpc.cert")
	require.NoError(t, os.WriteFile(certFile, rpcCfg.Certificates, 0o600))

	baseArgs := []string{
		"--regtest",
		"--rpcserver=" + rpcCfg.Host,
		"--rpcuser=" + rpcCfg.User,
		"--rpcpass=" + rpcCfg.Pass,
		"--rpccert=" + certFile,
	}

	t.Run("invalidate_and_reconsider_block", func(t *testing.T) {
		blockHashes, err := h.Client.Generate(2)
		require.NoError(t, err)
		require.Len(t, blockHashes, 2)

		stdout := runCLI(t, btcctlPath, append(baseArgs, "invalidateblock",
			blockHashes[1].String())...)
		require.Empty(t, stdout)

		bestHash, err := h.Client.GetBestBlockHash()
		require.NoError(t, err)
		require.Equal(t, *blockHashes[0], *bestHash)

		stdout = runCLI(t, btcctlPath, append(baseArgs, "reconsiderblock",
			blockHashes[1].String())...)
		require.Empty(t, stdout)

		bestHash, err = h.Client.GetBestBlockHash()
		require.NoError(t, err)
		require.Equal(t, *blockHashes[1], *bestHash)
	})

	t.Run("testmempoolaccept_and_gettxspendingprevout", func(t *testing.T) {
		addr, err := h.NewAddress()
		require.NoError(t, err)
		pkScript, err := txscript.PayToAddrScript(addr)
		require.NoError(t, err)

		output := wire.NewTxOut(int64(btcutil.SatoshiPerBitcoin), pkScript)
		tx, err := h.CreateTransaction([]*wire.TxOut{output}, 10, true)
		require.NoError(t, err)

		var txBuf bytes.Buffer
		require.NoError(t, tx.Serialize(&txBuf))
		txHex := hex.EncodeToString(txBuf.Bytes())

		stdout := runCLI(t, btcctlPath, append(baseArgs,
			"testmempoolaccept",
			`["`+txHex+`"]`,
			"0.1",
		)...)

		var acceptResults []btcjson.TestMempoolAcceptResult
		require.NoError(t, json.Unmarshal([]byte(stdout), &acceptResults))
		require.Len(t, acceptResults, 1)
		require.True(t, acceptResults[0].Allowed)
		require.Equal(t, tx.TxHash().String(), acceptResults[0].Txid)

		txHash, err := h.Client.SendRawTransaction(tx, true)
		require.NoError(t, err)

		prevOut := tx.TxIn[0].PreviousOutPoint
		outpointsJSON := `[{"txid":"` + prevOut.Hash.String() + `","vout":` +
			jsonUint(prevOut.Index) + `}]`

		stdout = runCLI(t, btcctlPath, append(baseArgs,
			"gettxspendingprevout",
			outpointsJSON,
		)...)

		var spendingResults []btcjson.GetTxSpendingPrevOutResult
		require.NoError(t, json.Unmarshal([]byte(stdout), &spendingResults))
		require.Len(t, spendingResults, 1)
		require.Equal(t, prevOut.Hash.String(), spendingResults[0].Txid)
		require.Equal(t, prevOut.Index, spendingResults[0].Vout)
		require.Equal(t, txHash.String(), spendingResults[0].SpendingTxid)
	})
}

func buildBTCCTLBinary(t *testing.T) string {
	t.Helper()

	btcctlBuildOnce.Do(func() {
		repoRoot := repoRootFromIntegration(t)
		outputPath := filepath.Join(t.TempDir(), "btcctl")
		if runtime.GOOS == "windows" {
			outputPath += ".exe"
		}

		cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/btcctl")
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			btcctlBuildErr = err
			btcctlBuildErr = execBuildError(err, output)
			return
		}

		btcctlBinary = outputPath
	})

	require.NoError(t, btcctlBuildErr)
	return btcctlBinary
}

func runCLI(t *testing.T, binary string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return string(bytes.TrimSpace(output))
}

func repoRootFromIntegration(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	root := filepath.Clean(filepath.Join(wd, ".."))
	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	return root
}

func execBuildError(err error, output []byte) error {
	if len(output) == 0 {
		return err
	}
	return &cliBuildError{cause: err, output: string(output)}
}

type cliBuildError struct {
	cause  error
	output string
}

func (e *cliBuildError) Error() string {
	return e.cause.Error() + ": " + e.output
}

func jsonUint(v uint32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
