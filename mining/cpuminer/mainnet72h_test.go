// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cpuminer

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestMainNet72hMiningReadinessIsolation(t *testing.T) {
	tests := []struct {
		name   string
		params *chaincfg.Params
		want   bool
	}{
		{name: "nil params remain protected", want: true},
		{name: "bitcoin mainnet remains protected", params: &chaincfg.MainNetParams, want: true},
		{name: "official obtc mainnet remains protected", params: &chaincfg.ObtcMainNetParams, want: true},
		{name: "private rehearsal can mine in isolation", params: &chaincfg.ObtcMainNet72hParams, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			miner := &CPUMiner{cfg: Config{ChainParams: test.params}}
			if got := miner.requiresNetworkReadiness(); got != test.want {
				t.Fatalf("requiresNetworkReadiness got %v want %v", got, test.want)
			}
		})
	}
}
