// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// OBTC-only: network parameters and expiry policy.

package chaincfg

import (
	"fmt"
	"math/big"
	"time"

	"github.com/btcsuite/btcd/wire"
)

// obtcPowLimit is the highest proof of work value a OBTC block can have.
// This is the same as Bitcoin's testnet limit to allow easier testing.
var obtcPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 224), bigOne)

// ExpiryParams defines parameters for UTXO expiry calculation
// OBTC uses height-based expiry for deterministic and consensus-friendly behavior
type ExpiryParams struct {
	WindowBlocks             uint64 // Expiry window in blocks
	ListBatchLimit           int    // Max items returned in one RPC scan
	StartScanHeight          int32  // Height to start building index (genesis for OBTC)
	EnableAtHeight           int32  // Height to enable expiry enforcement (Week 3+)
	ReapConsensusAtHeight    int32  // Height to enforce canonical REAP ordering / limits
	ReplayProtectionAtHeight int32  // Height to enforce OBTC replay-protected sighash domain
	ReapMaxInputs            int    // Consensus max REAP inputs per transaction (0 = disabled)
	ReapTaxNumerator         int64  // Consensus REAP tax numerator
	ReapTaxDenominator       int64  // Consensus REAP tax denominator
	ReapDustThresholdSat     int64  // Refunds for inputs below this value fold fully into tax

	// ExpiryCommitmentEnableAtHeight is the height at which the expiry
	// commitment in coinbase becomes mandatory. Before this height the
	// commitment is optional; at or above it, blocks must include a valid
	// expiry commitment and the root must match the local state.
	ExpiryCommitmentEnableAtHeight int32
}

// OBTC Hard Fork Heights
//
// These constants define the block heights at which OBTC diverges from Bitcoin.
// Before these heights: Follow Bitcoin consensus rules exactly
// After these heights: Apply OBTC-specific consensus modifications
const (
	// ObtcMainNetForkHeight defines when OBTC mainnet forks from Bitcoin mainnet
	// The current candidate value targets the Q2 2026 launch window.
	ObtcMainNetForkHeight int32 = 950000

	// ObtcTestNetForkHeight defines when OBTC testnet forks from Bitcoin testnet
	// For testing purposes, use a recent testnet block
	ObtcTestNetForkHeight int32 = 2800000

	// ObtcRegTestForkHeight defines when OBTC regtest forks (for development)
	// Set to a low value for immediate testing
	ObtcRegTestForkHeight int32 = 100
)

// ObtcMainNetParams defines the network parameters for the OBTC main network.
//
// IMPORTANT: OBTC is a hard fork of Bitcoin, not a new chain from genesis.
// This means OBTC shares Bitcoin's history up to the fork height, then
// diverges with OBTC-specific consensus rules and network isolation.
//
// Fork Height: Block 950000 (target: Q2 2026)
// Before fork: Identical to Bitcoin mainnet
// After fork: OBTC-specific consensus rules apply
var ObtcMainNetParams = Params{
	Name:        "obtcmainnet",
	Net:         wire.ObtcMainNet,
	DefaultPort: "9527", // Stephen Chow's iconic number from "Flirting Scholar"

	// DNS seeds are still placeholders until production seed nodes are ready.
	DNSSeeds: []DNSSeed{
		{"seed.obtc.example.com", true},
	},

	// CRITICAL: As a hard fork, OBTC uses Bitcoin's original genesis block
	// and shares the same blockchain history up to the fork height.
	GenesisBlock: &genesisBlock, // Bitcoin's genesis block (shared history)
	GenesisHash:  &genesisHash,  // Bitcoin's genesis hash (shared history)

	// Proof of work parameters
	PowLimit:         obtcPowLimit,
	PowLimitBits:     0x1d00ffff,
	PoWNoRetargeting: false,
	EnforceBIP94:     true, // Enable timewarp protection

	// OBTC Fork Point and Consensus Rules:
	// All BIP activation heights are set relative to the fork point.
	// Before fork: Follow Bitcoin consensus rules exactly
	// After fork: Apply OBTC-specific consensus modifications
	BIP0034Height:            1, // Already active in Bitcoin at fork point
	BIP0065Height:            1, // Already active in Bitcoin at fork point
	BIP0066Height:            1, // Already active in Bitcoin at fork point
	CoinbaseMaturity:         100,
	SubsidyReductionInterval: 210000,              // Keep Bitcoin's halving schedule
	TargetTimespan:           time.Hour * 24 * 14, // 14 days (same as Bitcoin)
	TargetTimePerBlock:       time.Minute * 10,    // 10 minutes (same as Bitcoin)
	RetargetAdjustmentFactor: 4,                   // 25% less, 400% more
	ReduceMinDifficulty:      false,
	MinDiffReductionTime:     0,
	GenerateSupported:        true, // Allow CPU mining for testing

	// Checkpoints will be added once the network has stable historical anchors.
	Checkpoints: []Checkpoint{},

	// Consensus rule change parameters
	RuleChangeActivationThreshold: 1916, // 95% of MinerConfirmationWindow
	MinerConfirmationWindow:       2016, // 2 weeks worth of blocks
	Deployments: [DefinedDeployments]ConsensusDeployment{
		DeploymentTestDummy: {
			BitNumber:          28,
			AlwaysActiveHeight: 0, // Always active for OBTC after fork
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyMinActivation: {
			BitNumber:                 22,
			CustomActivationThreshold: 0,
			MinActivationHeight:       0,
			AlwaysActiveHeight:        0,
			DeploymentStarter:         NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:           NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentCSV: {
			BitNumber:          0,
			AlwaysActiveHeight: 1, // Already active in Bitcoin at fork point
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentSegwit: {
			BitNumber:          1,
			AlwaysActiveHeight: 1, // Already active in Bitcoin at fork point
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTaproot: {
			BitNumber:          2,
			AlwaysActiveHeight: 1, // Already active in Bitcoin at fork point
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyAlwaysActive: {
			BitNumber:          30,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
	},

	// Mempool parameters
	RelayNonStdTxs: false,

	// Address encoding parameters - CRITICAL: These are intentionally isolated
	// from Bitcoin namespaces to reduce replay/mis-transfer risk.
	Bech32HRPSegwit:         "obtc", // Human-readable part for bech32 addresses
	PubKeyHashAddrID:        0x47,
	ScriptHashAddrID:        0x32,
	PrivateKeyID:            0x9A,
	WitnessPubKeyHashAddrID: 0x2A,
	WitnessScriptHashAddrID: 0x2B,

	// BIP32 hierarchical deterministic extended key parameters.
	// These version bytes are unique to OBTC and must not overlap with
	// Bitcoin xpub/xprv/tpub/tprv/spub/sprv namespaces.
	HDPrivateKeyID: [4]byte{0x0B, 0x47, 0xB0, 0x1E},
	HDPublicKeyID:  [4]byte{0x0B, 0x47, 0xB5, 0xD4},

	// BIP44 coin type. Temporary project-local allocation until official
	// SLIP-0044 assignment is obtained.
	HDCoinType: 20260,
}

// ObtcTestNetParams defines the network parameters for OBTC test network.
// Like mainnet, this is a hard fork of Bitcoin testnet, sharing history up to the fork point.
//
// Fork Height: Block 2800000 (Bitcoin testnet)
// This allows testing OBTC functionality while maintaining testnet compatibility.
var ObtcTestNetParams = Params{
	Name:        "obtctestnet",
	Net:         wire.ObtcTestNet,
	DefaultPort: "19527", // TestNet variant of 9527

	// Use Bitcoin testnet's genesis and history up to fork point
	GenesisBlock: &testNet3GenesisBlock,
	GenesisHash:  &testNet3GenesisHash,

	// Similar proof of work parameters to Bitcoin testnet
	PowLimit:         testNet3PowLimit,
	PowLimitBits:     0x1d00ffff,
	PoWNoRetargeting: false,

	// Consensus parameters - inherit from Bitcoin at fork point
	BIP0034Height:            21111, // Bitcoin testnet values
	BIP0065Height:            581885,
	BIP0066Height:            330776,
	CoinbaseMaturity:         100,
	SubsidyReductionInterval: 210000,
	TargetTimespan:           time.Hour * 24 * 14,
	TargetTimePerBlock:       time.Minute * 10,
	RetargetAdjustmentFactor: 4,

	// Address/HD namespaces are isolated from Bitcoin testnet/regtest to
	// prevent cross-chain mis-transfer during testing.
	Bech32HRPSegwit:         "obtct", // "obtc testnet"
	PubKeyHashAddrID:        0x71,
	ScriptHashAddrID:        0xD1,
	PrivateKeyID:            0xF1,
	WitnessPubKeyHashAddrID: 0x2C,
	WitnessScriptHashAddrID: 0x2D,
	HDPrivateKeyID:          [4]byte{0x0B, 0x48, 0xB0, 0x1E},
	HDPublicKeyID:           [4]byte{0x0B, 0x48, 0xB5, 0xD4},
	HDCoinType:              20261,

	// Consensus deployments for OBTC testnet.
	// Keep existing soft forks explicitly configured to avoid nil deployment
	// starters/enders on threshold checks.
	RuleChangeActivationThreshold: 1512, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       2016,
	Deployments: [DefinedDeployments]ConsensusDeployment{
		DeploymentTestDummy: {
			BitNumber:          28,
			AlwaysActiveHeight: 0,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyMinActivation: {
			BitNumber:                 22,
			CustomActivationThreshold: 0,
			MinActivationHeight:       0,
			AlwaysActiveHeight:        0,
			DeploymentStarter:         NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:           NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentCSV: {
			BitNumber:          0,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentSegwit: {
			BitNumber:          1,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTaproot: {
			BitNumber:          2,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyAlwaysActive: {
			BitNumber:          30,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
	},

	// TODO: Complete remaining testnet-specific fields in Week 3
}

// ObtcRegTestParams defines the network parameters for OBTC regression testing.
// RegTest is used for development and testing, inheriting Bitcoin regtest characteristics.
//
// Fork Height: Block 100 (for immediate development testing)
// This allows developers to quickly test OBTC-specific features.
var ObtcRegTestParams = Params{
	Name:        "obtcregtest",
	Net:         wire.ObtcRegNet, // Use separate magic for regtest isolation
	DefaultPort: "29527",         // RegTest variant of 9527

	// Use Bitcoin regtest genesis for shared development environment
	GenesisBlock: &regTestGenesisBlock,
	GenesisHash:  &regTestGenesisHash,

	// Regression test parameters - very easy mining for development
	PowLimit:                 regressionPowLimit,
	PowLimitBits:             0x207fffff,
	PoWNoRetargeting:         true, // No difficulty adjustment for testing
	ReduceMinDifficulty:      false,
	GenerateSupported:        true,
	CoinbaseMaturity:         100,
	BIP0034Height:            100000000, // Not activated on regtest
	BIP0065Height:            1351,
	BIP0066Height:            1251,
	SubsidyReductionInterval: 150,
	TargetTimespan:           time.Hour * 24 * 14, // 14 days
	TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
	RetargetAdjustmentFactor: 4,                   // 25% less, 400% more

	// No checkpoints for regtest
	Checkpoints: []Checkpoint{},

	// Consensus deployments always active
	RuleChangeActivationThreshold: 108, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       144, // Faster than normal
	Deployments: [DefinedDeployments]ConsensusDeployment{
		DeploymentTestDummy: {
			BitNumber:          28,
			AlwaysActiveHeight: 0,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyMinActivation: {
			BitNumber:                 22,
			CustomActivationThreshold: 0,
			MinActivationHeight:       0,
			AlwaysActiveHeight:        0,
			DeploymentStarter:         NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:           NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentCSV: {
			BitNumber:          0,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentSegwit: {
			BitNumber:          1,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTaproot: {
			BitNumber:          2,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTestDummyAlwaysActive: {
			BitNumber:          30,
			AlwaysActiveHeight: 1,
			DeploymentStarter:  NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:    NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
	},

	// Regtest also uses isolated namespaces (instead of reusing Bitcoin
	// test/reg prefixes) to avoid accidental cross-network key/address usage.
	Bech32HRPSegwit:         "obtcrt", // "obtc regtest"
	PubKeyHashAddrID:        0x72,
	ScriptHashAddrID:        0xD2,
	PrivateKeyID:            0xF2,
	WitnessPubKeyHashAddrID: 0x2E,
	WitnessScriptHashAddrID: 0x2F,
	HDPrivateKeyID:          [4]byte{0x0B, 0x49, 0xB0, 0x1E},
	HDPublicKeyID:           [4]byte{0x0B, 0x49, 0xB5, 0xD4},
	HDCoinType:              20262,

	// TODO: Complete remaining fields in Week 3
}

// IsOBTC returns true if the network parameters represent an OBTC network.
// This function is critical for isolating OBTC-specific behavior from Bitcoin logic.
func IsOBTC(params *Params) bool {
	return params.Net == wire.ObtcMainNet ||
		params.Net == wire.ObtcTestNet ||
		params.Net == wire.ObtcRegNet
}

// GetOBTCForkHeight returns the fork height for the given OBTC network.
// Returns -1 if the network is not an OBTC network.
func GetOBTCForkHeight(params *Params) int32 {
	switch params.Net {
	case wire.ObtcMainNet:
		return ObtcMainNetForkHeight
	case wire.ObtcTestNet:
		return ObtcTestNetForkHeight
	case wire.ObtcRegNet:
		return ObtcRegTestForkHeight
	default:
		return -1 // Not an OBTC network
	}
}

// IsPostOBTCFork returns true if the given block height is at or after
// the OBTC fork point for the specified network.
// This determines whether OBTC-specific consensus rules should be applied.
func IsPostOBTCFork(params *Params, height int32) bool {
	if !IsOBTC(params) {
		return false // Bitcoin networks never have OBTC rules
	}

	forkHeight := GetOBTCForkHeight(params)
	return height >= forkHeight
}

// GetOBTCReplayProtectionHeight returns the height at which OBTC replay-
// protected sighash domain enforcement becomes active for the given network.
// Returns -1 when the network is not OBTC or does not define the activation.
func GetOBTCReplayProtectionHeight(params *Params) int32 {
	expiryParams := GetExpiryParams(params)
	if expiryParams == nil || expiryParams.ReplayProtectionAtHeight <= 0 {
		return -1
	}

	return expiryParams.ReplayProtectionAtHeight
}

// IsOBTCReplayProtectionActive reports whether OBTC replay-protected sighash
// domain enforcement is active at the provided block height.
func IsOBTCReplayProtectionActive(params *Params, height int32) bool {
	activationHeight := GetOBTCReplayProtectionHeight(params)
	if activationHeight < 0 {
		return false
	}

	return height >= activationHeight
}

// GetExpiryParams returns expiry parameters for the given network.
// Returns nil if the network is not an OBTC network.
func GetExpiryParams(params *Params) *ExpiryParams {
	if !IsOBTC(params) {
		return nil // Only OBTC networks have expiry
	}

	switch params.Net {
	case wire.ObtcMainNet:
		mainnetActivationHeight := ObtcMainNetForkHeight + 2016 // one retarget window after fork
		return &ExpiryParams{
			WindowBlocks:                   362880, // 9! (factorial of 9) ≈ 6.91 years at 10min blocks
			ListBatchLimit:                 10000,
			StartScanHeight:                0,
			EnableAtHeight:                 mainnetActivationHeight,
			ReapConsensusAtHeight:          mainnetActivationHeight,
			ReplayProtectionAtHeight:       mainnetActivationHeight,
			ReapMaxInputs:                  256,
			ReapTaxNumerator:               30,
			ReapTaxDenominator:             100,
			ReapDustThresholdSat:           720,
			ExpiryCommitmentEnableAtHeight: mainnetActivationHeight,
		}

	case wire.ObtcTestNet:
		return &ExpiryParams{
			WindowBlocks:                   1008, // ~1 week for testing (144 * 7)
			ListBatchLimit:                 5000,
			StartScanHeight:                0,
			EnableAtHeight:                 ObtcTestNetForkHeight + 100,
			ReapConsensusAtHeight:          ObtcTestNetForkHeight + 120,
			ReplayProtectionAtHeight:       ObtcTestNetForkHeight + 130,
			ReapMaxInputs:                  500,
			ReapTaxNumerator:               30,
			ReapTaxDenominator:             100,
			ReapDustThresholdSat:           720,
			ExpiryCommitmentEnableAtHeight: ObtcTestNetForkHeight + 100,
		}

	case wire.ObtcRegNet:
		return &ExpiryParams{
			WindowBlocks:                   144, // ~1 day for development
			ListBatchLimit:                 1000,
			StartScanHeight:                0,
			EnableAtHeight:                 ObtcRegTestForkHeight + 10,
			ReapConsensusAtHeight:          ObtcRegTestForkHeight + 12,
			ReplayProtectionAtHeight:       ObtcRegTestForkHeight + 14,
			ReapMaxInputs:                  200,
			ReapTaxNumerator:               30,
			ReapTaxDenominator:             100,
			ReapDustThresholdSat:           720,
			ExpiryCommitmentEnableAtHeight: ObtcRegTestForkHeight + 10,
		}

	default:
		return nil
	}
}

// CalculateExpiryKey calculates the expiry key for a UTXO created at the given height.
func (p *ExpiryParams) CalculateExpiryKey(createHeight int32) uint64 {
	return uint64(createHeight) + p.WindowBlocks
}

func validateOBTCNamespaceIsolation() error {
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

	checkByteField := func(field string, getter func(*Params) byte) error {
		seen := make(map[byte]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision within OBTC: %s and %s both use 0x%02x", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision between %s and %s: 0x%02x", field, prev, net.name, v)
			}
		}
		return nil
	}

	checkStringField := func(field string, getter func(*Params) string) error {
		seen := make(map[string]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision within OBTC: %s and %s both use %q", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision between %s and %s: %q", field, prev, net.name, v)
			}
		}
		return nil
	}

	checkHDField := func(field string, getter func(*Params) [4]byte) error {
		seen := make(map[[4]byte]string)
		for _, net := range obtcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision within OBTC: %s and %s both use %x", field, prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := getter(net.params)
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("%s collision between %s and %s: %x", field, prev, net.name, v)
			}
		}
		return nil
	}

	checkCoinType := func() error {
		seen := make(map[uint32]string)
		for _, net := range obtcNets {
			v := net.params.HDCoinType
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("HDCoinType collision within OBTC: %s and %s both use %d", prev, net.name, v)
			}
			seen[v] = net.name
		}
		for _, net := range btcNets {
			v := net.params.HDCoinType
			if prev, ok := seen[v]; ok {
				return fmt.Errorf("HDCoinType collision between %s and %s: %d", prev, net.name, v)
			}
		}
		return nil
	}

	for _, net := range obtcNets {
		if net.params.HDPrivateKeyID == net.params.HDPublicKeyID {
			return fmt.Errorf("%s has identical HD private/public version bytes: %x", net.name, net.params.HDPrivateKeyID)
		}
	}

	if err := checkStringField("Bech32 HRP", func(p *Params) string { return p.Bech32HRPSegwit }); err != nil {
		return err
	}
	if err := checkStringField("DefaultPort", func(p *Params) string { return p.DefaultPort }); err != nil {
		return err
	}
	if err := checkByteField("PubKeyHashAddrID", func(p *Params) byte { return p.PubKeyHashAddrID }); err != nil {
		return err
	}
	if err := checkByteField("ScriptHashAddrID", func(p *Params) byte { return p.ScriptHashAddrID }); err != nil {
		return err
	}
	if err := checkByteField("PrivateKeyID", func(p *Params) byte { return p.PrivateKeyID }); err != nil {
		return err
	}
	if err := checkByteField("WitnessPubKeyHashAddrID", func(p *Params) byte { return p.WitnessPubKeyHashAddrID }); err != nil {
		return err
	}
	if err := checkByteField("WitnessScriptHashAddrID", func(p *Params) byte { return p.WitnessScriptHashAddrID }); err != nil {
		return err
	}
	if err := checkHDField("HDPrivateKeyID", func(p *Params) [4]byte { return p.HDPrivateKeyID }); err != nil {
		return err
	}
	if err := checkHDField("HDPublicKeyID", func(p *Params) [4]byte { return p.HDPublicKeyID }); err != nil {
		return err
	}
	if err := checkCoinType(); err != nil {
		return err
	}

	return nil
}

// init registers the OBTC network parameters so they can be used with the
// network selection functions.
func init() {
	if err := validateOBTCNamespaceIsolation(); err != nil {
		panic("invalid OBTC namespace isolation: " + err.Error())
	}

	// Register OBTC networks
	err := Register(&ObtcMainNetParams)
	if err != nil {
		panic("failed to register OBTC mainnet parameters: " + err.Error())
	}

	err = Register(&ObtcTestNetParams)
	if err != nil {
		panic("failed to register OBTC testnet parameters: " + err.Error())
	}

	err = Register(&ObtcRegTestParams)
	if err != nil {
		panic("failed to register OBTC regtest parameters: " + err.Error())
	}
}
