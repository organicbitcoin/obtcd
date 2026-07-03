# OBTC Mainnet 72h REAP-Active Rehearsal

This document describes the private `obtcmainnet72h` rehearsal network. It is
not the official OBTC mainnet and must not be advertised as a production
network.

## Network

| Field | Value |
|---|---:|
| Flag | `--obtcmainnet72h` |
| Network name | `obtcmainnet72h` |
| P2P port | `39527` |
| RPC port | `39528` |
| P2P magic | `0x4f483732` |
| DNS seeds | none |

The network uses isolated address and HD namespaces so rehearsal wallets and
addresses do not look like official OBTC mainnet wallets or addresses.

## Rehearsal Parameters

| Field | Value |
|---|---:|
| BTC fork height | `956542` |
| BTC fork hash | `0000000000000000000200bad2d8d62a198f06b4390e7ca9be8f15581b42102e` |
| First OBTC block | `956543` |
| Replay protection | `956543` |
| Expiry / REAP / commitment activation | `956566` |
| Expiry window | `362880` |
| REAP normal max inputs | `256` |
| REAP dust max inputs | `1024` |
| REAP max weight | `400000` |
| REAP tax | `30%` |
| REAP dust threshold | `720 sat` |

Before deployment, re-check the anchor against local BTC history and public
APIs:

```bash
scripts/phase6/verify_72h_fork_anchor.sh \
  --height 956542 \
  --hash 0000000000000000000200bad2d8d62a198f06b4390e7ca9be8f15581b42102e \
  --bitcoin-cli /path/to/bitcoin-cli
```

## Evidence

Use a run ID in this format:

```text
mainnet72h-reap-<forkheight>-<YYYYMMDDTHHMMSSZ>
```

Raw artifacts should be stored privately:

```text
s3://obtc-private-rehearsal-artifacts/mainnet-72h-reap-active/<run_id>/
```

Generate a redacted manifest for control-plane:

```bash
scripts/phase6/generate_72h_rehearsal_manifest.sh \
  --run-id mainnet72h-reap-956542-<timestamp> \
  --raw-artifact-uri s3://obtc-private-rehearsal-artifacts/mainnet-72h-reap-active/<run_id>/ \
  --start-utc <start> \
  --end-utc <end> \
  --node 'seed-1|observer-miner|aws|p2p-private|rpc-private' \
  --out /tmp/manifest.redacted.json
```

Run the 72h collector:

```bash
scripts/phase6/collect_72h_observation.sh \
  --network obtcmainnet72h \
  --notls \
  --rpcuser <rpc_user> \
  --rpcpass <rpc_pass> \
  --duration-hours 72 \
  --interval-hours 1 \
  --new-file \
  --out /tmp/obtc-mainnet72h-72h.md
```

Do not commit raw node logs, RPC credentials, private keys, raw UTXO rows, or
secret endpoints to a public repository.
