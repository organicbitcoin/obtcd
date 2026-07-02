# OBTC Mainnet Candidate External Review Packet

This packet is the starting point for modular external technical review of
`v0.1.0-mainnet-candidate.1`. It is a source-only external technical review
release. It is not a production launch announcement, not investment material,
and not an exchange listing document. It is also not a claim that a formal
third-party security audit has been completed.

## What This Packet Is

This is a map for reviewers who want to evaluate one focused mechanism or a
larger set of mechanisms:

- consensus and activation boundaries;
- expiry index behavior;
- REAP validity and mining template behavior;
- replay protection;
- expiry commitment validation;
- wallet expiry and renewal safety;
- operator procedures and known limitations.

Reviewers do not need to review the whole project to provide useful feedback.
The preferred flow is modular: choose one review card, inspect the linked code
and tests, run the commands, and report a reproducible finding or challenge.

## Read First

1. [MAINNET_CANDIDATE_RELEASE_NOTES.md](MAINNET_CANDIDATE_RELEASE_NOTES.md)
2. [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md)
3. [FINAL_ISSUE_GATE_REVIEW.md](FINAL_ISSUE_GATE_REVIEW.md)
4. [OBTC_REVIEWER_PRIMER.md](OBTC_REVIEWER_PRIMER.md)
5. [review-cards/](review-cards/)
6. [REVIEW_TEST_VECTORS.md](REVIEW_TEST_VECTORS.md)
7. [REVIEW_FIXTURE_VECTORS.md](REVIEW_FIXTURE_VECTORS.md)
8. [MAINNET_RC_ENTRY_CRITERIA.md](MAINNET_RC_ENTRY_CRITERIA.md)
9. [MAINNET_CANDIDATE_TEST_REPORT.md](MAINNET_CANDIDATE_TEST_REPORT.md)
10. [docs/mainnet-params.md](docs/mainnet-params.md)
11. [SECURITY_REVIEW_CHECKLIST.md](SECURITY_REVIEW_CHECKLIST.md)
12. [NODE_OPERATOR_RUNBOOK.md](NODE_OPERATOR_RUNBOOK.md)
13. [WALLET_OPERATOR_RUNBOOK.md](WALLET_OPERATOR_RUNBOOK.md)

Existing lower-level documents:

- [docs/reviewer-quickstart.md](docs/reviewer-quickstart.md)
- [docs/mining-review-checklist.md](docs/mining-review-checklist.md)
- [docs/testnet-join.md](docs/testnet-join.md)
- [docs/mainnet-ops-runbook.md](docs/mainnet-ops-runbook.md)
- [docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md](docs/OBTC_PARAMETER_CONSISTENCY_REPORT.md)

## 15 Minute Quick Path

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
python3 scripts/check_obtc_parameters.py --format json
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|Replay' -count=1
```

Then read:

- [OBTC_REVIEWER_PRIMER.md](OBTC_REVIEWER_PRIMER.md)
- one card from [review-cards/](review-cards/)
- [REVIEW_TEST_VECTORS.md](REVIEW_TEST_VECTORS.md)
- [REVIEW_FIXTURE_VECTORS.md](REVIEW_FIXTURE_VECTORS.md)
- [docs/mainnet-params.md](docs/mainnet-params.md)
- [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md)

## Modular Review Path

Choose one review card:

| Card | Focus |
|---|---|
| [Replay Protection](review-cards/replay-protection.md) | OBTC signature-domain activation and mempool consistency |
| [Expiry Formula](review-cards/expiry-formula.md) | UTXO expiry height calculation and wallet parity |
| [Expiry Index And Reorg](review-cards/expiry-index-and-reorg.md) | persisted expiry state, commitment root, reorg, rebuild |
| [REAP Validity](review-cards/reap-validity.md) | expired-input validity, caps, dust, tax, refund |
| [REAP Marker](review-cards/reap-marker.md) | marker payload, count, height, digest |
| [REAP Canonical Ordering](review-cards/reap-canonical-ordering.md) | global prefix and deterministic selection |
| [Coinbase Accounting](review-cards/coinbase-accounting.md) | subsidy, fees, REAP tax, expiry commitment |
| [Wallet Renewal](review-cards/wallet-renewal.md) | `obtc.getexpiry`, `obtc.renew`, `renewall` |
| [Auto-Renew Safety](review-cards/auto-renew-safety.md) | disabled default, limits, backoff, locked wallet behavior |
| [Historical Backlog](review-cards/historical-backlog.md) | large expired sets, pagination, backlog carry-over |

For each card:

1. read the mechanism summary and invariant;
2. inspect the listed code locations;
3. run the listed tests;
4. try to break the invariant;
5. open a focused issue using the matching issue template.

## 60 Minute Engineering Path

```bash
go test ./chaincfg ./wire ./cmd/btcctl ./cmd/obtc-status -count=1
go test ./mempool -run 'REAP|Replay' -count=1
go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1
go test ./blockchain/expiryindex -run 'Commitment|REAP|Rebuild|Reorg' -count=1
go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|ExpiryCommitment' -count=1
```

Review:

- [MAINNET_CANDIDATE_TEST_REPORT.md](MAINNET_CANDIDATE_TEST_REPORT.md)
- [SECURITY_REVIEW_CHECKLIST.md](SECURITY_REVIEW_CHECKLIST.md)
- [NODE_OPERATOR_RUNBOOK.md](NODE_OPERATOR_RUNBOOK.md)

## Deep Consensus Review Path

Focus files:

- `OBTC_REVIEWER_PRIMER.md`
- `REVIEW_TEST_VECTORS.md`
- `chaincfg/params_obtc.go`
- `blockchain/validation_reap.go`
- `blockchain/validation_obtc_replay_test.go`
- `blockchain/fullblocks_obtc_test.go`
- `blockchain/expiryindex/commitment_test.go`
- `blockchain/expiryindex/reorg_safety_test.go`
- `mining/reap/selector_test.go`
- `mining/reap/budget_test.go`
- `mining/newblocktemplate_reap_template_tests_test.go`

Challenge:

- expiry boundary;
- REAP validity;
- dust fold;
- marker digest;
- canonical order;
- replay protection;
- expiry commitment;
- index reorg/rebuild;
- coinbase accounting.

If this is too broad, pick only one of the review cards and report the scoped
result.

## Wallet Review Path

Use the companion repository:

```bash
git clone https://github.com/organicbitcoin/obtcwallet.git
cd obtcwallet
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1
```

Review:

- `obtcwallet/README.md`
- `obtcwallet/WALLET_RENEWAL_RUNBOOK.md`
- `obtcwallet/WALLET_LIFECYCLE_TESTS.md`
- `obtcwallet/AUTO_RENEW_SAFETY_NOTES.md`
- `obtcwallet/SECURITY.md`

Challenge:

- wallet expiry status;
- manual renew parameter validation;
- fee limit handling;
- budget limit handling;
- locked wallet behavior;
- auto-renew disabled default and runaway prevention;
- avoiding real Bitcoin private-key use.

## Mining Review Path

Run:

```bash
go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1
go test ./mempool -run 'RejectREAP|REAPRejectsOrphan' -count=1
```

Review:

- [docs/mining-review-checklist.md](docs/mining-review-checklist.md)
- `mining/template_reap.go`
- `mining/newblocktemplate_reap_template_tests_test.go`
- `mining/newblocktemplate_accounting_and_helpers_test.go`
- `mining/reap/`

Challenge:

- REAP template append;
- normal transaction reservation under REAP budget;
- canonical prefix behavior;
- marker count/digest;
- refund outputs;
- tax accounting in coinbase upper bound.

## Security Review Path

Start with [SECURITY_REVIEW_CHECKLIST.md](SECURITY_REVIEW_CHECKLIST.md) and
[SECURITY.md](SECURITY.md).

Known attack scenarios to review:

- forged marker;
- non-expired REAP input;
- reordered REAP inputs;
- oversized REAP input set;
- coinbase overclaim;
- replay attack;
- fake mempool REAP;
- poisoned index/rebuild;
- stale expiry entry after reorg;
- auto-renew runaway.

## Run A Local Demo

Current release branch note:

- Plan 07 local demo files were merged in PR #14:
  <https://github.com/organicbitcoin/obtcd/pull/14>
- The current MC1 source commit includes the local demo script and runbooks. A
  regtest demo run for the earlier candidate tag was captured on 2026-07-01;
  reviewers should rerun the demo locally from the current source commit if
  they need fresh reproduction evidence.

Existing local devnet tools on this branch:

```bash
./scripts/devnet-up.sh start
./scripts/devnet-up.sh demo
./scripts/devnet-up.sh status
./scripts/devnet-up.sh stop
```

For public testnet:

- [docs/testnet-join.md](docs/testnet-join.md)
- [docs/limited-public-testnet-user-test.md](docs/limited-public-testnet-user-test.md)

## Check Parameters

```bash
python3 scripts/check_obtc_parameters.py --format json
```

Expected summary for this draft:

```text
Counter({'exact match': 68})
68
```

## Report Bugs

Public non-sensitive reports:

<https://github.com/organicbitcoin/obtcd/issues>

Use the closest template:

- consensus bug;
- replay protection issue;
- REAP issue;
- expiry index issue;
- wallet renewal issue;
- reproducibility failure;
- documentation confusion.

Include:

- repository and commit;
- network flag;
- exact command;
- expected and observed behavior;
- height, txid, or block hash where relevant;
- redacted logs.

Sensitive reports should use GitHub private vulnerability reporting if enabled.
If not enabled, open a minimal public issue asking for a secure channel and do
not include exploit details or secrets.

## Audit Status

This packet provides public review materials, test vectors, reproducibility
commands, and testnet-validation entry points. It does not record a formal
third-party security audit for the current candidate.

## Source-Only Release Scope

MC1 distributes source references, not project-built binary artifacts. Reviewers
should build from:

- `obtcd` commit `115c9255919f8f266e1b7c7ed2ede8df47087807`;
- `obtcwallet` commit `0bde8d27b8853fd9cf58e0084dba12788a32fab2`.

No project `SHA256SUMS` file is produced for MC1 because no project-built
binary archives are distributed for this scope.

## Out Of Scope

- price;
- exchange listing;
- marketing;
- community growth;
- investment narrative;
- real-fund wallet import or claim support unless a separate reviewed release
  explicitly covers it.
