# OBTC Demo Troubleshooting

This page covers common local demo, regtest, testnet, wallet, mining, expiry,
and REAP review failures.

## Wrong Network

Symptom:

- `getblockchaininfo.chain` is not the network you expected.

Fix:

- Use exactly one network flag.
- Local demo: `--obtcregtest`.
- Public testnet: `--obtctestnet`.
- Mainnet-candidate review: `--obtcmainnet`.

Do not mix node, wallet, and `btcctl` network flags.

## RPC Connection Refused

Symptom:

- `connection refused`;
- `status-obtc-demo: RPC connection failed`.

Fix:

- Confirm the node process is running.
- Confirm `--rpclisten` matches the CLI `--rpcserver`.
- Regtest default node RPC is `127.0.0.1:29528`.
- Testnet default node RPC is `127.0.0.1:19528`.
- If using the demo script, check `$OBTC_DEMO_DIR/obtcd.log`.

## RPC Authentication Failure

Symptom:

- HTTP `401`;
- `authorization failed`;
- repeated auth errors in node logs.

Fix:

- Use the same `--rpcuser` and `--rpcpass` in node and CLI commands.
- The runbooks use `obtc` and `obtcpass` only as local examples.
- Do not paste RPC credentials into public issues.

## TLS Mismatch

Symptom:

- HTTP client speaks plain HTTP to a TLS endpoint, or the opposite.

Fix:

- The demo scripts start node RPC with `--notls`.
- Pass `--notls` to `btcctl`.
- Use `http://` URLs for curl when `--notls` is set.

## Port Already In Use

Symptom:

- node fails to bind `29527`, `29528`, `19527`, or `19528`.

Fix:

- Stop the old node, or choose alternate ports.
- Demo example:

```bash
OBTC_RPC_PORT=30528 OBTC_P2P_PORT=30527 RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

Then pass the same RPC port to `btcctl` or `scripts/status-obtc-demo.sh`.

## Expiry Index Disabled

Symptom:

- `getexpiryindexstats.disabled` is `true`;
- `listexpiring` reports the expiry index is disabled;
- `getreapplan` says `expiry index disabled`.

Fix:

- Start the node with `--expiryindex`.
- For review runs, also use `--txindex` unless you have a reason not to.
- If enabling the index on an existing data directory, allow it to catch up or
  rebuild from a clean demo directory.

## Expiry Indexed Tip Lags

Symptom:

- node height is ahead of `getexpiryindexstats.tip_height`.

Fix:

- Wait for indexing to catch up.
- Check logs for index errors.
- On a disposable demo, run the script with `RESET=1`.

## No Expiring UTXOs

Symptom:

- `listexpiring` returns an empty list.

Fix:

- Query the right expiry-height range.
- In regtest, the window is 144 blocks. Outputs created at height `H` expire at
  `H + 144`.
- Mine more blocks, or create ordinary outputs with `devnetsim prepare`.

## REAP Plan Has Zero Picks

Symptom:

- `getreapplan.picked` is `0`.

Fix:

- Confirm the next block height is at or after the REAP enable height.
- Confirm expired candidates exist in `listexpiring 0 <next_height>`.
- Confirm the node was started with `--expiryindex`.
- Check whether candidates were already processed by previous REAP blocks.

## REAP-Like Mempool Transaction Rejected

Symptom:

- a user-created version-3 transaction with a `REAP:` marker is rejected by
  mempool policy.

Expected behavior:

- REAP is a block-internal system transaction built by mining/template code.
- It is not normal mempool relay.
- Use `getreapplan` and mined blocks to inspect REAP behavior.

Focused test:

```bash
go test ./mempool -run 'RejectREAP|REAPRejectsOrphan' -count=1
```

## Mining Fails Without Address

Symptom:

- `generate` fails because no mining address is configured.

Fix:

- Start the node with `--miningaddr=<obtc address>`.
- For local regtest, use:

```bash
./devnetsim miningaddr \
  --network obtcregtest \
  --statefile ~/obtc-demo/data/devnetsim-state.json \
  --seed-tag demo-miner
```

Then pass the printed address to `btcd --miningaddr`.

## Coinbase Funds Not Spendable

Symptom:

- balance is visible but not spendable;
- `devnetsim prepare` cannot find enough spendable funds;
- wallet cannot spend mined funds yet.

Fix:

- Coinbase maturity is 100 blocks.
- Mine at least 100 additional blocks after the coinbase output.
- Confirm wallet and node are synced to the same height.

## Wallet Cannot Connect To Node

Symptom:

- wallet reports btcd RPC connection errors.

Fix:

- Match wallet network flag to node network flag.
- Regtest node RPC is `127.0.0.1:29528`.
- Testnet node RPC is `127.0.0.1:19528`.
- Wallet flags must use node credentials:
  `--btcdusername` and `--btcdpassword`.
- If the node uses `--notls`, wallet examples use `--noclienttls`.

## Wallet RPC Not Reachable

Symptom:

- curl to wallet RPC fails.

Fix:

- Confirm `btcwallet` is running.
- Confirm `--rpclisten` matches the URL.
- Regtest wallet legacy RPC default is `29554`.
- Testnet wallet legacy RPC default is `19554`.
- Confirm wallet RPC user/password match curl credentials.

## Wallet Locked For Renewal

Symptom:

- renewal signing fails with locked-wallet or decrypt-key errors.

Fix:

- Query-only calls such as `obtc.getexpiry` can run while locked.
- Signing renewal transactions requires local unlock or signer access.
- Type wallet passphrases only into a local terminal prompt or local
  environment. Do not include passphrases in issues or review transcripts.

## Renewall Dry Run Finds No Candidates

Symptom:

- `renewall --dry-run` prints no selected candidates.

Fix:

- Confirm the wallet has funded UTXOs.
- Confirm wallet and node are synced.
- Adjust renewal window filters.
- Use `obtc.getexpiry` to inspect statuses and `blocks_to_expiry`.

## Expiry Commitment Mismatch Is Hard To Demo Manually

Symptom:

- reviewer wants to see mismatch rejection by hand.

Recommendation:

- Use automated tests for malformed, missing, duplicate, and mismatched roots:

```bash
go test ./blockchain/expiryindex -run 'Commitment' -count=1
```

Hand-mutating a mined block is not the fastest reviewer path.

## Replay Protection Is Hard To Demo Manually

Symptom:

- reviewer wants to sign both protected and unprotected transaction variants.

Recommendation:

- Use automated tests unless your review specifically covers transaction
  signing internals:

```bash
go test ./mempool -run 'ReplayProtection' -count=1
go test ./blockchain -run 'Replay|OBTC' -count=1
```

Do not use a real Bitcoin transaction or real Bitcoin private key.

## Go Build Fails

Symptom:

- parser or module errors during `go build`.

Fix:

- Confirm `go version` is 1.24.6 or newer for wallet builds.
- Run from the repository root.
- Run `go env GOPATH GOMOD GOTOOLCHAIN` for diagnostics.
- If module cache corruption is suspected, retry after `go clean -modcache`.

## Testnet Has No Peers

Symptom:

- `getpeerinfo` returns `[]`.

Fix:

- Confirm outbound network access to TCP `19527`.
- Check firewall or VPN rules.
- Add explicit peers from the current testnet runbook.
- Confirm the node is using `--obtctestnet`, not regtest or mainnet.

## Stale Demo State

Symptom:

- demo output does not match the documented heights;
- old blocks or ports are reused.

Fix:

- Run:

```bash
RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

- Or set a fresh directory:

```bash
OBTC_DEMO_DIR=/tmp/obtc-demo-clean RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

## What To Include In A Bug Report

Include redacted:

- OS and architecture;
- Go version;
- `git rev-parse HEAD` for `obtcd` and `obtcwallet`;
- exact command and network flag;
- node height and best hash;
- expiry indexed tip and commitment root;
- relevant RPC output;
- log excerpt around the error.

Do not include seed phrases, private keys, wallet passphrases, RPC passwords, or
private wallet backups.
