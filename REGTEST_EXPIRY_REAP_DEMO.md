# Regtest Expiry And REAP Demo

This document explains the local regtest path exercised by
`scripts/demo-regtest-expiry-reap.sh`. It is for protocol and mining reviewers.
It does not describe mainnet behavior.

## 1. Regtest Parameters Used

Current `obtcregtest` lifecycle parameters:

| Parameter | Value |
|---|---:|
| OBTC regtest fork height | `100` |
| expiry window | `144` blocks |
| expiry enforcement enable height | `110` |
| expiry commitment enable height | `110` |
| canonical REAP consensus height | `112` |
| replay protection height | `114` |
| REAP normal input cap | `200` |
| REAP dust input cap | `400` |
| REAP max weight | `400000` weight units |
| REAP tax ratio | `30/100` |
| REAP dust threshold | `720` sat |

These are regtest review parameters. Do not copy them into mainnet operating
material.

## 2. Run The Demo

```bash
cd ~/obtc-demo/obtcd
RESET=1 ./scripts/demo-regtest-expiry-reap.sh
```

The script:

1. builds `btcd`, `btcctl`, `devnetsim`, and `obtc-status`;
2. starts `btcd --obtcregtest --txindex --expiryindex --notls`;
3. mines to height `120`, past expiry, commitment, REAP, and replay activation;
4. prints chain info, expiry index stats, expiry commitment state, and
   `getreapplan`;
5. mines to height `144`, where the next block height is the first coinbase
   expiry height under the regtest window;
6. prints `listexpiring 145 145 20` and the next-block REAP plan;
7. mines one more block and prints the full verbose block;
8. prints minimal status JSON.

Use `KEEP_NODE=1` to leave RPC online:

```bash
RESET=1 KEEP_NODE=1 ./scripts/demo-regtest-expiry-reap.sh
```

## 3. Manual RPC Commands

CLI helper shape:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls <method> [params...]
```

Check current height:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblockcount
```

Check expiry index:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpiryindexstats
```

Scan UTXOs by expiry height:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls listexpiring 120 160 20
```

Read the next-block REAP dry run:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getreapplan
```

Mine one block:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls generate 1
```

## 4. Observe Expiry State

`listexpiring` returns:

- `txid` and `vout`;
- `create_height`;
- `expiry_height`;
- `blocks_to_expiry`;
- `amount_sat`.

Examples:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls listexpiring 120 130 20

./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls listexpiring 144 144 20
```

At height `120`, early outputs are approaching expiry. At height `144`, the
next block is `145`, and outputs created at height `1` have reached the regtest
expiry window for next-block REAP planning. Normal spending eligibility is based
on chain height, not wall-clock time.

Wallet-side `ok`, `expiring`, and `expired` labels are shown through
`obtc.getexpiry`; see [RUN_WALLET.md](RUN_WALLET.md).

## 5. Observe REAP Template Planning

`getreapplan` is a read-only dry run for the next block. It reports:

- `height`;
- `enabled` and `active`;
- `picked`;
- `tax_total`;
- `refund_total`;
- `est_weight`;
- `marker_hash`;
- optional `reason`.

Before candidates expire, `picked` should be `0`. Once expired candidates exist,
`picked` should be positive if the selector can build a budgeted prefix.

REAP selection is block-internal mining/template behavior. It is not mempool
relay and is not a user-broadcast transaction flow.

## 6. Inspect The REAP Transaction

After mining the REAP block:

```bash
HEIGHT="$(./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblockcount)"

HASH="$(./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblockhash "$HEIGHT")"

./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getblock "$HASH" 2
```

Look for a non-coinbase transaction with:

- `version: 3`;
- inputs spending expired outpoints;
- refund outputs before the marker output, when refunds are not dust-folded;
- the last output with value `0` and an `OP_RETURN` marker payload shaped as
  `REAP:<height>:<input_count>:<digest>`.

The marker digest commits to the ordered REAP inputs. The refund outputs are
grouped by script and sorted by script. Dust inputs below the configured dust
threshold can fold fully into tax and therefore may not create refund outputs.

## 7. Tax Accounting

The REAP transaction does not pay tax by creating an explicit miner output.
Instead, REAP input value minus refund output value is accounted as the REAP tax
and increases the coinbase upper bound for that block.

For a next-block plan:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getreapplan
```

Review:

- `tax_total`;
- `refund_total`;
- `picked`;
- `est_weight`.

Consensus and mining tests covering overclaim, exact claim, underclaim, refund
exclusion, and dust folding are in:

- `mining/newblocktemplate_reap_template_tests_test.go`
- `mining/newblocktemplate_accounting_and_helpers_test.go`
- `blockchain/validation_reap_test.go`
- `blockchain/validation_reap_extra_test.go`

## 8. Canonical Selection And Caps

The selector uses deterministic canonical order:

1. expiry height;
2. amount;
3. outpoint.

It then applies normal input cap, dust input cap, and weight budget rules while
preserving a canonical prefix. There is no discretionary skip of earlier
candidates in favor of later candidates.

Relevant tests:

- `mining/newblocktemplate_reap_template_tests_test.go`
- `mining/reap/selector_test.go`
- `mining/reap/budget_test.go`
- `mining/reap/stress_regression_test.go`
- `blockchain/expiryindex/reap_prefix_test.go`
- `blockchain/fullblocks_obtc_test.go`

## 9. Mempool Isolation

REAP system transactions are not ordinary user transactions and should not be
relayed through the mempool. User-created fake REAP-like transactions are
rejected by mempool policy.

Focused tests:

```bash
go test ./mempool -run 'RejectREAP|REAPRejectsOrphan|Replay' -count=1
```

Relevant files:

- `mempool/reap_policy_test.go`
- `mempool/reap_policy_extra_test.go`
- `mempool/policy_matrix_test.go`

## 10. Expiry Commitment

Current commitment state:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getexpirycommitment
```

Fields:

- `enabled`;
- `root`;
- `tip_height`;
- `tip_hash`;
- `enable_at_height`;
- `active`;
- `active_at_next_height`.

Before regtest height `110`, the commitment RPC can report index state, but the
coinbase commitment is not yet mandatory. At and after height `110`, block
coinbase transactions must include a valid expiry commitment root. Mismatch,
missing, duplicate, and malformed commitment rejection are easier to verify in
automated tests than by hand-mutating a mined block:

```bash
go test ./blockchain/expiryindex -run 'Commitment' -count=1
```

Relevant files:

- `blockchain/expiryindex/commitment_test.go`
- `blockchain/expiryindex/commitment_edge_test.go`
- `blockchain/expiryindex/recovery_integration_test.go`

## 11. Replay Protection

Regtest replay protection activates at height `114`. Before height `114`, the
post-fork replay-protection rule is not active on regtest. At and after height
`114`, normal signed OBTC transactions must use the OBTC replay-protected
signature domain, and missing replay protection is rejected.

Manual CLI demonstration requires constructing and signing pre-activation and
post-activation transactions with different sighash flags. For a reviewer demo,
the deterministic test coverage is the preferred evidence:

```bash
go test ./mempool -run 'ReplayProtection' -count=1
go test ./blockchain -run 'Replay|OBTC' -count=1
```

Relevant files:

- `mempool/policy_matrix_test.go`
- `blockchain/validation_obtc_replay_test.go`
- `blockchain/scriptval_obtc_test.go`

Do not use a real Bitcoin transaction or a real Bitcoin private key to test
replay protection.

## 12. Lightweight Backlog Observation

The one-command demo creates enough early block outputs to observe REAP append
behavior. For larger local backlog pressure, use `devnetsim prepare` to create
more confirmed local outputs, mine past their expiry heights, and observe
`getreapplan` over multiple blocks:

```bash
./devnetsim prepare \
  --network obtcregtest \
  --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc \
  --rpcpass=obtcpass \
  --statefile ~/obtc-demo/data/devnetsim-state.json \
  --seed-tag demo-miner \
  --utxos 256 \
  --value 300000 \
  --fee-rate 10 \
  --fanout-size 64
```

Then mine forward by the regtest expiry window and sample:

```bash
./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls generate 144

./btcctl --obtcregtest --rpcserver=127.0.0.1:29528 \
  --rpcuser=obtc --rpcpass=obtcpass --notls getreapplan
```

For deterministic, resource-bounded stress coverage, prefer the unit and
integration tests listed above. They avoid laptop-specific mining speed and I/O
variance.
