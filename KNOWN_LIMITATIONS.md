# OBTC Mainnet Candidate Known Limitations

This file lists limitations for `v0.1.0-mainnet-candidate.1`. It is intentionally
direct. It is not marketing material.

## Release Scope

- Current status is source-only Mainnet Candidate for external technical review.
- This is not a production mainnet launch.
- This is not a seed-backed public mainnet operating network.
- This is not investment material.
- Testnet coins have no real-world value.
- Regtest and testnet behavior are review tools, not production evidence.

## Source-Only Policy

- MC1 distributes source references, not project-built binary artifacts.
- `obtcd` gate-cleanup baseline:
  `2828dad2aeba136ae1539ccc47b0a28c331a8729`.
- `obtcwallet` gate-cleanup baseline:
  `ea0070517641fa9f0ba5731b903aae1c57f24d5a`.
- No project `SHA256SUMS` file is produced for MC1 because no project-built
  binary archives are distributed for this release scope.
- Reviewers should verify the Git commits and build locally.

## Implementation And Review Limits

- Independent third-party implementation evidence is not recorded here.
- No formal third-party security audit has been completed or recorded for the
  current candidate.
- Release and review materials must not describe the current candidate as
  audited unless a formal third-party audit is completed and published.
- Third-party miner, pool, exchange, custody, and explorer integrations are not
  in the current release scope.
- Miner adoption is not assumed.
- The Go module path still inherits upstream `github.com/btcsuite/btcd`.
- Some binary names still inherit upstream names such as `btcd`, `btcctl`, and
  `btcwallet`.

## Network And Parameter Limits

- Mainnet fork height `1000000` remains the candidate value under review.
- Expiry / REAP / commitment activation height `1002016` is derived from the
  candidate fork height.
- Consensus rules, mainnet parameters, replay protection, expiry, and REAP rules
  were not changed during final issue-gate cleanup.
- DNS seed A/AAAA records are not live for MC1.
- Long-lived public seed/fallback nodes are not deployed for MC1.
- Fresh-node sync through the final published mainnet bootstrap policy is not
  demonstrated for MC1.
- A 72h observation window for the final mainnet-candidate node set is not
  recorded for MC1.

These network items are non-blocking for source-only external review, but they
remain blockers before seed-backed public operation or production launch claims.

## Wallet Limits

- `obtcwallet` is a controlled operator/reviewer wallet path, not a production
  wallet for valuable funds.
- Auto-renew is disabled by default and is not recommended as an MC1 operator
  path.
- Funded auto-renew scheduler evidence is deferred to a later RC/production
  readiness gate.
- Manual renewal and `renewall` review paths exist, but operators should treat
  funded runs as controlled-environment drills unless a later reviewed release
  expands the scope.
- Remote signer usage remains outside the MC1 recommended operator path unless
  a separate end-to-end operator run is recorded.
- Never import a Bitcoin seed phrase, Bitcoin private key, or real wallet backup
  into experimental OBTC software.

The wallet failure-mode guidance is recorded in the companion repository:
`WALLET_OPERATOR_RISK_REVIEW.md`.

## Observability Limits

- `obtc-status` is a minimal operator status page, not a full explorer.
- Explorer/status dashboard coverage is minimal and should not be treated as a
  complete public monitoring system.
- REAP candidate counts are exposed through node RPC summaries and related
  tooling, not a full public analytics surface.

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
