# obtc-expiryindex-shadow

Private rehearsal tool for building and benchmarking an OBTC expiry index from a
synced BTC mainnet UTXO set without writing to the BTC blocks database.

```bash
go build -o ./obtc-expiryindex-shadow ./cmd/obtc-expiryindex-shadow

./obtc-expiryindex-shadow \
  --source-dbpath=/var/lib/obtcd/mainnet/blocks_ffldb \
  --index-dbpath=/mnt/obtc-rehearsal/expiryindex-shadow/index_ffldb \
  --dbnet=mainnet \
  --fork-height=<btc_snapshot_height> \
  --fork-hash=<btc_snapshot_hash> \
  --reset-index \
  --batch-size=5000 \
  --out=/mnt/obtc-rehearsal/expiryindex-shadow/expiryindex-shadow-<height>.json
```

The tool:

- validates the local BTC anchor hash,
- reads the BTC UTXO set directly,
- writes the production expiryindex bucket/key structure into an independent
  shadow DB,
- reports build rate, indexed count, bucket stats, disk size, and query latency.

The JSON result is safe to copy into control-plane. The shadow index DB itself
is private rehearsal data and should stay on the AWS node or private storage.
