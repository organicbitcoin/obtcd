# OBTC Reviewer Primer

This primer is for reviewers who can read code but do not already know the
OBTC protocol changes. It explains the baseline Bitcoin data structures first,
then maps the OBTC-specific mechanisms onto those structures.

This is review material only. It is not a mainnet launch announcement and not a
claim that a formal third-party audit has been completed.

## Reading Map

Useful starting points:

- `chaincfg/params_obtc.go`: OBTC network and expiry parameters.
- `blockchain/validation_reap.go`: expiry and REAP validation.
- `blockchain/validation_obtc_replay.go`: replay-protection activation.
- `blockchain/expiryindex/`: persisted expiry index and commitment logic.
- `mining/template_reap.go` and `mining/reap/`: mining-side REAP construction.
- `mempool/`: mempool policy, including REAP isolation and replay activation.
- Sibling wallet repo `organicbitcoin/obtcwallet`: expiry display, renewal, and
  auto-renew logic.

For a focused review, start with one file in `review-cards/`.

## UTXO

A UTXO is an unspent transaction output. A transaction creates outputs, and a
later transaction spends an output by referencing it as an input. A node's UTXO
set is the current set of spendable outputs.

Important fields for OBTC review:

- value in satoshis;
- locking script (`PkScript`);
- creation height;
- whether the output is coinbase;
- whether it has already been spent.

OBTC adds expiry semantics to confirmed UTXOs. A UTXO has an expiry key derived
from its creation height:

```text
expiry_height = create_height + WindowBlocks
```

The implementation lives in `chaincfg/params_obtc.go` as
`(*ExpiryParams).CalculateExpiryKey`. Mainnet-candidate `WindowBlocks` is
`362880` blocks. Testnet and regtest use shorter windows so reviewers can run
local tests.

After expiry activation, a normal transaction must not spend an expired UTXO.
A REAP transaction must not spend a non-expired UTXO. That split is enforced in
`blockchain/validation_reap.go`.

## OutPoint

An OutPoint identifies exactly one transaction output:

```text
txid:vout
```

In code it is `wire.OutPoint`, containing a transaction hash and output index.
Reviewers will see OutPoints in:

- transaction inputs;
- expiry-index keys;
- REAP marker digest inputs;
- wallet renewal RPC parameters.

OBTC relies on deterministic OutPoint ordering when multiple expired UTXOs are
eligible for REAP. The canonical order used for REAP review is:

1. expiry key;
2. amount;
3. OutPoint.

This prevents miners from cherry-picking arbitrary expired UTXOs when a global
expired backlog exists.

## Coinbase

The coinbase transaction is the first transaction in a block. It creates the
block subsidy and claims fees. It has no normal inputs.

OBTC adds two coinbase-sensitive review areas:

- expiry commitments: the coinbase includes an `OP_RETURN` commitment to the
  expiry-index state when commitments are mandatory;
- REAP accounting: valid REAP tax can increase the amount claimable by the
  coinbase, but REAP refunds must not be counted as miner income.

Commitment format and validation are in `blockchain/expiryindex/commitment.go`
and `blockchain/expiryindex/expiryindex.go`. Mining template accounting tests
are in `mining/newblocktemplate_accounting_and_helpers_test.go`, and consensus
overclaim coverage is in `blockchain/consensus_obtc_edge_test.go`.

## Mempool

The mempool is the node's pool of valid unconfirmed transactions. It is policy
state, not consensus state: a transaction can be valid by consensus but still
not accepted into a node's mempool.

OBTC uses the mempool for ordinary user transactions, but REAP transactions are
system transactions constructed for blocks. A fake or user-broadcast REAP-like
transaction should not enter the mempool. Review `mempool/reap_policy_test.go`
and `mempool/policy_matrix_test.go` for the isolation rules.

Replay protection also appears in mempool policy. After replay activation,
transactions must use the OBTC replay-protected signature hash domain.

## Block Template

A block template is the block a miner is asked to work on. It includes:

- coinbase transaction;
- selected mempool transactions;
- required commitments;
- optional system transactions such as REAP, when valid candidates exist.

OBTC mining template construction is split across:

- `mining/template_reap.go`: attach REAP when expiry is active and candidates
  exist;
- `mining/reap/selector.go`: select a deterministic prefix of expired UTXOs;
- `mining/reap/packer.go`: build refund outputs and the marker output;
- `mining/newblocktemplate_*_test.go`: template and accounting behavior.

The template path is not allowed to invent looser rules than consensus. A REAP
transaction built by mining code must also pass `blockchain/validation_reap.go`.

## Reorg

A reorg replaces the current best chain with another valid branch. During a
reorg, blocks are disconnected from the old branch and blocks are connected from
the new branch.

OBTC expiry state must survive this correctly:

- UTXOs created by disconnected blocks must be removed from the expiry index;
- UTXOs spent by disconnected blocks must be restored;
- the expiry accumulator root must return to the state implied by the active
  chain;
- REAP prefix scans must not keep stale entries from the disconnected branch.

The core code is in `blockchain/expiryindex/expiryindex.go`, with reorg tests
in `blockchain/expiryindex/reorg_safety_test.go`.

## OBTC Mechanisms In One Pass

OBTC keeps the Bitcoin UTXO model and adds lifecycle constraints:

- replay protection separates post-fork OBTC signatures from Bitcoin
  signatures;
- expiry assigns each confirmed UTXO an expiry height;
- normal spending rejects expired UTXOs after activation;
- REAP allows expired UTXOs to be reclaimed through a constrained system
  transaction;
- REAP tax/refund rules split expired value between a miner-claimable tax and
  deterministic refunds, with sub-dust inputs folded into tax;
- the expiry index tracks live UTXOs by expiry key and commits to that state in
  coinbase data;
- wallet renewal lets holders create a fresh UTXO before expiry, subject to
  wallet-side fee, confirmation, and safety checks.

The review goal is not to trust a single full-project review. The goal is to
make each mechanism public, locally testable, and independently challengeable.

## Suggested First Hour

```bash
go test ./chaincfg -run 'OBTC|Expiry' -count=1
go test ./txscript -run 'OBTCReplay|ReplayProtection' -count=1
go test ./mempool -run 'REAP|Replay' -count=1
go test ./mining/reap -run 'Marker|Select|Blueprint|Tax|Dust|Weight' -count=1
go test ./blockchain/expiryindex -run 'Commitment|ReapPrefix|Reorg|Rebuild|Expiry' -count=1
go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|Consensus' -count=1
```

For wallet renewal behavior, use the sibling `obtcwallet` checkout:

```bash
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -count=1
```
