# Developer Resources

* [Code Contribution Guidelines](code-contribution-guidelines.md)

* [JSON-RPC Reference](../reference/json-rpc-api.md)
  * [RPC Examples](../reference/json-rpc-api.md#ExampleCode)

* The inherited btcd packages and OBTC extensions:
  * [rpcclient](../../rpcclient) - Implements a
    robust and easy to use Websocket-enabled Bitcoin JSON-RPC client
  * [btcjson](../../btcjson) - Provides an extensive API
    for the underlying JSON-RPC command and return values
  * [wire](../../wire) - Implements the wire protocol
  * [peer](../../peer) -
    Provides a common base for creating and managing Bitcoin network peers.
  * [blockchain](../../blockchain) -
    Implements block handling, chain selection, and OBTC validation rules
  * [blockchain/expiryindex](../../blockchain/expiryindex) -
    Implements OBTC expiry indexing and expiry commitments
  * [blockchain/fullblocktests](../../blockchain/fullblocktests) -
    Provides a set of block tests for testing the consensus validation rules
  * [txscript](../../txscript) -
    Implements the Bitcoin transaction scripting language
  * [btcec](../../btcec) - Implements
    support for the elliptic curve cryptographic functions needed for the
    Bitcoin scripts
  * [database](../../database) -
    Provides a database interface for the block chain
  * [mempool](../../mempool) -
    Package mempool provides a policy-enforced pool of unmined
    transactions.
  * [btcutil](../../btcutil) - Provides Bitcoin-derived
    convenience functions and types
  * [chainhash](../../chaincfg/chainhash) -
    Provides a generic hash type and associated functions that allows the
    specific hash algorithm to be abstracted.
  * [connmgr](../../connmgr) -
    Package connmgr implements a generic Bitcoin network connection manager.
