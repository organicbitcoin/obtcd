// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"fmt"
	"math/big"
	"time"

	"github.com/btcsuite/btcd/wire"
)

// obtcPowLimit is the highest proof-of-work value an OBTC block can have.
var obtcPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 224), bigOne)

// ExpiryParams defines parameters for UTXO expiry calculation.
type ExpiryParams struct {
	WindowBlocks             uint64 // Expiry window in blocks
	ListBatchLimit           int    // Max items returned in one RPC scan
	StartScanHeight          int32  // Height to start building the index
	EnableAtHeight           int32  // Height to enable expiry enforcement
	ReapConsensusAtHeight    int32  // Height to enforce canonical REAP ordering and limits
	ReplayProtectionAtHeight int32  // Height to enable replay-protected sighash domains
	ReapMaxInputs            int    // Consensus max REAP inputs per transaction (0 = disabled)
	ReapTaxNumerator         int64  // Consensus REAP tax numerator
	ReapTaxDenominator       int64  // Consensus REAP tax denominator
	ReapDustThresholdSat     int64  // Inputs below this value fold fully into tax

	// ExpiryCommitmentEnableAtHeight is the height at which the expiry
	// commitment in coinbase becomes mandatory. Before this height the
	// commitment is optional; at or above it, blocks must include a valid
	// expiry commitment and the root must match the local state.
	ExpiryCommitmentEnableAtHeight int32
}

// Fork heights for OBTC networks.
const (
	// ObtcMainNetForkHeight is the OBTC mainnet fork height.
	ObtcMainNetForkHeight int32 = 950000

	// ObtcTestNetForkHeight is zero because the public OBTC testnet is an
	// independent chain, not a fork of Bitcoin testnet3 history.
	ObtcTestNetForkHeight int32 = 0

	// ObtcRegTestForkHeight is the OBTC regtest fork height.
	ObtcRegTestForkHeight int32 = 100
)

// ObtcMainNetParams defines the network parameters for OBTC mainnet.
var ObtcMainNetParams = Params{
	Name:        "obtcmainnet",
	Net:         wire.ObtcMainNet,
	DefaultPort: "9527",

	DNSSeeds: []DNSSeed{
		{"seed.obtc.example.com", true},
	},

	GenesisBlock: &genesisBlock,
	GenesisHash:  &genesisHash,

	PowLimit:                 obtcPowLimit,
	PowLimitBits:             0x1d00ffff,
	PoWNoRetargeting:         false,
	EnforceBIP94:             true,
	BIP0034Height:            1,
	BIP0065Height:            1,
	BIP0066Height:            1,
	CoinbaseMaturity:         100,
	SubsidyReductionInterval: 210000,
	TargetTimespan:           time.Hour * 24 * 14,
	TargetTimePerBlock:       time.Minute * 10,
	RetargetAdjustmentFactor: 4,
	ReduceMinDifficulty:      false,
	MinDiffReductionTime:     0,
	GenerateSupported:        true,

	Checkpoints: []Checkpoint{},

	RuleChangeActivationThreshold: 1916,
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

	RelayNonStdTxs: false,

	// Address encoding parameters are isolated from Bitcoin namespaces.
	Bech32HRPSegwit:         "obtc",
	PubKeyHashAddrID:        0x47,
	ScriptHashAddrID:        0x32,
	PrivateKeyID:            0x9A,
	WitnessPubKeyHashAddrID: 0x2A,
	WitnessScriptHashAddrID: 0x2B,

	// Extended key versions are isolated from Bitcoin namespaces.
	HDPrivateKeyID: [4]byte{0x0B, 0x47, 0xB0, 0x1E},
	HDPublicKeyID:  [4]byte{0x0B, 0x47, 0xB5, 0xD4},

	// Project-local BIP44 coin type.
	HDCoinType: 20260,
}

// ObtcTestNetParams defines the network parameters for OBTC testnet.
var ObtcTestNetParams = Params{
	Name:        "obtctestnet",
	Net:         wire.ObtcTestNet,
	DefaultPort: "19527",

	GenesisBlock: &obtcTestNetGenesisBlock,
	GenesisHash:  &obtcTestNetGenesisHash,

	PowLimit:            regressionPowLimit,
	PowLimitBits:        0x207fffff,
	PoWNoRetargeting:    true,
	ReduceMinDifficulty: false,
	GenerateSupported:   true,

	BIP0034Height:            1,
	BIP0065Height:            1,
	BIP0066Height:            1,
	CoinbaseMaturity:         20,
	SubsidyReductionInterval: 210000,
	TargetTimespan:           time.Hour * 24 * 14,
	TargetTimePerBlock:       time.Minute * 10,
	RetargetAdjustmentFactor: 4,
	Checkpoints:              []Checkpoint{},

	Bech32HRPSegwit:         "obtct",
	PubKeyHashAddrID:        0x71,
	ScriptHashAddrID:        0xD1,
	PrivateKeyID:            0xF1,
	WitnessPubKeyHashAddrID: 0x2C,
	WitnessScriptHashAddrID: 0x2D,
	HDPrivateKeyID:          [4]byte{0x0B, 0x48, 0xB0, 0x1E},
	HDPublicKeyID:           [4]byte{0x0B, 0x48, 0xB5, 0xD4},
	HDCoinType:              20261,

	RuleChangeActivationThreshold: 1512,
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
}

// ObtcRegTestParams defines the network parameters for OBTC regtest.
var ObtcRegTestParams = Params{
	Name:        "obtcregtest",
	Net:         wire.ObtcRegNet,
	DefaultPort: "29527",

	GenesisBlock: &regTestGenesisBlock,
	GenesisHash:  &regTestGenesisHash,

	PowLimit:                 regressionPowLimit,
	PowLimitBits:             0x207fffff,
	PoWNoRetargeting:         true,
	ReduceMinDifficulty:      false,
	GenerateSupported:        true,
	CoinbaseMaturity:         100,
	BIP0034Height:            100000000,
	BIP0065Height:            1351,
	BIP0066Height:            1251,
	SubsidyReductionInterval: 150,
	TargetTimespan:           time.Hour * 24 * 14,
	TargetTimePerBlock:       time.Minute * 10,
	RetargetAdjustmentFactor: 4,

	Checkpoints: []Checkpoint{},

	RuleChangeActivationThreshold: 108,
	MinerConfirmationWindow:       144,
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

	Bech32HRPSegwit:         "obtcrt",
	PubKeyHashAddrID:        0x72,
	ScriptHashAddrID:        0xD2,
	PrivateKeyID:            0xF2,
	WitnessPubKeyHashAddrID: 0x2E,
	WitnessScriptHashAddrID: 0x2F,
	HDPrivateKeyID:          [4]byte{0x0B, 0x49, 0xB0, 0x1E},
	HDPublicKeyID:           [4]byte{0x0B, 0x49, 0xB5, 0xD4},
	HDCoinType:              20262,
}

// IsOBTC reports whether params belongs to an OBTC network.
func IsOBTC(params *Params) bool {
	return params.Net == wire.ObtcMainNet ||
		params.Net == wire.ObtcTestNet ||
		params.Net == wire.ObtcRegNet
}

// GetOBTCForkHeight returns the fork height for an OBTC network, or -1.
func GetOBTCForkHeight(params *Params) int32 {
	switch params.Net {
	case wire.ObtcMainNet:
		return ObtcMainNetForkHeight
	case wire.ObtcTestNet:
		return ObtcTestNetForkHeight
	case wire.ObtcRegNet:
		return ObtcRegTestForkHeight
	default:
		return -1
	}
}

// IsPostOBTCFork reports whether height is at or after the network fork point.
func IsPostOBTCFork(params *Params, height int32) bool {
	if !IsOBTC(params) {
		return false
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

// GetExpiryParams returns expiry parameters for an OBTC network, or nil.
func GetExpiryParams(params *Params) *ExpiryParams {
	if !IsOBTC(params) {
		return nil
	}

	switch params.Net {
	case wire.ObtcMainNet:
		mainnetActivationHeight := ObtcMainNetForkHeight + 2016
		return &ExpiryParams{
			WindowBlocks:                   362880,
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
			WindowBlocks:                   144,
			ListBatchLimit:                 5000,
			StartScanHeight:                0,
			EnableAtHeight:                 100,
			ReapConsensusAtHeight:          120,
			ReplayProtectionAtHeight:       130,
			ReapMaxInputs:                  500,
			ReapTaxNumerator:               30,
			ReapTaxDenominator:             100,
			ReapDustThresholdSat:           720,
			ExpiryCommitmentEnableAtHeight: 100,
		}

	case wire.ObtcRegNet:
		return &ExpiryParams{
			WindowBlocks:                   144,
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

// CalculateExpiryKey returns the expiry key for a UTXO created at createHeight.
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

// init registers the OBTC network parameters.
func init() {
	if err := validateOBTCNamespaceIsolation(); err != nil {
		panic("invalid OBTC namespace isolation: " + err.Error())
	}

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
