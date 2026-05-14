btcjson
=======

Package btcjson implements concrete JSON-RPC command and result types.

The obtcd fork keeps the inherited btcd command set and adds OBTC extensions:

- `getexpiryindexstats`
- `listexpiring`
- `getreapplan`
- `getexpirycommitment`

The corresponding command and result structs live in `obtcextcmds.go` and
`obtcextresults.go`.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package btcjson is licensed under the [copyfree](http://copyfree.org) ISC
License.
