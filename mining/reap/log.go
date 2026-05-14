// Copyright (c) 2016 The btcsuite developers
// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

import "github.com/btcsuite/btclog"

var log btclog.Logger

func init() {
	DisableLog()
}

func DisableLog() {
	log = btclog.Disabled
}

func UseLogger(logger btclog.Logger) {
	log = logger
}
