# Mining REAP Template Tests

This note maps the mining, block template, and REAP construction test coverage.
The goal is to keep REAP observable as a deterministic block-internal system
transaction, not a mempool-selected user transaction.

## Primary Template Coverage

- `mining/newblocktemplate_reap_template_tests_test.go`
  - `TestNewBlockTemplateREAPNotAppendedWithoutExpiredCandidates`
    verifies that a template with indexed but unexpired UTXOs remains
    coinbase-only and does not append REAP.
  - `TestNewBlockTemplateREAPAppendStructureRefundsAndAccounting`
    verifies REAP append, version, locktime, input sequence, marker
    height/count/digest, refund output grouping, tax fee accounting, and that
    unexpired candidate outputs are not selected.
  - `TestNewBlockTemplateREAPUsesCanonicalPrefixAndIsStable`
    verifies large-backlog prefix selection against
    `ExpiryIndex.ReapPrefixCandidates`, normal input cap behavior, weight
    budget adherence, marker integrity after truncation, and repeated template
    stability.

- `mining/newblocktemplate_reap_boundary_test.go`
  - Verifies that regular mempool transactions use only the normal transaction
    weight region when a REAP transaction is planned.
  - Verifies that REAP can occupy the reserved region without unboundedly
    displacing regular transactions.

- `mining/newblocktemplate_accounting_and_helpers_test.go`
  - Verifies fee vector consistency and coinbase value accounting with and
    without REAP.

## REAP Selection And Blueprint Coverage

- `mining/reap/selector_test.go`
  - Covers deterministic selection.
  - Covers expiry, amount, and outpoint ordering through strict sort mode.
  - Covers max input truncation, dust cap truncation, and prefix-only tail
    truncation.
  - Covers missing/spent UTXO filtering and canceled context behavior.

- `blockchain/expiryindex/reap_prefix_test.go`
  - Covers persisted global prefix order by expiry, amount, and outpoint.
  - Covers prefix limits and removal behavior.

- `mining/reap/budget_test.go`
  - Covers soft weight budget trimming, hard block weight trimming, single
    over-budget rejection, missing UTXOs, and plan stat normalization.

- `mining/reap/stress_regression_test.go`
  - Covers thousands of candidates, repeated deterministic selection, scanner
    order independence, concurrent selection, and multi-round backlog
    processing.

- `mining/reap/staircase_pressure_test.go`
  - Covers larger staircase backlog pressure, stale/spent filtering, cap and
    order invariants, and repeated deterministic results.

## Mempool Isolation Coverage

- `mempool/reap_policy_test.go`
  - Verifies that a likely REAP system transaction is rejected from mempool.

- `mempool/reap_policy_extra_test.go`
  - Verifies version and marker shape boundaries.
  - Verifies fake marker payloads, digest-mismatch shapes, and invalid marker
    text do not enter the mempool or orphan pool.

- `mempool/policy_matrix_test.go`
  - Verifies REAP-like transactions are rejected before orphan handling.

## Coinbase Accounting Coverage

- `mining/newblocktemplate_accounting_and_helpers_test.go`
  - Verifies template coinbase value equals subsidy plus non-coinbase fees,
    including REAP tax as the REAP transaction fee.

- `mining/newblocktemplate_reap_template_tests_test.go`
  - Verifies refund outputs are not counted as miner tax and coinbase includes
    only the computed tax total.

- `blockchain/consensus_obtc_edge_test.go`
  - `TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim` verifies consensus
    rejects coinbase overclaim when a block includes a valid REAP transaction.

## Suggested Local Commands

```sh
go test ./mining -run 'REAP|Accounting|Boundary|Template' -count=1
go test ./mining/reap -run 'Select|Budget|Blueprint|Pressure|Stress' -count=1
go test ./mempool -run 'REAP|FakeMarkers' -count=1
go test ./blockchain ./blockchain/expiryindex -run 'REAP|ReapPrefix|CoinbaseOverclaim' -count=1
```

The lightweight backlog pressure coverage is implemented as deterministic Go
tests instead of a shell-driven regtest script. This keeps timing, memory use,
and chain state bounded while still exercising the template and selector paths
that miners depend on.
