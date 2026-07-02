# Review Card: Expiry Index And Reorg

## Mechanism Summary

The expiry index stores live UTXOs by expiry key so nodes and miners can scan
expired outputs deterministically. It also maintains an accumulator root used by
the expiry commitment in coinbase data. Reorg handling disconnects old-chain
effects and connects new-chain effects.

## Core Invariant

After any connect, disconnect, reorg, restart, or rebuild, the expiry index must
match the active chain's live UTXO set and must not expose stale REAP
candidates.

## Code Location

- `blockchain/expiryindex/expiryindex.go`: index lifecycle, connect,
  disconnect, scan, commitment validation.
- `blockchain/expiryindex/buckets.go`: persisted bucket layout.
- `blockchain/expiryindex/encode.go`: OutPoint and expiry-key encodings.
- `blockchain/expiryindex/accumulator.go`: commitment accumulator.
- `blockchain/expiryindex/reindex.go`: rebuild path.
- `blockchain/expiry_chain_accessor.go`: chain accessor bridge.

## Test Location

- `blockchain/expiryindex/reorg_safety_test.go`
- `blockchain/expiryindex/rebuild_test.go`
- `blockchain/expiryindex/reindex_test.go`
- `blockchain/expiryindex/recovery_integration_test.go`
- `blockchain/expiryindex/sequence_fuzz_test.go`
- `blockchain/expiryindex/reap_prefix_test.go`

## How To Run Tests

```bash
go test ./blockchain/expiryindex -run 'Reorg|Rebuild|Recovery|Sequence|ReapPrefix|ConnectDisconnect' -count=1
go test ./blockchain/expiryindex -run 'TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild' -count=1 -v
```

## What To Challenge

- Disconnect ordering for spent outputs and created outputs.
- Reorgs where a REAP spend exists on one branch but not the other.
- Duplicate expiry keys with many OutPoints.
- Rebuild behavior when spend journals are unavailable.
- Whether commitment roots are validated against the correct pre-state.

## Known Limitations

- The reorg tests are synthetic and local; they do not replace long-running
  public testnet observation.
- Pruned-node and snapshot-restore support are not claimed in the current
  review packet.
- No formal third-party security audit is recorded for this mechanism.
