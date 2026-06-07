# OBTC Mainnet Parameters

This page records the current `ObtcMainNetParams` values used by `obtcd`.
The code remains the source of truth; this document is an operator and review
checklist for the mainnet-candidate branch.

Source files:

* `chaincfg/params_obtc.go`
* `wire/protocol.go`
* `params.go`
* `cmd/btcctl/config.go`
* companion wallet reference: `obtcwallet/netparams/params.go`

## Network Identity

| Field | Value |
|---|---|
| Chaincfg name | `obtcmainnet` |
| CLI flag | `--obtcmainnet` |
| Wire magic | `0x4F425443` |
| P2P port | `9527` |
| Node RPC port | `9528` |
| Default `btcctl` RPC server | `127.0.0.1:9528` |
| Companion wallet RPC client port | `9528` |
| Companion wallet RPC server port | `9554` |
| Segwit Bech32 HRP | `obtc` |
| DNS seed | `seed.obtc.example.com` |

`seed.obtc.example.com` is still a placeholder. Replace it with final seed
infrastructure, or clear DNS seeds and use explicit peer bootstrapping, before
cutting a public mainnet release.

## Address And Key Namespaces

| Namespace | Value |
|---|---|
| P2PKH `PubKeyHashAddrID` | `0x47` |
| P2SH `ScriptHashAddrID` | `0x32` |
| WIF `PrivateKeyID` | `0x9A` |
| Witness pubkey hash `WitnessPubKeyHashAddrID` | `0x2A` |
| Witness script hash `WitnessScriptHashAddrID` | `0x2B` |
| HD private key version | `0B47B01E` |
| HD public key version | `0B47B5D4` |
| BIP44 coin type | `20260` |

These values intentionally avoid Bitcoin mainnet, Bitcoin testnet, and OBTC
test/regtest namespaces.

## Consensus Baseline

| Field | Value |
|---|---|
| Genesis block/hash | Bitcoin mainnet genesis |
| Proof-of-work limit bits | `0x1d00ffff` |
| Retargeting | enabled |
| BIP94 enforcement | enabled |
| BIP34 height | `1` |
| BIP65 height | `1` |
| BIP66 height | `1` |
| Coinbase maturity | `100` blocks |
| Subsidy reduction interval | `210000` blocks |
| Target timespan | `14 days` |
| Target block interval | `10 minutes` |
| Retarget adjustment factor | `4` |
| Minimum difficulty reduction | disabled |
| Local mining support | enabled |
| Checkpoints | none |
| Relay non-standard transactions | disabled |

Version-bits deployments:

| Deployment | Mainnet setting |
|---|---|
| CSV | always active from height `1` |
| Segwit | always active from height `1` |
| Taproot | always active from height `1` |
| Miner confirmation window | `2016` blocks |
| Rule change activation threshold | `1916` blocks |

## OBTC Fork And Expiry Parameters

| Field | Value |
|---|---|
| OBTC mainnet fork height | `1000000` (provisional; may change before final activation) |
| Mainnet activation height | `1002016` (derived from the provisional fork height) |
| Expiry index start scan height | `0` |
| Expiry enforcement height | `1002016` |
| REAP consensus height | `1002016` |
| Replay protection height | `1002016` |
| Expiry commitment mandatory height | `1002016` |
| Expiry window | `362880` blocks |
| Expiry scan list batch limit | `10000` |
| REAP max inputs | `256` |
| REAP tax | `30 / 100` |
| REAP dust threshold | `720` satoshis |

The activation height is `ObtcMainNetForkHeight + 2016`, matching the current
implementation. The current fork height is a mainnet-candidate value and may
change before final activation.

## Operational Notes

* Keep P2P on `9527`.
* Bind RPC to a private interface, normally `127.0.0.1:9528`.
* Treat the DNS seed placeholder as a release blocker.
* Use explicit `--addpeer` or `--connect` entries until seed infrastructure is
  final.
* Keep this document synchronized when changing `ObtcMainNetParams`, wire
  magic, default RPC ports, wallet net parameters, or OBTC expiry parameters.

## Verification Commands

```bash
rg -n "ObtcMainNetParams|ObtcMainNet|9527|9528|seed.obtc.example.com|HDCoinType|PrivateKeyID" \
  chaincfg wire params.go cmd

go test ./chaincfg ./wire ./cmd/btcctl ./cmd/obtc-status
```
