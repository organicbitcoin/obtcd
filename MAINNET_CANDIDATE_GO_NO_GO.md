# OBTC Mainnet Candidate Go/No-Go Report

Assessment date: 2026-07-02

Decision: **GO** for publishing and maintaining the OBTC Mainnet Candidate
external technical review release.

This is **not** a GO for a production mainnet launch, market release, exchange
integration, custody integration, or public seed-backed operating network.

## Scope Boundary

This report answers one question:

> Can an external technical reviewer build, run, inspect, test, and reproduce
> the OBTC Mainnet Candidate package from the current public repository state
> and attached release artifacts?

It does not decide whether OBTC should launch a production mainnet. It does not
decide whether public seed nodes, DNS seed discovery, exchange support, funded
wallet use, or public mining operations are ready.

## Source State

| Item | Value |
|---|---|
| `obtcd` repository | `organicbitcoin/obtcd` |
| Current `master` assessed | `98abdb8d3b14087fa234ee5ad067f5db5905bae1` |
| Candidate source tag | `mainnet-candidate-2026-07` |
| Candidate source tag commit | `0bdb3f1671756e75a20cb4807684491c25b6367e` |
| `obtcwallet` matching tag | `mainnet-candidate-2026-07` |
| `obtcwallet` matching tag commit | `c0e8a03fd7fac02a831a65f117679d05aa4625ef` |
| GitHub Release | <https://github.com/organicbitcoin/obtcd/releases/tag/mainnet-candidate-2026-07> |
| Release type | GitHub pre-release |
| Release assets | 15 uploaded assets |
| Local release evidence | Captured outside the repository; not required for reviewer source checkout |

The `obtcd` candidate tag intentionally points at the frozen candidate source
commit. Later documentation commit `98abdb8d3b14087fa234ee5ad067f5db5905bae1`
records release evidence after the tag was cut.

## Mechanical Decision Rules

| Rule | Requirement | Result | Evidence |
|---|---|---:|---|
| R1 | P0 blockers must be zero. | PASS | `BASELINE_AUDIT.md` records no P0; no open GitHub issue is labelled P0 or critical. |
| R2 | P1 blockers for this release scope must be zero. | PASS | Historical P1 items are either resolved for external review, explicitly excluded from this release scope, or remain public-network operations outside this decision. See P1 ledger below. |
| R3 | Parameter consistency must pass. | PASS | `scripts/check_obtc_parameters.py --format json` returned `Counter({'exact match': 68})`, total `68`. |
| R4 | Consensus, expiry, REAP, replay protection, mining template, mempool isolation, and wallet renewal tests must pass. | PASS | Focused `go test` commands listed below passed on 2026-07-02. |
| R5 | External reviewers must be able to build and reproduce a local flow from docs. | PASS | `RUN_LOCAL_DEMO.md`, `RUN_TESTNET_NODE.md`, `RUN_WALLET.md`, `REGTEST_EXPIRY_REAP_DEMO.md`, and scripts are merged. Final-tag regtest demo evidence was captured. |
| R6 | Release artifacts, checksums, and signatures must be available. | PASS | GitHub Release exists with packages, `SHA256SUMS`, `MANIFEST.md`, SSH detached signatures, and `ALLOWED_SIGNERS`. Local verification passed. |
| R7 | Unknown or unverifiable critical items default to NO-GO. | PASS | Known unverifiable public-network items are explicitly outside this external-review scope and listed as NO-GO for broader scopes. |
| R8 | Protocol rules must not be changed to make the decision pass. | PASS | This assessment is documentation/evidence only; no consensus, REAP, tax/refund ratio, caps, or miner policy changes are made. |

## P0/P1 Ledger

### P0

Result: **0 open P0 blockers for external technical review.**

Evidence:

- `BASELINE_AUDIT.md` records: "No P0 issue was found in this audit."
- No open GitHub issue uses a P0 or critical label.
- No failed command was recorded in this Go/No-Go assessment.

### P1 For External Technical Review Release

Result: **0 open P1 blockers for the external technical review release.**

Historical P1 items were reviewed as follows:

| Item | Source | Disposition for this scope |
|---|---|---|
| Release artifact matrix/checksums/signatures missing | `BASELINE_AUDIT.md`, issue #6 | Resolved for this scope. Release artifacts, checksums, manifest, signatures, and GitHub Release are available. Issue #6 may still need administrative closure/update. |
| Bootstrap DNS/fallback policy unresolved | `BASELINE_AUDIT.md`, issues #2, #3, #4 | Not a blocker for repository/local external technical review. It is still a blocker for any release promise that depends on public DNS seed discovery or public seed nodes. |
| Wallet funded-operation readiness evidence-gated | `BASELINE_AUDIT.md`, `KNOWN_LIMITATIONS.md` | Not claimed for this release. Wallet tests and dry-run/renewal paths are covered; funded production wallet operation remains excluded. |
| Consensus edge-case follow-ups | `CONSENSUS_EDGE_CASES_REMAINING.md` | Not blockers for this external-review release because core consensus/expiry/REAP/replay/mining tests pass and the file frames remaining items as reviewer follow-ups or additional named scenarios. |

### Public-Network/Operational NO-GO Items

These are not blockers for the external technical review release, but they are
**NO-GO** for a public seed-backed mainnet-candidate operating network or any
production launch claim:

| Item | Issue | Status |
|---|---|---|
| DNS seed provisioning and A/AAAA records | #2 | Open |
| Long-lived seed/fallback nodes | #3 | Open |
| Fresh-node sync from final published bootstrap policy | #4 | Open |
| 72h mainnet-candidate public observation window | #5 | Open |
| Formal external security audit evidence | docs limitation | Not recorded |
| Independent third-party implementation evidence | docs limitation | Not recorded |

If the intended release definition includes any of the above public-network
claims, the decision changes from GO to **NO-GO**.

## Verification Commands Run

All commands in this section were run on 2026-07-02 unless otherwise noted.

### Parameter Consistency

```bash
python3 scripts/check_obtc_parameters.py --format json | \
  python3 -c 'import json,sys; from collections import Counter; d=json.load(sys.stdin); print(Counter(d["parameter_statuses"].values())); print(len(d["parameter_statuses"]));'
```

Observed result:

```text
Counter({'exact match': 68})
68
```

### Node Build/CLI Surface

```bash
go test ./chaincfg ./wire ./cmd/btcctl ./cmd/obtc-status -count=1
```

Observed result:

```text
ok github.com/btcsuite/btcd/chaincfg
ok github.com/btcsuite/btcd/wire
ok github.com/btcsuite/btcd/cmd/btcctl
ok github.com/btcsuite/btcd/cmd/obtc-status
```

### Mempool, REAP, Mining Template

```bash
go test ./mempool -run 'REAP|Replay' -count=1
go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1
```

Observed result:

```text
ok github.com/btcsuite/btcd/mempool
ok github.com/btcsuite/btcd/mining
```

### Expiry Index, Rebuild/Reorg, Consensus

```bash
go test ./blockchain/expiryindex -run 'Commitment|REAP|Rebuild|Reorg' -count=1
go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|ExpiryCommitment' -count=1
```

Observed result:

```text
ok github.com/btcsuite/btcd/blockchain/expiryindex
ok github.com/btcsuite/btcd/blockchain
```

### Wallet Renewal Surface

Run from the sibling `obtcwallet` checkout:

```bash
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1
```

Observed result:

```text
ok github.com/btcsuite/btcwallet/wallet
ok github.com/btcsuite/btcwallet/rpc/legacyrpc
ok github.com/btcsuite/btcwallet/rpc/rpcserver
ok github.com/btcsuite/btcwallet/cmd/renewall
```

### Release Artifact Verification

```bash
EVIDENCE_DIR="<local release evidence directory>"
cd "$EVIDENCE_DIR/packages"
shasum -a 256 -c SHA256SUMS
ssh-keygen -Y verify -f ALLOWED_SIGNERS -I maintainer@organicbitcoin.org -n file -s SHA256SUMS.sig < SHA256SUMS
ssh-keygen -Y verify -f ALLOWED_SIGNERS -I maintainer@organicbitcoin.org -n file -s MANIFEST.md.sig < MANIFEST.md
```

Observed result:

- all 10 package archives returned `OK`;
- `MANIFEST.md` returned `OK`;
- `SHA256SUMS.sig` verified successfully;
- `MANIFEST.md.sig` verified successfully.

## Release Package Evidence

The GitHub Release is available at:

<https://github.com/organicbitcoin/obtcd/releases/tag/mainnet-candidate-2026-07>

Uploaded release assets:

- `ALLOWED_SIGNERS`
- `MANIFEST.md`
- `MANIFEST.md.sig`
- `SHA256SUMS`
- `SHA256SUMS.sig`
- `obtcd-mainnet-candidate-2026-07-darwin-amd64-0bdb3f167175.tar.gz`
- `obtcd-mainnet-candidate-2026-07-darwin-arm64-0bdb3f167175.tar.gz`
- `obtcd-mainnet-candidate-2026-07-linux-amd64-0bdb3f167175.tar.gz`
- `obtcd-mainnet-candidate-2026-07-linux-arm64-0bdb3f167175.tar.gz`
- `obtcd-mainnet-candidate-2026-07-windows-amd64-0bdb3f167175.zip`
- `obtcwallet-mainnet-candidate-2026-07-darwin-amd64-c0e8a03fd7fa.tar.gz`
- `obtcwallet-mainnet-candidate-2026-07-darwin-arm64-c0e8a03fd7fa.tar.gz`
- `obtcwallet-mainnet-candidate-2026-07-linux-amd64-c0e8a03fd7fa.tar.gz`
- `obtcwallet-mainnet-candidate-2026-07-linux-arm64-c0e8a03fd7fa.tar.gz`
- `obtcwallet-mainnet-candidate-2026-07-windows-amd64-c0e8a03fd7fa.zip`

## Reviewer Reproducibility Evidence

The following reviewer entry points exist in the repository:

- `README.md`
- `RUN_LOCAL_DEMO.md`
- `RUN_TESTNET_NODE.md`
- `RUN_WALLET.md`
- `REGTEST_EXPIRY_REAP_DEMO.md`
- `TROUBLESHOOTING.md`
- `MAINNET_CANDIDATE_RELEASE_NOTES.md`
- `MAINNET_CANDIDATE_TEST_REPORT.md`
- `KNOWN_LIMITATIONS.md`
- `EXTERNAL_REVIEW_PACKET.md`
- `SECURITY_REVIEW_CHECKLIST.md`
- `NODE_OPERATOR_RUNBOOK.md`
- `WALLET_OPERATOR_RUNBOOK.md`

The final-tag regtest demo evidence was captured on 2026-07-01 using:

```bash
RESET=1 KEEP_NODE=0 OBTC_DEMO_DIR=<local evidence dir>/regtest-demo-state \
  scripts/demo-regtest-expiry-reap.sh
```

Observed behavior included:

- local demo binaries built;
- `obtcregtest` node started with `--txindex` and `--expiryindex`;
- chain mined through expiry/REAP/replay activation heights;
- expiry index stats and expiry commitment reported;
- REAP plan identified one expired candidate;
- block height `145` contained a version `3` REAP transaction;
- status JSON reported `last_reap`, commitment root, indexed tip, and next
  `reap_plan`.

## Final Decision

**GO**: The current repository and release package meet the requirements for an
OBTC Mainnet Candidate external technical review release.

The GO is limited to technical review of source, docs, artifacts, local demo,
regtest/testnet workflows, and protocol behavior. It must be presented as a
candidate review package only.

**NO-GO for broader scopes**: Production mainnet launch, public seed-backed
network readiness, public DNS bootstrap readiness, 72h mainnet-candidate
operation, funded wallet production readiness, and formal audit completion are
not established by this report.
