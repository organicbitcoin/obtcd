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

// ObtcMainNetParams defines the network parameters for the OBTC main network.
// 
// Note: This is currently a skeleton implementation for Week 1 of the OBTC
// development plan. Many parameters are placeholders and will be finalized
// in Week 3 when the genesis block and unique constants are frozen.
var ObtcMainNetParams = Params{
	Name:        "obtcmainnet",
	Net:         wire.ObtcMainNet,
	DefaultPort: "8555", // Different from Bitcoin's 8333

	// DNS seeds - TODO: Replace with actual OBTC seed nodes in Week 6
	DNSSeeds: []DNSSeed{
		// Placeholder seeds - will be replaced with actual OBTC seed nodes
		{"seed.obtc.example.com", true},
	},

	// Genesis block - TODO: Generate unique OBTC genesis block in Week 3
	GenesisBlock: &genesisBlock, // Temporarily using Bitcoin's genesis
	GenesisHash:  &genesisHash,  // Temporarily using Bitcoin's genesis hash

	// Proof of work parameters
	PowLimit:                 obtcPowLimit,
	PowLimitBits:             0x1d00ffff,
	PoWNoRetargeting:         false,
	EnforceBIP94:             true, // Enable timewarp protection
	BIP0034Height:            1,    // Enable from block 1
	BIP0065Height:            1,    // Enable CHECKLOCKTIMEVERIFY from block 1
	BIP0066Height:            1,    // Enable strict DER signatures from block 1
	CoinbaseMaturity:         100,
	SubsidyReductionInterval: 210000, // Same as Bitcoin for now
	TargetTimespan:           time.Hour * 24 * 14, // 14 days
	TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
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
			BitNumber:           28,
			AlwaysActiveHeight:  0, // Always active for OBTC
			DeploymentStarter:   NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:     NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentCSV: {
			BitNumber:           0,
			AlwaysActiveHeight:  1, // Active from block 1
			DeploymentStarter:   NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:     NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentSegwit: {
			BitNumber:           1,
			AlwaysActiveHeight:  1, // Active from block 1
			DeploymentStarter:   NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:     NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
		},
		DeploymentTaproot: {
			BitNumber:           2,
			AlwaysActiveHeight:  1, // Active from block 1
			DeploymentStarter:   NewMedianTimeDeploymentStarter(time.Unix(0, 0)),
			DeploymentEnder:     NewMedianTimeDeploymentEnder(time.Unix(0, 0)),
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

// ObtcTestNetParams defines the network parameters for the OBTC test network.
var ObtcTestNetParams = Params{
	Name:        "obtctestnet",
	Net:         wire.ObtcTestNet, // Use separate magic for testnet
	DefaultPort: "18555",         // Different from Bitcoin testnet's 18333

	// Most parameters inherit from mainnet but with easier difficulty
	PowLimit:                 obtcPowLimit,
	PowLimitBits:             0x1d00ffff,
	PoWNoRetargeting:         false,
	ReduceMinDifficulty:      true,                  // Allow difficulty reduction
	MinDiffReductionTime:     time.Minute * 20,      // Reduce difficulty after 20 min
	GenerateSupported:        true,                  // Allow CPU mining
	RuleChangeActivationThreshold: 1512,            // 75% instead of 95%
	MinerConfirmationWindow:       2016,
	
	// Same address encoding as mainnet - they'll be distinguished by network context
	Bech32HRPSegwit:         "tob", // "test obtc" 
	PubKeyHashAddrID:        0x6F,  // Different from mainnet
	ScriptHashAddrID:        0xC4,  // Different from mainnet
	PrivateKeyID:            0xEF,
	WitnessPubKeyHashAddrID: 0x03,
	WitnessScriptHashAddrID: 0x28,
	HDPrivateKeyID:          [4]byte{0x04, 0x35, 0x83, 0x94}, // "tprv" equivalent
	HDPublicKeyID:           [4]byte{0x04, 0x35, 0x87, 0xCF}, // "tpub" equivalent
	HDCoinType:              1, // Testnet coin type
	
	// TODO: Complete remaining fields in Week 3
}

// ObtcRegTestParams defines the network parameters for OBTC regression testing.
var ObtcRegTestParams = Params{
	Name:        "obtcregtest", 
	Net:         wire.ObtcRegNet, // Use separate magic for regtest
	DefaultPort: "18444",         // Same as Bitcoin regtest for now

	// Regression test parameters - very easy mining
	PowLimit:                regressionPowLimit,
	PowLimitBits:            0x207fffff,
	PoWNoRetargeting:        true,
	ReduceMinDifficulty:     false,
	GenerateSupported:       true,
	CoinbaseMaturity:        100,
	BIP0034Height:           100000000, // Not activated on regtest
	BIP0065Height:           1351,
	BIP0066Height:           1251,
	SubsidyReductionInterval: 150,
	TargetTimespan:          time.Hour * 24 * 14, // 14 days
	TargetTimePerBlock:      time.Minute * 10,    // 10 minutes  
	RetargetAdjustmentFactor: 4,                  // 25% less, 400% more

	// No checkpoints for regtest
	Checkpoints: []Checkpoint{},

	// Consensus deployments always active
	RuleChangeActivationThreshold: 108, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       144, // Faster than normal
	
	// TODO: Complete remaining fields in Week 3
}

// IsOBTC returns true if the network parameters represent an OBTC network.
// This function is critical for isolating OBTC-specific behavior from Bitcoin logic.
func IsOBTC(params *Params) bool {
	return params.Net == wire.ObtcMainNet || 
		   params.Net == wire.ObtcTestNet || 
		   params.Net == wire.ObtcRegNet
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