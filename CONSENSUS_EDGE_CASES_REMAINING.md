# OBTC Consensus Edge Cases Remaining

This file lists consensus-adjacent scenarios that are not fully covered by the
focused tests added in Plan 03. Items marked "needs human confirmation" should
not be resolved by guessing from tests alone.

## P1 Remaining

| Area | Gap | Reason | Recommended Follow-up |
|---|---|---|---|
| Expiry index reorg evidence | Current tests cover connect/disconnect and long rollback mechanics, but do not include a reviewer-facing scenario that names "expiry commitment root after competing-chain reorg". | Existing expiryindex tests prove the mechanics, but the scenario is spread across recovery/rebuild/sequence tests. | Add a named integration test that connects chain A, disconnects to fork point, connects chain B, and asserts the accumulator root equals a rebuilt index. |
| Coinbase maturity for REAP tax outputs | REAP tax is accounted as transaction fee and paid through coinbase. Existing consensus code therefore uses normal coinbase maturity, but no OBTC-specific named test spends the REAP-tax-derived coinbase output after/before maturity. | The value is not separately tagged once it enters the coinbase output. | Add a full-block or rpctest scenario that overclaims/claims REAP tax, then attempts to spend the resulting coinbase before and after maturity. |
| Full-block exact REAP tax claim | Mining template tests assert coinbase value includes REAP tax. Full-block tests now reject overclaim, but do not connect a hand-built full block that claims exactly `subsidy + REAP tax`. | Template path covers the positive case; full-block path covers the negative overclaim case. | Add a full-block positive test if external reviewers want both positive and negative cases in the same package. |
| Taproot wrong-domain signature failure | Taproot tests cover missing option, replay option success, and VM success with replay-protected hash type. A test that signs with the wrong taproot domain but appends the replay bit would be more direct. | Current signing helpers compute the domain from the hash type and option, so constructing this requires lower-level manual schnorr signing. | Add explicit wrong-domain key-path and script-path taproot tests using manual digest construction. |

## P2 Remaining

| Area | Gap | Reason | Recommended Follow-up |
|---|---|---|---|
| REAP marker "multiple marker" error specificity | Multiple marker outputs are rejected because non-tail marker outputs are interpreted as invalid refund outputs. | Current implementation recognizes only the last output as the marker. | Needs human confirmation: decide whether consensus should keep this implicit rejection or add an explicit duplicate-marker check in a separate semantic-change review. |
| REAP marker missing error specificity | A version-3 transaction without a tail marker is treated as non-REAP and rejected if it spends expired UTXOs. | Current implementation defines REAP identity by version plus tail marker payload. | Needs human confirmation: decide whether "version 3 expired spend without marker" should remain non-REAP rejection or receive a dedicated marker-missing error. |
| Zero-input REAP-like transaction | `CheckTransactionSanity` rejects zero-input transactions as `ErrNoTxInputs` before REAP-specific checks. | This is current consensus pipeline behavior. | No protocol change in this plan. Keep documented unless a later consensus review explicitly changes semantics. |
| Prefix selection under very large live backlog | `mining/reap` tests cover deterministic prefix selection and cap behavior. They do not benchmark a production-scale index with millions of expired candidates in unit tests. | Unit tests should stay deterministic and fast. | Add a non-default stress or benchmark target for external performance review. |
| Expiry commitment in full block processing | `expiryindex` unit tests validate commitment rules, and mining adds commitments. Full block chain processing coverage depends on index manager wiring, not only block validation. | A full node-level scenario is heavier than unit coverage. | Add rpctest coverage that mines/submitblocks at activation with missing/mismatched commitments. |

## P3 Remaining

| Area | Gap | Reason | Recommended Follow-up |
|---|---|---|---|
| Error-code granularity for REAP tax mismatch | Tests assert `ErrBadTxOutValue` and error text for refund mismatch. | The code intentionally reuses broad transaction output error codes. | Only refine if reviewer asks for more precise error codes; that would be behavior/API cleanup, not required for consensus proof. |
| Property/fuzz tests for REAP accounting | Matrix tests include dust, threshold, normal, and large values, but not a fuzz/property test over all values. | Focused deterministic tests are easier for external review. | Add a bounded property test asserting `input = refund + tax` across representative ranges. |
| Full release test command profile | This plan runs all existing local tests, but does not create a new make target grouping only consensus tests. | Existing Makefile already has `unit`, `unit-race`, and integration targets. | Add `make check-obtc-consensus` later if reviewers want a single command. |

## Rules With Current Explicit Coverage

- Expired ordinary spends are rejected after expiry activation.
- Ordinary spends before expiry are accepted.
- Mainnet/testnet/regtest expiry windows are exercised at boundary heights.
- REAP non-expired inputs, missing outpoints, duplicate inputs, spoofed normal
  transactions, marker mismatch, canonical order, caps, and weight are covered.
- Tax/refund/dust accounting is covered for sub-dust, threshold, normal, and
  large values.
- Replay protection is covered for activation flagging, legacy VM, segwit v0
  VM, taproot key path, and taproot script path.
- Expiry commitment missing, mismatch, duplicate, unsupported version, and
  matching root are covered.
- Coinbase REAP tax overclaim is rejected.

## Manual Confirmation Required

- Whether REAP marker missing/duplicate should have dedicated consensus error
  codes instead of current implicit rejection paths.
- Whether a hand-built full-block exact `subsidy + REAP tax` acceptance test is
  required in addition to the mining template positive test.
- Whether taproot wrong-domain manual-signature tests are required before
  external pre-review.
