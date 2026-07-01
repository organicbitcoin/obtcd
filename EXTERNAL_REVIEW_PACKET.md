# OBTC Mainnet Candidate External Review Packet

This packet is the starting point for external technical review of
`mainnet-candidate-2026-07`. It is not a production launch announcement, not
investment material, and not an exchange listing document.

## What This Packet Is

This is a map for reviewers who want to evaluate:

- consensus and activation boundaries;
- expiry index behavior;
- REAP validity and mining template behavior;
- replay protection;
- expiry commitment validation;
- wallet expiry and renewal safety;
- operator procedures and known limitations.

## Read First

1. [MAINNET_CANDIDATE_RELEASE_NOTES.md](MAINNET_CANDIDATE_RELEASE_NOTES.md)
2. [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md)
3. [MAINNET_CANDIDATE_TEST_REPORT.md](MAINNET_CANDIDATE_TEST_REPORT.md)
4. [docs/mainnet-params.md](docs/mainnet-params.md)
5. [SECURITY_REVIEW_CHECKLIST.md](SECURITY_REVIEW_CHECKLIST.md)
6. [NODE_OPERATOR_RUNBOOK.md](NODE_OPERATOR_RUNBOOK.md)
7. [WALLET_OPERATOR_RUNBOOK.md](WALLET_OPERATOR_RUNBOOK.md)

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

- [docs/mainnet-params.md](docs/mainnet-params.md)
- [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md)

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

## Wallet Review Path

Use the companion repository:

```bash
git clone https://github.com/organicbitcoin/obtcwallet.git
cd obtcwallet
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1
```

Review:

- `README.md`
- `WALLET_RENEWAL_RUNBOOK.md`
- `WALLET_LIFECYCLE_TESTS.md`
- `AUTO_RENEW_SAFETY_NOTES.md`
- `SECURITY.md`

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
- Final tag `mainnet-candidate-2026-07` includes the local demo script and
  runbooks. A final-tag regtest demo run was captured on 2026-07-01.

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

## Out Of Scope

- price;
- exchange listing;
- marketing;
- community growth;
- investment narrative;
- real-fund wallet import or claim support unless a separate reviewed release
  explicitly covers it.
