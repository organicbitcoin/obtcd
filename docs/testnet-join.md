# OBTC Testnet Join Guide

> Status: Phase 6 bootstrap guide (implementation-aligned)

This guide explains how to run and validate an **OBTC testnet** node using the current repository state.

## 1. Prerequisites

- Go toolchain installed
- Build artifacts available in this repo
- A machine that can expose TCP `19527` for inbound P2P (optional but recommended for seed candidates)

Build binaries from repo root:

```bash
go build ./...
```

## 2. Network basics (current code baseline)

- Network flag: `--obtctestnet`
- Network name: `obtctestnet`
- Default P2P port: `19527`
- Recommended RPC port for operators: `19528`
- Bech32 HRP: `obtct`

## 3. Start a node

### Option A (recommended): phase6 helper script

```bash
# from repo root
scripts/phase6/run_testnet_node.sh start
scripts/phase6/run_testnet_node.sh status
```

With custom peers and credentials:

```bash
RPC_USER=myuser RPC_PASS=mypass \
ADDPEERS=seed1.example.com:19527,seed2.example.com:19527 \
scripts/phase6/run_testnet_node.sh start
```

### Option B: raw btcd command

```bash
./btcd \
  --obtctestnet \
  --datadir=$HOME/.obtcd-testnet \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --txindex \
  --expiryindex \
  --notls \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --addpeer=<peer1:19527> \
  --addpeer=<peer2:19527>
```

### Option C: generate a config file first

```bash
scripts/phase6/gen_testnet_conf.sh \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --addpeers=<peer1:19527>,<peer2:19527>

# then start node with generated config
./btcd --configfile=./phase6-obtctestnet.conf
```

## 4. Validate node health

Run smoke checks:

```bash
scripts/validation/testnet_smoke.sh \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --rpcserver=127.0.0.1:19528
```

Manual checks:

```bash
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 getblockchaininfo
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 getpeerinfo
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 getchaintips
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 getmempoolinfo
```

If `--expiryindex` is enabled:

```bash
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 getexpiryindexstats
./cmd/btcctl/btcctl --obtctestnet --rpcuser=<u> --rpcpass=<p> --rpcserver=127.0.0.1:19528 listexpiring 0 99999999 100
```

## 5. Collect validation evidence (recommended)

Append a markdown snapshot into a local validation record:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --rpcuser=<u> \
  --rpcpass=<p> \
  --rpcserver=127.0.0.1:19528 \
  --append /tmp/obtc-phase6-validation.md
```

Or write to a standalone file:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --rpcuser=<u> \
  --rpcpass=<p> \
  --out /tmp/obtc-phase6-validation-snapshot.md
```

## 6. Seed/peering notes (Phase 6)

- Seed readiness requires **2-3 long-lived nodes** (preferably multi-region).
- At this stage, bootstrap should rely on explicit `--addpeer` entries until seed rollout is finalized.
- Keep at least one observability node with `--expiryindex` enabled.

Run a preflight check before accepting a seed candidate:

```bash
scripts/phase6/seed_preflight.sh \
  --rpcuser=<u> \
  --rpcpass=<p> \
  --rpcserver=127.0.0.1:19528 \
  --min-peers=1
```

For observer nodes that must expose expiry RPC:

```bash
scripts/phase6/seed_preflight.sh \
  --rpcuser=<u> \
  --rpcpass=<p> \
  --strict-expiryindex
```

## 7. Common issues

### No peers

- Confirm port `19527` is reachable from Internet/VPN peers.
- Add explicit peers via `--addpeer`.
- Check firewall/security-group inbound and outbound rules.

### RPC works but expiry RPC fails

`getexpiryindexstats`/`listexpiring` may fail if node was started without `--expiryindex`.

### Wrong chain/network

If height/peers look wrong, verify you actually started with `--obtctestnet`.

## 8. Minimal operator security baseline

- Keep RPC bound to localhost (`127.0.0.1`).
- Use strong, unique `rpcuser`/`rpcpass`.
- Do not expose RPC port publicly unless behind strict access controls.
- Rotate logs and monitor disk growth for long-running nodes.
