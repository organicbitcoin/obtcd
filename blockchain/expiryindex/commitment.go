// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"bytes"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
)

// Expiry commitment constants.
//
// The expiry commitment is embedded in the coinbase transaction as an
// OP_RETURN output with a specific tag. It commits to the expiry state
// root of the parent block (pre-state commitment).
//
// Script format:
//   OP_RETURN OP_DATA_37 <TAG(4B)> <VERSION(1B)> <ROOT(32B)>
//
// Total pkScript length: 39 bytes.

// ExpiryCommitmentTag is the 4-byte magic tag for the expiry commitment.
// ASCII: "OEXP" (OBTC EXPiry).
var ExpiryCommitmentTag = []byte{0x4F, 0x45, 0x58, 0x50}

// ExpiryCommitmentVersion is the current commitment format version.
const ExpiryCommitmentVersion = 0x01

// expiryCommitmentDataLen is the data payload length: tag(4) + version(1) + root(32).
const expiryCommitmentDataLen = 4 + 1 + AccumulatorDigestSize // 37

// ExpiryCommitmentScriptLen is the total pkScript length:
// OP_RETURN(1) + OP_DATA_37(1) + data(37) = 39.
const ExpiryCommitmentScriptLen = 1 + 1 + expiryCommitmentDataLen // 39

// BuildExpiryCommitmentScript creates the pkScript for an expiry commitment
// coinbase output.
func BuildExpiryCommitmentScript(root [AccumulatorDigestSize]byte) []byte {
	script := make([]byte, ExpiryCommitmentScriptLen)
	script[0] = txscript.OP_RETURN
	script[1] = txscript.OP_DATA_37
	copy(script[2:6], ExpiryCommitmentTag)
	script[6] = ExpiryCommitmentVersion
	copy(script[7:39], root[:])
	return script
}

// ExtractExpiryCommitment searches a coinbase transaction for an expiry
// commitment output. It scans outputs from last to first (matching the
// witness commitment pattern).
//
// Returns the version, root digest, and whether a commitment was found.
func ExtractExpiryCommitment(tx *btcutil.Tx) (version byte, root [AccumulatorDigestSize]byte, found bool) {
	msgTx := tx.MsgTx()
	for i := len(msgTx.TxOut) - 1; i >= 0; i-- {
		pk := msgTx.TxOut[i].PkScript
		if len(pk) != ExpiryCommitmentScriptLen {
			continue
		}
		if pk[0] != txscript.OP_RETURN || pk[1] != txscript.OP_DATA_37 {
			continue
		}
		if !bytes.Equal(pk[2:6], ExpiryCommitmentTag) {
			continue
		}
		version = pk[6]
		copy(root[:], pk[7:7+AccumulatorDigestSize])
		found = true
		return
	}
	return
}

// CountExpiryCommitments counts how many outputs in a coinbase transaction
// match the expiry commitment tag. Used to detect duplicate commitments.
func CountExpiryCommitments(tx *btcutil.Tx) int {
	count := 0
	msgTx := tx.MsgTx()
	for _, txOut := range msgTx.TxOut {
		pk := txOut.PkScript
		if len(pk) == ExpiryCommitmentScriptLen &&
			pk[0] == txscript.OP_RETURN &&
			pk[1] == txscript.OP_DATA_37 &&
			bytes.Equal(pk[2:6], ExpiryCommitmentTag) {
			count++
		}
	}
	return count
}
