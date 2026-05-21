# OBTC Mainnet Operations Runbook

> Status: draft for `mainnet-candidate-2026-07`

This runbook covers the operator setup for OBTC mainnet-candidate seed and
observer nodes. It complements [OBTC Mainnet Join Runbook](mainnet-join.md) and
[OBTC Mainnet Parameters](mainnet-params.md).

The release is a mainnet-candidate for technical validation. It is not a
production financial network claim.

## Roles

| Role | Purpose | Required flags |
|---|---|---|
| seed | Provide stable P2P bootstrap | `--obtcmainnet`, `--listen=0.0.0.0:9527` |
| observer | Provide release evidence and state snapshots | seed flags plus `--txindex`, `--expiryindex` |
| fresh-sync probe | Validate public bootstrap from a clean data dir | `--obtcmainnet` plus published `--addpeer` or `--connect` list |

At least one observer should run with both `--txindex` and `--expiryindex`.

## Host Baseline

Use a host with enough disk for a full node and index growth. Keep RPC on
localhost or a private management network.

Expected public exposure:

| Port | Exposure | Purpose |
|---:|---|---|
| `9527/tcp` | public | P2P |
| `9528/tcp` | private only | node RPC |

Do not publish wallet RPC, mining credentials, RPC credentials, or node private
keys in release material.

## Build

Run from the repository root:

```bash
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Record the commit:

```bash
git rev-parse HEAD
```

## Data Layout

Example layout:

```text
/opt/obtcd/btcd
/opt/obtcd/btcctl
/var/lib/obtcd-mainnet
/var/log/obtcd
/etc/obtcd/obtcd-mainnet.conf
```

Keep config and credentials out of the repository.

## Config

Example `/etc/obtcd/obtcd-mainnet.conf`:

```ini
obtcmainnet=1
datadir=/var/lib/obtcd-mainnet
listen=0.0.0.0:9527
rpclisten=127.0.0.1:9528
rpcuser=<rpc_user>
rpcpass=<rpc_pass>
notls=1
txindex=1
expiryindex=1
addpeer=<published-peer-1:9527>
addpeer=<published-peer-2:9527>
addpeer=<published-peer-3:9527>
```

Use `--connect=<peer:9527>` only for a probe node that must connect exclusively
to the listed peers.

## systemd

Example `/etc/systemd/system/obtcd-mainnet.service`:

```ini
[Unit]
Description=OBTC mainnet-candidate node
After=network-online.target
Wants=network-online.target

[Service]
User=obtcd
Group=obtcd
Type=simple
ExecStart=/opt/obtcd/btcd --configfile=/etc/obtcd/obtcd-mainnet.conf
Restart=on-failure
RestartSec=10
LimitNOFILE=65536
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/obtcd-mainnet /var/log/obtcd

[Install]
WantedBy=multi-user.target
```

Enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now obtcd-mainnet
sudo systemctl status obtcd-mainnet
```

## Firewall

Example with UFW:

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 9527/tcp
sudo ufw deny 9528/tcp
sudo ufw enable
sudo ufw status verbose
```

If RPC must be reachable from an admin host, allow only that source address.

```bash
sudo ufw allow from <admin-ip> to any port 9528 proto tcp
```

## Log Rotation

Example `/etc/logrotate.d/obtcd-mainnet`:

```text
/var/log/obtcd/*.log {
    daily
    rotate 14
    compress
    missingok
    notifempty
    copytruncate
}
```

If the node logs only to systemd journal, record the journal retention policy
instead of creating a file logrotate rule.

## Health Checks

Set shell variables for local checks:

```bash
export RPC_USER=<rpc_user>
export RPC_PASS=<rpc_pass>
export RPC_SERVER=127.0.0.1:9528
```

Run:

```bash
./btcctl --obtcmainnet --rpcserver="$RPC_SERVER" \
  --rpcuser="$RPC_USER" --rpcpass="$RPC_PASS" --notls getblockchaininfo

./btcctl --obtcmainnet --rpcserver="$RPC_SERVER" \
  --rpcuser="$RPC_USER" --rpcpass="$RPC_PASS" --notls getpeerinfo

./btcctl --obtcmainnet --rpcserver="$RPC_SERVER" \
  --rpcuser="$RPC_USER" --rpcpass="$RPC_PASS" --notls getchaintips

./btcctl --obtcmainnet --rpcserver="$RPC_SERVER" \
  --rpcuser="$RPC_USER" --rpcpass="$RPC_PASS" --notls getmempoolinfo

./btcctl --obtcmainnet --rpcserver="$RPC_SERVER" \
  --rpcuser="$RPC_USER" --rpcpass="$RPC_PASS" --notls getexpiryindexstats
```

Or append a reusable snapshot:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --network obtcmainnet \
  --rpcuser="$RPC_USER" \
  --rpcpass="$RPC_PASS" \
  --rpcserver="$RPC_SERVER" \
  --notls \
  --append /tmp/obtc-mainnet-72h-observation.md
```

External P2P check:

```bash
nc -vz <node-host> 9527
```

Local preflight check:

```bash
scripts/phase6/seed_preflight.sh \
  --network obtcmainnet \
  --notls \
  --rpcuser="$RPC_USER" \
  --rpcpass="$RPC_PASS" \
  --rpcserver="$RPC_SERVER" \
  --p2p-port=9527 \
  --strict-expiryindex
```

RPC exposure check from an external host should fail unless that host is
explicitly allowlisted:

```bash
nc -vz <node-host> 9528
```

## 72h Observation Template

Record one row at start and then at least every 12 hours.

| Time UTC | Node | Height | Best hash | Peers | Mempool tx | Chain tips | Expiry index | CPU/RAM/disk | Notes |
|---|---|---:|---|---:|---:|---|---|---|---|
| T+0h | seed-1 |  |  |  |  |  |  |  |  |
| T+12h | seed-1 |  |  |  |  |  |  |  |  |
| T+24h | seed-1 |  |  |  |  |  |  |  |  |
| T+36h | seed-1 |  |  |  |  |  |  |  |  |
| T+48h | seed-1 |  |  |  |  |  |  |  |  |
| T+60h | seed-1 |  |  |  |  |  |  |  |  |
| T+72h | seed-1 |  |  |  |  |  |  |  |  |

For pre-activation periods, REAP production and backlog metrics are
`N/A (pre-activation)` unless the chain has reached activation height `952016`.

## Fresh Sync Evidence

Use a clean data directory and the release bootstrap policy:

```bash
./btcd \
  --obtcmainnet \
  --datadir=/tmp/obtc-fresh-sync \
  --listen=127.0.0.1:0 \
  --rpclisten=127.0.0.1:9528 \
  --txindex \
  --expiryindex \
  --notls \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --addpeer=<published-peer-1:9527> \
  --addpeer=<published-peer-2:9527> \
  --addpeer=<published-peer-3:9527>
```

Record:

| Field | Value |
|---|---|
| Start time UTC |  |
| First peer time UTC |  |
| Headers synced time UTC |  |
| Tip synced time UTC |  |
| Final height |  |
| Final hash |  |
| Peer count |  |
| Errors / warnings |  |
| Conclusion |  |

## Go / No-Go

No-Go conditions:

* fresh sync cannot connect using the published bootstrap policy;
* seed nodes disagree on best hash at the same height;
* unexplained reorg depth is greater than 1;
* RPC is reachable from the public Internet unintentionally;
* node data corruption or expiry index rebuild failures are unexplained;
* release docs still contain placeholder peer policy or stale parameters.

Allowed limitations for a first candidate:

* DNS seed may be `N/A` if explicit peer bootstrap is published and tested;
* external miner validation may be pending if release notes say so;
* wallet funded renewal evidence may be pending if wallet claims remain limited.
