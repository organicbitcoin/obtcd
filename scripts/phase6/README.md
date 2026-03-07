# Phase 6 Testnet Operations Helpers

This directory contains minimal scripts/templates for Phase 6 testnet deployment and validation.

## Files

- `run_testnet_node.sh`
  - Start/stop/restart/status/tail for a local or remote OBTC testnet node.
- `systemd/obtcd-testnet.service`
  - Example `systemd` unit for long-running seed/observer nodes.
- `collect_validation_snapshot.sh`
  - Capture a markdown snapshot of key RPC evidence for `docs/phase6-validation.md`.
- `seed_preflight.sh`
  - Run seed-candidate readiness checks (RPC health, peers, active tip, optional expiryindex, local P2P listener).
- `gen_testnet_conf.sh`
  - Generate a minimal `btcd` testnet config template for seed/observer nodes.

## Quick start

From repository root:

```bash
# Build first
go build ./...

# Start node
RPC_USER=u RPC_PASS=p scripts/phase6/run_testnet_node.sh start

# Check status
RPC_USER=u RPC_PASS=p scripts/phase6/run_testnet_node.sh status

# Stop node
scripts/phase6/run_testnet_node.sh stop
```

Collect a validation snapshot:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --rpcuser=u \
  --rpcpass=p \
  --append docs/phase6-validation.md
```

Run seed-candidate preflight checks:

```bash
scripts/phase6/seed_preflight.sh \
  --rpcuser=u \
  --rpcpass=p \
  --rpcserver=127.0.0.1:19528 \
  --min-peers=1
```

Use `--strict-expiryindex` for observer/validator roles that must expose expiry RPC.

Generate a baseline config file:

```bash
scripts/phase6/gen_testnet_conf.sh \
  --rpcuser=u \
  --rpcpass=p \
  --addpeers=seed1.example.com:19527,seed2.example.com:19527
```

## Common environment overrides

```bash
BTCD_BIN=/opt/obtcd/btcd
BTCCTL_BIN=/opt/obtcd/cmd/btcctl/btcctl
DATA_DIR=/var/lib/obtcd-testnet
P2P_LISTEN=0.0.0.0:19527
RPC_LISTEN=127.0.0.1:19528
RPC_SERVER=127.0.0.1:19528
ADDPEERS=seed1.example.com:19527,seed2.example.com:19527
ENABLE_EXPIRYINDEX=1
```

## Notes

- Keep RPC on localhost unless you have strict access controls.
- Use at least one observer node with `--expiryindex` enabled.
- For seed-candidate nodes, ensure inbound TCP `19527` is reachable.
