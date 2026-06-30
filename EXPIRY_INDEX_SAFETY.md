# OBTC Expiry Index Safety

This note summarizes the expiry index correctness model and the tests that
exercise connect, disconnect, reconnect, reorg, scan, and rebuild behavior. It
is a reviewer aid, not a protocol specification.

## Data Structure

The expiry index stores live spendable UTXOs in three coordinated views:

| View | Purpose |
|---|---|
| `outpoint -> expiryKey` | Direct lookup used when a spend removes an indexed UTXO. |
| `expiryKey -> outpoint` | RPC-style expiry scans in deterministic expiry/outpoint order. |
| `expiryKey -> amount -> outpoint` | REAP strict candidate prefix order. |

The index also stores a MuHash accumulator root, indexed tip hash, and indexed
tip height. The accumulator covers the live indexed outpoint set by expiry key.
It does not encode amount; the amount-aware REAP ordering is tracked in the
strict candidate bucket.

## Expected Behavior

| Operation | Expected effect |
|---|---|
| `ConnectBlock` | Remove spent indexed inputs, add spendable created outputs, skip genesis-created and unspendable outputs, update root and indexed tip. |
| `DisconnectBlock` | Remove outputs created by the disconnected block, restore indexed spent outputs from the spent journal, roll root and indexed tip back one height. |
| Reconnect | `connect -> disconnect -> reconnect` must produce the same root, scan rows, REAP prefix rows, and tip as the first connect. |
| Reorg | Disconnect inactive branch blocks, then connect active branch blocks. Stale outpoints from the inactive branch must disappear; restored unspent outpoints must reappear. |
| Fast rebuild | Rebuild from the current UTXO set. The rebuilt root, scan rows, REAP prefix rows, tip hash, and tip height must match a live-maintained index for the same active chain. |
| Scan pagination | Bounded scans plus `startAfter` cursor must compose to the same ordered result as one full scan. |

## Coverage Matrix

| Scenario | Test |
|---|---|
| Coinbase and multiple created outputs enter the index with `create height + WindowBlocks`. | `TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan` |
| Connect root, indexed tip, scan rows, expiry key, and REAP strict amount order. | `TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan` |
| Disconnect removes block-created outputs and restores previous root/tip. | `TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan` |
| Reconnect is identical to the first connect, without duplicate entries. | `TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan` |
| Ordinary spent input removal. | `TestExpiryIndexSpendRemoveReapRefundAndRepeatedDelete` |
| REAP-like spend removal, refund output insertion, marker output skip. | `TestExpiryIndexSpendRemoveReapRefundAndRepeatedDelete` |
| Repeated delete of absent outpoint is a no-op and does not change the accumulator snapshot. | `TestExpiryIndexSpendRemoveReapRefundAndRepeatedDelete` |
| A->B reorg drops inactive-branch stale UTXO. | `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` |
| A->B reorg restores UTXO spent only on inactive branch. | `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` |
| Same amount on different branch outpoints does not leak stale branch entries. | `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` |
| Reorg live-maintained state equals fast rebuild state. | `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` |
| Reorg scan pagination equals one full scan. | `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` |
| Fast rebuild uses the current UTXO set and does not fetch historical spend journals. | `TestExpiryIndexFastRebuildUsesUTXOSetWithoutSpendJournals` |
| Existing connect/disconnect mixed-input rollback coverage. | `TestConnectDisconnectBlockRoundTripWithMixedInputs` |
| Existing incremental catch-up recovery coverage. | `TestSetChainAccessorIncrementalCatchUpMatchesLiveIndex` |
| Existing full scan and cursor contract coverage. | `TestScanExpiringUTXOsContract`, `TestScanExpiringUTXOsPaginationAndStartAfter` |
| Existing shadow/private UTXO rebuild coverage. | `TestBuildShadowIndexFromUTXO` |

## How To Run

Focused Plan 04 tests:

```bash
go test ./blockchain/expiryindex -run 'TestExpiryIndex(ConnectDisconnectReconnectRestoresRootAndScan|SpendRemoveReapRefundAndRepeatedDelete|ReorgDropsStaleEntriesAndMatchesFastRebuild|FastRebuildUsesUTXOSetWithoutSpendJournals)' -count=1 -v
```

Expiry index package:

```bash
go test ./blockchain/expiryindex -count=1
```

Broader local validation:

```bash
go test ./blockchain ./blockchain/expiryindex ./mining ./mining/reap -count=1
make unit
go test -p 1 -tags=rpctest ./integration/... -count=1 -v
```

## Known Limits

- The new reorg test drives `ExpiryIndex.ConnectBlock` and
  `ExpiryIndex.DisconnectBlock` directly. It proves index state transitions,
  not full node chain selection.
- The fast rebuild test proves rebuild does not require spent journals or old
  transaction bodies when a current UTXO iterator is available. A full pruned
  node startup/rebuild scenario should still be covered by a later rpctest or
  rehearsal run.
- Expiry commitment validation itself is covered by existing commitment tests.
  This Plan 04 coverage checks that the post-reorg accumulator root matches a
  rebuild of the active UTXO set.

## Manual Review

- Confirm whether external reviewers require a full `ProcessBlock` reorg test
  with the index manager wired in addition to the direct expiry index unit test.
- Confirm whether a pruned-node end-to-end rebuild test should be required
  before Mainnet Candidate, or whether the current UTXO-set rebuild unit test
  plus operational rehearsal is sufficient.
