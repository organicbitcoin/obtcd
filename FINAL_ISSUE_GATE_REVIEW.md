# Final Issue Gate Review

Plan: Final Issue Gate Cleanup Before Mainnet Candidate Seal

Assessment date: 2026-07-02

Scope: `v0.1.0-mainnet-candidate.1` source-only external technical review
release. This is not a production mainnet launch, not a seed-backed public
mainnet operation, and not a real-funds wallet release.

## Source State

These are the gate-cleanup merge commits used for this final issue-gate
decision. Any later issue-status-only documentation commit does not change the
implementation baseline or the source-only release policy.

| Repository | Assessed commit | Branch state |
|---|---|---|
| `organicbitcoin/obtcd` | `2828dad2aeba136ae1539ccc47b0a28c331a8729` | PR #20 merged to `master` |
| `organicbitcoin/obtcwallet` | `ea0070517641fa9f0ba5731b903aae1c57f24d5a` | PR #13 merged to `master` |

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
| #7 | Publish release artifacts and checksums for operator commit | `mainnet-blocker`, `evidence`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by scope decision | Later binary releases only | Close after docs update | MC1 wallet release is source-only at gate-cleanup baseline `ea0070517641fa9f0ba5731b903aae1c57f24d5a`. No wallet binary artifact is distributed, so no project `SHA256SUMS` is produced for MC1. |
| #8 | Review funded-wallet failure modes before mainnet-candidate | `mainnet-blocker`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by `WALLET_OPERATOR_RISK_REVIEW.md` | Follow-up evidence remains useful | Close after risk note lands | The required risk note now covers wrong passphrase, stale chain, RPC failure, expired-boundary fail-closed behavior, backup/restore/rescan, wrong network, wrong endpoint, TLS, exposed RPC, confirmations, renewal amount, and wallet DB/network directory mixups. |

## Final GitHub Issue Actions

Issue updates were posted on 2026-07-02 with explicit scope rationale. No open
issue remains labeled `mainnet-blocker`.

### `organicbitcoin/obtcd`

| Issue | Final state | Final labels | Final milestone | Final action |
|---|---|---|---|---|
| #2 | Open | `evidence`, `post-launch` | none | Downgraded to RC / production-mainnet evidence follow-up. |
| #3 | Open | `evidence`, `post-launch` | none | Downgraded to RC / production-mainnet evidence follow-up. |
| #4 | Open | `evidence`, `post-launch` | none | Downgraded to RC / production-mainnet evidence follow-up. |
| #5 | Open | `evidence`, `post-launch` | none | Downgraded to RC / production-mainnet evidence follow-up. |
| #6 | Closed as completed | `mainnet-blocker`, `evidence` | `mainnet-candidate-2026-07` | Resolved by MC1 source-only policy; no obtcd binary artifacts or `SHA256SUMS` are distributed for MC1. |
| #7 | Open | none | none | Kept open as non-blocking feedback intake. |

### `organicbitcoin/obtcwallet`

| Issue | Final state | Final labels | Final milestone | Final action |
|---|---|---|---|---|
| #4 | Open | `evidence`, `operator-readiness`, `post-launch` | none | Downgraded to MC2 / RC operator-readiness follow-up; Auto-Renew is disabled by default and not recommended for MC1. |
| #7 | Closed as completed | `mainnet-blocker`, `evidence`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by MC1 source-only policy; no wallet binary artifacts or `SHA256SUMS` are distributed for MC1. |
| #8 | Closed as completed | `mainnet-blocker`, `operator-readiness` | `mainnet-candidate-2026-07` | Resolved by `WALLET_OPERATOR_RISK_REVIEW.md`. |

## Artifact And Checksum Policy

MC1 is source-only.

- `obtcd` gate-cleanup merge commit:
  `2828dad2aeba136ae1539ccc47b0a28c331a8729`.
- `obtcwallet` gate-cleanup merge commit:
  `ea0070517641fa9f0ba5731b903aae1c57f24d5a`.
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

Final decision:

**GO WITH NON-BLOCKING LIMITATIONS** for `v0.1.0-mainnet-candidate.1` as a
source-only external technical review release.

Required condition satisfied: after the downgrade/closure comments were posted,
no open issue retained `mainnet-blocker`.
