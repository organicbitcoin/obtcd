reap
====

Package reap contains OBTC REAP transaction planning helpers.

It is responsible for:

- scanning expiry index candidates through a narrow interface
- selecting inputs deterministically under consensus limits
- applying dust and tax/refund rules
- building marker payloads that commit to selected inputs
- packing REAP transaction blueprints for mining templates
- producing dry-run plans for RPC observability

This package does not relay REAP transactions through the mempool. The mining
package uses it while building candidate blocks.

## License

Package reap is licensed under the [copyfree](http://copyfree.org) ISC License.
