# OBTC Testnet Node Runbook

This runbook starts an OBTC public testnet node from source. Testnet coins have
no real-world value, and this document does not apply to mainnet operation.

Use `--obtctestnet` for every node and CLI command in this file.

## 1. Build

```bash
mkdir -p ~/obtc-testnet
cd ~/obtc-testnet

git clone https://github.com/organicbitcoin/obtcd.git
cd obtcd

go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Record the commit:

```bash
git rev-parse HEAD
```

Run a focused local check:

```bash
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|Replay' -count=1
go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1
```

## 2. Choose Data And RPC Settings

Use a clean testnet data directory:

```bash
mkdir -p ~/obtc-testnet/data/obtcd
```

Default OBTC testnet ports:

| Surface | Default |
|---|---:|
| P2P | `19527` |
| node RPC | `19528` |

Keep RPC bound to loopback unless you have a separate firewall and credential
plan. The examples use placeholder credentials; replace them on shared hosts.

## 3. Start Node

```bash
cd ~/obtc-testnet/obtcd

./btcd --obtctestnet \
  --datadir="$HOME/obtc-testnet/data/obtcd" \
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

Leave this terminal running. Use another terminal for checks.

## 4. Confirm Chain State

```bash
cd ~/obtc-testnet/obtcd

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblockchaininfo

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getpeerinfo

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpiryindexstats

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpirycommitment
```

Expected checks:

- `getblockchaininfo.chain` is `obtctestnet`.
- Peer count is non-zero after discovery or explicit peers connect.
- `getexpiryindexstats.disabled` is `false`.
- `getexpiryindexstats.tip_height` catches up to the node height.
- `getexpirycommitment.root` is populated after the index has a tip.

## 5. Status JSON Or Page

One-shot JSON:

```bash
OBTC_NETWORK=obtctestnet \
OBTC_RPC_PORT=19528 \
OBTC_RPC_USER=obtc \
OBTC_RPC_PASS=obtcpass \
./scripts/status-obtc-demo.sh
```

Read-only local page:

```bash
./obtc-status --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls \
  --listen=127.0.0.1:9680
```

Open:

- `http://127.0.0.1:9680/status`
- `http://127.0.0.1:9680/`

## 6. Mining Review

Mining is not the default public testnet user path. If you are reviewing mining
or block templates, start the node with a testnet mining address:

```bash
./btcd --obtctestnet \
  --datadir="$HOME/obtc-testnet/data/obtcd" \
  --listen=0.0.0.0:19527 \
  --rpclisten=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --txindex \
  --expiryindex \
  --notls \
  --miningaddr=<testnet_obtc_address> \
  --addpeer=seed1.testnet.organicbitcoin.org:19527
```

Then inspect:

```bash
./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getmininginfo

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblocktemplate

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getreapplan
```

Mining review should also read
[docs/mining-review-checklist.md](docs/mining-review-checklist.md).

## 7. Expiry And REAP Inspection

Testnet expiry parameters are intentionally short for review. Query the node:

```bash
./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpiryindexstats

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls listexpiring <start_height> <end_height> 50

./btcctl --obtctestnet --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getreapplan
```

`getreapplan` is a read-only next-block dry run. It does not broadcast a
transaction and does not depend on mempool relay.

## 8. Wallet Next Step

After the node is synced, use [RUN_WALLET.md](RUN_WALLET.md) to create a new
test wallet, get a receive address, inspect expiry, and rehearse renewal.

Do not use a Bitcoin seed phrase, a Bitcoin private key, or a real wallet backup
for this testnet flow.

## 9. Evidence To Capture

For review notes, capture redacted output only:

- OS and architecture;
- Go version;
- `obtcd` commit hash;
- exact node command;
- network flag used;
- height, best hash, peer count;
- expiry indexed tip and commitment root;
- `getreapplan` output if reviewing REAP;
- log excerpts around failures with credentials removed.
