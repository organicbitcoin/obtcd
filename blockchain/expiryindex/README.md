expiryindex
===========

Package expiryindex maintains OBTC UTXO expiry state.

It is used by:

- RPC scans such as `getexpiryindexstats`
- REAP candidate selection for mining templates
- expiry commitment accumulator snapshots
- `--reindex-expiry` rebuilds from chain state

The package stores UTXOs in expiry-height buckets, supports deterministic
pagination, and maintains a MuHash accumulator so blocks can commit to the
expected expiry state once the network activation height requires it.

## Operational Notes

`--expiryindex` enables scan and observability RPC features. Expiry commitment
consensus state is maintained independently by the node where required by the
active OBTC network parameters.

## License

Package expiryindex is licensed under the [copyfree](http://copyfree.org) ISC
License.
