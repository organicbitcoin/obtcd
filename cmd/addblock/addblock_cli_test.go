package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
)

const (
	// block1Hex builds on top of the regtest genesis block.
	block1Hex = "0000002006226e46111a0b59caaf126043eb5bbf28c34f3a5e332a1fc7b2b73cf18891" +
		"0f71881025ae0d41ce8748b79ac40e5f3197af3bb83a594def7943aff0fce504c638ea6d63f" +
		"fff7f2000000000010200000000010100000000000000000000000000000000000000000000" +
		"00000000000000000000ffffffff025100ffffffff0200f2052a010000001600149b0f9d020" +
		"8b3b425246e16830562a63bf1c701180000000000000000266a24aa21a9ede2f61c3f71d1de" +
		"fd3fa999dfa36953755c690689799962b48bebd836974e8cf90120000000000000000000000" +
		"000000000000000000000000000000000000000000000000000"

	// block2Hex builds on top of block1Hex.
	block2Hex = "00000020e5cd0eed3121abeea4d5ecd9ca792b2bcf3ae1e4957930f689058c7e2456c0" +
		"362a78a11b875d31af2ea493aa5b6b623e0d481f11e69f7147ab974be9da087f3e24696f63f" +
		"fff7f2001000000010200000000010100000000000000000000000000000000000000000000" +
		"00000000000000000000ffffffff025200ffffffff0200f2052a0100000016001470fea1feb" +
		"4969c1f237753ae29c0217c6637835c0000000000000000266a24aa21a9ede2f61c3f71d1de" +
		"fd3fa999dfa36953755c690689799962b48bebd836974e8cf90120000000000000000000000" +
		"000000000000000000000000000000000000000000000000000"
)

var (
	addblockBuildOnce sync.Once
	addblockBinary    string
	addblockBuildErr  error
)

func TestAddBlockCLISmoke(t *testing.T) {
	addblockPath := buildAddBlockBinary(t)

	tempDir := t.TempDir()
	bootstrapPath := filepath.Join(tempDir, "bootstrap.dat")

	block1 := decodeBlockHex(t, block1Hex)
	block2 := decodeBlockHex(t, block2Hex)
	writeBootstrapFile(t, bootstrapPath, []*btcutil.Block{block1, block2})

	cmd := exec.Command(addblockPath,
		"--regtest",
		"--datadir="+tempDir,
		"--infile="+bootstrapPath,
		"--progress=0",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("addblock failed: %v\n%s", err, output)
	}

	stdout := string(output)
	if !strings.Contains(stdout, "2 imported") {
		t.Fatalf("expected import summary to mention 2 imported blocks, got:\n%s", stdout)
	}

	dbPath := filepath.Join(tempDir, netName(&chaincfg.RegressionNetParams), "blocks_ffldb")
	db, err := database.Open("ffldb", dbPath, chaincfg.RegressionNetParams.Net)
	if err != nil {
		t.Fatalf("open imported db: %v", err)
	}
	defer db.Close()

	chain, err := blockchain.New(&blockchain.Config{
		DB:          db,
		ChainParams: &chaincfg.RegressionNetParams,
		TimeSource:  blockchain.NewMedianTime(),
	})
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}

	best := chain.BestSnapshot()
	if best.Height != 2 {
		t.Fatalf("unexpected best height: got %d want %d", best.Height, 2)
	}
	if best.Hash != *block2.Hash() {
		t.Fatalf("unexpected best hash: got %s want %s", best.Hash, block2.Hash())
	}
}

func buildAddBlockBinary(t *testing.T) string {
	t.Helper()

	addblockBuildOnce.Do(func() {
		repoRoot := repoRootFromAddBlock(t)
		outputPath := filepath.Join(t.TempDir(), "addblock")
		if runtime.GOOS == "windows" {
			outputPath += ".exe"
		}

		cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/addblock")
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			addblockBuildErr = execBuildError(err, output)
			return
		}

		addblockBinary = outputPath
	})

	if addblockBuildErr != nil {
		t.Fatal(addblockBuildErr)
	}
	return addblockBinary
}

func repoRootFromAddBlock(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := filepath.Clean(filepath.Join(wd, "../.."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root stat: %v", err)
	}
	return root
}

func decodeBlockHex(t *testing.T, hexStr string) *btcutil.Block {
	t.Helper()

	serializedBlock, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}

	block, err := btcutil.NewBlockFromBytes(serializedBlock)
	if err != nil {
		t.Fatalf("NewBlockFromBytes: %v", err)
	}
	return block
}

func writeBootstrapFile(t *testing.T, path string, blocks []*btcutil.Block) {
	t.Helper()

	var buf bytes.Buffer
	for _, block := range blocks {
		serialized, err := block.Bytes()
		if err != nil {
			t.Fatalf("block.Bytes: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(chaincfg.RegressionNetParams.Net)); err != nil {
			t.Fatalf("write network magic: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(serialized))); err != nil {
			t.Fatalf("write block length: %v", err)
		}
		if _, err := buf.Write(serialized); err != nil {
			t.Fatalf("write block bytes: %v", err)
		}
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
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
