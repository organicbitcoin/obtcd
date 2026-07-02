# Review Card: Replay Protection

## Mechanism Summary

OBTC activates an OBTC-specific signature hash domain after the fork boundary.
Post-activation signatures must use the OBTC replay-protected hash type so the
same transaction signature cannot be replayed as an ordinary Bitcoin spend.

## Core Invariant

After replay activation on an OBTC network, ordinary transaction validation and
mempool admission must reject signatures that do not use the OBTC replay domain.
Non-OBTC networks must not enable this rule.

## Code Location

- `chaincfg/params_obtc.go`: `GetOBTCReplayProtectionHeight` and
  `IsOBTCReplayProtectionActive`.
- `blockchain/validation_obtc_replay.go`: script flag activation.
- `txscript/sighash.go`: OBTC replay hash type and domain tag.
- `txscript/engine.go`: enforcement through script flags.
- `mempool/policy.go` and `mempool/policy_matrix_test.go`: mempool activation
  consistency.

## Test Location

- `blockchain/validation_obtc_replay_test.go`
- `txscript/sighash_obtc_replay_test.go`
- `txscript/taproot_obtc_replay_test.go`
- `mempool/policy_matrix_test.go`
- `integration/obtc_integration_test.go`

## How To Run Tests

```bash
go test ./chaincfg -run 'OBTCReplay|ReplayProtection' -count=1
go test ./blockchain -run 'Replay' -count=1
go test ./txscript -run 'OBTCReplay|ReplayProtection' -count=1
go test ./mempool -run 'ReplayProtection' -count=1
```

## What To Challenge

- Activation height off-by-one behavior.
- Whether Bitcoin, simnet, or non-OBTC params accidentally inherit the OBTC
  script flag.
- Legacy, SegWit v0, and Taproot coverage symmetry.
- Mempool and block validation disagreement at the activation boundary.
- Wallet or test wallet signing paths that might still create non-protected
  signatures after activation.

## Known Limitations

- The tests are focused on deterministic local cases and integration harnesses.
- This card does not prove every external wallet signs with the OBTC replay
  domain.
- No formal third-party security audit is recorded for this mechanism.
