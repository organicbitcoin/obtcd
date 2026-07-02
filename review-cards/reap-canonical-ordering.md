# Review Card: REAP Canonical Ordering

## Mechanism Summary

REAP does not let a miner pick an arbitrary subset from the expired set. After
hardening, a REAP transaction must spend a canonical global prefix of expired
UTXOs. Candidate order is based on expiry key, amount, and OutPoint.

## Core Invariant

For a given chain tip and expiry index state, all honest nodes must agree on the
same ordered prefix of eligible REAP inputs, and a block must be rejected if it
cherry-picks a non-prefix candidate.

## Code Location

- `blockchain/validation_reap.go`: `checkReapGlobalPrefix` and input ordering
  validation.
- `blockchain/expiryindex/expiryindex.go`: `ReapPrefixCandidates`.
- `blockchain/expiryindex/encode.go`: ordered key encoding.
- `mining/reap/selector.go`: mining-side strict ordering.
- `mining/template_reap.go`: template use of prefix candidates.

## Test Location

- `blockchain/fullblocks_obtc_test.go`
- `blockchain/validation_reap_test.go`
- `blockchain/expiryindex/reap_prefix_test.go`
- `blockchain/expiryindex/scan_staircase_test.go`
- `mining/reap/selector_test.go`
- `mining/reap/staircase_pressure_test.go`
- `mining/reap/stress_regression_test.go`

## How To Run Tests

```bash
go test ./blockchain -run 'Prefix|Canonical|OBTCFullBlock' -count=1
go test ./blockchain/expiryindex -run 'ReapPrefix|ScanExpiring|Ordering' -count=1
go test ./mining/reap -run 'SelectCandidates|SortCandidates|Staircase|LongRun' -count=1
```

## What To Challenge

- Same-expiry tie breaks by amount and OutPoint.
- Scanner pagination preserving deterministic order.
- Reversed or shuffled source order producing the same strict selection.
- Blocks with a valid expired input set that is not the global prefix.
- Prefix-source tip mismatch during block validation.

## Known Limitations

- Stress tests are deterministic local simulations and not a formal proof of
  all possible backlog distributions.
- The card assumes the expiry index itself is correct; pair with the expiry
  index and reorg card for stale-entry challenges.
- No formal third-party security audit is recorded for this mechanism.
