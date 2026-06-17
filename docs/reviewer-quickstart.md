# OBTC Reviewer Quick Start

This guide is for protocol, wallet, mining, pool, and documentation reviewers
who want to understand OBTC before deciding whether to run anything.

OBTC is a separate Bitcoin-derived proof-of-work experiment around UTXO
lifecycle rules. It is not a Bitcoin consensus proposal, not an investment
project, not an endorsement request, and not a request that any pool, firmware
project, or protocol project adopt OBTC.

## 1. What OBTC is testing

OBTC makes long-dormant UTXOs explicit protocol state:

- **expiry:** outputs eventually enter an expired state after the configured
  lifecycle window;
- **renewal:** active holders can renew eligible outputs before they expire;
- **REAP:** expired candidates can enter a rule-bound reclaim path;
- **refund/security-budget accounting:** REAP keeps a refund share while
  directing the protocol-defined remainder to the security budget;
- **expiry commitments:** block templates commit to expiry-index state;
- **replay protection:** OBTC uses separate network parameters and activation
  boundaries from the inherited Bitcoin history.

The current public status is mainnet-candidate and public testnet review. Treat
the code and documentation as review material, not as final release guidance for
valuable funds.

## 2. What to read first

1. [Repository README](../README.md) for the project scope, status, and
   non-goals.
2. [Testnet Join Guide](testnet-join.md) for the fastest external node path.
3. [Mining Review Checklist](mining-review-checklist.md) if your review touches
   `getblocktemplate`, coinbase construction, pools, or Stratum assumptions.
4. [Mainnet Join Runbook](mainnet-join.md) and
   [Mainnet Parameters](mainnet-params.md) if you are reviewing the
   mainnet-candidate boundary.
5. [Network Parameters](network-parameters.md) for ports, address namespaces,
   and activation heights.

## 3. What to review

Useful review is specific. The best reports include the commit, command,
output, expected behavior, host role, and whether the issue affects consensus,
sync, wallet behavior, mining, or documentation clarity.

Protocol reviewers:

- activation heights and network isolation;
- expiry-index state transitions;
- expiry commitment construction and validation;
- REAP candidate selection, ordering, weight limits, and refund accounting;
- replay-protection behavior at and after the fork boundary.

Wallet reviewers:

- expiry visibility through RPC and wallet flows;
- renewal eligibility and error handling;
- default-off automation boundaries;
- backup, restore, rescan, and near-expiry behavior.

Mining and pool reviewers:

- `getblocktemplate` behavior and returned capabilities;
- coinbase value/output handling;
- expiry commitment and REAP commitment preservation;
- template mutation boundaries;
- share difficulty versus block target handling;
- `submitblock` behavior and error reporting;
- Stratum v1 and Stratum v2 translator assumptions.

Documentation reviewers:

- whether the first page explains what OBTC is and why it exists;
- whether the current status is clear;
- whether any wording implies maturity, adoption, or revenue that the project
  does not claim;
- whether a clean external operator can follow the testnet guide.

## 4. What to run

Build from the repository root:

```bash
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Run focused checks that do not require a full external network:

```bash
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|RejectREAPSystemTxFromMempool' -count=1
go test ./mining -run 'REAP|Accounting|Witness' -count=1
```

Start a testnet node:

```bash
./btcd \
  --obtctestnet \
  --datadir=$HOME/.obtcd-testnet \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --txindex \
  --expiryindex \
  --notls \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --addpeer=<peer1:19527>
```

Check basic node state:

```bash
./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getblockchaininfo

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpiryindexstats
```

For mining review, continue with
[Mining Review Checklist](mining-review-checklist.md).

## 5. Evidence to capture

Include only redacted output. Do not share RPC passwords, private keys, seed
phrases, wallet passphrases, private local paths, or screenshots containing
sensitive data.

Capture:

- operating system and architecture;
- `git rev-parse --short HEAD`;
- exact command and flags;
- network flag used (`--obtctestnet`, `--obtcmainnet`, or regtest);
- node height, best hash, peer count, and chain info;
- relevant RPC output;
- logs around the first error;
- expected behavior and actual behavior.

## 6. Where to file feedback

Use the public issue tracker for node bugs, release evidence,
mainnet-candidate blockers, mining-template review, and operator feedback:

<https://github.com/organicbitcoin/obtcd/issues>

Use a focused title, for example:

- `docs: unclear testnet peer setup in testnet-join.md`
- `mining: getblocktemplate coinbasevalue question at testnet height N`
- `review: expiry commitment boundary unclear in mining checklist`

If a finding includes private logs or sensitive host details, redact first and
include only the minimum reproduction data needed for review.
