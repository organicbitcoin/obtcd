# OBTC Mainnet Candidate Release Notes

Status: `v0.1.0-mainnet-candidate.1` source-only external technical review
release.

This document is review material. It is not a production mainnet launch
announcement, not investment material, and not an exchange or custody
integration notice.

## Candidate Identity

| Field | Value |
|---|---|
| Candidate version | `v0.1.0-mainnet-candidate.1` |
| Release date | `2026-07-02` |
| Release status | source-only external technical review |
| Production launch status | not a production mainnet launch |
| `obtcd` gate-cleanup baseline | `2828dad2aeba136ae1539ccc47b0a28c331a8729` |
| `obtcwallet` gate-cleanup baseline | `ea0070517641fa9f0ba5731b903aae1c57f24d5a` |
| Binary artifacts | not distributed for MC1 |
| Project checksums | not produced because no project-built archives are distributed |
| Security contact | GitHub private vulnerability reporting if enabled; otherwise open a minimal public issue asking for a secure channel |

Repository entry points:

- `obtcd`: <https://github.com/organicbitcoin/obtcd>
- `obtcwallet`: <https://github.com/organicbitcoin/obtcwallet>
- website/docs: <https://organicbitcoin.org>; repository docs remain the review
  source unless a later release points to a specific public docs URL.

## Included Review Surface

- `obtcd`: OBTC node, consensus validation, RPC, mining template, expiry index,
  expiry commitment, replay protection, REAP validation, and operator tools.
- `obtcwallet`: wallet expiry visibility, manual renewal, `renewall`, local
  signer path, OBTC replay-signing policy, and default-off auto-renew controls.
- Review docs: issue gate review, external review packet, reviewer primer,
  review cards, concrete fixture vectors, known limitations, and test report.
- Runbooks: node, wallet, testnet/regtest, security checklist, and known
  operational limits.

## Mainnet Parameter Summary

The source of truth is `chaincfg/params_obtc.go`.

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
| Fork height | `1000000` candidate value |
| First independent OBTC block | `1000001` |
| Replay protection height | `1000001` |
| Expiry / REAP / commitment activation height | `1002016` |
| Expiry window | `362880` blocks |
| REAP normal input cap | `256` |
| REAP refundless dust input cap | `1024` |
| REAP max weight | `400000` weight units |
| REAP tax ratio | `30 / 100` |
| REAP dust threshold | `720` satoshis |

For the full table, read `docs/mainnet-params.md` and
`docs/network-parameters.md`.

## Issue Gate Result

`FINAL_ISSUE_GATE_REVIEW.md` records the final issue gate.

Resolved or scoped out for MC1:

- wallet replay-signing P1 fixed and merged in `obtcwallet` PR #12;
- wallet HD coin-type and imported xpub namespace policy documented and tested;
- replay sighash matrix and concrete review fixture vectors added in `obtcd`;
- wallet artifact issue handled by source-only MC1 policy;
- node artifact issue handled by source-only MC1 policy;
- wallet funded failure-mode review documented.

Deferred out of MC1:

- DNS seed provisioning;
- long-lived seed/fallback nodes;
- fresh sync from final bootstrap policy;
- 72h final mainnet-candidate node-set observation;
- funded auto-renew scheduler drill;
- binary artifact/checksum/signature release.

## Test Status

See `MAINNET_CANDIDATE_TEST_REPORT.md`.

Summary:

- focused `obtcd` replay, REAP, expiry, mining, mempool, and blockchain tests
  passed;
- broad `obtcd` `go test ./...` passed;
- focused `obtcwallet` replay/signing/PSBT/import/waddrmgr/renewal tests
  passed;
- scoped broad `obtcwallet` local gate passed with the upstream chain package
  excluded because it requires external Bitcoin-node dependencies.

## Known Limitations

See `KNOWN_LIMITATIONS.md`.

Important limits:

- this is source-only candidate review material, not a production launch;
- no formal third-party security audit has been completed or recorded;
- testnet and regtest behavior are accelerated review environments;
- no live MC1 DNS seed or long-lived public seed/fallback node set is claimed;
- no 72h final mainnet-candidate node-set observation is claimed;
- auto-renew is disabled by default and not recommended as an MC1 operator path;
- no project-built binary archives or project `SHA256SUMS` are distributed for
  MC1.

## Review Focus Areas

- expiry boundary behavior;
- REAP validity, marker digest, canonical order, dust fold, caps, and weight
  budget;
- coinbase accounting and overclaim rejection;
- mempool isolation for REAP-like transactions;
- replay protection at activation boundaries and across allowed sighash types;
- expiry commitment root validation;
- expiry index reorg/rebuild behavior;
- wallet replay-signing safety and renewal controls;
- node/wallet operator procedures and limitations.

## Reporting Path

Use public issues for non-sensitive technical reports:

<https://github.com/organicbitcoin/obtcd/issues>

For wallet-specific reports:

<https://github.com/organicbitcoin/obtcwallet/issues>

Include commit hash, network, command, expected behavior, observed behavior,
height, txid or block hash where applicable, and redacted logs.

For sensitive security reports, use GitHub private vulnerability reporting if
available. If it is unavailable, open a minimal public issue asking for a secure
channel and include no exploit details or secrets.

## Build From Source

```bash
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

The companion wallet is built from its repository:

```bash
go build -o ./btcwallet .
go build -o ./renewall ./cmd/renewall
```
