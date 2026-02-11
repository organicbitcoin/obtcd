# Week 3.1 Validation (Kickoff)

## Scope in this kickoff commit

- Added marker digest helper and fixed test vector:
  - `mining/reap/marker.go`
  - `mining/reap/marker_vector_test.go`
- Refactored blueprint marker construction to use shared digest helper.
- Added network-aware REAP parameter defaults + validation:
  - `DefaultREAPParamsForNet(...)`
  - `REAPParams.Validate()`
- Added tests:
  - `mining/reap/params_test.go`

## Validation commands

```bash
go test ./mining/reap -v
go test ./...
```

## Next steps (not started in kickoff)

- dry-run command/RPC output (`picked/tax/burn/estWeight/markerHash`)
- stronger integration tests for scanner + realistic view fetch path
- documentation of marker serialization in developer docs
