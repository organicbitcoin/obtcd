# OBTC Mainnet Candidate Release Notes

Status: Mainnet Candidate for external technical review.

This document is release-draft material for reviewers. It is not a production
mainnet launch announcement, not investment material, and not an exchange or
custody integration notice.

## Candidate Identity

| Field | Value |
|---|---|
| Candidate version | `mainnet-candidate-2026-07` |
| Candidate tag | `mainnet-candidate-2026-07` |
| Release date | `2026-07-01` |
| Release status | Mainnet Candidate for external technical review |
| Production launch status | Not a production mainnet launch |
| `obtcd` commit reviewed for this draft | `0bdb3f1671756e75a20cb4807684491c25b6367e` |
| `obtcwallet` commit observed locally | `c0e8a03fd7fac02a831a65f117679d05aa4625ef` |
| `obtc-website` commit observed locally | `55649a5fe960aa9da5ed8f34b1714c4d21a42fbd` |
| Final release artifact URLs | Not published to a GitHub Release yet |
| Final checksums/signatures | Generated locally with `SHA256SUMS`, `SHA256SUMS.sig`, and `MANIFEST.md.sig` |
| Security contact | GitHub private vulnerability reporting if enabled; otherwise open a minimal public issue asking for a secure channel |

Repository entry points:

- `obtcd`: <https://github.com/organicbitcoin/obtcd>
- `obtcwallet`: <https://github.com/organicbitcoin/obtcwallet>
- website/docs: <https://organicbitcoin.org>; repository docs remain the
  review source until a final public docs URL is published.

## Included Components

- `obtcd`: OBTC node, consensus validation, RPC, mining template, expiry index,
  expiry commitment, replay protection, REAP validation, and operator tools.
- `obtcwallet`: wallet expiry visibility, manual renewal, `renewall`, and
  opt-in auto-renew controls.
- website/docs: public review pages and parameter references.
- test scripts: parameter checker, validation scripts, phase6 node/seed
  helpers, status page, and artifact build helpers.
- runbooks: node, wallet, external review, security checklist, known
  limitations, mainnet operations, testnet, and mining review documents.

## Mainnet Parameter Summary

The code source of truth is `chaincfg/params_obtc.go`. The current parameter
checker result for this draft is `68 exact match, 0 missing, 0 mismatch, 0
ambiguous, 0 warning`.

| Parameter | Value |
|---|---|
| Network flag | `--obtcmainnet` |
| Chain name | `obtcmainnet` |
| Wire magic | `0x4F425443` |
| P2P port | `9527` |
| Node RPC port | `9528` |
| Wallet RPC port | `9554` |
| Bech32 HRP | `obtc` |
| P2PKH prefix | `0x47` |
| P2SH prefix | `0x32` |
| WIF prefix | `0x9A` |
| BIP44 coin type | `20260` |
| Fork height | `1000000` provisional candidate value |
| First independent OBTC block | `1000001` |
| Replay protection height | `1000001` |
| Expiry / REAP / commitment activation height | `1002016` |
| Expiry window | `362880` blocks |
| REAP normal input cap | `256` |
| REAP refundless dust input cap | `1024` |
| REAP max weight | `400000` weight units |
| REAP tax ratio | `30 / 100` |
| REAP dust threshold | `720` satoshis |

For the full table, read [docs/mainnet-params.md](docs/mainnet-params.md) and
[docs/network-parameters.md](docs/network-parameters.md).

## Activation Schedule

| Event | Height | Notes |
|---|---:|---|
| Fork anchor | `1000000` | Provisional candidate value; may change before final artifacts |
| First independent OBTC block | `1000001` | Replay protection active from this height |
| Replay protection | `1000001` | OBTC signature-domain separation |
| Expiry enforcement | `1002016` | Derived from provisional fork height + 2016 |
| REAP consensus / hardening | `1002016` | Canonical REAP validation path |
| Expiry commitment mandatory | `1002016` | Coinbase commitment root required |

## Major Changes Since Earlier Engineering Drafts

- Added OBTC mainnet, testnet, and regtest parameter sets and network
  namespaces.
- Added expiry index state, RPC inspection, rebuild paths, and reorg tests.
- Added expiry commitment construction and validation.
- Added replay protection activation checks.
- Added REAP validation, canonical selection, marker digest, dust fold, input
  caps, weight budget, and coinbase accounting coverage.
- Added mining template REAP append tests and mempool isolation coverage.
- Added `obtc-status`, phase6 operational scripts, and release artifact helper
  scripts.
- Added wallet expiry and renewal flows in the companion wallet repository.
- Added external reviewer and operator documentation.

## Test Status

See [MAINNET_CANDIDATE_TEST_REPORT.md](MAINNET_CANDIDATE_TEST_REPORT.md).

Summary for this draft:

- local node build commands passed on macOS arm64 with Go `1.25.3`;
- parameter checker passed with 68 exact matches;
- focused consensus, mempool, mining/template, expiry index, and wallet
  lifecycle commands passed;
- Plan 07 reproducible regtest demo PR #14 was merged into `master`;
- Plan 08 release package PR #15 was merged into `master`;
- final tag `mainnet-candidate-2026-07` points to
  `0bdb3f1671756e75a20cb4807684491c25b6367e`;
- final-tag local regtest demo evidence was captured on 2026-07-01.

## Known Limitations

See [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md). Important limits:

- this is candidate review material, not a production mainnet launch;
- independent third-party implementation and formal external security audit
  evidence are not recorded in this repository;
- testnet and regtest behavior are accelerated review environments and should
  not be treated as mainnet economics or timing evidence;
- wallet UI and status dashboards are engineering-oriented;
- seed/DNS and 72h public observation evidence still require operator capture;
- public GitHub Release artifact upload remains a release operation.

## Review Focus Areas

- expiry boundary behavior;
- REAP validity, marker digest, canonical order, dust fold, caps, and weight
  budget;
- coinbase accounting and overclaim rejection;
- mempool isolation for REAP-like transactions;
- replay protection at activation boundaries;
- expiry commitment root validation;
- expiry index reorg/rebuild behavior;
- wallet renewal safety and auto-renew controls;
- node operator procedures and observability.

## Reporting Path

Use public issues for non-sensitive technical reports:

<https://github.com/organicbitcoin/obtcd/issues>

Use `mainnet-candidate-2026-07` in the title or labels when relevant. Include
commit hash, network, command, expected behavior, observed behavior, height,
txid or block hash where applicable, and redacted logs.

For sensitive security reports, use GitHub private vulnerability reporting if
available. If it is unavailable, open a minimal public issue asking for a secure
channel and include no exploit details or secrets.

## Reproducibility Notes

Build commands:

```bash
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Parameter check:

```bash
python3 scripts/check_obtc_parameters.py --format json
```

Artifact helper:

```bash
scripts/phase6/build_release_artifacts.sh \
  --version mainnet-candidate-2026-07 \
  --goos linux \
  --goarch amd64
```

Release operations still required before a public GitHub Release:

- upload the generated artifact packages, checksums, and signatures;
- decide whether the companion wallet should receive a matching Git tag;
- publish the final website/docs URL if the review packet should link outside
  this repository;
- confirm GitHub private vulnerability reporting or publish the preferred
  secure contact path;
- capture public seed/DNS and 72h observation evidence if those are in scope.
