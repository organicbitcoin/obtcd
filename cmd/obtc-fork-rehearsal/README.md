# obtc-fork-rehearsal

Private rehearsal tool for validating OBTC H+1 header rules against a synced BTC
mainnet database at a chosen fork anchor.

This tool is for rehearsal only. It does not publish or modify chain state.

```bash
go build -o ./obtc-fork-rehearsal ./cmd/obtc-fork-rehearsal

./obtc-fork-rehearsal \
  --dbpath=/var/lib/obtcd/mainnet/blocks_ffldb \
  --dbnet=mainnet \
  --fork-height=<btc_anchor_height> \
  --fork-hash=<btc_anchor_hash> \
  --blocks=48 \
  --out=/mnt/obtc-rehearsal/fork-rehearsal-<height>.json
```

Checks performed:

- Local BTC DB hash at `--fork-height` must exactly match `--fork-hash`.
- H+1 difficulty must reset to `0x1d00ffff`.
- A BTC-style H+1 header that keeps BTC difficulty bits is rejected.
- The tool simulates the requested number of post-fork OBTC headers using
  ForkDAA bootstrap and normal ASERT parameters.

Use this with a cleanly stopped BTC node for the most deterministic DB view.
