# Review Card: REAP Marker

## Mechanism Summary

A REAP transaction is identified by transaction version and a final zero-value
`OP_RETURN` marker output. The marker payload records the block height, input
count, and digest of the ordered REAP inputs.

## Core Invariant

The marker must be present at the expected position, parse cleanly, match the
block height, match the input count, and match the digest recomputed from the
transaction inputs.

## Code Location

- `blockchain/validation_reap.go`: `isLikelyReapTx`, marker parsing, digest
  recomputation, and `checkReapMarker`.
- `mining/reap/reaptx.go`: mining-side REAP identification helpers.
- `mining/reap/marker.go`: marker digest helper.
- `mining/reap/packer.go`: marker output construction.

## Test Location

- `blockchain/validation_reap_test.go`
- `blockchain/validation_reap_extra_test.go`
- `blockchain/consensus_obtc_edge_test.go`
- `mining/reap/marker_vector_test.go`
- `mining/reap/packer_test.go`
- `mining/reap/reaptx_test.go`

## How To Run Tests

```bash
go test ./blockchain -run 'Marker|REAPMarker|ReapMarker' -count=1
go test ./mining/reap -run 'Marker|REAPTx|Blueprint' -count=1
```

## What To Challenge

- Digest mismatch with otherwise valid inputs.
- Count mismatch and height mismatch.
- Marker output not last.
- Non-zero marker output value.
- Non-REAP transactions that contain marker-like data.
- Nil or empty input marker determinism in helper code.

## Known Limitations

- The marker identifies REAP shape; final validity also depends on UTXO view,
  expiry, order, caps, and accounting.
- The marker vector is compact and should be extended if the marker encoding
  changes.
- No formal third-party security audit is recorded for this mechanism.
