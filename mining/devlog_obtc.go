package mining

import "github.com/btcsuite/btcd/chaincfg"

func isOBTCDevLogNet(params *chaincfg.Params) bool {
	return params != nil && params.Net == chaincfg.ObtcRegTestParams.Net
}

func logOBTCDevf(params *chaincfg.Params, format string, args ...interface{}) {
	if !isOBTCDevLogNet(params) {
		return
	}

	log.Debugf(format, args...)
}
