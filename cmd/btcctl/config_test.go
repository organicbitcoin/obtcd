package main

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
)

func TestNormalizeAddressOBTCDefaultPorts(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		chain     *chaincfg.Params
		useWallet bool
		want      string
	}{
		{
			name:  "obtc mainnet node rpc",
			addr:  "127.0.0.1",
			chain: &chaincfg.ObtcMainNetParams,
			want:  "127.0.0.1:9528",
		},
		{
			name:  "obtc testnet node rpc",
			addr:  "127.0.0.1",
			chain: &chaincfg.ObtcTestNetParams,
			want:  "127.0.0.1:19528",
		},
		{
			name:  "obtc regtest node rpc",
			addr:  "127.0.0.1",
			chain: &chaincfg.ObtcRegTestParams,
			want:  "127.0.0.1:29528",
		},
		{
			name:      "obtc mainnet wallet rpc",
			addr:      "127.0.0.1",
			chain:     &chaincfg.ObtcMainNetParams,
			useWallet: true,
			want:      "127.0.0.1:9554",
		},
		{
			name:      "obtc testnet wallet rpc",
			addr:      "127.0.0.1",
			chain:     &chaincfg.ObtcTestNetParams,
			useWallet: true,
			want:      "127.0.0.1:19554",
		},
		{
			name:      "obtc regtest wallet rpc",
			addr:      "127.0.0.1",
			chain:     &chaincfg.ObtcRegTestParams,
			useWallet: true,
			want:      "127.0.0.1:29554",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeAddress(tc.addr, tc.chain, tc.useWallet)
			if err != nil {
				t.Fatalf("normalizeAddress returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeAddress() = %q, want %q", got, tc.want)
			}
		})
	}
}
