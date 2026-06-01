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
| OBTC mainnet fork anchor | `974000` provisional |
| First OBTC-independent block | `974001` |
| Mainnet activation height | `976016` |
| Expiry index start scan height | `0` |
| Expiry enforcement height | `976016` |
| REAP consensus height | `976016` |
| Replay protection height | `976016` |
| Expiry commitment mandatory height | `976016` |
| Expiry window | `362880` blocks |
| Expiry scan list batch limit | `10000` |
| REAP max inputs | `256` |
| REAP tax | `30 / 100` |
| REAP dust threshold | `720` satoshis |

The fork anchor is a mainnet-candidate engineering value, not a final public
launch commitment. Recalculate it during release freeze using current Bitcoin
height, miner readiness, seed readiness, and public testnet results. The
activation height is `ObtcMainNetForkHeight + 2016`.

## AuxPoW And Difficulty

| Field | Value |
|---|---|
| AuxPoW parent format | Bitcoin-style SHA-256d block header |
| AuxPoW chain ID | `20260` |
| AuxPoW start height | `974001` |
| Mixed mining | ordinary PoW and AuxPoW both accepted after `974001` |
| Parent BTC block acceptance | not required by OBTC consensus |
| Coinbase commitment prefix | required `fabe6d6d` merged-mining header |
| Coinbase commitment root | chain merkle root in reversed byte order |
| Serialized parent hash field | retained for compatibility, normalized to zero, ignored by consensus |
| First OBTC block difficulty | reset to `0x1d00ffff` |
| Bootstrap DAA | ASERT, 10 minute spacing, 1 hour half-life, anchor `974001` |
| Normal DAA | ASERT, 10 minute spacing, 48 hour half-life, anchor `976016` |
| AuxPoW RPCs | `createauxblock`, `submitauxblock`, `getauxblock` |
| AuxPoW RPC target fields | `target` and Namecoin-compatible `_target`, reversed byte order |

Public `ObtcTestNetParams` keeps AuxPoW disabled so existing public testnet
testing behavior is unchanged.

OBTC intentionally requires the `fabe6d6d` merged-mining header. It does not
accept the legacy Namecoin fallback form where the root appears near the start
of the coinbase without the header. Pools must use the explicit header form.

## Operational Notes

* Keep P2P on `9527`.
* Bind RPC to a private interface, normally `127.0.0.1:9528`.
* Treat the DNS seed placeholder as a release blocker.
* Treat fork height `974000` as provisional until release freeze.
* Use explicit `--addpeer` or `--connect` entries until seed infrastructure is
  final.
* Keep this document synchronized when changing `ObtcMainNetParams`, wire
  magic, default RPC ports, wallet net parameters, or OBTC expiry parameters.

## Verification Commands

```bash
rg -n "ObtcMainNetParams|ObtcMainNet|9527|9528|seed.obtc.example.com|HDCoinType|PrivateKeyID" \
  chaincfg wire params.go cmd

go test . ./chaincfg ./wire ./blockchain ./mining ./btcjson ./cmd/btcctl ./cmd/obtc-status
```
