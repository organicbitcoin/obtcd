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
