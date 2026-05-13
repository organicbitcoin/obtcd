# OBTC Node Documentation

This repository contains the OBTC node implementation derived from btcd. The
binary names and Go module path stay close to upstream btcd to reduce merge
friction, but the runtime networks, ports, address namespaces, expiry index,
REAP validation, and replay-protection behavior are OBTC-specific.

Start with the OBTC operator documents first. Use the inherited btcd reference
documents when you need details about configuration, RPC, networking, Docker,
mining, or development practices that are still shared with btcd.

## Start Here

* [Getting Started](getting-started.md)
* [OBTC Mainnet Join Runbook](mainnet-join.md)
* [OBTC Testnet Join Guide](testnet-join.md)
* [Network Parameters](network-parameters.md)
* [OBTC Mainnet Parameters](mainnet-params.md)

## Reference

* [Configuration](reference/configuration.md)
* [JSON RPC API](reference/json-rpc-api.md)
* [Controlling btcd with btcctl](reference/controlling.md)
* [Mining](reference/mining.md)
* [Wallet Boundary](reference/wallet.md)
* [Tor](reference/tor.md)
* [Docker](reference/docker.md)
* [Update](reference/update.md)

## Development

* [Developer Resources](development/developer-resources.md)
* [Code Contribution Guidelines](development/code-contribution-guidelines.md)
* [Code Formatting Rules](development/code-formatting-rules.md)
* [Contact](development/contact.md)

## Heritage

* [btcd Heritage Notes](heritage/btcd-notes.md)

## License

btcd and obtcd are licensed under the [copyfree](http://copyfree.org) ISC
License.
