// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import "github.com/btcsuite/btcd/chaincfg"

func (idx *ExpiryIndex) devLogEnabled() bool {
	return idx != nil && idx.params != nil &&
		idx.params.Net == chaincfg.ObtcRegTestParams.Net
}

func (idx *ExpiryIndex) devLogf(format string, args ...interface{}) {
	if !idx.devLogEnabled() {
		return
	}

	log.Debugf(format, args...)
}
