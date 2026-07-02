# OBTC Review Test Vectors

This file is a compact valid / invalid case index for modular review. The
canonical executable vectors are the listed tests. Reviewers are encouraged to
add smaller standalone fixtures if a case is difficult to reproduce from the
current unit or integration tests.

For copyable concrete inputs and expected results, see
[REVIEW_FIXTURE_VECTORS.md](REVIEW_FIXTURE_VECTORS.md).

This is not a formal audit report.

## Common Commands

```bash
go test ./chaincfg ./txscript ./mempool -run 'OBTC|Replay|REAP' -count=1
go test ./blockchain -run 'REAP|Replay|OBTCFullBlock|Consensus|Coinbase' -count=1
go test ./blockchain/expiryindex -run 'Commitment|ReapPrefix|Reorg|Rebuild|Expiry|Staircase|Pressure' -count=1
go test ./mining ./mining/reap -run 'REAP|Template|Accounting|Boundary|Marker|Select|Blueprint|Dust|Tax|Weight' -count=1
```

Wallet cases require the sibling `obtcwallet` checkout:

```bash
go test ./wallet ./rpc/legacyrpc ./rpc/rpcserver ./cmd/renewall -run 'Expiry|Renew|AutoRenew|GetRenew|RunRenewAll|Unsynced' -count=1
```

## Replay Protection

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| RP-V-001 | OBTC replay flag activates at the configured OBTC height | Valid protected signatures are accepted after activation | `blockchain/validation_obtc_replay_test.go`, `chaincfg/params_obtc_test.go` |
| RP-I-001 | Bitcoin network params at the same height | OBTC replay flag is not enabled | `blockchain/validation_obtc_replay_test.go` |
| RP-I-002 | Post-activation signature without OBTC replay domain | Rejected by script validation | `txscript/sighash_obtc_replay_test.go`, `blockchain/scriptval_obtc_test.go` |
| RP-V-002 | Legacy, SegWit v0 P2WPKH, SegWit v0 P2WSH multisig, Taproot key path, and Taproot script path protected hash matrix | `SIGHASH_ALL`, `SIGHASH_NONE`, `SIGHASH_SINGLE`, and all valid `ANYONECANPAY` combinations are accepted with the OBTC replay bit | `txscript/sighash_obtc_replay_test.go`, `txscript/taproot_obtc_replay_test.go` |
| RP-I-003 | Mempool transaction crosses activation without protected domain | Rejected by mempool policy | `mempool/policy_matrix_test.go` |
| RP-I-004 | Missing replay bit, unknown extra bits, Taproot default, or base type `0` after activation | Rejected by script validation | `txscript/sighash_obtc_replay_test.go`, `txscript/taproot_obtc_replay_test.go` |

## Expiry Index

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| EI-V-001 | UTXO created at height `h` with window `w` | Expiry key equals `h + w` | `chaincfg/params_obtc_test.go`, `blockchain/expiryindex/expiryindex_test.go` |
| EI-V-002 | Connect block with spendable outputs | Index stores OutPoints under the expected expiry key | `blockchain/expiryindex/expiryindex_test.go` |
| EI-I-001 | Missing mandatory expiry commitment after activation | Block rejected | `blockchain/expiryindex/commitment_test.go` |
| EI-I-002 | Duplicate or mismatched expiry commitment | Block rejected | `blockchain/expiryindex/commitment_test.go` |
| EI-V-003 | Disconnect and reconnect the same block | Root and scan output return to the expected state | `blockchain/expiryindex/reorg_safety_test.go` |
| EI-I-003 | Reorg drops old branch UTXOs | Stale entries are absent and rebuild matches live index | `blockchain/expiryindex/reorg_safety_test.go` |

## REAP

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| REAP-V-001 | REAP spends confirmed expired UTXO with correct marker and refunds | Accepted | `blockchain/fullblocks_obtc_test.go`, `blockchain/validation_reap_test.go` |
| REAP-I-001 | Ordinary transaction spends expired UTXO after activation | Rejected | `blockchain/validation_reap_test.go`, `blockchain/fullblocks_obtc_test.go` |
| REAP-I-002 | REAP spends non-expired UTXO | Rejected | `blockchain/validation_reap_test.go` |
| REAP-I-003 | Marker digest, count, or height mismatch | Rejected | `blockchain/validation_reap_test.go`, `mining/reap/marker_vector_test.go` |
| REAP-I-004 | Multiple REAP transactions after hardening | Rejected | `blockchain/validation_reap_test.go`, `blockchain/fullblocks_obtc_test.go` |
| REAP-I-005 | REAP input set is not the global canonical prefix | Rejected | `blockchain/fullblocks_obtc_test.go`, `blockchain/validation_reap_test.go` |
| REAP-I-006 | Normal-tier or dust-tier input cap exceeded | Rejected | `blockchain/validation_reap_test.go`, `mining/reap/selector_test.go` |
| REAP-I-007 | REAP transaction weight exceeds the configured hard limit | Rejected | `blockchain/validation_reap_test.go`, `mining/reap/budget_test.go` |
| REAP-V-002 | Dust input below threshold | Refund folds into tax, marker remains final output | `mining/reap/dust_test.go`, `mining/reap/dust_extreme_test.go` |
| REAP-I-008 | Fake REAP-like transaction submitted to mempool | Rejected before mempool admission | `mempool/reap_policy_test.go`, `mempool/policy_matrix_test.go` |

## Coinbase Accounting

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| CB-V-001 | Template includes normal transaction fees and REAP tax | Coinbase value equals subsidy plus all valid fee entries | `mining/newblocktemplate_accounting_and_helpers_test.go` |
| CB-V-002 | Template has no REAP candidates | Coinbase accounting excludes REAP tax | `mining/newblocktemplate_accounting_and_helpers_test.go` |
| CB-I-001 | Coinbase claims more than subsidy plus normal fees plus REAP tax | Block rejected | `blockchain/consensus_obtc_edge_test.go` |
| CB-I-002 | REAP refund is counted as miner income | Accounting invariant fails or block is rejected | `mining/newblocktemplate_accounting_and_helpers_test.go`, `blockchain/validation_reap_test.go` |
| CB-I-003 | Missing, duplicate, malformed, or wrong-root expiry commitment | Block rejected after commitment activation | `blockchain/expiryindex/commitment_test.go`, `blockchain/expiryindex/commitment_edge_test.go` |

## Wallet Renewal

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| WR-V-001 | `obtc.getexpiry` on confirmed UTXOs | Returns outpoint, amount, create height, expiry height, status, warnings | `obtcwallet/rpc/legacyrpc/obtc_methods_test.go` |
| WR-V-002 | Manual renewal selects one explicit OutPoint | Signed transaction spends selected OutPoint and creates target output | `obtcwallet/wallet/renewal_lifecycle_test.go` |
| WR-I-001 | Empty or malformed renewal OutPoint | RPC parameter validation fails before wallet interaction | `obtcwallet/rpc/legacyrpc/obtc_methods_test.go` |
| WR-I-002 | Invalid amount, fee rate, or minconf | RPC parameter validation fails | `obtcwallet/rpc/legacyrpc/obtc_methods_test.go` |
| WR-V-003 | `renewall --dry-run` with candidates | Prints selected OutPoints without signing or submitting | `obtcwallet/cmd/renewall/main_test.go` |
| WR-I-003 | Unsynced wallet state | Renewal preview or submit path rejects | `obtcwallet/rpc/rpcserver/agentwallet_server_test.go`, `obtcwallet/cmd/renewall/main_test.go` |
| WR-V-004 | Auto-renew disabled by default | No scheduler run is configured | `obtcwallet/wallet/autorenew_test.go`, `obtcwallet/config_autorenew_test.go` |
| WR-I-004 | Auto-renew invalid window, fee, amount, interval, or budget | Configuration validation fails | `obtcwallet/wallet/autorenew_test.go`, `obtcwallet/config_autorenew_test.go` |
| WR-I-005 | Auto-renew locked-wallet or over-budget run | Candidate execution is skipped, limited, or fails without unsafe signing | `obtcwallet/wallet/renewal_lifecycle_test.go`, `obtcwallet/wallet/autorenew_test.go` |

## Historical Backlog

| ID | Case | Expected Result | Executable Reference |
|---|---|---|---|
| HB-V-001 | Many expired UTXOs share the same expiry key | Scanner pagination preserves deterministic order | `blockchain/expiryindex/scan_staircase_test.go`, `blockchain/expiryindex/pressure_theoretical_max_test.go` |
| HB-V-002 | Backlog exceeds one REAP transaction budget | Selector processes only the allowed prefix and carries the rest forward | `mining/reap/staircase_pressure_test.go`, `cmd/obtc-utxo-export/preview_test.go` |
| HB-I-001 | Source order is shuffled or reversed | Strict selector still produces the same canonical prefix | `mining/reap/stress_regression_test.go` |
