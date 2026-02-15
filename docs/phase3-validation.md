# Phase 3 Validation (REAP selector & blueprint)

## Implemented

- `mining/reap/selector.go`: deterministic candidate selection from ExpiryIndex scan + UTXO view filtering
- `mining/reap/packer.go`: REAP system transaction blueprint construction
- `mining/reap/params.go`: configurable params and defaults
- `mining/reap/weight.go`: conservative blueprint weight estimate
- `mining/reap/types.go`: public types and constants

## Tests

Added:

- `mining/reap/selector_test.go`
  - deterministic ordering across repeated runs
  - tax rounding invariant check (`tax + refund == input`)
- `mining/reap/packer_test.go`
  - blueprint IO/totals/marker structure checks
  - refund outputs are returned to original script(s), with deterministic grouping
  - missing UTXO error path
- `mining/reap/bench_test.go`
  - benchmark scaffolding for 1k/10k candidates

## Local verification

Commands run locally:

```bash
go test ./mining/reap -v
go test ./...
```

Result: PASS.

## Notes

- Sorting modes implemented: `Strict` and `Simple`.
- Tax computation is per-input floor division.
- Marker output format: `REAP:<height>:<count>:<sha256(inputs)>`.
- Core rule aligned with OBTC spec: 70% refund to original locking script, 30% tax.
