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
