# OBTC Mainnet Candidate Known Limitations

This file lists limitations that reviewers and operators should see before
evaluating the Mainnet Candidate. It is intentionally direct. It is not a
marketing document.

## Release Scope

- Current status is Mainnet Candidate for external technical review.
- This is not a production mainnet launch.
- This is not investment material.
- Testnet coins have no real-world value.
- Regtest and testnet behavior are review tools, not mainnet operating evidence.

## Implementation And Review Limits

- Independent third-party implementation evidence is not recorded here.
- No formal third-party security audit has been completed or recorded for the
  current candidate.
- Release and review materials must not describe the current candidate as
  audited unless a formal third-party audit is actually completed and published.
- Third-party miner, pool, exchange, custody, and explorer integrations are not
  in the current release scope.
- Miner adoption is not assumed.
- The Go module path still inherits upstream `github.com/btcsuite/btcd`.
- Some binary names still inherit upstream names such as `btcd`, `btcctl`, and
  `btcwallet`.

## Network And Parameter Limits

- Mainnet fork height `1000000` is the value in the
  `mainnet-candidate-2026-07` tag. A later candidate or production release would
  require explicit review before changing it.
- Expiry / REAP / commitment activation height `1002016` is derived from the
  provisional fork height.
- Parameter freeze state is represented by the `mainnet-candidate-2026-07` tag.
- DNS seed and fallback-peer publication require final operator evidence.
- A fresh-node sync through final published bootstrap peers is
  not demonstrated in this repository.

## Test Environment Limits

- Testnet has accelerated parameters and does not represent mainnet timing.
- Regtest has accelerated expiry and does not represent mainnet time horizon.
- Local focused tests do not replace long-running public node observation.
- Plan 07 reproducible demo files are merged into `master`.
- PR #14 and PR #15 CI completed successfully before merge.

## Wallet Limits

- Wallet UI and walletapp are engineering-oriented.
- Non-dry-run funded renewal evidence remains release-scope dependent.
- Auto-renew is opt-in and should stay disabled unless explicitly tested for the
  operator environment.
- Auto-renew persistence and long wall-clock scheduler evidence remain human
  review items in the companion wallet notes.
- Never import a Bitcoin seed phrase, Bitcoin private key, or real wallet backup
  into experimental OBTC software.

## Observability Limits

- `obtc-status` is a minimal operator status page, not a full explorer.
- Explorer/status dashboard coverage is minimal and should not be treated as a
  complete public monitoring system.
- REAP candidate counts are currently exposed through node RPC summaries and
  related tooling, not a full public analytics surface.

## Out Of Scope

- Price discussion.
- Exchange listing.
- Custody integration.
- Promotion or marketing metrics.
- Any investment narrative.
- Real-fund claim tooling unless a separate reviewed release explicitly covers
  it.

## Issue Severity Classification

| Severity | Examples | Expected handling |
|---|---|---|
| Critical | consensus split, coinbase overclaim acceptance, replay bypass, invalid REAP acceptance, private-key exposure | private security channel if sensitive; otherwise immediate public blocker issue |
| High | expiry index corruption after reorg, missing mandatory commitment accepted, fake REAP mempool acceptance, wallet signs unsafe renewal | focused issue with reproduction and affected commit |
| Medium | operator runbook causes node outage, status output misleads operator, wallet error blocks safe manual renewal | public issue with logs and commands |
| Low | wording ambiguity, missing link, minor CLI ergonomics, non-sensitive dashboard gap | public issue or documentation PR |

Sensitive reports should use GitHub private vulnerability reporting if enabled.
If not enabled, open a minimal public issue asking for a secure reporting path
and include no exploit details or secrets.
