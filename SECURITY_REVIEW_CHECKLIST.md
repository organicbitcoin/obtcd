# OBTC Mainnet Candidate Security Review Checklist

This checklist is for external technical review. It is not a statement that all
items have been independently audited.

For each item, the reviewer should verify expected behavior, inspect the test
location, and decide whether manual review needs additional evidence.

## Consensus Validation Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| OBTC network selection | Consensus rules apply only on OBTC params | `blockchain/consensus_obtc_edge_test.go`, `chaincfg/params_obtc.go` | Check no Bitcoin network path inherits OBTC expiry rules |
| Expired normal spend | Non-REAP spend of expired UTXO is rejected after activation | `blockchain/fullblocks_obtc_test.go`, `blockchain/validation_reap_test.go` | Review boundary at activation height |
| Non-expired spend | Normal spend of non-expired UTXO remains valid if otherwise valid | `blockchain/validation_reap_test.go` | Check wallet renewal does not depend on mempool REAP behavior |
| Coinbase value | Coinbase upper bound includes fees plus valid REAP tax only | `mining/newblocktemplate_accounting_and_helpers_test.go`, `blockchain/validation_reap_test.go` | Review exact/under/over claim behavior |

## REAP Transaction Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Forged marker | Marker digest mismatch is rejected | `blockchain/validation_reap_test.go`, `mining/reap/marker_vector_test.go` | Recompute digest over ordered inputs |
| Non-expired REAP input | REAP-like tx spending non-expired input is rejected | `blockchain/validation_reap_test.go` | Check current height and input create height |
| Reordered inputs | Inputs must follow canonical order | `blockchain/fullblocks_obtc_test.go`, `mining/reap/selector_test.go` | Order is expiry, amount, outpoint |
| Oversized input set | REAP normal and dust caps are enforced | `blockchain/validation_reap_test.go`, `mining/reap/budget_test.go` | Review cap values against `docs/mainnet-params.md` |
| Weight budget | REAP tx remains within configured budget | `mining/reap/budget_test.go`, `mining/reap/params_test.go` | Confirm no normal transaction starvation beyond reserved budget |
| Dust fold | Refundless dust inputs fold into tax as specified | `mining/reap/dust_test.go`, `mining/reap/dust_extreme_test.go` | Confirm refund output absence is expected for dust |
| Refund outputs | Refunds are grouped and deterministic | `mining/reap/packer_test.go` | Check output script grouping and marker-at-tail |

## Replay Protection Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Activation | Replay protection activates at configured OBTC height | `blockchain/validation_obtc_replay_test.go`, `mempool/policy_matrix_test.go` | Mainnet candidate height is `1000001` |
| Missing protection | Post-activation signatures without OBTC domain fail | `blockchain/scriptval_obtc_test.go` | Check wallet signing path uses OBTC params |
| Mempool consistency | Mempool policy and block validation agree | `mempool/policy_matrix_test.go`, `blockchain/validation_obtc_replay_test.go` | Review orphan and replacement cases |

## Expiry Commitment Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Missing commitment | Missing mandatory coinbase commitment is rejected | `blockchain/expiryindex/commitment_test.go` | Applies at/after activation |
| Mismatch | Mismatched root is rejected | `blockchain/expiryindex/commitment_test.go` | Compare against local accumulator root |
| Duplicate | Duplicate commitments are rejected | `blockchain/expiryindex/commitment_test.go` | Check last-output ambiguity is not accepted |
| Format/version | Malformed or unsupported version is rejected | `blockchain/expiryindex/commitment_edge_test.go` | Check OP_RETURN format |
| Recovery | Recovery rebuilds consistent root | `blockchain/expiryindex/recovery_integration_test.go` | Review after restart/reindex |

## Expiry Index Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Connect/disconnect | Index updates on block connect and disconnect | `blockchain/expiryindex/expiryindex_test.go` | Check spent and created UTXO handling |
| Reorg safety | Reorg restores correct expiry state | `blockchain/expiryindex/reorg_safety_test.go` | Challenge stale expiry entry scenarios |
| Rebuild | Rebuild matches live index | `blockchain/expiryindex/rebuild_test.go`, `blockchain/expiryindex/reindex_test.go` | Operator uses `--reindex-expiry` |
| Scan order | Scan and REAP prefix are deterministic | `blockchain/expiryindex/reap_prefix_test.go` | Compare order with selector |

## Mining/Template Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Template append | REAP appended when expired candidates exist | `mining/newblocktemplate_reap_template_tests_test.go` | Verify no candidate means no REAP |
| Reserved budget | Normal tx selection leaves REAP room | `mining/newblocktemplate_reap_boundary_test.go` | Review block full behavior |
| Conflicts | Mempool tx conflicts with REAP are handled deterministically | `mining/newblocktemplate_reap_conflict_and_dependency_test.go` | Check no discretionary selection |
| Coinbase accounting | REAP tax is reflected in coinbase value limit | `mining/newblocktemplate_accounting_and_helpers_test.go` | Check refund not counted as tax |

## Wallet Renewal Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Expiry visibility | Wallet exposes UTXO expiry status | `obtcwallet/rpc/legacyrpc/obtc_methods_test.go` | Confirm `ok`, `expiring`, `expired` fields |
| Manual renew | Selected outpoints and target amount are validated | `obtcwallet/wallet/renewal_lifecycle_test.go` | Confirm target address and fee limit behavior |
| Renewall dry-run | Dry-run does not sign or publish | `obtcwallet/cmd/renewall/main_test.go` | Use before any funded run |
| Auto-renew default | Auto-renew remains opt-in and disabled by default | `obtcwallet/AUTO_RENEW_SAFETY_NOTES.md`, `obtcwallet/wallet/autorenew_test.go` | Check operator config before enabling |
| Locked wallet | Locked wallet should not auto-sign | `obtcwallet/WALLET_LIFECYCLE_TESTS.md` | Human product wording review remains useful |

## Mempool Isolation Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Fake REAP | User-broadcast REAP-like tx is rejected | `mempool/reap_policy_test.go` | REAP is block-internal |
| Orphan path | REAP-like orphan is rejected before pooling | `mempool/policy_matrix_test.go` | Check orphan pollution |
| Marker-like non-REAP | Non-version-3 marker-like tx is not overclassified | `mempool/reap_policy_extra_test.go` | Confirm policy shape |

## Reorg/Rebuild Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Reorg disconnect | Expiry index removes disconnected effects | `blockchain/expiryindex/reorg_safety_test.go` | Test stale expiry entry after reorg |
| Fast rebuild | Rebuilt index matches live index | `blockchain/expiryindex/rebuild_test.go` | Use `--reindex-expiry` for operator recovery |
| Shadow validation | Shadow tool compares index behavior | `cmd/obtc-expiryindex-shadow/main_test.go` | Consider longer external observation |

## Snapshot/Pruned Node Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Pruned mode claims | No unsupported pruned-node claim is made in this package | `KNOWN_LIMITATIONS.md` | Require separate evidence if pruned support becomes release scope |
| Snapshot restore | No snapshot restore support is claimed here | `KNOWN_LIMITATIONS.md` | Require separate evidence before documenting |

## Logging/Observability Checklist

| Item | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Node status | `obtc-status` reports chain, peers, expiry, commitment, and REAP plan | `cmd/obtc-status/status_test.go` | Confirm RPC bound to private interface |
| Seed preflight | Seed script checks peers, tip, P2P, and optional expiry index | `scripts/phase6/seed_preflight.sh` | Capture output for release evidence |
| 72h observation | Collector appends repeated node snapshots | `scripts/phase6/collect_72h_observation.sh` | Requires operator-run public observation window |

## Known Attack Scenarios

| Scenario | Expected behavior | Test location | Manual review note |
|---|---|---|---|
| Forged marker | Reject marker digest/count/height mismatch | `blockchain/validation_reap_test.go` | Recompute digest manually for sample tx |
| Non-expired REAP input | Reject REAP-like spend | `blockchain/validation_reap_test.go` | Review expiry calculation |
| Reordered REAP inputs | Reject non-canonical order | `blockchain/fullblocks_obtc_test.go` | Compare prefix source |
| Oversized REAP input set | Enforce caps and weight | `blockchain/validation_reap_test.go`, `mining/reap/budget_test.go` | Check both dust and normal caps |
| Coinbase overclaim | Reject block claiming above allowed value | `mining/newblocktemplate_accounting_and_helpers_test.go` | Include REAP tax only |
| Replay attack | Reject missing OBTC replay protection after activation | `blockchain/validation_obtc_replay_test.go` | Mainnet height `1000001` |
| Fake mempool REAP | Reject from mempool | `mempool/reap_policy_test.go` | REAP comes from block template |
| Poisoned index/rebuild | Rebuild detects/repairs by chain state | `blockchain/expiryindex/rebuild_test.go` | Operator recovery uses clean rebuild |
| Stale expiry entry after reorg | Reorg safety tests preserve state | `blockchain/expiryindex/reorg_safety_test.go` | Challenge disconnect/connect order |
| Auto-renew runaway | Disabled default, caps, budget, backoff | `obtcwallet/AUTO_RENEW_SAFETY_NOTES.md` | Do not enable without local policy review |
