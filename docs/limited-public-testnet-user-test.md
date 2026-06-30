# OBTC Public Testnet Self-Test

This page is for invited testers who are helping try the OBTC limited public
testnet.

Canonical guide: [OBTC Public Testnet User Test](public-testnet-user-test.md).
Use that page for new tests; this page is kept as a compatibility entry point
for older limited-review links.

This is not a public launch, not a mainnet release, not an investment or yield
program, and not a request for promotion or endorsement. Testnet coins have no real-world value.

Expected time for a first pass: 30-90 minutes. Thank you for taking the time to
try this; short, honest feedback is enough.

## What To Try

The useful minimum test is:

1. Install Go and build `obtcd` and `obtcwallet` from source.
2. Start an `obtcd --obtctestnet` node from a clean data directory.
3. Sync through the public seed peers.
4. Create and start an `obtcwallet --obtctestnet` wallet.
5. Request a small amount of testnet coin from the person who invited you.
6. Confirm the wallet receives funds.
7. Query UTXO expiry state with `obtc.getexpiry`.
8. Renew one active UTXO with `obtc.renew`.
9. Send a brief note about what worked and where you got stuck.

## If You Use Cursor, Claude, Codex, Or Another AI Assistant

You can ask an AI coding assistant to run most of this for you. It can install
Go, build the repos, start the node and wallet, check sync, query expiry, and
prepare the short feedback note.

Important: do not paste wallet seed phrases, private keys, wallet private
passphrases, or RPC passwords into an AI chat. If a command needs the wallet
private passphrase, type it only into your local terminal when prompted.

Copy this prompt into your AI assistant:

```text
Please help me try the OBTC limited public testnet on this machine.

Follow this guide:
https://github.com/organicbitcoin/obtcd/blob/master/docs/public-testnet-user-test.md

Rules:
- Use ~/obtc-testnet as the working directory.
- Install Go 1.24.6+ only if it is missing or too old.
- Clone and build:
  https://github.com/organicbitcoin/obtcd
  https://github.com/organicbitcoin/obtcwallet
- Always use --obtctestnet.
- Keep node RPC and wallet RPC bound to 127.0.0.1.
- Do not expose wallet RPC or wallet agent ports to the public internet.
- Do not ask me to paste wallet seed phrases, private keys, wallet private
  passphrases, or RPC passwords into chat.
- If the wallet private passphrase is needed, give me a command that prompts me
  locally in the terminal.
- Start obtcd, wait until it has peers and is synced, then create/start
  btcwallet.
- Generate one testnet receive address and stop so I can ask for testnet coins.
- After I confirm coins were sent, check balance, run obtc.getexpiry, choose one
  active outpoint, unlock locally, and run one obtc.renew.
- At the end, give me a short note with:
  OS, Go version, obtcd commit, obtcwallet commit, height, best hash, peer count,
  whether obtc.getexpiry worked, renew txid if any, and the one thing that was
  confusing.
```

The AI assistant should stop at two points:

1. After generating a receive address, so you can ask for testnet coins.
2. Before unlocking the wallet, so you can type the wallet private passphrase
   locally instead of sending it through chat.

## Safety Rules

- Always pass `--obtctestnet` to node, wallet, and CLI commands.
- Do not use mainnet funds or mainnet wallets.
- Do not post seed phrases, private keys, wallet private passphrases, RPC
  passwords, private IP addresses, local private paths, or screenshots that
  contain secrets.
- If you use an AI assistant, do not paste wallet seed phrases, private keys, or
  wallet private passphrases into the chat.
- Keep node RPC and wallet RPC bound to `127.0.0.1`.
- Do not expose wallet RPC `19554` or wallet agent gRPC `19556` to the public
  internet.
- Mining is not the default path for this test. If you want to test mining,
  please coordinate first and use an agreed time window.

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
is not the preferred path for this limited test round.

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

For your own notes, record:

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

There is no public faucet during this limited test stage. Testnet coins are sent
manually.

Default request size:

- normal test request: `0.1` test OBTC;
- maximum without extra explanation: `1.0` test OBTC;
- larger requests must explain the test scenario.

Generate a receive address:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"addr","method":"getnewaddress","params":[]}' \
  http://127.0.0.1:19554/
```

Send the person who invited you only:

```text
OBTC testnet address:
Requested amount:
Reason:
```

Do not send seed phrases, private keys, wallet private passphrases, or RPC
passwords.

After testnet coins are sent, check your balance:

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
python3 - <<'PY'
import getpass
import json
import subprocess

payload = {
    "jsonrpc": "1.0",
    "id": "unlock",
    "method": "walletpassphrase",
    "params": [getpass.getpass("Wallet private passphrase: "), 600],
}

subprocess.run(
    [
        "curl",
        "--user",
        "wallet:walletpass",
        "-H",
        "content-type: text/plain;",
        "--data-binary",
        json.dumps(payload),
        "http://127.0.0.1:19554/",
    ],
    check=True,
)
PY
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

Do not run non-dry-run `renewall` unless you have been asked to test batch
renewal.

## Optional: Coordinated Mining Test

Mining is not the default path for this test. Please do not run open-ended
mining on the public testnet without coordination.

If you specifically want to try mining:

1. Ask first and mark it as a mining test.
2. Include your node commit, public P2P address if any, requested time window,
   and mining payout address.
3. Wait for confirmation.
4. Mine only during the agreed window.
5. Send the block hash, height, node commit, and logs with secrets removed.

For a short coordinated test, the command shape is:

```bash
./obtcd/btcctl --obtctestnet \
  --rpcserver=127.0.0.1:19528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --notls generate 1
```

## Short Feedback

Please send a short note through the agreed channel. A few sentences are enough.
For example:

```text
I was able to build both repos and sync the node to height <height>.
The wallet received testnet coins and obtc.getexpiry worked / did not work.
The renew tx was <txid> and it confirmed / did not confirm.
The confusing part was <one short note>.
```

If something fails, the most helpful details are:

```text
OS and architecture:
Go version:
obtcd commit:
obtcwallet commit:
Node height:
Best block hash:
Peer count:
Command:
Observed error:
Relevant logs with secrets removed:
```

Do not include seed phrases, private keys, wallet private passphrases, or RPC
passwords in your feedback.
