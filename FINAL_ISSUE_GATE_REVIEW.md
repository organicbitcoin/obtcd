# Final Issue Gate Review

Plan: Final Issue Gate Cleanup Before Mainnet Candidate Seal

Assessment date: 2026-07-02

Scope: `v0.1.0-mainnet-candidate.1` source-only external technical review
release. This is not a production mainnet launch, not a seed-backed public
mainnet operation, and not a real-funds wallet release.

## Source State

| Repository | Assessed commit | Branch state |
|---|---|---|
| `organicbitcoin/obtcd` | `115c9255919f8f266e1b7c7ed2ede8df47087807` | `master` at `origin/master` before this gate branch |
| `organicbitcoin/obtcwallet` | `0bde8d27b8853fd9cf58e0084dba12788a32fab2` | PR #12 merged to `origin/master` |

## Gate Rule

If an open issue still has `mainnet-blocker`, the Go/No-Go decision must be
NO-GO unless the issue is explicitly kept blocking. For a GO WITH
NON-BLOCKING LIMITATIONS decision, every remaining open item must be either
closed or publicly downgraded with a written reason.

## Open Issue Review

### `organicbitcoin/obtcd`

| Issue | Title | Labels at review | Milestone | MC1 external review blocker | RC / production blocker | Recommended action | Reason |
|---|---|---|---|---|---|---|---|
| #2 | Track deferred OBTC mainnet DNS seed provisioning | `mainnet-blocker`, `evidence` | none | No | Yes | Downgrade to `post-launch` / evidence follow-up | MC1 reviewers can build, test, and inspect source without live DNS seed discovery. Live DNS seeds are required before any seed-backed public operating network claim. |
| #3 | Deploy and validate mainnet-candidate seed/fallback nodes | `mainnet-blocker`, `evidence` | `mainnet-candidate-2026-07` | No | Yes | Downgrade to `post-launch` / evidence follow-up | Long-lived public seed/fallback nodes are not required for source-only external review. They remain required before a seed-backed network or production launch claim. |
| #4 | Record fresh-node sync from published bootstrap policy | `mainnet-blocker`, `evidence` | `mainnet-candidate-2026-07` | No | Yes | Downgrade to `post-launch` / evidence follow-up | Public testnet fresh-sync evidence exists, but final mainnet-candidate bootstrap sync depends on the deferred seed/fallback policy. Not required for source-only review. |
| #5 | Run 72h mainnet-candidate observation window | `mainnet-blocker`, `evidence` | `mainnet-candidate-2026-07` | No | Yes | Downgrade to `post-launch` / evidence follow-up | A 72h mainnet-candidate node set is not part of MC1 source-only review. It remains required before broader public-network or production-readiness claims. |
| #6 | Produce obtcd mainnet-candidate release artifacts and checksums | `mainnet-blocker`, `evidence` | `mainnet-candidate-2026-07` | Resolved by scope decision | Later binary releases only | Close after docs update | MC1 is explicitly source-only. No project-built binary artifact is distributed, so no project `SHA256SUMS` is produced for MC1. The source commit is the verification anchor. |
| #7 | Review request: is OBTC's what / why / review path clear? | none | none | No | No | Keep open as feedback intake | This is a non-blocking documentation clarity request and has no blocker label. |

### `organicbitcoin/obtcwallet`

| Issue | Title | Labels at review | Milestone | MC1 external review blocker | RC / production blocker | Recommended action | Reason |
|---|---|---|---|---|---|---|---|
| #4 | Validate autorenew in funded controlled environment | `mainnet-blocker`, `evidence`, `operator-readiness` | `mainnet-candidate-2026-07` | No | Yes, if auto-renew is in scope | Downgrade to `post-launch` / operator-readiness follow-up | Auto-renew is disabled by default and not recommended for the MC1 operator path. Existing tests cover disabled default, config validation, selection limits, and backoff behavior. Funded scheduler evidence remains required before recommending auto-renew. |
| #7 | Publish release artifacts and checksums for operator commit | `mainnet-blocker`, `evidence`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by scope decision | Later binary releases only | Close after docs update | MC1 wallet release is source-only at merged commit `0bde8d27b8853fd9cf58e0084dba12788a32fab2`. No wallet binary artifact is distributed, so no project `SHA256SUMS` is produced for MC1. |
| #8 | Review funded-wallet failure modes before mainnet-candidate | `mainnet-blocker`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by `WALLET_OPERATOR_RISK_REVIEW.md` | Follow-up evidence remains useful | Close after risk note lands | The required risk note now covers wrong passphrase, stale chain, RPC failure, expired-boundary fail-closed behavior, backup/restore/rescan, wrong network, wrong endpoint, TLS, exposed RPC, confirmations, renewal amount, and wallet DB/network directory mixups. |

## Artifact And Checksum Policy

MC1 is source-only.

- `obtcd` source commit: `115c9255919f8f266e1b7c7ed2ede8df47087807`.
- `obtcwallet` source commit: `0bde8d27b8853fd9cf58e0084dba12788a32fab2`.
- No project-built binary archive is distributed for MC1.
- No project `SHA256SUMS` file is produced for MC1 because there are no
  project-built release archives to checksum.
- Reviewers should verify source by Git commit ID and build locally with the
  documented Go versions.

This does not prohibit later binary RC artifacts. It only prevents MC1 from
claiming artifact completeness that it does not need for source review.

## Scope Downgrades

The following items are non-blocking for MC1 but remain blockers before any
seed-backed public mainnet operation or production launch wording:

- final DNS seed A/AAAA records;
- long-lived public seed/fallback nodes;
- fresh sync from the final published bootstrap policy;
- 72h observation of the final mainnet-candidate node set;
- funded auto-renew scheduler drill;
- binary artifact/checksum/signature release process.

## Decision

Recommended decision after issue labels are updated:

**GO WITH NON-BLOCKING LIMITATIONS** for `v0.1.0-mainnet-candidate.1` as a
source-only external technical review release.

Required condition: no open issue may retain `mainnet-blocker` after the
downgrade/closure comments are posted. If any issue remains open with
`mainnet-blocker`, the decision must be **NO-GO**.

