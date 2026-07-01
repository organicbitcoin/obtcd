# OBTC Mainnet Candidate Node Operator Runbook

This runbook is for operators and reviewers running `obtcd` for Mainnet
Candidate review, public testnet, or local regtest. It is not production launch
guidance.

## Build `obtcd`

```bash
git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd
git rev-parse HEAD

go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Optional release artifacts:

```bash
scripts/phase6/build_release_artifacts.sh \
  --version mainnet-candidate-2026-07 \
  --goos linux \
  --goarch amd64
```

TODO-HUMAN-CONFIRM final release tag, checksums, and signed manifest before
publishing artifacts.

## Configure Mainnet Candidate Parameters

Use `--obtcmainnet` for Mainnet Candidate review.

| Field | Value |
|---|---|
| P2P | `9527` |
| RPC | `9528` |
| Bech32 HRP | `obtc` |
| Fork height | `1000000` provisional |
| Replay protection | `1000001` |
| Expiry / REAP / commitment activation | `1002016` |

Config example:

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
```

Do not expose RPC to the public Internet.

## Run A Testnet Node

```bash
./btcd --obtctestnet \
  --datadir="$HOME/.obtcd-testnet" \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --txindex \
  --expiryindex \
  --notls \
  --addpeer=seed1.testnet.organicbitcoin.org:19527 \
  --addpeer=seed2.testnet.organicbitcoin.org:19527 \
  --addpeer=seed3.testnet.organicbitcoin.org:19527
```

More detail: [docs/testnet-join.md](docs/testnet-join.md).

## Run Regtest

```bash
MINING_ADDR="<obtcregtest address>"

./btcd --obtcregtest \
  --datadir="$HOME/.obtcd-regtest" \
  --listen=127.0.0.1:29527 \
  --rpclisten=127.0.0.1:29528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --txindex \
  --expiryindex \
  --notls \
  --miningaddr="$MINING_ADDR"
```

Plan 07 reproducible demo scripts are in PR #14 at the time of this package.
TODO-HUMAN-CONFIRM merge or equivalent final branch demo evidence.

## View Peers

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getconnectioncount

./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getpeerinfo
```

For testnet, replace `--obtcmainnet` with `--obtctestnet` and port `9528` with
`19528`.

## View Sync State

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getblockchaininfo

./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getchaintips
```

Record height, best hash, peer count, and whether the node is in initial block
download.

## View Expiry Index State

Start with `--expiryindex`, then run:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpiryindexstats

./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls listexpiring 0 1002016 100
```

Expected for an enabled index:

- `disabled: false`;
- `tip_height` catches up to chain tip;
- `network_params` matches [docs/mainnet-params.md](docs/mainnet-params.md).

## View Expiry Commitment

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpirycommitment
```

Review:

- `root`;
- `tip_height`;
- `tip_hash`;
- `enable_at_height`;
- `active`;
- `active_at_next_height`.

## View REAP State And Logs

Dry-run next-block plan:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getreapplan
```

Search logs for REAP/template entries:

```bash
rg -n "REAP|reap|template|expiry commitment|coinbase" /var/log/obtcd
```

Pre-activation mainnet-candidate nodes should report inactive or no candidates
until activation conditions are met.

## Status Page

```bash
./obtc-status --obtcmainnet \
  --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --notls \
  --listen=127.0.0.1:9680
```

Open `http://127.0.0.1:9680/status` from the local machine or an SSH tunnel.

## Reindex And Corrupted Index Handling

If the chain data is intact but ExpiryIndex state is suspected stale or corrupt:

1. Stop the node safely.
2. Back up logs and the data directory metadata needed for review.
3. Restart with `--reindex-expiry` and `--expiryindex`.
4. Wait for the index to rebuild.
5. Confirm `getexpiryindexstats.tip_height` reaches the chain tip.
6. Capture logs and `getexpirycommitment`.

Example:

```bash
./btcd --obtcmainnet \
  --datadir=/var/lib/obtcd-mainnet \
  --expiryindex \
  --reindex-expiry \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --notls
```

Do not delete data directories unless you are intentionally starting a fresh
sync or have a separate backup and incident record.

## Safe Stop

Preferred:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls stop
```

With systemd:

```bash
sudo systemctl stop obtcd-mainnet
sudo systemctl status obtcd-mainnet
```

Avoid killing the process unless the node is unresponsive.

## Upgrade Candidate Version

1. Read [CHANGELOG.md](CHANGELOG.md) and release notes.
2. Record current commit, height, best hash, and expiry index tip.
3. Stop the node safely.
4. Install new binaries.
5. Start with the same network flag and data directory.
6. Confirm peer count, chain tip, expiry index, commitment, and logs.
7. Keep rollback binaries and logs until the candidate has run long enough for
   your operator policy.

## Bug Report

File public non-sensitive reports at:

<https://github.com/organicbitcoin/obtcd/issues>

Include:

- commit hash;
- network flag;
- node command or config with secrets removed;
- height and best hash;
- peer count;
- expiry index stats;
- relevant logs;
- expected and observed behavior.

Sensitive reports should use GitHub private vulnerability reporting if enabled.
