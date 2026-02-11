# Week 3.1 Validation (Kickoff)

## Scope in this phase

- Added marker digest helper and fixed test vector:
  - `mining/reap/marker.go`
  - `mining/reap/marker_vector_test.go`
- Refactored blueprint marker construction to use shared digest helper.
- Added network-aware REAP parameter defaults + validation:
  - `DefaultREAPParamsForNet(...)`
  - `REAPParams.Validate()`
- Added dry-run summary helper for auditable output fields:
  - `mining/reap/dryrun.go`
- Added/extended tests:
  - `mining/reap/params_test.go`
  - `mining/reap/dryrun_test.go`
  - `mining/reap/selector_test.go` (MaxInputs/WeightBudget boundary + integration-style filtering/paging path)

## Validation commands

```bash
go test ./mining/reap -v
go test ./...
```

## Next steps

- expose `BuildDryRunSummary` via command/RPC output (`picked/tax/burn/estWeight/markerHash`)
- add a real chain-backed integration test once Week4 hooks are available
- document marker serialization spec in developer docs
