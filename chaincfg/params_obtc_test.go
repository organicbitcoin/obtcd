// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"testing"
	"time"

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

// TestOBTCAddressParameters verifies that OBTC uses isolated address/key
// namespaces versus Bitcoin networks and across OBTC networks.
func TestOBTCAddressParameters(t *testing.T) {
	type namedParams struct {
		name   string
		params *Params
	}

	obtcNets := []namedParams{
		{name: "obtc mainnet", params: &ObtcMainNetParams},
		{name: "obtc testnet", params: &ObtcTestNetParams},
		{name: "obtc regtest", params: &ObtcRegTestParams},
	}
	btcNets := []namedParams{
		{name: "bitcoin mainnet", params: &MainNetParams},
		{name: "bitcoin testnet3", params: &TestNet3Params},
		{name: "bitcoin testnet4", params: &TestNet4Params},
		{name: "bitcoin regtest", params: &RegressionNetParams},
		{name: "bitcoin simnet", params: &SimNetParams},
		{name: "bitcoin signet", params: &SigNetParams},
	}

	checkByteField := func(field string, getter func(*Params) byte) {
		t.Helper()
		seen := make(map[byte]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision within OBTC: %s and %s both use 0x%02x", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision between %s and %s: 0x%02x", field, prev, net.name, v)
			}
		}
	}

	checkStringField := func(field string, getter func(*Params) string) {
		t.Helper()
		seen := make(map[string]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision within OBTC: %s and %s both use %q", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision between %s and %s: %q", field, prev, net.name, v)
			}
		}
	}

	checkHDField := func(field string, getter func(*Params) [4]byte) {
		t.Helper()
		seen := make(map[[4]byte]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision within OBTC: %s and %s both use %x", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				t.Fatalf("%s collision between %s and %s: %x", field, prev, net.name, v)
			}
		}
	}

	checkCoinType := func() {
		t.Helper()
		seen := make(map[uint32]string)
		for _, net := range obtcNets {
			v := net.params.HDCoinType
			if prev, ok := seen[v]; ok {
				t.Fatalf("HDCoinType collision within OBTC: %s and %s both use %d", prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			if prev, ok := seen[net.params.HDCoinType]; ok {
				t.Fatalf("HDCoinType collision between %s and %s: %d", prev, net.name, net.params.HDCoinType)
			}
		}
	}

	checkStringField("Bech32HRPSegwit", func(p *Params) string { return p.Bech32HRPSegwit })
	checkByteField("PubKeyHashAddrID", func(p *Params) byte { return p.PubKeyHashAddrID })
	checkByteField("ScriptHashAddrID", func(p *Params) byte { return p.ScriptHashAddrID })
	checkByteField("PrivateKeyID", func(p *Params) byte { return p.PrivateKeyID })
	checkByteField("WitnessPubKeyHashAddrID", func(p *Params) byte { return p.WitnessPubKeyHashAddrID })
	checkByteField("WitnessScriptHashAddrID", func(p *Params) byte { return p.WitnessScriptHashAddrID })
	checkHDField("HDPrivateKeyID", func(p *Params) [4]byte { return p.HDPrivateKeyID })
	checkHDField("HDPublicKeyID", func(p *Params) [4]byte { return p.HDPublicKeyID })
	checkCoinType()

	for _, net := range obtcNets {
		if net.params.HDPrivateKeyID == net.params.HDPublicKeyID {
			t.Fatalf("%s has identical HD private/public ids: %x", net.name, net.params.HDPrivateKeyID)
		}
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

func TestOBTCTestNetPublicParams(t *testing.T) {
	params := &ObtcTestNetParams
	if *params.GenesisHash == *TestNet3Params.GenesisHash {
		t.Fatalf("OBTC testnet must not share Bitcoin testnet3 genesis")
	}
	if params.GenesisBlock.BlockHash() != *params.GenesisHash {
		t.Fatalf("OBTC testnet genesis hash mismatch: block=%s params=%s",
			params.GenesisBlock.BlockHash(), params.GenesisHash)
	}
	if params.TargetTimePerBlock != 10*time.Minute {
		t.Fatalf("expected 10m target block time, got %s", params.TargetTimePerBlock)
	}
	if params.CoinbaseMaturity != 20 {
		t.Fatalf("expected coinbase maturity 20, got %d", params.CoinbaseMaturity)
	}
	if !params.GenerateSupported {
		t.Fatalf("OBTC testnet must support controlled generate RPC")
	}
	if !params.PoWNoRetargeting {
		t.Fatalf("OBTC testnet should keep controlled public-testnet PoW stable")
	}

	expiryParams := GetExpiryParams(params)
	if expiryParams == nil {
		t.Fatal("expected OBTC testnet expiry params")
	}
	if expiryParams.WindowBlocks != 144 {
		t.Fatalf("expected 144-block expiry window, got %d", expiryParams.WindowBlocks)
	}
	if expiryParams.EnableAtHeight != 100 ||
		expiryParams.ExpiryCommitmentEnableAtHeight != 100 ||
		expiryParams.ReapConsensusAtHeight != 120 ||
		expiryParams.ReplayProtectionAtHeight != 130 {

		t.Fatalf("unexpected public testnet activation heights: %+v", expiryParams)
	}
}

// BenchmarkIsOBTC benchmarks the IsOBTC function performance.
func BenchmarkIsOBTC(b *testing.B) {
	params := &ObtcMainNetParams
	for i := 0; i < b.N; i++ {
		IsOBTC(params)
	}
}

// TestOBTCForkHeights verifies that OBTC fork heights are properly defined.
func TestOBTCForkHeights(t *testing.T) {
	tests := []struct {
		name           string
		params         *Params
		expectedHeight int32
	}{
		{
			name:           "OBTC MainNet fork height",
			params:         &ObtcMainNetParams,
			expectedHeight: ObtcMainNetForkHeight,
		},
		{
			name:           "OBTC TestNet fork height",
			params:         &ObtcTestNetParams,
			expectedHeight: ObtcTestNetForkHeight,
		},
		{
			name:           "OBTC RegTest fork height",
			params:         &ObtcRegTestParams,
			expectedHeight: ObtcRegTestForkHeight,
		},
		{
			name:           "Bitcoin MainNet (no fork)",
			params:         &MainNetParams,
			expectedHeight: -1, // Not an OBTC network
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			height := GetOBTCForkHeight(test.params)
			if height != test.expectedHeight {
				t.Errorf("%s: expected fork height %d, got %d",
					test.name, test.expectedHeight, height)
			}
		})
	}
}

// TestIsPostOBTCFork verifies the post-fork detection logic.
func TestIsPostOBTCFork(t *testing.T) {
	tests := []struct {
		name       string
		params     *Params
		height     int32
		expectPost bool
	}{
		{
			name:       "OBTC MainNet before fork",
			params:     &ObtcMainNetParams,
			height:     ObtcMainNetForkHeight - 1,
			expectPost: false,
		},
		{
			name:       "OBTC MainNet at fork",
			params:     &ObtcMainNetParams,
			height:     ObtcMainNetForkHeight,
			expectPost: true,
		},
		{
			name:       "OBTC MainNet after fork",
			params:     &ObtcMainNetParams,
			height:     ObtcMainNetForkHeight + 1000,
			expectPost: true,
		},
		{
			name:       "OBTC TestNet at genesis",
			params:     &ObtcTestNetParams,
			height:     ObtcTestNetForkHeight,
			expectPost: true,
		},
		{
			name:       "OBTC TestNet after fork",
			params:     &ObtcTestNetParams,
			height:     ObtcTestNetForkHeight + 100,
			expectPost: true,
		},
		{
			name:       "Bitcoin MainNet (never post-fork)",
			params:     &MainNetParams,
			height:     1000000, // Any height
			expectPost: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isPost := IsPostOBTCFork(test.params, test.height)
			if isPost != test.expectPost {
				t.Errorf("%s: expected post-fork %v, got %v",
					test.name, test.expectPost, isPost)
			}
		})
	}
}

// TestOBTCForkHeightValues verifies fork heights are reasonable.
func TestOBTCForkHeightValues(t *testing.T) {
	// MainNet fork height should remain in the current provisional launch range.
	if ObtcMainNetForkHeight < 1_000_000 || ObtcMainNetForkHeight > 1_100_000 {
		t.Errorf("MainNet fork height %d seems outside the current provisional launch range", ObtcMainNetForkHeight)
	}
	if ObtcMainNetFirstBlockHeight != ObtcMainNetForkHeight+1 {
		t.Errorf("MainNet first block height got %d want %d",
			ObtcMainNetFirstBlockHeight, ObtcMainNetForkHeight+1)
	}
	if ObtcMainNetActivationHeight != ObtcMainNetForkHeight+2016 {
		t.Errorf("MainNet activation height got %d want %d",
			ObtcMainNetActivationHeight, ObtcMainNetForkHeight+2016)
	}
	if ObtcMainNetParams.ForkDAAStartHeight != ObtcMainNetFirstBlockHeight {
		t.Errorf("MainNet ForkDAA start got %d want %d",
			ObtcMainNetParams.ForkDAAStartHeight, ObtcMainNetFirstBlockHeight)
	}
	if ObtcMainNetParams.ForkDAABootstrapEndHeight != ObtcMainNetActivationHeight {
		t.Errorf("MainNet ForkDAA bootstrap end got %d want %d",
			ObtcMainNetParams.ForkDAABootstrapEndHeight, ObtcMainNetActivationHeight)
	}
	if ObtcMainNetParams.ForkDAAForkResetBits != 0x1d00ffff {
		t.Errorf("MainNet ForkDAA reset bits got %08x", ObtcMainNetParams.ForkDAAForkResetBits)
	}

	// TestNet is an independent chain, so its OBTC rules are active from genesis.
	if ObtcTestNetForkHeight != 0 {
		t.Errorf("TestNet fork height %d should be 0", ObtcTestNetForkHeight)
	}
	if ObtcTestNetParams.ForkDAAForkResetBits != 0 {
		t.Errorf("TestNet ForkDAA should remain disabled, got reset bits %08x",
			ObtcTestNetParams.ForkDAAForkResetBits)
	}

	// RegTest fork height should be low for development
	if ObtcRegTestForkHeight <= 0 || ObtcRegTestForkHeight > 1000 {
		t.Errorf("RegTest fork height %d should be low for development", ObtcRegTestForkHeight)
	}
}

func TestOBTCReplayProtectionActivation(t *testing.T) {
	tests := []struct {
		name       string
		params     *Params
		before     int32
		at         int32
		expectInit bool
	}{
		{
			name:       "obtc mainnet",
			params:     &ObtcMainNetParams,
			expectInit: true,
		},
		{
			name:       "obtc testnet",
			params:     &ObtcTestNetParams,
			expectInit: true,
		},
		{
			name:       "obtc regtest",
			params:     &ObtcRegTestParams,
			expectInit: true,
		},
		{
			name:       "bitcoin mainnet",
			params:     &MainNetParams,
			expectInit: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := GetOBTCReplayProtectionHeight(tc.params)
			if tc.expectInit {
				if h <= 0 {
					t.Fatalf("expected positive replay activation height, got %d", h)
				}
				if IsOBTCReplayProtectionActive(tc.params, h-1) {
					t.Fatalf("replay protection should be inactive before activation")
				}
				if !IsOBTCReplayProtectionActive(tc.params, h) {
					t.Fatalf("replay protection should be active at activation height")
				}
			} else {
				if h != -1 {
					t.Fatalf("expected non-OBTC replay activation height -1, got %d", h)
				}
				if IsOBTCReplayProtectionActive(tc.params, 1_000_000) {
					t.Fatalf("bitcoin network should never activate OBTC replay protection")
				}
			}
		})
	}
}

func TestGetExpiryParamsDirect(t *testing.T) {
	if p := GetExpiryParams(&MainNetParams); p != nil {
		t.Fatalf("bitcoin mainnet should not have OBTC expiry params")
	}

	for _, tc := range []struct {
		name   string
		params *Params
	}{
		{"obtc mainnet", &ObtcMainNetParams},
		{"obtc testnet", &ObtcTestNetParams},
		{"obtc regtest", &ObtcRegTestParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := GetExpiryParams(tc.params)
			if p == nil {
				t.Fatalf("expected non-nil expiry params")
			}
			if p.WindowBlocks == 0 {
				t.Fatalf("expected positive WindowBlocks")
			}
			if p.StartScanHeight != 0 {
				t.Fatalf("expected StartScanHeight to begin at genesis, got %d", p.StartScanHeight)
			}
			if tc.params.Net == ObtcMainNetParams.Net {
				want := ObtcMainNetActivationHeight
				if p.EnableAtHeight != want {
					t.Fatalf("expected mainnet EnableAtHeight %d, got %d", want, p.EnableAtHeight)
				}
				if p.ReapConsensusAtHeight != want || p.ReplayProtectionAtHeight != want {
					t.Fatalf("expected mainnet staged heights to collapse to %d: %+v", want, p)
				}
				if p.ExpiryCommitmentEnableAtHeight != want {
					t.Fatalf("expected mainnet commitment activation %d, got %d",
						want, p.ExpiryCommitmentEnableAtHeight)
				}
			}
			if p.ReapConsensusAtHeight < p.EnableAtHeight {
				t.Fatalf("expected ReapConsensusAtHeight >= EnableAtHeight: %+v", p)
			}
			if p.ReplayProtectionAtHeight < p.ReapConsensusAtHeight {
				t.Fatalf("expected ReplayProtectionAtHeight >= ReapConsensusAtHeight: %+v", p)
			}
			if p.ReapMaxInputs <= 0 {
				t.Fatalf("expected positive ReapMaxInputs: %+v", p)
			}
			if p.ReapDustMaxInputs <= 0 {
				t.Fatalf("expected positive ReapDustMaxInputs: %+v", p)
			}
			if p.ReapTaxNumerator <= 0 || p.ReapTaxDenominator <= 0 {
				t.Fatalf("expected positive REAP tax parameters: %+v", p)
			}
			if p.ReapDustThresholdSat < 0 {
				t.Fatalf("expected non-negative REAP dust threshold: %+v", p)
			}
		})
	}
}

func TestCalculateExpiryKeyDirect(t *testing.T) {
	p := &ExpiryParams{WindowBlocks: 144}
	if got, want := p.CalculateExpiryKey(100), uint64(244); got != want {
		t.Fatalf("CalculateExpiryKey mismatch: got %d want %d", got, want)
	}
}

func TestOBTCDeploymentsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		params *Params
	}{
		{"obtc mainnet", &ObtcMainNetParams},
		{"obtc testnet", &ObtcTestNetParams},
		{"obtc regtest", &ObtcRegTestParams},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for i, dep := range tc.params.Deployments {
				if dep.DeploymentStarter == nil {
					t.Fatalf("deployment %d starter is nil", i)
				}
				if dep.DeploymentEnder == nil {
					t.Fatalf("deployment %d ender is nil", i)
				}
			}
		})
	}
}

func TestValidateOBTCNamespaceIsolationDirect(t *testing.T) {
	if err := validateOBTCNamespaceIsolation(); err != nil {
		t.Fatalf("namespace isolation validation failed: %v", err)
	}
}
