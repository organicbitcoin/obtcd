// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

// ApplyOBTCReplayProtectionScriptFlag enables OBTC replay-protected sighash
// verification for the provided script flags when the feature is active at the
// given block height.
func ApplyOBTCReplayProtectionScriptFlag(base txscript.ScriptFlags,
	params *chaincfg.Params, height int32) txscript.ScriptFlags {

	if chaincfg.IsOBTCReplayProtectionActive(params, height) {
		return base | txscript.ScriptVerifyOBTCReplayProtection
	}

	return base
}
