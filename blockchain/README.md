blockchain
==========

Package blockchain implements block processing, validation, and best-chain
selection for obtcd.

The package retains the upstream Bitcoin validation architecture and adds the
OBTC consensus paths needed by this fork:

- expiry-aware transaction input checks
- REAP marker, ordering, tax/refund, and spend validation
- expiry commitment validation against the expiry index accumulator
- replay-protection script flag activation from chaincfg parameters

## Related OBTC Code

- `validation_reap.go` implements REAP consensus checks.
- `validation_obtc_replay.go` activates replay-protected script validation.
- `expiry_chain_accessor.go` adapts the chain state for expiry index rebuilds.
- `expiryindex/` tracks expiry-height buckets and accumulator commitments.

## Module Path

The repository still uses the upstream Go module path
`github.com/btcsuite/btcd`. For OBTC behavior, build from this repository
checkout rather than installing the upstream module with `go get`.

## License

Package blockchain is licensed under the [copyfree](http://copyfree.org) ISC
License.
