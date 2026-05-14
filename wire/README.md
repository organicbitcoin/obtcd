wire
====

Package wire implements Bitcoin P2P message encoding and decoding.

The obtcd fork adds OBTC network identifiers:

- `ObtcMainNet`
- `ObtcTestNet`
- `ObtcRegNet`

These magic values isolate OBTC peers from Bitcoin peers while retaining the
inherited Bitcoin wire message formats.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package wire is licensed under the [copyfree](http://copyfree.org) ISC License.
