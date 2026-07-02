# Review Card: Coinbase Accounting

## Mechanism Summary

The coinbase transaction may claim block subsidy, normal transaction fees, and
valid REAP tax. REAP refunds remain transaction outputs and must not be counted
as miner income. Expiry commitments also appear in coinbase data when mandatory.

## Core Invariant

A block must be rejected if coinbase claims more than subsidy plus normal fees
plus valid REAP tax, and block templates must expose fee accounting that sums
consistently across normal and REAP transactions.

## Code Location

- `blockchain/validation_reap.go`: REAP tax and refund checks.
- `blockchain/validate.go`: block connection and coinbase amount validation
  paths.
- `mining/template_reap.go`: REAP tax returned to block template construction.
- `mining/mining.go`: template coinbase assembly and fee accounting in
  `NewBlockTemplate`.
- `blockchain/expiryindex/commitment.go`: coinbase expiry commitment script.

## Test Location

- `mining/newblocktemplate_accounting_and_helpers_test.go`
- `mining/newblocktemplate_reap_template_tests_test.go`
- `blockchain/consensus_obtc_edge_test.go`
- `blockchain/validation_reap_test.go`
- `blockchain/expiryindex/commitment_test.go`

## How To Run Tests

```bash
go test ./mining -run 'Accounting|REAPAppendStructure|Template' -count=1
go test ./blockchain -run 'Coinbase|REAPTax|Consensus' -count=1
go test ./blockchain/expiryindex -run 'Commitment' -count=1
```

## What To Challenge

- Coinbase overclaim by one satoshi.
- Counting REAP refund outputs as miner fees.
- Negative, zero, and dust-folded REAP totals.
- Template fee array consistency with transaction order.
- Missing, duplicated, malformed, or wrong-root expiry commitment outputs.

## Known Limitations

- The tests verify local template and consensus cases; they do not review third
  party pool accounting software.
- Coinbase commitment review should be paired with expiry-index reorg review.
- No formal third-party security audit is recorded for this mechanism.
