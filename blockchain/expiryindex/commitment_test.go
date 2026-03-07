package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

func TestBuildAndExtractCommitmentRoundTrip(t *testing.T) {
	var root [AccumulatorDigestSize]byte
	for i := range root {
		root[i] = byte(i)
	}

	script := BuildExpiryCommitmentScript(root)
	if len(script) != ExpiryCommitmentScriptLen {
		t.Fatalf("script length: got %d, want %d", len(script), ExpiryCommitmentScriptLen)
	}

	// Build a fake coinbase tx with the commitment.
	msgTx := wire.NewMsgTx(1)
	msgTx.AddTxIn(&wire.TxIn{})
	msgTx.AddTxOut(&wire.TxOut{Value: 5000000000}) // block reward
	msgTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: script})
	tx := btcutil.NewTx(msgTx)

	version, extracted, found := ExtractExpiryCommitment(tx)
	if !found {
		t.Fatal("commitment not found")
	}
	if version != ExpiryCommitmentVersion {
		t.Fatalf("version: got %d, want %d", version, ExpiryCommitmentVersion)
	}
	if extracted != root {
		t.Fatalf("root mismatch: got %x, want %x", extracted, root)
	}
}

func TestExtractCommitmentNotFound(t *testing.T) {
	msgTx := wire.NewMsgTx(1)
	msgTx.AddTxIn(&wire.TxIn{})
	msgTx.AddTxOut(&wire.TxOut{Value: 5000000000})
	tx := btcutil.NewTx(msgTx)

	_, _, found := ExtractExpiryCommitment(tx)
	if found {
		t.Fatal("should not find commitment in tx without one")
	}
}

func TestExtractCommitmentScansFromEnd(t *testing.T) {
	var root1, root2 [AccumulatorDigestSize]byte
	root1[0] = 0xAA
	root2[0] = 0xBB

	msgTx := wire.NewMsgTx(1)
	msgTx.AddTxIn(&wire.TxIn{})
	msgTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: BuildExpiryCommitmentScript(root1)})
	msgTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: BuildExpiryCommitmentScript(root2)})
	tx := btcutil.NewTx(msgTx)

	_, extracted, found := ExtractExpiryCommitment(tx)
	if !found {
		t.Fatal("should find commitment")
	}
	// Should find the LAST one.
	if extracted != root2 {
		t.Fatalf("should extract last commitment: got %x, want %x", extracted, root2)
	}
}

func TestCountExpiryCommitments(t *testing.T) {
	var root [AccumulatorDigestSize]byte
	script := BuildExpiryCommitmentScript(root)

	msgTx := wire.NewMsgTx(1)
	msgTx.AddTxIn(&wire.TxIn{})
	msgTx.AddTxOut(&wire.TxOut{Value: 5000000000})
	tx := btcutil.NewTx(msgTx)
	if CountExpiryCommitments(tx) != 0 {
		t.Fatal("expected 0 commitments")
	}

	msgTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: script})
	tx = btcutil.NewTx(msgTx)
	if CountExpiryCommitments(tx) != 1 {
		t.Fatal("expected 1 commitment")
	}

	msgTx.AddTxOut(&wire.TxOut{Value: 0, PkScript: script})
	tx = btcutil.NewTx(msgTx)
	if CountExpiryCommitments(tx) != 2 {
		t.Fatal("expected 2 commitments")
	}
}

func TestCommitmentScriptFormat(t *testing.T) {
	var root [AccumulatorDigestSize]byte
	root[0] = 0xFF
	script := BuildExpiryCommitmentScript(root)

	// Check OP_RETURN.
	if script[0] != 0x6a {
		t.Fatalf("expected OP_RETURN (0x6a), got 0x%02x", script[0])
	}
	// Check OP_DATA_37.
	if script[1] != 37 {
		t.Fatalf("expected OP_DATA_37, got %d", script[1])
	}
	// Check tag.
	if script[2] != 0x4F || script[3] != 0x45 || script[4] != 0x58 || script[5] != 0x50 {
		t.Fatalf("tag mismatch: got %x", script[2:6])
	}
	// Check version.
	if script[6] != ExpiryCommitmentVersion {
		t.Fatalf("version: got %d, want %d", script[6], ExpiryCommitmentVersion)
	}
	// Check root.
	if script[7] != 0xFF {
		t.Fatalf("root[0]: got 0x%02x, want 0xFF", script[7])
	}
}
