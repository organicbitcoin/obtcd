chaincfg
========

Package chaincfg defines network parameters used by obtcd and packages that
need to encode addresses, keys, wire messages, or network-specific consensus
activation state.

This fork keeps the upstream Bitcoin network parameters and adds OBTC mainnet,
testnet, and regtest in `params_obtc.go`.

## OBTC Parameters

OBTC networks are isolated from Bitcoin networks through distinct:

- wire magic values
- default P2P ports
- address and private-key prefixes
- extended-key versions
- BIP44 coin types
- fork heights and activation heights

The package also exposes helpers for OBTC-specific behavior:

- `IsOBTC`
- `GetOBTCForkHeight`
- `IsPostOBTCFork`
- `GetOBTCReplayProtectionHeight`
- `IsOBTCReplayProtectionActive`
- `GetExpiryParams`

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package chaincfg is licensed under the [copyfree](http://copyfree.org) ISC
License.
