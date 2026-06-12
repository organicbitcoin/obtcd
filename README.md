# OBTCD

[![Build Status](https://github.com/organicbitcoin/obtcd/workflows/Build%20and%20Test/badge.svg)](https://github.com/organicbitcoin/obtcd/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/organicbitcoin/obtcd)

OBTC is a Bitcoin-derived lifecycle-money experiment.

`obtcd` is the OBTC node implementation. It is derived from
[btcsuite/btcd](https://github.com/btcsuite/btcd) and adds OBTC network
parameters, expiry-aware indexing, replay protection, expiry commitment support,
REAP validation paths, and operator tooling for testnet and mainnet-candidate
work.

This repository is public for developer, node operator, miner, and reviewer
inspection. It is not production financial infrastructure.

## Status

The current public target is `mainnet-candidate-2026-07`, not a mature
production mainnet.

Current milestone:

- [mainnet-candidate-2026-07](https://github.com/organicbitcoin/obtcd/milestone/1)

Current companion wallet repository:

- [obtcwallet](https://github.com/organicbitcoin/obtcwallet)

Website:

- <https://organicbitcoin.org>

## What is implemented

- OBTC mainnet, testnet, and regtest network parameters.
- Network isolation through distinct magic values, ports, and address prefixes.
- Bitcoin shared-history fork parameters.
- ExpiryIndex state and scan RPC support.
- Expiry commitment support in coinbase data.
- REAP candidate selection, transaction validation, and mining template paths.
- Replay protection and OBTC-specific consensus/policy tests.
- `--reindex-expiry` for rebuilding persisted ExpiryIndex state.
- `obtc-status`, a read-only node status page for operators.
- Devnet traffic simulation and validation scripts.

## Important limits

- This is a mainnet-candidate codebase, not a production financial system.
- Seed replacement, public observation, and release hardening are still active
  launch work.
- Miner-facing material must not be read as a revenue guarantee. REAP-related
  miner accounting depends on activation state, candidate availability, and
  block template validation.
- The Go module path still inherits upstream `github.com/btcsuite/btcd`.

## Network parameters

Current implementation values are defined in `chaincfg/params_obtc.go`.
The dedicated mainnet checklist is maintained in
[`docs/mainnet-params.md`](docs/mainnet-params.md).
The mainnet-candidate join draft is maintained in
[`docs/mainnet-join.md`](docs/mainnet-join.md).

| Parameter | Mainnet | Testnet | Regtest |
| --- | --- | --- | --- |
| Network magic | `0x4F425443` | `0x4F544553` | `0x4F524547` |
| P2P port | `9527` | `19527` | `29527` |
| RPC port | `9528` | `19528` | `29528` |
| Fork height | `1000000`* | `0`** | `100` |
| Bech32 HRP | `obtc` | `obtct` | `obtcrt` |
| P2PKH prefix | `0x47` | `0x71` | `0x72` |
| P2SH prefix | `0x32` | `0xD1` | `0xD2` |
| BIP44 coin type | `20260` | `20261` | `20262` |

* Mainnet fork height is provisional and may change before final
  mainnet-candidate release artifacts are published. Current derived mainnet
  activation height is `1002016`.

** Testnet fork height `0` is intentional. The public OBTC testnet is an
   independent accelerated test chain, so its fork/activation values are not
   Bitcoin mainnet-derived.

## Requirements

- Go 1.24.0 or newer
- Git

## Build

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd

go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

## Test

```bash
go test ./...
```

Focused OBTC checks:

```bash
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|RejectREAPSystemTxFromMempool' -count=1
go test ./mining -run 'REAP|Accounting|Witness' -count=1
```

## Minimal testnet node

```bash
./btcd --obtctestnet \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --txindex \
  --expiryindex \
  --notls
```

Check basic RPC connectivity:

```bash
./btcctl --obtctestnet \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --rpcserver=127.0.0.1:19528 \
  --notls \
  getinfo
```

Inspect OBTC expiry and REAP state:

```bash
./btcctl --obtctestnet \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --rpcserver=127.0.0.1:19528 \
  --notls \
  getexpiryindexstats

./btcctl --obtctestnet \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --rpcserver=127.0.0.1:19528 \
  --notls \
  getexpirycommitment

./btcctl --obtctestnet \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --rpcserver=127.0.0.1:19528 \
  --notls \
  getreapplan
```

Start the read-only status page against the same node:

```bash
./obtc-status \
  --obtctestnet \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --rpcserver=127.0.0.1:19528 \
  --notls
```

## Local devnet

The repository includes a two-node simnet/devnet helper for repeatable local
traffic and restart testing:

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh demo
./scripts/devnet-up.sh scenario feemarket
./scripts/devnet-up.sh scenario conflict
./scripts/devnet-up.sh scenario multisource
./scripts/devnet-up.sh status
./scripts/devnet-up.sh stop
```

## Documentation

- [OBTC Node Documentation](docs/)
- [OBTC Public Testnet User Test](docs/public-testnet-user-test.md)
- [OBTC Mainnet Join Runbook](docs/mainnet-join.md)
- [OBTC Testnet Join Guide](docs/testnet-join.md)
- [Network Parameters](docs/network-parameters.md)

## Issues

Use this repository's issue tracker for invited-reviewer test feedback, manual
test coin issuance coordination, node bugs, release evidence,
mainnet-candidate blockers, mining-template review, and operator feedback.
There is no public faucet or open public test coin request queue:

<https://github.com/organicbitcoin/obtcd/issues>

For launch-tracking work, prefer the
[`mainnet-candidate-2026-07`](https://github.com/organicbitcoin/obtcd/milestone/1)
milestone and the labels `mainnet-blocker`, `evidence`, `comms`, and
`post-launch`.

## Upstream attribution

`obtcd` is derived from [btcsuite/btcd](https://github.com/btcsuite/btcd). The
upstream project's networking, wallet-adjacent RPC foundations, database
interfaces, and consensus architecture remain visible throughout this
repository. OBTC-specific changes are layered on top for lifecycle rules,
expiry/reclaim validation, network isolation, and launch tooling.

## License

OBTCD is licensed under the [copyfree](http://copyfree.org) ISC License.
