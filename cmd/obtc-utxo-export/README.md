# obtc-utxo-export

Private rehearsal tool for exporting the full OBTC expiry-indexed live UTXO set
and generating an offline REAP tax preview.

This command is intended for private mainnet-data rehearsal only. Do not publish
raw `utxo-expiry-snapshot-*.jsonl.gz` files because they contain txid/vout-level
UTXO detail. Public preview material should use the aggregate
`reap-preview-summary-*.json` output.

## RPC Expiry-Index Export

```bash
go build -o ./obtc-utxo-export ./cmd/obtc-utxo-export

./obtc-utxo-export \
  --network=obtcmainnet \
  --source=rpc \
  --rpcserver=127.0.0.1:9528 \
  --rpcuser="$RPC_USER" \
  --rpcpass="$RPC_PASS" \
  --notls \
  --outdir=/mnt/obtc-rehearsal/utxo-export
```

The command scans `listexpiring` from `0` to `snapshot_height + expiry_window`,
using the node's configured batch limit unless `--page-size` is set.

The chain tip is checked before and after export. By default the command fails
after writing the manifest if the tip changes during export. Use
`--allow-moving-tip` only when deliberately collecting a moving-tip sample.

## BTC ffldb Direct Export

Use this mode for the private historical fork rehearsal when the data source is
a synced BTC mainnet node and OBTC `expiryindex` is not enabled.

```bash
./obtc-utxo-export \
  --network=obtcmainnet \
  --source=btcd-db \
  --dbpath=/var/lib/obtcd/mainnet/blocks_ffldb \
  --dbnet=mainnet \
  --fork-height=<btc_snapshot_height> \
  --fork-hash=<btc_snapshot_hash> \
  --outdir=/mnt/obtc-rehearsal/utxo-export
```

Direct export reads the on-disk live UTXO set, skips immature coinbase outputs,
derives OBTC `create_height`, `expiry_height`, and `blocks_to_expiry`, and
writes the same snapshot/manifest format as RPC export.

Safety checks:

- The local DB hash at `--fork-height` must exactly match `--fork-hash`.
- The DB best height/hash must equal the fork anchor.
- The flushed UTXO state hash must match the best hash unless
  `--allow-stale-utxo` is explicitly set.

For a deterministic full snapshot, stop the node cleanly at the rehearsal
anchor height before running direct export.

## Offline Preview

```bash
./obtc-utxo-export \
  --network=obtcmainnet \
  --input=/mnt/obtc-rehearsal/utxo-export/utxo-expiry-snapshot-<height>-<hash>.jsonl.gz \
  --outdir=/mnt/obtc-rehearsal/utxo-export
```

Offline mode does not require RPC credentials. It validates that all input rows
belong to one snapshot before generating preview files.

## Outputs

- `utxo-expiry-snapshot-<height>-<hash>.jsonl.gz`: private raw UTXO rows.
- `utxo-expiry-snapshot-<height>-<hash>.manifest.json`: export counts,
  integrity metrics, duration, page size, and hashes.
- `reap-preview-detail-<height>-<hash>.jsonl.gz`: private per-REAP block input
  details.
- `reap-preview-summary-<height>-<hash>.json`: aggregate preview by height,
  day, and week.

The manifest `sha256` is computed over the uncompressed JSONL content so the
same snapshot can be compared across repeated exports.
