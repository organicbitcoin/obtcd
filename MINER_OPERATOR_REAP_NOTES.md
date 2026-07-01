# Miner Operator REAP Notes

REAP is a block-internal system transaction. It is constructed by the mining
template path from the node's expired UTXO view and is validated by consensus
rules when the block is checked.

## Why REAP Does Not Use Mempool

REAP spends expired UTXOs according to global canonical rules. Ordinary mempool
relay would let users or miners submit arbitrary REAP-like transactions, which
would create ordering games and invalid partial selections. OBTC therefore
rejects likely REAP system transactions at mempool admission and constructs the
valid REAP transaction inside `getblocktemplate` / mining code.

## Candidate Discovery

The mining template code asks the expiry index for the global expired live UTXO
prefix at the next block height. The expiry index tracks UTXOs by expiry key and
keeps a strict REAP candidate bucket that includes amount and outpoint ordering.

If there are no expired candidates, no REAP transaction is appended. Indexed but
unexpired UTXOs are ignored until their expiry height is reached.

## Canonical Order

REAP inputs are selected by this key:

```text
expiry height -> amount -> outpoint
```

The template must use the prefix of that global order. It may stop because of
caps or weight limits, but it must not skip an earlier candidate in order to
select a later one.

## Caps And Weight Budget

Current network parameters define:

- Normal REAP input cap.
- Dust REAP input cap for refundless dust inputs.
- REAP transaction weight budget.

Template construction stops at the first reached tier cap and trims only from
the tail when applying budget limits. This preserves the canonical prefix.
Regular mempool transactions are selected with REAP headroom reserved when a
REAP transaction is planned, so REAP cannot grow without bound and ordinary
transactions can still enter blocks unless the block is full.

## Coinbase Accounting

REAP tax is represented as the REAP transaction fee:

```text
tax = sum(expired inputs) - sum(REAP refund outputs)
```

The miner's coinbase upper bound is:

```text
block subsidy + ordinary transaction fees + REAP tax
```

Refund outputs stay in the REAP transaction and are not miner revenue. A miner
may claim less than the upper bound, may claim exactly the upper bound, and must
not claim more. Consensus rejects overclaim.

## Operator Metrics To Watch

- Expiry index tip height versus chain tip height.
- Expiry commitment root/tip synchronization.
- `getreapplan` candidate count, picked count, skipped count, estimated weight,
  refund total, and tax total.
- Number of mined REAP blocks.
- REAP transaction input count and weight.
- Template logs for REAP build skipped/appended reasons.
- Coinbase value compared with subsidy plus ordinary fees plus REAP tax.
- Mempool size should not include REAP system transactions.

## Common Failure Causes

- Expiry index disabled, missing, corrupt, or behind the chain tip.
- Expiry commitment source not synchronized with the best chain snapshot.
- No candidates are expired at the next block height.
- Candidate backlog exists but the first candidate cannot fit the effective
  hard block weight.
- REAP marker height, count, or digest mismatch.
- Refund output total or grouping mismatch.
- Coinbase overclaims beyond subsidy plus fees plus REAP tax.
- Node is before the network's REAP activation height.

## Testnet And Regtest Verification

On regtest or testnet, use the following checks:

```sh
go test ./mining -run 'REAP|Accounting|Boundary|Template' -count=1
go test ./mining/reap -run 'Select|Budget|Blueprint|Pressure|Stress' -count=1
go test ./mempool -run 'REAP|FakeMarkers' -count=1
go test ./blockchain ./blockchain/expiryindex -run 'REAP|ReapPrefix|CoinbaseOverclaim' -count=1
```

For a live node, compare `getreapplan` before mining with the next mined block:
the mined REAP inputs should be the same canonical prefix, the marker digest
should match those inputs, refund outputs should match the source scripts, and
coinbase should claim no more than subsidy plus ordinary fees plus REAP tax.
