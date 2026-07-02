# Review Card: Historical Backlog

## Mechanism Summary

If many UTXOs expire at or near the same height, REAP cannot necessarily process
the entire backlog in one block. The expiry index, preview tools, selector, and
template path must carry the backlog forward deterministically.

## Core Invariant

Given a large expired backlog, each block may process only the allowed
canonical prefix, and unprocessed expired UTXOs must remain indexed for later
blocks without reordering, dropping, or duplicating candidates.

## Code Location

- `blockchain/expiryindex/expiryindex.go`: paged expiry scans and prefix
  candidate queries.
- `blockchain/expiryindex/encode.go`: key ordering.
- `mining/reap/selector.go`: tier caps, weight budget, and prefix selection.
- `mining/template_reap.go`: REAP candidate collection for templates.
- `cmd/obtc-utxo-export/preview.go`: preview and backlog reporting.

## Test Location

- `blockchain/expiryindex/scan_staircase_test.go`
- `blockchain/expiryindex/pressure_theoretical_max_test.go`
- `blockchain/expiryindex/reap_prefix_test.go`
- `mining/reap/staircase_pressure_test.go`
- `mining/reap/stress_regression_test.go`
- `cmd/obtc-utxo-export/preview_test.go`

## How To Run Tests

```bash
go test ./blockchain/expiryindex -run 'Staircase|Pressure|ReapPrefix' -count=1
go test ./mining/reap -run 'Staircase|LongRun|SelectCandidates' -count=1
go test ./cmd/obtc-utxo-export -run 'Backlog|Preview|Aggregate' -count=1
```

## What To Challenge

- Very large same-expiry-key sets.
- Pagination boundaries and `startAfter` cursor behavior.
- Backlog carry-over after a block processes only part of the prefix.
- Selector behavior when the weight budget or tier cap stops selection.
- Preview output matching the actual selector and consensus limits.
- Reorgs that remove or replace part of a historical backlog.

## Known Limitations

- Local synthetic pressure tests do not replace long-running public testnet
  validation with real node restarts and reorgs.
- Preview tooling is an aid, not consensus.
- No formal third-party security audit is recorded for this mechanism.
