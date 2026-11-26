# OBTCD

[![Build Status](https://github.com/organicbitcoin/obtcd/workflows/Build%20and%20Test/badge.svg)](https://github.com/organicbitcoin/obtcd/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://pkg.go.dev/github.com/organicbitcoin/obtcd)

**OBTC is a separate Bitcoin-derived experiment in which UTXOs have an
explicit lifecycle.** It keeps Bitcoin's UTXO model but adds a new rule: very
old outputs must be spent or renewed. It does not change Bitcoin.

[Understand OBTC in one page](https://organicbitcoin.org/overview.html) ·
[Website](https://organicbitcoin.org) ·
[Whitepaper](https://organicbitcoin.org/whitepaper.html) ·
[Run it locally](#run-it-locally) ·
[Review the design](EXTERNAL_REVIEW_PACKET.md)

## OBTC in one minute

Bitcoin represents wallet value as unspent transaction outputs (UTXOs):
individual pieces of value that remain spendable until used. OBTC tests a
deliberately controversial alternative—giving each output a long lifecycle.
An output's age only measures when it was created onchain; it does **not** show
that its keys are lost or that its value has been abandoned.

Under the current candidate design:

1. A UTXO has a lifecycle window of `362,880` blocks, approximately 6.9 years
   at ten minutes per block.
2. Before expiry, its holder can spend it normally or renew it into a fresh
   output.
3. After expiry, ordinary spending is rejected. The output becomes eligible
   for deterministic processing called **REAP** (Reclaim Expired Assets
   Protocol).
4. For an output of at least 720 satoshis, REAP sends 70% back to its original
   locking script and 30% to the proof-of-work security budget. Smaller outputs
   go entirely to that budget.

REAP ordering and limits are consensus rules verified by full nodes; miners do
not choose arbitrary expired outputs. The experiment tests whether this makes
long-lived UTXO state and security funding easier to reason about. Its cost is
equally explicit: holders inherit a maintenance obligation and an expiry risk.
That tradeoff is the question OBTC exists to test, not a settled claim.

## Run it locally

The fastest path is the repository's isolated two-node devnet. It does not use
valuable funds or require access to Bitcoin wallet keys.

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
go test ./...

./scripts/devnet-up.sh start
./scripts/devnet-up.sh demo
./scripts/devnet-up.sh status
./scripts/devnet-up.sh stop
```

For external network testing, follow the
[Public Testnet Self-Test](docs/limited-public-testnet-user-test.md). Public
testnet coins have no real-world value.

## Current status and boundaries

`obtcd` is the OBTC node implementation, derived from
[btcsuite/btcd](https://github.com/btcsuite/btcd). The current public target is
[`mainnet-candidate-2026-07`](https://github.com/organicbitcoin/obtcd/milestone/1),
and the code and public testnet are open for technical review.

- This is mainnet-candidate software, not a production financial system.
- The fork height, activation height, and other candidate parameters remain
  provisional until final release artifacts are published.
- OBTC is not an investment project, an adoption request, or a promise of
  miner income.
- Do not import Bitcoin private keys, seed phrases, or wallet files into review
  software.
- The companion wallet implementation is
  [organicbitcoin/obtcwallet](https://github.com/organicbitcoin/obtcwallet).

Useful places to continue:

- **New to the idea:** [plain-language overview](https://organicbitcoin.org/overview.html)
- **Technical reviewer:** [external review packet](EXTERNAL_REVIEW_PACKET.md)
  and [reviewer primer](OBTC_REVIEWER_PRIMER.md)
- **Protocol tests:** [review test vectors](REVIEW_TEST_VECTORS.md) and
  [fixture vectors](REVIEW_FIXTURE_VECTORS.md)
- **Known risks:** [known limitations](KNOWN_LIMITATIONS.md) and
  [security review checklist](SECURITY_REVIEW_CHECKLIST.md)
- **Node or wallet operator:** [node runbook](NODE_OPERATOR_RUNBOOK.md) and
  [wallet runbook](WALLET_OPERATOR_RUNBOOK.md)
- **Mining reviewer:** [mining review checklist](docs/mining-review-checklist.md)

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
  mainnet-candidate release artifacts are published. Current mainnet
  replay-protection height is `1000001` (fork + 1). Current expiry / REAP /
  commitment activation height is `1002016` (fork + 2016).

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

## Documentation

- [Mainnet Candidate External Review Packet](EXTERNAL_REVIEW_PACKET.md)
- [Mainnet Candidate Release Notes](MAINNET_CANDIDATE_RELEASE_NOTES.md)
- [Mainnet Candidate Test Report](MAINNET_CANDIDATE_TEST_REPORT.md)
- [Known Limitations](KNOWN_LIMITATIONS.md)
- [OBTC Node Documentation](docs/)
- [OBTC Public Testnet User Test](docs/public-testnet-user-test.md)
- [OBTC Mainnet Join Runbook](docs/mainnet-join.md)
- [Public Testnet Self-Test](docs/limited-public-testnet-user-test.md)
- [OBTC Testnet Join Guide](docs/testnet-join.md)
- [Network Parameters](docs/network-parameters.md)

Additional devnet scenarios are available after starting the local devnet:

```bash
./scripts/devnet-up.sh scenario feemarket
./scripts/devnet-up.sh scenario conflict
./scripts/devnet-up.sh scenario multisource
```

Public testnet coins have no real-world value. There is no public faucet at
this stage; test coins are sent manually for node, wallet, expiry, and renewal
testing.

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

Maintained by [Pengyu Zhao](https://github.com/zpengyu).

## Upstream attribution

`obtcd` is derived from [btcsuite/btcd](https://github.com/btcsuite/btcd). The
upstream project's networking, wallet-adjacent RPC foundations, database
interfaces, and consensus architecture remain visible throughout this
repository. OBTC-specific changes are layered on top for lifecycle rules,
expiry/reclaim validation, network isolation, and launch tooling.

## License

OBTCD is licensed under the [copyfree](http://copyfree.org) ISC License.
