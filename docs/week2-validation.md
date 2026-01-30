# Week2 Validation Record (ExpiryIndex)

## Date
- 2026-01-30

## Environment
- Host: local
- Network: obtcregtest
- RPC: localhost:18667
- RPC Auth: test/test
- ExpiryIndex: enabled

## Unit Tests
Command:
```
go test ./...
```
Result:
- All packages passed.

## Regtest Validation (Quick)
Start node:
```
./obtcd --obtcregtest --expiryindex --rpcuser=test --rpcpass=test --notls
```
Run validation:
```
./scripts/validation/quick_validate.sh obtcregtest --rpcuser=test --rpcpass=test
```
Summary:
- connectivity ✅
- index_availability ✅
- basic_query ✅
- parameter_validation ✅
- pagination ✅
- edge_cases ✅

Notes:
- All 6 tests passed; success rate 100%.

