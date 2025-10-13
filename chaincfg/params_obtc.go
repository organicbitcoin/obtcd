// Copyright (c) 2024 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"math/big"
	"time"

	"github.com/btcsuite/btcd/wire"
)

// obtcPowLimit is the highest proof of work value a OBTC block can have.
// This is the same as Bitcoin's testnet limit to allow easier testing.
var obtcPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 224), bigOne)

// OBTC Hard Fork Heights
//
// These constants define the block heights at which OBTC diverges from Bitcoin.
// Before these heights: Follow Bitcoin consensus rules exactly
// After these heights: Apply OBTC-specific consensus modifications
//
// TODO Week 3: Finalize exact fork heights based on Bitcoin mainnet conditions
const (
	// ObtcMainNetForkHeight defines when OBTC mainnet forks from Bitcoin mainnet
	// Target: Q2 2026 (estimated around block 950000)
	// This will be set to a specific Bitcoin block hash in Week 3
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
//
// Note: This is currently a skeleton implementation for Week 1 of the OBTC
// development plan. The fork height and final parameters will be determined
// in Week 3 during the "freeze constants" phase.
var ObtcMainNetParams = Params{
	Name:        "obtcmainnet",
	Net:         wire.ObtcMainNet,
	DefaultPort: "8555", // Different from Bitcoin's 8333

	// DNS seeds - TODO: Replace with actual OBTC seed nodes in Week 6
	DNSSeeds: []DNSSeed{
		// Placeholder seeds - will be replaced with actual OBTC seed nodes
		{"seed.obtc.example.com", true},
	},

	// CRITICAL: As a hard fork, OBTC uses Bitcoin's original genesis block
	// and shares the same blockchain history up to the fork height.
	// TODO Week 3: Determine exact fork height based on technical requirements
	GenesisBlock: &genesisBlock, // Bitcoin's genesis block (shared history)
	GenesisHash:  &genesisHash,  // Bitcoin's genesis hash (shared history)

	// Proof of work parameters
	PowLimit:         obtcPowLimit,
	PowLimitBits:     0x1d00ffff,
	PoWNoRetargeting: false,
	EnforceBIP94:     true, // Enable timewarp protection

	// OBTC Fork Point and Consensus Rules:
	// TODO Week 3: Set exact fork height (estimated: block 870000+)
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

	// Checkpoints - TODO: Add OBTC-specific checkpoints as network grows
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
	},

	// Mempool parameters
	RelayNonStdTxs: false,

	// Address encoding parameters - CRITICAL: These MUST be unique to OBTC
	// TODO: Generate cryptographically unique values in Week 3 to prevent
	// address reuse and replay attacks between Bitcoin and OBTC networks
	Bech32HRPSegwit:         "obtc", // Human-readable part for bech32 addresses
	PubKeyHashAddrID:        0x47,   // TODO: Generate unique value (currently placeholder)
	ScriptHashAddrID:        0x32,   // TODO: Generate unique value (currently placeholder)
	PrivateKeyID:            0xEF,   // TODO: Generate unique value (currently placeholder)
	WitnessPubKeyHashAddrID: 0x06,   // TODO: Generate unique value (currently placeholder)
	WitnessScriptHashAddrID: 0x0A,   // TODO: Generate unique value (currently placeholder)

	// BIP32 hierarchical deterministic extended key parameters
	// TODO: Generate cryptographically unique values in Week 3
	// These MUST NOT conflict with Bitcoin's xpub/xprv prefixes
	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xB2, 0x1E}, // TODO: Generate unique "oprv" prefix
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xAD, 0xE4}, // TODO: Generate unique "opub" prefix

	// BIP44 coin type - TODO: Register official coin type for OBTC
	HDCoinType: 1, // Using testnet coin type temporarily
}

// ObtcTestNetParams defines the network parameters for OBTC test network.
// Like mainnet, this is a hard fork of Bitcoin testnet, sharing history up to the fork point.
//
// Fork Height: Block 2800000 (Bitcoin testnet)
// This allows testing OBTC functionality while maintaining testnet compatibility.
var ObtcTestNetParams = Params{
	Name:        "obtctestnet",
	Net:         wire.ObtcTestNet,
	DefaultPort: "18555", // Different from Bitcoin testnet's 18333

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

	// Address encoding parameters - unique to OBTC testnet
	Bech32HRPSegwit:         "obtct", // "obtc testnet"
	PubKeyHashAddrID:        0x6F,    // Different from mainnet
	ScriptHashAddrID:        0xC4,    // Different from mainnet
	PrivateKeyID:            0xEF,
	WitnessPubKeyHashAddrID: 0x03,
	WitnessScriptHashAddrID: 0x28,
	HDPrivateKeyID:          [4]byte{0x04, 0x35, 0x83, 0x94}, // "tprv" equivalent
	HDPublicKeyID:           [4]byte{0x04, 0x35, 0x87, 0xCF}, // "tpub" equivalent
	HDCoinType:              1,                               // Testnet coin type

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
	DefaultPort: "18666",         // Different from Bitcoin regtest's 18444 for development isolation

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

	// Address encoding for regtest
	Bech32HRPSegwit:         "obtcrt",                        // "obtc regtest"
	PubKeyHashAddrID:        0x6F,                            // Same as Bitcoin regtest
	ScriptHashAddrID:        0xC4,                            // Same as Bitcoin regtest
	PrivateKeyID:            0xEF,                            // Same as Bitcoin regtest
	WitnessPubKeyHashAddrID: 0x03,                            // Same as Bitcoin regtest
	WitnessScriptHashAddrID: 0x28,                            // Same as Bitcoin regtest
	HDPrivateKeyID:          [4]byte{0x04, 0x35, 0x83, 0x94}, // Same as Bitcoin regtest
	HDPublicKeyID:           [4]byte{0x04, 0x35, 0x87, 0xCF}, // Same as Bitcoin regtest
	HDCoinType:              1,                               // Regtest coin type

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

// init registers the OBTC network parameters so they can be used with the
// network selection functions.
func init() {
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
