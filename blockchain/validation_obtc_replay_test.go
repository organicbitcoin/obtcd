// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func TestApplyOBTCReplayProtectionScriptFlag(t *testing.T) {
	base := txscript.ScriptVerifyWitness

	if got := ApplyOBTCReplayProtectionScriptFlag(base, &chaincfg.MainNetParams, 1_000_000); got != base {
		t.Fatalf("bitcoin network should not enable OBTC replay flag: got %v want %v", got, base)
	}

	obtcParams := &chaincfg.ObtcRegTestParams
	replayHeight := chaincfg.GetOBTCReplayProtectionHeight(obtcParams)
	if replayHeight <= 0 {
		t.Fatalf("invalid replay height for obtc regtest: %d", replayHeight)
	}

	before := ApplyOBTCReplayProtectionScriptFlag(base, obtcParams, replayHeight-1)
	if before&txscript.ScriptVerifyOBTCReplayProtection != 0 {
		t.Fatalf("replay flag unexpectedly enabled before activation: height=%d", replayHeight-1)
	}

	after := ApplyOBTCReplayProtectionScriptFlag(base, obtcParams, replayHeight)
	if after&txscript.ScriptVerifyOBTCReplayProtection == 0 {
		t.Fatalf("replay flag not enabled at activation height: %d", replayHeight)
	}
}
