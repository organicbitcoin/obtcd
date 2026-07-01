# OBTC Wallet Runbook

This runbook covers wallet setup for OBTC regtest or public testnet review.
Wallet code lives in the separate `organicbitcoin/obtcwallet` repository. The
binary names still inherit upstream names: `btcwallet`, `renewall`, and
`walletapp`.

Do not import a Bitcoin seed phrase, Bitcoin private key, or real wallet backup
into OBTC review software. Create a fresh test wallet.

## 1. Build Wallet

```bash
mkdir -p ~/obtc-demo
cd ~/obtc-demo

git clone https://github.com/organicbitcoin/obtcwallet.git
cd obtcwallet

go build -o ./btcwallet .
go build -o ./renewall ./cmd/renewall
go build -o ./walletapp ./cmd/walletapp
git rev-parse HEAD
```

Optional local gate:

```bash
go test $(go list ./... | grep -v github.com/btcsuite/btcwallet/chain) -count=1
```

## 2. Start A Node First

For regtest, start the node from [RUN_LOCAL_DEMO.md](RUN_LOCAL_DEMO.md) with
`KEEP_NODE=1`:

```bash
cd ~/obtc-demo/obtcd
RESET=1 KEEP_NODE=1 ./scripts/demo-regtest-expiry-reap.sh
```

For testnet, start the node from [RUN_TESTNET_NODE.md](RUN_TESTNET_NODE.md).

Use the matching network flag for the wallet:

| Network | Node RPC | Wallet legacy RPC | Wallet flag |
|---|---:|---:|---|
| regtest | `29528` | `29554` | `--obtcregtest` |
| testnet | `19528` | `19554` | `--obtctestnet` |
| mainnet-candidate | `9528` | `9554` | `--obtcmainnet` |

The examples below use regtest. Replace ports and flags consistently for
testnet.

## 3. Create A Fresh Test Wallet

```bash
mkdir -p ~/obtc-demo/data/obtcwallet-regtest
cd ~/obtc-demo/obtcwallet

./btcwallet --create \
  --obtcregtest \
  --appdata="$HOME/obtc-demo/data/obtcwallet-regtest"
```

The command prompts locally for wallet passphrases. Do not paste passphrases
into tickets, issues, or AI chat.

## 4. Start Wallet RPC

```bash
cd ~/obtc-demo/obtcwallet

./btcwallet --obtcregtest \
  --appdata="$HOME/obtc-demo/data/obtcwallet-regtest" \
  --rpcconnect=127.0.0.1:29528 \
  --btcdusername=obtc \
  --btcdpassword=obtcpass \
  --username=wallet \
  --password=walletpass \
  --rpclisten=127.0.0.1:29554 \
  --experimentalrpclisten=127.0.0.1:29556 \
  --noservertls \
  --noclienttls \
  --autorenew=0
```

Keep wallet RPC and the experimental gRPC listener bound to `127.0.0.1` for
reviewer machines.

## 5. Basic Wallet Checks

Legacy wallet RPC examples:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"height","method":"getblockcount","params":[]}' \
  http://127.0.0.1:29554/

curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"addr","method":"getnewaddress","params":[]}' \
  http://127.0.0.1:29554/

curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"bal","method":"getbalance","params":[]}' \
  http://127.0.0.1:29554/
```

If the wallet has no funds, mine to the wallet address on regtest by restarting
the node with that address as `--miningaddr`, or send funds from a deterministic
`devnetsim` wallet. Coinbase funds require 100 confirmations before they are
spendable.

## 6. Query Expiry

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"obtc","method":"obtc.getexpiry","params":[20]}' \
  http://127.0.0.1:29554/
```

Review these fields:

| Field | Meaning |
|---|---|
| `outpoint` | `txid:vout` for the wallet UTXO. |
| `amount_sat` | Output value. |
| `create_height` | Block height where the UTXO was created. |
| `expiry_height` | Height at which normal spending expires. |
| `blocks_to_expiry` | Remaining blocks at the wallet tip. |
| `status` | `ok`, `expiring`, or `expired`. |
| `dust_risk` | Whether projected refund value is below dust threshold. |
| `renewal_risk` | Wallet-side advisory state. |

Expiry is height based. Advancing regtest blocks changes the state; waiting for
wall-clock time does not.

## 7. Manual Renew

Start with a dry review of candidates:

```bash
./renewall \
  --connect=127.0.0.1:29556 \
  --notls \
  --amount=0.5 \
  --limit=10 \
  --dry-run
```

Manual renewal through legacy wallet RPC:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"renew","method":"obtc.renew","params":[["TXID:VOUT"],0.5]}' \
  http://127.0.0.1:29554/
```

Renew a specific outpoint to a specific target address with a fee limit and
minimum-confirmation requirement:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"renew","method":"obtc.renew","params":[["TXID:VOUT"],0.5,"TARGET_OBTC_ADDRESS",0.00005,1]}' \
  http://127.0.0.1:29554/
```

Parameter order is:

1. outpoint list;
2. renewal amount in OBTC;
3. optional target address;
4. optional max fee rate in OBTC/kB;
5. optional minimum confirmations.

Check the wallet repository `WALLET_RENEWAL_RUNBOOK.md` before funded
non-dry-run execution, because wallet-side operator defaults may be updated
outside this node repository.

After a renewal confirms, rerun `obtc.getexpiry`. The renewed output should have
a new create height and a later expiry height.

## 8. Renewall Execution

Non-dry-run `renewall` requires a synced, funded wallet and local passphrase
entry:

```bash
read -s WALLET_PRIVATE_PASSPHRASE

./renewall \
  --connect=127.0.0.1:29556 \
  --notls \
  --walletpass="$WALLET_PRIVATE_PASSPHRASE" \
  --amount=0.5 \
  --limit=5 \
  --target-address=TARGET_OBTC_ADDRESS \
  --maxfeerate=0.00005 \
  --minconf=1
```

Use `--dry-run` first and record the selected outpoints and max fee settings.
The current `renewall` CLI does not support the `publish_only` signer backend.

## 9. Locked And Unlocked Behavior

Expected review behavior:

- creating a wallet requires local passphrase setup;
- ordinary locked operation can query balance and expiry state;
- signing renewal transactions requires unlock or signer access;
- auto-renew is opt-in and should remain disabled unless the test explicitly
  covers it;
- locked wallets should not auto-sign.

To test unlock-sensitive flows, type the wallet private passphrase only into the
local terminal prompt or local environment. Do not include it in review reports.

## 10. Optional Local Wallet UI

```bash
./walletapp \
  --wallet-rpc=http://127.0.0.1:29554/ \
  --wallet-user=wallet \
  --wallet-pass=walletpass
```

Open `http://127.0.0.1:19580/` unless the app prints a different local URL.
The UI uses the already-running wallet RPC. It does not create or import
wallets.

## 11. Status Script With Wallet

If the node and wallet are both running:

```bash
cd ~/obtc-demo/obtcd

OBTC_RPC_PORT=29528 \
OBTC_RPC_USER=obtc \
OBTC_RPC_PASS=obtcpass \
OBTC_WALLET_RPC_URL=http://127.0.0.1:29554/ \
OBTC_WALLET_RPC_USER=wallet \
OBTC_WALLET_RPC_PASS=walletpass \
./scripts/status-obtc-demo.sh
```

The wallet section is best-effort. Legacy wallet RPC does not currently expose a
single stable auto-renew status field, so the script reports that field as
unknown or not exposed unless future wallet RPC adds it.
