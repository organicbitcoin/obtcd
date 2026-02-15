# REAP Consensus & Template Integration Validation

## Scope
- REAP marker validation in consensus input checks
- Expiry spend rules:
  - non-REAP tx cannot spend expired UTXOs
  - REAP tx must spend expired UTXOs only
- Mining template wiring:
  - template generator can build and include REAP system tx when expiry index is wired

## Implemented checks
- `blockchain/validation_reap.go`
  - marker payload parse/validation (`REAP:<height>:<count>:<digest>`)
  - digest verification from ordered tx inputs
  - expiry spend enforcement based on `chaincfg.GetExpiryParams(...).EnableAtHeight`
- `blockchain/validate.go`
  - `CheckTransactionInputs` now calls REAP marker + expiry spend checks
- `blockchain/scriptval.go`
  - skips script execution for REAP system tx (validated via dedicated path)
- `mining/template_reap.go`
  - build REAP tx from expiry index + reap selector/blueprint
- `mining/mining.go`
  - NewBlockTemplate attempts REAP inclusion and accounts REAP tax as fees
- `server.go`
  - wires expiry index into block template generator (`SetREAPIndex`)

## Tests
- `blockchain/validation_reap_test.go`
  - non-REAP expired spend rejected
  - REAP non-expired spend rejected
  - REAP marker digest mismatch rejected

## Command
```bash
go test ./...
```

## Result
- PASS on all packages.

## Failure Cases -> Expected Errors

| Case | Trigger | Expected error keyword |
|---|---|---|
| Non-REAP tx spends expired UTXO | regular tx input points to expired output | `expired utxo` |
| REAP tx spends non-expired UTXO | REAP marker tx input not yet expired | `non-expired utxo` |
| REAP marker digest mismatch | marker digest does not match ordered inputs | `digest mismatch` |
| REAP marker count mismatch | marker count != tx input count | `count mismatch` |
| REAP marker payload malformed | bad prefix/height/count/digest format | `invalid REAP marker` |
| Template collect path with broken index state | expiry index buckets missing/uninitialized | scan/collect error from expiry index |

Notes:
- These failure paths are covered by direct unit tests in `blockchain/validation_reap_test.go` and `mining/template_reap_test.go`.
- For template path failures, REAP tx is skipped and normal template assembly continues.
