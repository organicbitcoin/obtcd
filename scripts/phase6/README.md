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
    nodes that use local no-TLS RPC, or `--network obtcmainnet72h --notls` for
    the private 72h REAP-active rehearsal network.
- `collect_72h_observation.sh`
  - Repeatedly append validation snapshots for a 72h mainnet-candidate
    observation window. Use `--plan` to verify the schedule without RPC calls.
- `build_release_artifacts.sh`
  - Build local `btcd`, `btcctl`, and `obtc-status` release artifacts with
    `SHA256SUMS` and a `MANIFEST.md`.
- `firewall_preflight.sh`
  - Check public P2P reachability and RPC non-exposure for seed/fallback nodes.
- `seed_preflight.sh`
  - Run seed-candidate readiness checks (RPC health, peers, active tip, optional expiryindex, local P2P listener). Defaults to OBTC testnet; pass `--network obtcmainnet --notls` for mainnet-candidate nodes or `--network obtcmainnet72h --notls` for private rehearsal nodes.
- `gen_testnet_conf.sh`
  - Generate a minimal `btcd` config template for OBTC testnet, mainnet-candidate, or private rehearsal seed/observer nodes.
- `verify_72h_fork_anchor.sh`
  - Verify the rehearsal fork anchor hash against public BTC APIs and optionally a local Bitcoin Core source.
- `generate_72h_rehearsal_manifest.sh`
  - Generate a redacted run manifest with commits, parameters, nodes, timing, and private artifact URI.
- `monitor_mainnet72h_sync.sh`
  - Capture machine-readable multi-node sync, expiryindex, REAP, and RPC latency evidence for the private `obtcmainnet72h` rehearsal. This is the command to attach to Codex Automation every 4 hours.
- `export_reap_block_evidence.sh`
  - Export actual blocks containing REAP marker payloads and confirm that an independent validator node has accepted the same hash at the same height.

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

For private 72h REAP-active rehearsal:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --network=obtcmainnet72h \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --append /tmp/obtc-mainnet72h-reap.md
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

Run the private 72h REAP-active observation collector:

```bash
scripts/phase6/collect_72h_observation.sh \
  --network=obtcmainnet72h \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --rpcserver=127.0.0.1:39528 \
  --new-file \
  --out /tmp/obtc-mainnet72h-reap-observation.md
```

Run the 4-hour sync monitor command used by Codex Automation:

```bash
scripts/phase6/monitor_mainnet72h_sync.sh \
  --network=obtcmainnet72h \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --run-id=mainnet72h-reap-956542-20260704T000000Z \
  --node='miner-1|127.0.0.1:39528|miner' \
  --node='validator-1|10.0.1.12:39528|validator' \
  --s3-uri=s3://obtc-private-rehearsal-artifacts/mainnet-72h-reap-active/mainnet72h-reap-956542-20260704T000000Z/ \
  --upload
```

Export and independently confirm actual REAP blocks:

```bash
scripts/phase6/export_reap_block_evidence.sh \
  --network=obtcmainnet72h \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --source-rpc=127.0.0.1:39528 \
  --validator-rpc=10.0.1.12:39528 \
  --from-height=956566 \
  --to-height=956700 \
  --run-id=mainnet72h-reap-956542-20260704T000000Z \
  --s3-uri=s3://obtc-private-rehearsal-artifacts/mainnet-72h-reap-active/mainnet72h-reap-956542-20260704T000000Z/ \
  --upload \
  --strict
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

For private 72h REAP-active rehearsal:

```bash
scripts/phase6/seed_preflight.sh \
  --network=obtcmainnet72h \
  --notls \
  --rpcuser=u \
  --rpcpass=p \
  --rpcserver=127.0.0.1:39528 \
  --p2p-port=39527 \
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

For private 72h REAP-active rehearsal:

```bash
scripts/phase6/gen_testnet_conf.sh \
  --network=obtcmainnet72h \
  --rpcuser=u \
  --rpcpass=p \
  --addpeers=node1.internal:39527,node2.internal:39527
```

Verify the provisional rehearsal fork anchor before starting nodes:

```bash
scripts/phase6/verify_72h_fork_anchor.sh \
  --height=956542 \
  --hash=0000000000000000000200bad2d8d62a198f06b4390e7ca9be8f15581b42102e
```

Generate a redacted manifest after the run:

```bash
scripts/phase6/generate_72h_rehearsal_manifest.sh \
  --run-id=mainnet72h-reap-956542-20260703T000000Z \
  --raw-artifact-uri=s3://obtc-private-rehearsal-artifacts/mainnet-72h-reap-active/mainnet72h-reap-956542-20260703T000000Z/ \
  --node=observer-1 \
  --out=/tmp/manifest.redacted.json
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
scripts/phase6/firewall_preflight.sh --network=obtcmainnet72h --host node1.internal --plan
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
