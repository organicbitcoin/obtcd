rpcclient
=========

Package rpcclient implements a websocket-enabled JSON-RPC client.

The package keeps the inherited btcd/btcwallet client behavior and adds typed
helpers for OBTC RPC extensions:

- `GetExpiryIndexStats`
- `ListExpiring`
- `GetReapPlan`
- `GetExpiryCommitment`

These wrappers are defined in `obtc_extensions.go` and use the btcjson OBTC
result types.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package rpcclient is licensed under the [copyfree](http://copyfree.org) ISC
License.
