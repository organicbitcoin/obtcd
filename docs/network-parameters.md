# Network Parameters

The table below summarizes the public OBTC network namespaces used by this
repository.

| Network | Flag | P2P | Node RPC | Bech32 HRP | Magic |
|---|---|---:|---:|---|---|
| OBTC mainnet | `--obtcmainnet` | `9527` | `9528` | `obtc` | `0x4F425443` |
| OBTC testnet | `--obtctestnet` | `19527` | `19528` | `obtct` | `0x4F544553` |
| OBTC regtest | `--obtcregtest` | `29527` | `29528` | `obtcrt` | `0x4F524547` |

Mainnet key namespaces:

| Field | Value |
|---|---|
| WIF `PrivateKeyID` | `0x9A` |
| HD private version | `0B47B01E` |
| HD public version | `0B47B5D4` |
| BIP44 coin type | `20260` |

Source of truth:

* `chaincfg/params_obtc.go`
* `wire/protocol.go`
* `params.go`
