# OBTC Expiry Index Reorg And Rebuild Test Report

Plan 04 added targeted expiry index safety tests without changing consensus
rules, network parameters, expiry formulas, REAP selection semantics, or
commitment semantics.

## Added Tests

| Test | File | Purpose |
|---|---|---|
| `TestExpiryIndexConnectDisconnectReconnectRestoresRootAndScan` | `blockchain/expiryindex/reorg_safety_test.go` | Proves created UTXOs enter the index, coinbase is indexed when spendable, expiry key uses `create height + WindowBlocks`, disconnect restores root/tip, and reconnect is identical to the first connect. |
| `TestExpiryIndexSpendRemoveReapRefundAndRepeatedDelete` | `blockchain/expiryindex/reorg_safety_test.go` | Proves spent UTXOs are removed, REAP-like refund outputs are indexed with a new expiry height, marker outputs are skipped as unspendable, and repeated deletion of an absent outpoint is a no-op. |
| `TestExpiryIndexReorgDropsStaleEntriesAndMatchesFastRebuild` | `blockchain/expiryindex/reorg_safety_test.go` | Builds branch A and branch B at the same height, disconnects A, connects B, asserts stale A entries are gone, restored active UTXOs are present, paged scan equals full scan, and live state equals fast rebuild state. |
| `TestExpiryIndexFastRebuildUsesUTXOSetWithoutSpendJournals` | `blockchain/expiryindex/reorg_safety_test.go` | Proves fast rebuild consumes the current UTXO set and tip block hash without requesting historical spend journals. |

## Reorg Construction

The reorg test constructs:

1. A base block at height 1 creating `baseOut`.
2. Branch A block at height 2 spending `baseOut` and creating `staleAOut`.
3. A disconnect of branch A, which restores `baseOut`.
4. Branch B block at height 2 creating `activeBOut` with the same amount as
   the branch A output but a different outpoint.

After branch B connects, the scan must contain `baseOut` and `activeBOut`, and
must not contain `staleAOut`.

## Rebuild Consistency

The reorg test then builds a second expiry index from a mock current UTXO set
containing only the active-chain UTXOs:

- `baseOut`, create height 1, amount 1000.
- `activeBOut`, create height 2, amount 777.

The rebuilt index is compared against the live-maintained index for:

- accumulator root;
- indexed tip hash;
- indexed tip height;
- stats;
- full expiry scan order;
- REAP strict candidate order and amounts.

## Scan Determinism

After the A->B reorg, the test performs a full scan and a paged scan with
`maxResults=1`. The concatenated paged result must exactly match the full
scan. This guards cursor handling and deterministic order after rollback and
branch replacement.

## Results

No stale entry, missing restored entry, root mismatch, tip mismatch, scan order
drift, or rebuild mismatch was found by the added tests.

No implementation bug was confirmed.

## Remaining Scenarios

| Area | Status | Recommended follow-up |
|---|---|---|
| Full node chain-selection reorg | Not covered by the new unit test. | Add an rpctest or `ProcessBlock` integration test with the index manager enabled if reviewers want end-to-end chain selection evidence. |
| Pruned node startup/rebuild | Partially covered. Fast rebuild is proven not to fetch spend journals, but a real pruned database startup is not automated here. | Add a long-running pruned-node rehearsal or rpctest once the test harness can cheaply create pruned OBTC data. |
| Archive node mode | Covered at the index transition level. | Keep as operational smoke evidence for release candidates. |
| Expiry commitment in full block processing during reorg | Root consistency is covered by live-vs-rebuild comparison; commitment validation is covered by existing unit tests. | Add full block/rpctest coverage if external reviewers require commitment root validation during an actual submitted reorg. |
