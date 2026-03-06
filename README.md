OBTCD (Organic Bitcoin) 
=====================

[![Build Status](https://github.com/organicbitcoin/obtcd/workflows/Build%20and%20Test/badge.svg)](https://github.com/organicbitcoin/obtcd/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/organicbitcoin/obtcd)

> ⚠️ **Phase 1 Development Status**: OBTC is currently in active development. This implementation includes basic network parameters and infrastructure. OBTC-specific consensus rules (REAP) will be implemented in Phases 2-4.

OBTCD is a full node implementation of the Organic Bitcoin (OBTC) protocol, forked from [btcd](https://github.com/btcsuite/btcd). OBTC implements the **Resource Expiration and Allocation Protocol (REAP)**, introducing temporal scarcity to Bitcoin through UTXO expiration and value redistribution.

## 🎯 Key Features

- **Hard Fork of Bitcoin**: OBTC shares Bitcoin's history up to fork height (~950,000, Q2 2026)
- **REAP Protocol**: UTXOs expire after 7 years, with 30% value redistributed to miners
- **Network Isolation**: Complete separation from Bitcoin networks (unique ports, addresses, magic numbers)
- **btcd Foundation**: Built on the stable, production-tested btcd codebase

## 🚀 Quick Start

### Prerequisites

- [Go](http://golang.org) 1.22 or newer
- Git

### Build OBTCD

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
go build
```

### Start Development Network (2-node simnet)

```bash
# Start DevNet (simnet with 2 nodes)
./scripts/devnet-up.sh start

# Run demo transaction
./scripts/devnet-up.sh demo

# Check network status
./scripts/devnet-up.sh status

# Stop network
./scripts/devnet-up.sh stop
```

### Example Transaction

```bash
# Build btcctl
cd cmd/btcctl && go build

# Get network info
./btcctl --simnet --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:18556 getinfo

# Generate blocks (node 1)
./btcctl --simnet --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:18556 generate 101

# Get new address (node 2)  
./btcctl --simnet --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:18557 getnewaddress

# Send transaction
./btcctl --simnet --rpcuser=obtc --rpcpass=obtcpass --rpcserver=127.0.0.1:18556 sendtoaddress <address> 1.0
```

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path.  It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues.  It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain btcd, all dependencies, and install it:

## 📊 OBTC Network Parameters (Phase 1 Skeleton)

> 🚧 **Note**: These parameters are placeholders for Phase 1 skeleton implementation. **Final values will be frozen in Phase 3** during the "freeze constants" phase.

| Parameter | MainNet | TestNet | RegTest |
|-----------|---------|---------|---------|
| **Network Magic** | `0x4F425443` | `0x4F544553` | `0x4F524547` |
| **Default Port** | `9527` | `19527` | `29527` |
| **Fork Height** | `950000` (Q2 2026) | `2800000` | `100` |
| **Bech32 HRP** | `obtc` | `obtct` | `obtcrt` |
| **Address Prefixes** | *TBD Phase 3* | *TBD Phase 3* | *TBD Phase 3* |
| **HD Key Prefixes** | *TBD Phase 3* | *TBD Phase 3* | *TBD Phase 3* |
| **BIP44 Coin Type** | *TBD Phase 3* | `1` (testnet) | `1` (regtest) |

### Network Isolation Features

- ✅ **Unique Magic Numbers**: Prevents cross-network communication
- ✅ **Separate Ports**: Avoids conflicts with Bitcoin nodes  
- ✅ **Custom Address Encoding**: Will use unique prefixes (Phase 3)
- ✅ **Fork Height Detection**: `IsPostOBTCFork()` function available
- ✅ **Network Detection**: `IsOBTC()` function for conditional logic

## 🧪 Development & Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run OBTC-specific tests  
go test ./chaincfg ./wire -v -run "OBTC"

# Run with race detection
go test -race ./...
```

### Development Workflow

```bash
# Development network management
./scripts/devnet-up.sh start           # Start 2-node simnet
./scripts/devnet-up.sh demo            # Run demo transaction  
./scripts/devnet-up.sh logs            # View node logs
./scripts/devnet-up.sh clean           # Clean all data
```

## 🗓️ Development Roadmap

- **Phase 1** ✅: Network parameters and infrastructure (COMPLETED)
- **Phase 2**: Expiry index implementation
- **Phase 3**: REAP selector and transaction construction
- **Phase 4**: Validation rules and mining integration
- **Phase 5**: Wallet extension and RPC
- **Phase 6**: TestNet deployment and monitoring
- **Phase 7**: Hardening and stress testing
- **Phase 8**: MainNet candidate release

## 📚 Documentation

- [Development Roadmap](obtc_doc/obtc_roadmap_plan.md) - Complete 8-phase development plan
- [Phase 1 Implementation](obtc_doc/phase1_implementation.md) - Current week details
- [Original btcd Documentation](docs/) - Inherited btcd documentation

## ⚠️ Important Notes

- **Development Status**: Phase 1 skeleton implementation
- **Network**: Currently simnet/regtest only (production networks TBD)
- **Consensus Rules**: REAP protocol implementation starts Phase 2
- **Compatibility**: Shares Bitcoin history up to fork height
- **Production Use**: Not ready for production until Phase 8

## 🤝 Contributing

This project follows an 8-phase development timeline with specific milestones. Please refer to the [development roadmap](obtc_doc/obtc_roadmap_plan.md) for current priorities.

## 📜 License

OBTCD is licensed under the [copyfree](http://copyfree.org) ISC License.
is used for this project.

## Documentation

The documentation is a work-in-progress.  It is located in the [docs](https://github.com/btcsuite/btcd/tree/master/docs) folder.

## Release Verification

Please see our [documentation on the current build/verification
process](https://github.com/btcsuite/btcd/tree/master/release) for all our
releases for information on how to verify the integrity of published releases
using our reproducible build system.

## License

btcd is licensed under the [copyfree](http://copyfree.org) ISC License.
