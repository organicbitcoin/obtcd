# OBTC Mainnet Candidate Wallet Operator Runbook

Wallet operations use the companion repository:

<https://github.com/organicbitcoin/obtcwallet>

This runbook is for testnet/regtest review and Mainnet Candidate operator
preparation. It is not an instruction to import real Bitcoin wallet material.

## Build `obtcwallet`

```bash
git clone https://github.com/organicbitcoin/obtcwallet.git
cd obtcwallet
git rev-parse HEAD

go build -o ./btcwallet .
go build -o ./renewall ./cmd/renewall
go build -o ./walletapp ./cmd/walletapp
```

Local focused test:

```bash
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1
```

## Create A Test Wallet

For testnet:

```bash
mkdir -p ~/obtc-testnet/data/obtcwallet

./btcwallet --create \
  --obtctestnet \
  --appdata="$HOME/obtc-testnet/data/obtcwallet"
```

For regtest, use `--obtcregtest` and a separate data directory. Type wallet
passphrases only into the local terminal prompt.

## Connect Wallet To Node

Testnet example:

```bash
./btcwallet --obtctestnet \
  --appdata="$HOME/obtc-testnet/data/obtcwallet" \
  --rpcconnect=127.0.0.1:19528 \
  --btcdusername=obtc \
  --btcdpassword=obtcpass \
  --username=wallet \
  --password=walletpass \
  --rpclisten=127.0.0.1:19554 \
  --experimentalrpclisten=127.0.0.1:19556 \
  --noservertls \
  --noclienttls \
  --autorenew=0
```

Keep wallet RPC and the experimental gRPC listener on loopback unless you have a
separate access-control plan.

## View Wallet UTXOs And Balance

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"balance","method":"getbalance","params":[]}' \
  http://127.0.0.1:19554/

curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"utxo","method":"listunspent","params":[]}' \
  http://127.0.0.1:19554/
```

Generate a test receive address:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"addr","method":"getnewaddress","params":[]}' \
  http://127.0.0.1:19554/
```

## View Expiry

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"expiry","method":"obtc.getexpiry","params":[20]}' \
  http://127.0.0.1:19554/
```

Review:

- `outpoint`;
- `amount_sat`;
- `create_height`;
- `expiry_height`;
- `blocks_to_expiry`;
- `status`;
- `dust_risk`;
- `renewal_risk`.

## Manual Renew

Dry review the candidate first with `obtc.getexpiry`, then renew selected
outpoints:

```bash
curl --user wallet:walletpass \
  -H 'content-type: text/plain;' \
  --data-binary '{"jsonrpc":"1.0","id":"renew","method":"obtc.renew","params":[["TXID:VOUT"],0.5,"TARGET_OBTC_ADDRESS",0.00005,1]}' \
  http://127.0.0.1:19554/
```

Parameter order:

1. outpoint list;
2. renewal amount in OBTC;
3. optional target address;
4. optional max fee rate in OBTC/kB;
5. optional minimum confirmations.

After confirmation, run `obtc.getexpiry` again and verify the new output create
height and expiry height.

## Renewall Dry-Run

Dry-run must not sign or publish:

```bash
./renewall \
  --connect=127.0.0.1:19556 \
  --notls \
  --amount=0.5 \
  --limit=10 \
  --dry-run
```

Useful filters:

```bash
./renewall --connect=127.0.0.1:19556 --notls --amount=0.5 --dry-run --window-start=52560 --window-end=25920
./renewall --connect=127.0.0.1:19556 --notls --amount=0.5 --dry-run --include-near-expiry
```

## Configure Auto-Renew

Auto-renew is opt-in and should remain disabled unless this run explicitly tests
it.

Example controlled test configuration:

```bash
./btcwallet --obtctestnet \
  --autorenew=1 \
  --autorenewamount=0.5 \
  --autorenewinterval=30m \
  --autorenewfailurebackoff=15m \
  --autorenewwindowstart=52560 \
  --autorenewwindowend=25920 \
  --autorenewmaxutxos=10 \
  --autorenewmaxfeerate=5000 \
  --autorenewmaxrenewamountperrun=2.5
```

Before enabling:

- run `renewall --dry-run`;
- confirm wallet is synced;
- confirm fee and budget settings;
- confirm operator monitoring and logs;
- confirm rollback plan.

## Auto-Renew Logs

Review logs for:

- candidate count;
- selected outpoints;
- skipped reasons;
- fee-limit or budget-limit skips;
- renewal success and failure counts;
- backoff active messages;
- locked-wallet messages.

Logs must not contain seed phrases, private keys, or wallet passphrases.

## Fee Limit Handling

Use:

- `obtc.renew` max fee rate parameter;
- `renewall --maxfeerate`;
- auto-renew `--autorenewmaxfeerate`.

If a fee limit blocks renewal, record the configured limit, wallet height,
selected outpoint, and error message. Do not raise fee limits without an
operator decision.

## Budget Limit Handling

Use:

- `renewall --limit`;
- auto-renew `--autorenewmaxutxos`;
- auto-renew `--autorenewmaxrenewamountperrun`.

Budget truncation should be visible in run logs or dry-run output. If the
wallet selects unexpected candidates, file a wallet issue with redacted output.

## Wallet Locked

Expected behavior:

- `obtc.getexpiry` can be queried while locked;
- signing requires unlock or local signer access;
- locked wallets should not auto-sign;
- local passphrases should be typed only into a local terminal or local
  environment.

## Private-Key Safety

Do not use a Bitcoin seed phrase, Bitcoin private key, real wallet backup, or
real-fund claim attempt in this review flow.

## Wallet Bug Report

Use:

<https://github.com/organicbitcoin/obtcwallet/issues>

Include:

- wallet commit;
- node commit;
- network flag;
- exact wallet and node commands with secrets removed;
- wallet height and node height;
- `obtc.getexpiry` output with sensitive fields removed;
- txid if a testnet/regtest renewal was submitted;
- expected and observed behavior.
