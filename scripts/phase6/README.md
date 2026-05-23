# Phase 6 Testnet Operations Helpers

This directory contains minimal scripts/templates for Phase 6 testnet deployment and validation.

## Files

- `run_testnet_node.sh`
  - Start/stop/restart/status/tail for a local or remote OBTC testnet node.
- `systemd/obtcd-testnet.service`
  - Example `systemd` unit for long-running seed/observer nodes.
- `systemd/obtcd-mainnet.service`
  - Example `systemd` unit for mainnet-candidate seed/observer nodes.
- `collect_validation_snapshot.sh`
  - Capture a markdown snapshot of key RPC evidence to a local file. Defaults
    to OBTC testnet; pass `--network obtcmainnet --notls` for mainnet-candidate
    nodes that use local no-TLS RPC.
- `collect_72h_observation.sh`
  - Repeatedly append validation snapshots for a 72h mainnet-candidate
    observation window. Use `--plan` to verify the schedule without RPC calls.
- `build_release_artifacts.sh`
  - Build local `btcd`, `btcctl`, and `obtc-status` release artifacts with
    `SHA256SUMS` and a `MANIFEST.md`.
- `firewall_preflight.sh`
  - Check public P2P reachability and RPC non-exposure for seed/fallback nodes.
- `seed_preflight.sh`
  - Run seed-candidate readiness checks (RPC health, peers, active tip, optional expiryindex, local P2P listener). Defaults to OBTC testnet; pass `--network obtcmainnet --notls` for mainnet-candidate nodes.
- `gen_testnet_conf.sh`
  - Generate a minimal `btcd` config template for OBTC testnet or mainnet-candidate seed/observer nodes.

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
  --append /tmp/obtc-phase6-validation.md
```

For mainnet-candidate:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --network=obtcmainnet \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --append /tmp/obtc-mainnet-72h.md
```

Plan a 72h observation run:

```bash
scripts/phase6/collect_72h_observation.sh --plan
```

Run the observation collector:

```bash
scripts/phase6/collect_72h_observation.sh \
  --network=obtcmainnet \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --rpcserver=127.0.0.1:9528 \
  --new-file \
  --out /tmp/obtc-mainnet-72h-observation.md
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

For mainnet-candidate:

```bash
scripts/phase6/seed_preflight.sh \
  --network=obtcmainnet \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --rpcserver=127.0.0.1:9528 \
  --p2p-port=9527 \
  --strict-expiryindex
```

Generate a baseline config file:

```bash
scripts/phase6/gen_testnet_conf.sh \
  --rpcuser=u \
  --rpcpass=p \
  --addpeers=seed1.example.com:19527,seed2.example.com:19527
```

For mainnet-candidate:

```bash
scripts/phase6/gen_testnet_conf.sh \
  --network=obtcmainnet \
  --rpcuser=u \
  --rpcpass=p \
  --addpeers=seed1.example.com:9527,seed2.example.com:9527
```

Build release artifacts and checksums:

```bash
scripts/phase6/build_release_artifacts.sh \
  --version mainnet-candidate-2026-07 \
  --goos linux \
  --goarch amd64
```

The `OBTC release artifacts` GitHub Actions workflow can run the same build
manually and uploads the generated directory after `SHA256SUMS` verification.
Use it to stage operator artifacts before attaching the final release evidence.

Check seed/fallback firewall exposure:

```bash
scripts/phase6/firewall_preflight.sh --host seed1.example.com
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
OUT_DIR=/tmp/obtc-release-artifacts
```

## Notes

- Keep RPC on localhost unless you have strict access controls.
- Use at least one observer node with `--expiryindex` enabled.
- For seed-candidate nodes, ensure inbound TCP `19527` is reachable.
