# OBTC Limited Public Testnet User Test

This page is for invited technical reviewers who are helping validate the OBTC
limited public testnet.

This is not a public launch, not a mainnet release, not an investment or yield
program, and not a request for promotion or endorsement. Testnet coins have no
real-world value.

Expected time for a first pass: 30-90 minutes.

## What To Validate

The useful minimum test is:

1. Install Go and build `obtcd` and `obtcwallet` from source.
2. Start an `obtcd --obtctestnet` node from a clean data directory.
3. Sync through the public seed peers.
4. Create and start an `obtcwallet --obtctestnet` wallet.
5. Request a small amount of testnet coin from the maintainer.
6. Confirm the wallet receives funds.
7. Query UTXO expiry state with `obtc.getexpiry`.
8. Renew one active UTXO with `obtc.renew`.
9. Send a short validation report with the height, block hash, txid, and any
   confusing documentation or runtime behavior.

## Safety Rules

- Always pass `--obtctestnet` to node, wallet, and CLI commands.
- Do not use mainnet funds or mainnet wallets.
- Do not post seed phrases, private keys, wallet private passphrases, RPC
  passwords, private IP addresses, local private paths, or screenshots that
  contain secrets.
- Keep node RPC and wallet RPC bound to `127.0.0.1`.
- Do not expose wallet RPC `19554` or wallet agent gRPC `19556` to the public
  internet.
- Mining is not the default reviewer path. If you want to test mining, ask the
  maintainer first and use a coordinated time window.

## Install Go

Use Go `1.24.6` or newer. The node currently requires Go `1.24.0` or newer, and
the wallet requires Go `1.24.6` or newer.

### macOS

```bash
brew install go git
go version
```

If Homebrew is not available, install Go from:

```text
https://go.dev/dl/
```

### Ubuntu / Debian

The Go version in `apt` can be too old. If `apt` does not provide Go `1.24.6` or
newer, install from the official tarball.

```bash
sudo apt update
sudo apt install -y build-essential git curl ca-certificates

curl -LO https://go.dev/dl/go1.24.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.6.linux-amd64.tar.gz

echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.profile
source ~/.profile

go version
```

### Linux ARM64

For ARM64 Linux, use the ARM64 tarball instead:

```bash
curl -LO https://go.dev/dl/go1.24.6.linux-arm64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.6.linux-arm64.tar.gz

echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.profile
source ~/.profile

go version
```

### Windows

Use WSL2 with Ubuntu and follow the Ubuntu steps above. Native Windows testing
is not the preferred path for this limited validation round.

## Build From Source

```bash
mkdir -p ~/obtc-testnet
cd ~/obtc-testnet

git clone https://github.com/organicbitcoin/obtcd.git
git clone https://github.com/organicbitcoin/obtcwallet.git

(cd obtcd && go build -o ./btcd . && go build -o ./btcctl ./cmd/btcctl)
(cd obtcwallet && go build -o ./btcwallet . && go build -o ./renewall ./cmd/renewall)
```

Record the commits you used:

```bash
(cd ~/obtc-testnet/obtcd && git rev-parse HEAD)
(cd ~/obtc-testnet/obtcwallet && git rev-parse HEAD)
```

## Start An OBTC Testnet Node

Start from a clean data directory and connect to the public seed peers.

```bash
mkdir -p ~/obtc-testnet/data/obtcd
cd ~/obtc-testnet

./obtcd/btcd --obtctestnet \
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

Leave this terminal running. Open a second terminal for verification commands.

```bash
cd ~/obtc-testnet

./obtcd/btcctl --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls getblockchaininfo

./obtcd/btcctl --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls getpeerinfo

./obtcd/btcctl --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls getexpiryindexstats
```

For your validation report, record:

- node height;
- best block hash;
- peer count;
- whether `getexpiryindexstats.disabled` is `false`;
- whether `getexpiryindexstats.tip_height` matches the node height.

## Create And Start A Wallet

Create a new test wallet. Save the wallet private passphrase locally. You will
need it to unlock the wallet before signing a renewal transaction.

```bash
mkdir -p ~/obtc-testnet/data/obtcwallet
cd ~/obtc-testnet

./obtcwallet/btcwallet --create \
  --obtctestnet \
  --appdata="$HOME/obtc-testnet/data/obtcwallet"
```

Start the wallet and connect it to your local node:

```bash
cd ~/obtc-testnet

./obtcwallet/btcwallet --obtctestnet \
  --appdata="$HOME/obtc-testnet/data/obtcwallet" \
  --rpcconnect=127.0.0.1:19528 \
  --btcdusername=obtc \
  --btcdpassword=obtcpass \
  --username=wallet \
  --password=walletpass \
  --rpclisten=127.0.0.1:19554 \
  --experimentalrpclisten=127.0.0.1:19556 \
  --noservertls \
  --noclienttls
```

Leave this wallet terminal running.

In another terminal, verify the wallet RPC:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"height","method":"getblockcount","params":[]}' \
  http://127.0.0.1:19554/
```

## Request Testnet Coins

There is no public faucet during this limited validation stage. Testnet coins
are sent manually by the maintainer.

Default request size:

- normal reviewer request: `0.1` test OBTC;
- maximum without extra explanation: `1.0` test OBTC;
- larger requests must explain the test scenario.

Generate a receive address:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"addr","method":"getnewaddress","params":[]}' \
  http://127.0.0.1:19554/
```

Send the maintainer only:

```text
OBTC testnet address:
Requested amount:
Reason:
```

Do not send seed phrases, private keys, wallet private passphrases, or RPC
passwords.

After the maintainer sends testnet coins, check your balance:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"balance","method":"getbalance","params":[]}' \
  http://127.0.0.1:19554/
```

## Query Expiry State

After the funding transaction confirms, query wallet expiry state:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"expiry","method":"obtc.getexpiry","params":[10]}' \
  http://127.0.0.1:19554/
```

Record:

- `tip_height`;
- `window_blocks`;
- one funded outpoint;
- `create_height`;
- `expiry_height`;
- current status;
- any `near_expiry_warning` or `too_late_to_renew_warning` fields.

Only active or expiring UTXOs can be renewed. Expired UTXOs are intentionally
not renewable by the normal wallet renewal path.

## Renew One Active UTXO

Choose one active or expiring outpoint from `obtc.getexpiry`. The outpoint
format is:

```text
<txid>:<vout>
```

Unlock the wallet for signing:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"unlock","method":"walletpassphrase","params":["<your-wallet-private-passphrase>",600]}' \
  http://127.0.0.1:19554/
```

Run one funded renewal:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"renew","method":"obtc.renew","params":[["<txid:vout>"],0.0001,null,0.00003,1]}' \
  http://127.0.0.1:19554/
```

The parameter order is:

```text
outpoints, amount, targetAddress, maxFeeRate, minconf
```

Use `null` for `targetAddress` to let the wallet choose the renewal output
address.

After the renewal confirms, query `obtc.getexpiry` again and record the new
expiry height:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"expiry2","method":"obtc.getexpiry","params":[10]}' \
  http://127.0.0.1:19554/
```

## Optional: Dry-Run Renewall

If you want to test the batch renewal selector without signing or broadcasting
transactions:

```bash
cd ~/obtc-testnet

./obtcwallet/renewall \
  --connect=127.0.0.1:19556 \
  --walletpass='<your-wallet-private-passphrase>' \
  --amount=0.1 \
  --notls \
  --dry-run
```

Do not run non-dry-run `renewall` unless the maintainer explicitly asks you to
test batch renewal.

## Optional: Coordinated Mining Test

Mining is not the default reviewer path. Do not run open-ended mining on the
public testnet without coordination.

If you specifically want to validate mining:

1. Ask the maintainer first and mark it as a mining test.
2. Include your node commit, public P2P address if any, requested time window,
   and mining payout address.
3. Wait for maintainer confirmation.
4. Mine only during the agreed window.
5. Submit block hash, height, node commit, and logs with secrets removed.

For a short coordinated test, the command shape is:

```bash
./obtcd/btcctl --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls generate 1
```

## Validation Report

Send a short report to the maintainer through the agreed channel.

Use this template:

```text
OS and architecture:
Go version:
obtcd commit:
obtcwallet commit:
Node height:
Best block hash:
Peer count:
Expiry index disabled:
Expiry index tip height:
Wallet processed height:
Testnet receive address:
Funding txid:
Renewal txid:
Renewal confirmed block hash:
Did obtc.getexpiry show expiry data? yes/no
Did obtc.renew confirm? yes/no
Problems or confusing steps:
```

For failures, include:

```text
Command:
Observed error:
Expected behavior:
Relevant logs with secrets removed:
```

Do not include seed phrases, private keys, wallet private passphrases, or RPC
passwords in the report.
