# OBTCD

[![Build Status](https://github.com/organicbitcoin/obtcd/workflows/Build%20and%20Test/badge.svg)](https://github.com/organicbitcoin/obtcd/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/organicbitcoin/obtcd)

OBTC is a Bitcoin-derived lifecycle-money experiment. It asks whether a
Bitcoin-like UTXO system can make long dormancy explicit through expiry,
renewal, and a rule-bound reclaim path instead of treating every old output as
operationally active forever.

`obtcd` is the OBTC node implementation. It is derived from
[btcsuite/btcd](https://github.com/btcsuite/btcd) and adds OBTC network
parameters, expiry-aware indexing, replay protection, expiry commitment support,
REAP validation paths, and operator tooling for testnet and mainnet-candidate
review.

## Read this first

If you are new to OBTC, start here:

- **What it is:** a separate Bitcoin-derived proof-of-work chain experiment
  around UTXO lifecycle rules.
- **Core mechanisms:** expiry, active renewal, REAP, refund/security-budget
  accounting, expiry commitments, and replay protection.
- **Why it exists:** to test whether dormant UTXO state, state maintenance, and
  long-term security-budget pressure can be handled by explicit lifecycle rules
  in a separate experiment.
- **Current status:** mainnet-candidate and public testnet review. The code and
  docs are open for technical review, but this is not final release material or
  mature financial infrastructure.
- **Non-goals:** this is not a Bitcoin consensus proposal, not an endorsement
  request, not an investment project, not a promise of miner income, and not a
  request that any pool, firmware project, or protocol project adopt OBTC.
- **What review is useful:** protocol assumptions, replay/activation boundaries,
  wallet renewal behavior, mining-template and coinbase accounting, Stratum
  documentation assumptions, reproducibility of testnet instructions, and
  wording that could overstate readiness.

Reviewer entry points:

- [Reviewer Quick Start](docs/reviewer-quickstart.md)
- [Mining Review Checklist](docs/mining-review-checklist.md)
- [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md)
- [Testnet Join Guide](docs/testnet-join.md)
- [Mainnet Join Runbook](docs/mainnet-join.md)
- [Network Parameters](docs/network-parameters.md)

## Status

The current public target is `mainnet-candidate-2026-07`.

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
- Miner-facing material must not be read as an income projection. REAP-related
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
| Fork height | `1000000`* | `2800000` | `100` |
| Bech32 HRP | `obtc` | `obtct` | `obtcrt` |
| P2PKH prefix | `0x47` | `0x71` | `0x72` |
| P2SH prefix | `0x32` | `0xD1` | `0xD2` |
| BIP44 coin type | `20260` | `20261` | `20262` |

* Mainnet fork height is provisional and may change before final
  mainnet-candidate release artifacts are published. Current mainnet
  replay-protection height is `1000001` (fork + 1). Current expiry / REAP /
  commitment activation height is `1002016` (fork + 2016).

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

## Public testnet entry

Use [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md) as the
canonical public testnet self-test entry for external node and wallet testers.
It covers `obtcd` and `obtcwallet` builds, seed peers, node startup, wallet
creation, manual test coin requests, `obtc.getexpiry`, `obtc.renew`, and the
no-value testnet coin boundary.

The lower-level [Testnet Join Guide](docs/testnet-join.md) remains available for
seed candidates and operator-oriented network details.

## Minimal testnet node

```bash
./btcd --obtctestnet \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --rpcuser=testuser \
  --rpcpass=testpass \
  --txindex \
  --expiryindex \
  --notls \
  --addpeer=seed1.testnet.organicbitcoin.org:19527 \
  --addpeer=seed2.testnet.organicbitcoin.org:19527 \
  --addpeer=seed3.testnet.organicbitcoin.org:19527
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
- [OBTC Mainnet Join Runbook](docs/mainnet-join.md)
- [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md)
- [OBTC Testnet Join Guide](docs/testnet-join.md)
- [Network Parameters](docs/network-parameters.md)

Public testnet coins have no real-world value. There is no public faucet at
this stage; test coins are sent manually for node, wallet, expiry, and renewal
testing.

## Issues

Use this repository's issue tracker for node bugs, release evidence,
mainnet-candidate blockers, mining-template review, and operator feedback:

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
