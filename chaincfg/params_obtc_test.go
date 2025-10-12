// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"testing"

	"github.com/btcsuite/btcd/wire"
)

// TestOBTCNetworkMagic verifies that OBTC networks use unique magic numbers.
func TestOBTCNetworkMagic(t *testing.T) {
	tests := []struct {
		name   string
		params *Params
		magic  wire.BitcoinNet
	}{
		{
			name:   "OBTC MainNet",
			params: &ObtcMainNetParams,
			magic:  wire.ObtcMainNet,
		},
		{
			name:   "OBTC TestNet",
			params: &ObtcTestNetParams,
			magic:  wire.ObtcTestNet,
		},
		{
			name:   "OBTC RegTest",
			params: &ObtcRegTestParams,
			magic:  wire.ObtcRegNet,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.params.Net != test.magic {
				t.Errorf("%s: expected magic %x, got %x",
					test.name, test.magic, test.params.Net)
			}
		})
	}
}

// TestIsOBTC verifies the IsOBTC detection function works correctly.
func TestIsOBTC(t *testing.T) {
	tests := []struct {
		name     string
		params   *Params
		expected bool
	}{
		{
			name:     "OBTC MainNet should be detected as OBTC",
			params:   &ObtcMainNetParams,
			expected: true,
		},
		{
			name:     "OBTC TestNet should be detected as OBTC",
			params:   &ObtcTestNetParams,
			expected: true,
		},
		{
			name:     "OBTC RegTest should be detected as OBTC",
			params:   &ObtcRegTestParams,
			expected: true,
		},
		{
			name:     "Bitcoin MainNet should NOT be detected as OBTC",
			params:   &MainNetParams,
			expected: false,
		},
		{
			name:     "Bitcoin TestNet3 should NOT be detected as OBTC",
			params:   &TestNet3Params,
			expected: false,
		},
		{
			name:     "Bitcoin SimNet should NOT be detected as OBTC",
			params:   &SimNetParams,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsOBTC(test.params)
			if result != test.expected {
				t.Errorf("%s: expected %t, got %t",
					test.name, test.expected, result)
			}
		})
	}
}

// TestOBTCNetworkUniqueness verifies that OBTC networks don't conflict with Bitcoin networks.
func TestOBTCNetworkUniqueness(t *testing.T) {
	obtcNets := []wire.BitcoinNet{
		wire.ObtcMainNet,
		wire.ObtcTestNet,
		wire.ObtcRegNet,
	}

	bitcoinNets := []wire.BitcoinNet{
		wire.MainNet,
		wire.TestNet,
		wire.TestNet3,
		wire.TestNet4,
		wire.SigNet,
		wire.SimNet,
	}

	// Check that no OBTC network magic conflicts with Bitcoin networks
	for _, obtcNet := range obtcNets {
		for _, bitcoinNet := range bitcoinNets {
			if obtcNet == bitcoinNet {
				t.Errorf("OBTC network magic %x conflicts with Bitcoin network %x",
					obtcNet, bitcoinNet)
			}
		}
	}

	// Check that OBTC networks don't conflict with each other
	for i, net1 := range obtcNets {
		for j, net2 := range obtcNets {
			if i != j && net1 == net2 {
				t.Errorf("OBTC network magics conflict: %x == %x", net1, net2)
			}
		}
	}
}

// TestOBTCAddressParameters verifies that OBTC uses unique address parameters.
func TestOBTCAddressParameters(t *testing.T) {
	// Test that OBTC uses different address prefixes than Bitcoin
	tests := []struct {
		name          string
		obtcParams    *Params
		bitcoinParams *Params
	}{
		{
			name:          "OBTC MainNet vs Bitcoin MainNet",
			obtcParams:    &ObtcMainNetParams,
			bitcoinParams: &MainNetParams,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Verify HRP is different
			if test.obtcParams.Bech32HRPSegwit == test.bitcoinParams.Bech32HRPSegwit {
				t.Errorf("OBTC and Bitcoin use same Bech32 HRP: %s",
					test.obtcParams.Bech32HRPSegwit)
			}

			// Verify address IDs are different
			if test.obtcParams.PubKeyHashAddrID == test.bitcoinParams.PubKeyHashAddrID {
				t.Errorf("OBTC and Bitcoin use same PubKeyHashAddrID: %x",
					test.obtcParams.PubKeyHashAddrID)
			}

			if test.obtcParams.ScriptHashAddrID == test.bitcoinParams.ScriptHashAddrID {
				t.Errorf("OBTC and Bitcoin use same ScriptHashAddrID: %x",
					test.obtcParams.ScriptHashAddrID)
			}

			if test.obtcParams.PrivateKeyID == test.bitcoinParams.PrivateKeyID {
				t.Errorf("OBTC and Bitcoin use same PrivateKeyID: %x",
					test.obtcParams.PrivateKeyID)
			}
		})
	}
}

// TestOBTCPortsUnique verifies that OBTC uses different default ports than Bitcoin.
func TestOBTCPortsUnique(t *testing.T) {
	tests := []struct {
		name          string
		obtcParams    *Params
		bitcoinParams *Params
	}{
		{
			name:          "OBTC MainNet port vs Bitcoin MainNet",
			obtcParams:    &ObtcMainNetParams,
			bitcoinParams: &MainNetParams,
		},
		{
			name:          "OBTC TestNet port vs Bitcoin TestNet3",
			obtcParams:    &ObtcTestNetParams,
			bitcoinParams: &TestNet3Params,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.obtcParams.DefaultPort == test.bitcoinParams.DefaultPort {
				t.Errorf("%s: OBTC and Bitcoin use same default port: %s",
					test.name, test.obtcParams.DefaultPort)
			}
		})
	}
}

// BenchmarkIsOBTC benchmarks the IsOBTC function performance.
func BenchmarkIsOBTC(b *testing.B) {
	params := &ObtcMainNetParams
	for i := 0; i < b.N; i++ {
		IsOBTC(params)
	}
}
