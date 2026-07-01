# OBTC Local Demo Runbook

This runbook is for reviewers who want a clean local OBTC regtest run without
using real keys, real wallets, public infrastructure, or mainnet parameters.
The commands below use `obtcregtest` only.

The current node binary names still inherit upstream names. In this repository
the node builds as `btcd`, the command-line RPC client builds as `btcctl`, and
the wallet repository still builds `btcwallet` and `renewall`.

## 1. Prerequisites

Install:

- Git.
- Go 1.24.6 or newer. The node currently needs Go 1.24.0+, and the wallet
  needs Go 1.24.6+.
- `curl`, `python3`, and a POSIX shell for the demo scripts.

On macOS:

```bash
brew install go git curl
go version
```

On Ubuntu or Debian, use the official Go tarball if the package manager has an
older Go release:

```bash
sudo apt update
sudo apt install -y build-essential git curl ca-certificates python3

curl -LO https://go.dev/dl/go1.24.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.6.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.profile
source ~/.profile
go version
```

## 2. Clone

Use separate directories for the node and wallet repositories:

```bash
mkdir -p ~/obtc-demo
cd ~/obtc-demo

git clone https://github.com/organicbitcoin/obtcd.git
git clone https://github.com/organicbitcoin/obtcwallet.git
```

Record the commits:

```bash
(cd ~/obtc-demo/obtcd && git rev-parse HEAD)
(cd ~/obtc-demo/obtcwallet && git rev-parse HEAD)
```

## 3. Build

Build the node tools:

```bash
cd ~/obtc-demo/obtcd
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
go build -o ./devnetsim ./cmd/devnetsim
```

Build the wallet tools:

```bash
cd ~/obtc-demo/obtcwallet
go build -o ./btcwallet .
go build -o ./renewall ./cmd/renewall
go build -o ./walletapp ./cmd/walletapp
```

## 4. Test

Run focused node tests:

```bash
cd ~/obtc-demo/obtcd
go test ./chaincfg ./wire -run OBTC -count=1
go test ./mempool -run 'REAP|Replay' -count=1
go test ./mining -run 'REAP|Template|Accounting|Boundary' -count=1
go test ./blockchain/expiryindex -run 'Commitment|REAP|Rebuild' -count=1
```

Run the wallet local gate:

```bash
cd ~/obtc-demo/obtcwallet
go test $(go list ./... | grep -v github.com/btcsuite/btcwallet/chain) -count=1
```

If a full package test matrix takes too long on a laptop, keep the exact command
and failure output in the review notes.

## 5. One-Command Regtest Demo

The scripted demo builds the node tools into a local demo directory, starts
`btcd --obtcregtest`, mines through the regtest activation heights, shows expiry
index state, shows the next-block REAP plan, mines one more block, and prints a
minimal status JSON.

```bash
cd ~/obtc-demo/obtcd
RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

Useful environment variables:

```bash
OBTC_DEMO_DIR=/tmp/obtc-demo-regtest RESET=1 ./scripts/demo-regtest-expiry-reap.sh
KEEP_NODE=1 ./scripts/demo-regtest-expiry-reap.sh
OBTC_RPC_PORT=30528 OBTC_P2P_PORT=30527 RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

When `KEEP_NODE=1`, the script leaves the node running and prints the data
directory and log path. Stop it with `kill $(cat "$OBTC_DEMO_DIR/obtcd.pid")`.

The script intentionally uses regtest coinbase outputs and deterministic local
addresses. It does not create a production wallet and does not ask for a seed or
private key.

## 6. Manual Node Startup

The same regtest node can be started manually:

```bash
cd ~/obtc-demo/obtcd
mkdir -p ~/obtc-demo/data/obtcd-regtest

MINING_ADDR="$(./devnetsim miningaddr \
  --network obtcregtest \
  --statefile ~/obtc-demo/data/devnetsim-state.json \
  --seed-tag demo-miner)"

./btcd --obtcregtest \
  --datadir="$HOME/obtc-demo/data/obtcd-regtest" \
  --listen=127.0.0.1:29527 \
  --rpclisten=127.0.0.1:29528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --txindex \
  --expiryindex \
  --notls \
  --miningaddr="$MINING_ADDR"
```

Use another terminal for RPC:

```bash
cd ~/obtc-demo/obtcd

./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblockchaininfo

./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpiryindexstats
```

Confirm:

- `getblockchaininfo.chain` is `obtcregtest`.
- `getexpiryindexstats.disabled` is `false`.
- `getexpiryindexstats.tip_height` follows the best chain height.
- Logs show normal block acceptance and no repeated RPC/auth errors.

## 7. Mining And UTXO Creation

Regtest mining uses the node RPC `generate`. The node must have a
`--miningaddr`; this codebase does not wire a separate `generatetoaddress`
handler in the node RPC server.

Mine through regtest activation:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls generate 120
```

Coinbase maturity is 100 blocks. For ordinary test UTXOs without using a real
wallet, `devnetsim prepare` can spend matured deterministic local coinbase
outputs into confirmed local outputs:

```bash
./devnetsim prepare \
  --network obtcregtest \
  --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --statefile ~/obtc-demo/data/devnetsim-state.json \
  --seed-tag demo-miner \
  --utxos 8 \
  --value 300000 \
  --fee-rate 10 \
  --fanout-size 8
```

Then inspect expiry data:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls listexpiring 120 300 20
```

The `create_height` and `expiry_height` fields are height based. They do not
depend on wall-clock time.

## 8. Status Output

For a one-shot JSON summary:

```bash
cd ~/obtc-demo/obtcd
./scripts/status-obtc-demo.sh
```

The output includes:

- chain and current height;
- peer count;
- expiry indexed tip;
- expiry commitment root;
- next-block REAP dry-run plan;
- last observed REAP transaction in the recent chain window;
- optional wallet information if wallet RPC environment variables are set;
- build commit hash;
- network parameter summary from `getexpiryindexstats`.

For the existing HTTP status page:

```bash
./obtc-status --obtcregtest \
  --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls \
  --listen=127.0.0.1:9680
```

Open `http://127.0.0.1:9680/status` for JSON or
`http://127.0.0.1:9680/` for the local page.

## 9. Wallet Flow

Wallet creation, address generation, balance checks, expiry inspection, and
renewal are covered in [RUN_WALLET.md](RUN_WALLET.md). The wallet flow is
separate from the one-command node demo because wallet creation requires a
local passphrase prompt.

## 10. Network Separation

Use:

- `--obtcregtest` for this local demo;
- `--obtctestnet` for public testnet review;
- `--obtcmainnet` only for mainnet-candidate review with the corresponding
  mainnet runbook.

Do not import a Bitcoin wallet seed or private key into OBTC demo software.
Do not use the regtest commands as mainnet operating guidance.
