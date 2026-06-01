# OBTC Mainnet Join Runbook

> Status: draft for `mainnet-candidate-2026-10`

This runbook describes how to start and validate an OBTC mainnet-candidate node
with the current `obtcd` codebase. It is not a production mainnet launch
guarantee. Seed replacement, public observation, and release hardening are still
active launch work.

Before using this runbook, review [OBTC Mainnet Parameters](mainnet-params.md).

## Network Baseline

| Field | Value |
|---|---|
| Network flag | `--obtcmainnet` |
| Network name | `obtcmainnet` |
| P2P port | `9527` |
| Node RPC port | `9528` |
| Bech32 HRP | `obtc` |
| Wire magic | `0x4F425443` |
| Fork anchor | `974000` provisional |
| First OBTC-independent block | `974001` |
| OBTC activation height | `976016` |
| AuxPoW chain ID | `20260` |

The code still contains the placeholder DNS seed `seed.obtc.example.com`.
Mainnet-candidate bootstrap should therefore use explicit peers until final DNS
seed or fallback-node policy is published.

Fork anchor `974000` is provisional. Recalculate the final public value during
release freeze using current Bitcoin height, miner readiness, seed readiness,
and testnet results.

## Build

Build from the repository root:

```bash
go build -o ./btcd .
go build -o ./btcctl ./cmd/btcctl
go build -o ./obtc-status ./cmd/obtc-status
```

Run focused parameter checks before starting a public node:

```bash
go test . ./chaincfg ./wire ./blockchain ./mining ./btcjson ./cmd/btcctl ./cmd/obtc-status
```

## Start A Mainnet-Candidate Node

Choose a data directory with enough disk capacity for a full node and expiry
index state.

```bash
./btcd \
  --obtcmainnet \
  --datadir=$HOME/.obtcd-mainnet \
  --listen=0.0.0.0:9527 \
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

Use `--connect=<peer:9527>` instead of `--addpeer=<peer:9527>` only when the
node must connect exclusively to the listed peers.

## Validate Node Health

Check RPC connectivity:

```bash
./btcctl \
  --obtcmainnet \
  --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --notls \
  getblockchaininfo
```

Check peer state:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getconnectioncount

./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getpeerinfo
```

Check expiry index state when `--expiryindex` is enabled:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getexpiryindexstats

./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls listexpiring 0 99999999 100
```

Run the status page locally:

```bash
./obtc-status \
  --obtcmainnet \
  --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> \
  --rpcpass=<rpc_pass> \
  --listen=127.0.0.1:8080
```

Check mining and AuxPoW metadata:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls getmininginfo
```

After height `974001`, merged-mining pools can request work with:

```bash
./btcctl --obtcmainnet --rpcserver=127.0.0.1:9528 \
  --rpcuser=<rpc_user> --rpcpass=<rpc_pass> --notls createauxblock <obtc_address>
```

`submitauxblock <hash> <auxpow>` submits the serialized AuxPoW proof for the
hash returned by `createauxblock`. Ordinary OBTC PoW blocks remain valid after
the fork; AuxPoW is an additional accepted proof format.

## Seed Node Criteria

A public seed or fallback node candidate should meet the following minimum
criteria before it is listed in release material:

* TCP `9527` reachable from the public Internet.
* RPC bound to localhost or a private management interface.
* `--txindex` and `--expiryindex` enabled for observer nodes.
* Stable host identity, monitoring, log rotation, and disk alerts configured.
* At least one successful fresh-node bootstrap using the advertised peer list.
* Peer count, tip height, and expiry index state captured as release evidence.

## Security Baseline

* Do not expose RPC port `9528` publicly.
* Use strong unique RPC credentials.
* Prefer firewall rules that only expose TCP `9527`.
* Keep node logs and validation snapshots outside public web roots.
* Treat all mainnet-candidate keys, mining endpoints, and operator credentials
  as production-sensitive even before final release.

## Troubleshooting

### No Peers

* Confirm the node was started with `--obtcmainnet`.
* Confirm outbound TCP access to peer port `9527`.
* Confirm inbound TCP `9527` if this node is expected to accept peers.
* Replace placeholder peers with the current published peer list.
* Use `getaddednodeinfo true` and `getpeerinfo` to inspect connection state.

### RPC Works But Expiry RPC Fails

Restart with `--expiryindex`. If the node already has chain data, use the
documented reindex procedure for rebuilding expiry index state.

### Wrong Network Or Address Prefix

Confirm the run command uses `--obtcmainnet`. Mainnet addresses use Bech32 HRP
`obtc`; testnet and regtest use different namespaces.

## Open Release Checklist

Track these items before promoting this draft to a final public join guide:

* [ ] Replace `seed.obtc.example.com`, or document that DNS seed is intentionally
  unused for the candidate release. See
  <https://github.com/organicbitcoin/obtcd/issues/2>.
* [ ] Recalculate and freeze the final fork anchor; current code uses
  provisional `974000`.
* [ ] Publish the initial `--addpeer` or `--connect` peer list.
* [ ] Confirm at least 3 long-lived seed/fallback nodes across independent
  regions or providers.
* [ ] Add final DNS seed and fallback-node policy to release notes.
* [ ] Capture fresh-node sync evidence from a clean data directory.
* [ ] Capture 72-hour public observation evidence.
* [ ] Add signed release artifact and checksum links when releases are cut.
* [ ] Re-run `docs/mainnet-params.md` verification commands after any parameter
  change.
