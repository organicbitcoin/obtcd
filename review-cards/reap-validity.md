# Review Card: REAP Validity

## Mechanism Summary

REAP is the OBTC path for processing expired UTXOs. A REAP transaction is a
block-internal system transaction with constrained inputs, tax/refund
accounting, marker data, input caps, dust handling, and weight limits.

## Core Invariant

A REAP transaction is valid only when every input is a confirmed expired UTXO,
the marker matches the transaction inputs, the inputs satisfy caps and weight
limits, and the outputs match deterministic refund/tax rules.

## Code Location

- `blockchain/validation_reap.go`: consensus REAP checks.
- `mining/reap/selector.go`: mining-side candidate selection.
- `mining/reap/packer.go`: REAP transaction construction.
- `mining/reap/budget.go`: budgeted blueprint construction.
- `mempool/reap_policy_test.go` and `mempool/policy.go`: fake REAP isolation.

## Test Location

- `blockchain/validation_reap_test.go`
- `blockchain/validation_reap_extra_test.go`
- `blockchain/consensus_obtc_edge_test.go`
- `blockchain/fullblocks_obtc_test.go`
- `mining/reap/selector_test.go`
- `mining/reap/budget_test.go`
- `mempool/reap_policy_test.go`

## How To Run Tests

```bash
go test ./blockchain -run 'REAP|OBTCFullBlock|Consensus' -count=1
go test ./mining/reap -run 'Select|Budget|Blueprint|Dust|Tax|Weight' -count=1
go test ./mempool -run 'REAP' -count=1
```

## What To Challenge

- REAP spending a non-expired UTXO.
- Ordinary transaction spending an expired UTXO.
- Input count caps for normal and dust tiers.
- Transaction weight limit boundaries.
- Refund grouping by script and exact tax/refund totals.
- Mempool paths accepting fake REAP-like transactions.

## Known Limitations

- The local tests exercise known edge cases, not an exhaustive formal model.
- Long-running testnet observation is still needed for operational confidence
  under real backlog and miner behavior.
- No formal third-party security audit is recorded for this mechanism.
