package main

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestDefaultRPCPortOBTCNetworks(t *testing.T) {
	tests := []struct {
		name   string
		params *chaincfg.Params
		want   string
	}{
		{
			name:   "obtc mainnet",
			params: &chaincfg.ObtcMainNetParams,
			want:   "9528",
		},
		{
			name:   "obtc testnet",
			params: &chaincfg.ObtcTestNetParams,
			want:   "19528",
		},
		{
			name:   "obtc regtest",
			params: &chaincfg.ObtcRegTestParams,
			want:   "29528",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultRPCPort(tc.params); got != tc.want {
				t.Fatalf("defaultRPCPort() = %q, want %q", got, tc.want)
			}
		})
	}
}
