txscript
========

Package txscript implements Bitcoin transaction script parsing, signing helpers,
and validation.

The obtcd fork adds OBTC replay-protected sighash support:

- `SigHashOBTCReplayProtection`
- `ScriptVerifyOBTCReplayProtection`
- replay-protected legacy witness and taproot sighash domains

The script flag is activated by blockchain validation according to the active
OBTC chain parameters. Bitcoin networks keep the inherited behavior.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package txscript is licensed under the [copyfree](http://copyfree.org) ISC
License.
