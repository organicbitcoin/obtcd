# OBTC Test Coverage And Gaps

Audit date: 2026-06-30

Scope: OBTC-specific or OBTC-relevant tests in `obtcd`, `obtcwallet`, and CI.
This file does not list every inherited upstream btcd/btcwallet test.

## Coverage Inventory

### Consensus And Parameter Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:15` | OBTC network magic uniqueness. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:137` | Address/key namespace isolation. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:246` | OBTC port uniqueness. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:321` | Fork height lookup. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:418` | Fork height value sanity. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:458` | Replay protection activation. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:513` | Expiry parameter resolution. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:597` | Versionbits/deployment config. |
| `obtcd/chaincfg` | `chaincfg/params_obtc_test.go:621` | Namespace isolation validation. |

### Replay Protection Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/blockchain` | `blockchain/validation_obtc_replay_test.go:14` | Height-gated script flag activation. |
| `obtcd/txscript` | `txscript/sighash_obtc_replay_test.go:38` | Taproot sighash option rejected by default. |
| `obtcd/txscript` | `txscript/sighash_obtc_replay_test.go:48` | OBTC replay domain gating. |
| `obtcd/txscript` | `txscript/sighash_obtc_replay_test.go:66` | Legacy/witness replay domain tag gating. |
| `obtcd/txscript` | `txscript/taproot_obtc_replay_test.go:96` | Taproot signature hash option. |
| `obtcd/mempool` | `mempool/policy_matrix_test.go:165` | Mempool activation boundary. |
| `obtcd/mempool` | `mempool/policy_matrix_test.go:226` | Orphan resolution behavior. |
| `obtcd/mempool` | `mempool/policy_matrix_test.go:310` | Replacement matrix behavior. |
| `obtcd/integration` | `integration/obtc_integration_test.go:96` | Legacy replay activation in rpctest. |
| `obtcd/integration` | `integration/obtc_integration_test.go:204` | SegWit replay activation in rpctest. |
| `obtcd/integration` | `integration/obtc_integration_test.go:260` | Taproot replay activation in rpctest. |

### Expiry And REAP Consensus Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:114` | Ordinary spend of expired UTXO rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:137` | REAP spend of non-expired UTXO rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:152` | REAP marker digest mismatch rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:228` | REAP marker count mismatch rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:238` | Multiple REAP transactions rejected after hardening. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:305` | Global canonical prefix validation. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:401` | Canonical input order enforced. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:427` | Input count consensus limits. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:455` | Dust tier accepted at cap. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:480` | Dust tier cap exceeded rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:502` | Normal tier accepted at cap. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:523` | REAP max weight exceeded rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:568` | Normal tier cap exceeded rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:712` | Input after dust cap rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:736` | Input after normal cap rejected. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:762` | Expected refund distribution required. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:777` | Expected refund distribution accepted. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:787` | Dust-only expired input accepted without refund output. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:800` | Expiry enable boundary/idempotence. |
| `obtcd/blockchain` | `blockchain/validation_reap_test.go:893` | REAP spending unconfirmed parent rejected. |

### Expiry Commitment And Expiry Index Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/blockchain/expiryindex` | `commitment_test.go:16` | Commitment build/extract round trip. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:79` | Duplicate commitment counting. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:104` | Non-canonical length rejection. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:166` | Missing commitment rejected after activation. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:196` | Before-activation no-op. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:216` | Mismatched root rejected. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:254` | Duplicate commitment rejected. |
| `obtcd/blockchain/expiryindex` | `commitment_test.go:288` | Matching commitment accepted. |
| `obtcd/blockchain/expiryindex` | `expiryindex_test.go:940` | Connect/disconnect block. |
| `obtcd/blockchain/expiryindex` | `expiryindex_test.go:1275` | Round trip with mixed inputs. |
| `obtcd/blockchain/expiryindex` | `sequence_fuzz_test.go:20` | Long connect/disconnect rollback sequence. |
| `obtcd/blockchain/expiryindex` | `recovery_integration_test.go:66` | Recovery catch-up matches live state. |
| `obtcd/blockchain/expiryindex` | `reap_prefix_test.go:22` | Strict REAP prefix order and limit. |
| `obtcd/blockchain/expiryindex` | `encode_test.go:15` | Encoding and ordering helpers. |
| `obtcd/blockchain/expiryindex` | `accumulator_test.go:51` | Accumulator entry data and digest behavior. |

### Mining Template And REAP Builder Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/mining` | `mining/template_reap_test.go` | REAP template creation and activation behavior. |
| `obtcd/mining` | `mining/newblocktemplate_reap_boundary_test.go` | Template boundary behavior near activation and commitments. |
| `obtcd/mining` | `mining/newblocktemplate_accounting_and_helpers_test.go:55` | Coinbase fee accounting consistency. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:52` | Deterministic candidate selection. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:85` | Max input and weight budget selection. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:159` | Dust tier cap behavior. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:189` | Normal tier cap behavior. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:443` | Tax rounding invariant. |
| `obtcd/mining/reap` | `mining/reap/selector_test.go:533` | Tip cutoff and selection stats. |

### Mempool And Policy Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/mempool` | `mempool/reap_policy_test.go:17` | REAP system transactions rejected from mempool. |
| `obtcd/mempool` | `mempool/reap_policy_extra_test.go:17` | Non-v3 lookalikes not over-rejected. |
| `obtcd/mempool` | `mempool/reap_policy_extra_test.go:76` | REAP rejection with multiple outputs. |
| `obtcd/mempool` | `mempool/reap_policy_extra_test.go:112` | High-fee path does not bypass REAP rejection. |
| `obtcd/mempool` | `mempool/policy_matrix_test.go:491` | REAP orphan rejection before pooling. |

### RPC And Integration Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcd/rpc` | `rpcserver_obtc_test.go:105` | Disabled `listexpiring` behavior. |
| `obtcd/rpc` | `rpcserver_obtc_test.go:177` | Expiry index stats. |
| `obtcd/rpc` | `rpcserver_obtc_test.go:325` | Disabled REAP plan. |
| `obtcd/rpc` | `rpcserver_obtc_test.go:372` | REAP plan not active before height. |
| `obtcd/rpc` | `rpcserver_obtc_test.go:425` | Disabled expiry commitment RPC. |
| `obtcd/rpc` | `rpcserver_obtc_test.go:440` | Enabled expiry commitment RPC. |
| `obtcd/integration` | `integration/obtc_integration_test.go:799` | RPC observability and filters with `--expiryindex`. |
| `obtcd/integration` | `integration/obtc_integration_test.go:911` | `listexpiring` ordering and cursor contract. |

### Wallet Tests

| Module | Tests | Coverage |
|---|---|---|
| `obtcwallet/wallet` | `wallet/expiry_test.go:9` | Expiry height calculation. |
| `obtcwallet/wallet` | `wallet/expiry_test.go:23` | Expiry status boundaries. |
| `obtcwallet/wallet` | `wallet/expiry_test.go:88` | Renewal risk boundaries. |
| `obtcwallet/wallet` | `wallet/expiry_test.go:203` | Spendability uses next block height. |
| `obtcwallet/wallet` | `wallet/expiry_policy_test.go:30` | Mainnet policy resolution. |
| `obtcwallet/wallet` | `wallet/expiry_policy_test.go:52` | Testnet policy resolution. |
| `obtcwallet/wallet` | `wallet/expiry_policy_test.go:74` | Regtest policy resolution. |
| `obtcwallet/wallet` | `wallet/utxos_test.go:98` | Spendable views skip expired OBTC UTXOs. |
| `obtcwallet/wallet` | `wallet/createtx_test.go:604` | Coin selection skips expired OBTC UTXOs. |
| `obtcwallet/rpc/legacyrpc` | `rpc/legacyrpc/obtc_methods_test.go:15` | `getexpiry` result helper. |
| `obtcwallet/rpc/legacyrpc` | `rpc/legacyrpc/obtc_methods_test.go:209` | `obtc.renew` parameter validation. |
| `obtcwallet/cmd/renewall` | `cmd/renewall/main_test.go:308` | Dry-run uses agent risk query. |
| `obtcwallet/cmd/renewall` | `cmd/renewall/main_test.go:397` | Execution via agent flow. |
| `obtcwallet/cmd/renewall` | `cmd/renewall/main_test.go:552` | Scan continues after preview failure. |
| `obtcwallet/cmd/renewall` | `cmd/renewall/main_test.go:629` | `publish_only` rejected. |
| `obtcwallet` | `config_autorenew_test.go:25` | Auto-renew option parsing. |
| `obtcwallet/wallet` | `wallet/autorenew_test.go:12` | Auto-renew defaults. |
| `obtcwallet/wallet` | `wallet/autorenew_test.go:122` | Auto-renew window predicate. |
| `obtcwallet/wallet` | `wallet/autorenew_test.go:139` | Candidate selection helper. |
| `obtcwallet/rpc/rpcserver` | `rpc/rpcserver/agentwallet_server_test.go:640` | Unsynced expiry-risk rejection. |
| `obtcwallet/rpc/rpcserver` | `rpc/rpcserver/agentwallet_server_test.go:656` | Unsynced preview rejection. |
| `obtcwallet/rpc/rpcserver` | `rpc/rpcserver/agentwallet_server_test.go:943` | Signer session path. |

### CI Tests

| Repository | Workflow | Coverage |
|---|---|---|
| `obtcd` | `.github/workflows/main.yml:13` | Build. |
| `obtcd` | `.github/workflows/main.yml:28` | Unit coverage. |
| `obtcd` | `.github/workflows/main.yml:85` | Race tests. |
| `obtcd` | `.github/workflows/main.yml:100` | OBTC-focused parameter and script checks. |
| `obtcd` | `.github/workflows/main.yml:191` | `rpctest` integration with `-tags=rpctest`. |
| `obtcd` | `.github/workflows/main.yml:210` | Vet and gofmt. |
| `obtcd` | `.github/workflows/release-artifacts.yml:1` | Release artifact build/checksum workflow. |
| `obtcwallet` | `.github/workflows/obtc-wallet-smoke.yml:1` | Focused wallet readiness tests and claims checker. |
| `obtcwallet` | `.github/workflows/release-artifacts.yml:1` | Wallet artifact smoke. |
| `obtc-website` | `.github/workflows/static-checks.yml:1` | Link check only. |

## User-Listed Missing Tests: Current Status

| Candidate missing test | Current status |
|---|---|
| Expired UTXO ordinary spend rejected | Covered by `blockchain/validation_reap_test.go:114`. |
| Non-expired UTXO rejected by REAP | Covered by `blockchain/validation_reap_test.go:137`. |
| REAP marker mismatch | Covered by digest/count tests at `blockchain/validation_reap_test.go:152`, `:228`; height mismatch also in direct marker tests at `:200`. |
| REAP canonical order | Covered by `blockchain/validation_reap_test.go:401` and selector ordering tests. |
| Dust fold | Covered by `blockchain/validation_reap_test.go:787` and selector/tax tests. |
| Coinbase overclaim | Partially covered by mining template accounting tests at `mining/newblocktemplate_accounting_and_helpers_test.go:55`; needs an explicit invalid-block consensus test with REAP tax overclaim. |
| Expiry commitment missing/mismatch | Covered by `blockchain/expiryindex/commitment_test.go:166` and `:216`. |
| Reorg after expiry index consistency | Partially covered by expiry-index connect/disconnect/recovery tests. Needs human confirmation whether full-node reorg scenarios with active expiry commitment are sufficient. |
| Wallet renew dry-run | Covered for `renewall --dry-run` at `obtcwallet/cmd/renewall/main_test.go:308`; legacy `obtc.renew` has no dry-run parameter. |
| Auto-renew failure backoff | Runtime fields are covered by config/default tests, but backoff after actual renewal failure is not directly covered. |

## Remaining Gaps

### P1/P2 Test Gaps

1. Explicit invalid-block coinbase overclaim test with REAP tax.
   Existing mining tests check generated-template accounting, but no direct
   consensus/regtest test was found that mutates a block to overclaim REAP tax
   and asserts validation rejection.
   Files to target in a later plan: `blockchain/`, `mining/`,
   `integration/obtc_integration_test.go`.

2. Full-node reorg scenario with active expiry commitment and REAP prefix.
   Expiry index connect/disconnect/rebuild is well covered, but a complete
   rpctest reorg that validates expiry index state, commitment root, and REAP
   prefix after detach/attach needs human confirmation.
   Existing partial coverage:
   `blockchain/expiryindex/sequence_fuzz_test.go:20`,
   `blockchain/expiryindex/recovery_integration_test.go:66`,
   `blockchain/expiryindex/expiryindex_test.go:1275`.

3. Auto-renew failure backoff runtime behavior.
   `obtcwallet/wallet/autorenew.go:268` updates backoff after failed runs and
   `:322` skips when backoff is active, but tests currently focus on default
   policy, validation, window predicates, and helper selection.
   A later plan should test `runAutoRenewOnce`, `updateAutoRenewBackoff`, and
   scheduler skip behavior using a controlled wallet/test double.

4. Cross-repo parameter consistency test.
   No test was found that extracts values from `obtcd/chaincfg/params_obtc.go`
   and checks `obtcd` docs, `obtcwallet` docs, and `obtc-website` exports.

### P2/P3 Test Gaps

1. Website public-claim checker not wired into CI.
   `obtc-website/AGENTS.md:45` requires `check_public_claims.py`, but
   `.github/workflows/static-checks.yml:21` only runs link checks.

2. Release artifact matrix and signing/attestation verification.
   Existing workflows build and verify checksums for one selected target.
   A later plan should test all intended reviewer targets and validate final
   manifest/signature or attestation rules.

3. Wallet projected reclaim ratio drift guard.
   `obtcwallet/rpc/legacyrpc/obtc_methods.go:85` and
   `obtcwallet/wallet/autorenew.go:409` hardcode `70%`. Existing tests verify
   behavior, but a later plan should either derive the ratio from chaincfg or
   add a test that fails if chaincfg tax changes without wallet policy update.

4. `renewall` funded non-dry-run external evidence.
   Unit tests cover the agent flow, but `obtcwallet/docs/mainnet-readiness.md:57`
   still gates public non-dry-run claims on recorded txid/command/height
   evidence.

5. Funded auto-renew operator evidence.
   Unit tests cover default-off policy and helper behavior, but
   `obtcwallet/docs/mainnet-readiness.md:82` still gates funded deployment.

## CI Coverage Gaps

| Gap | Evidence | Risk |
|---|---|---|
| No cross-repo parameter consistency job | `obtcd/.github/workflows/main.yml:122`; `obtc-website/.github/workflows/static-checks.yml:21` | Docs can drift from code. |
| Website public-claim checker absent from CI | `obtc-website/AGENTS.md:45` vs `.github/workflows/static-checks.yml:21` | Launch/readiness wording can regress. |
| Release workflows single-target by input | `obtcd/.github/workflows/release-artifacts.yml:10`; `obtcwallet/.github/workflows/release-artifacts.yml:36` | Reviewer artifacts may be incomplete unless manually run per target. |
| Main workflow runner label needs confirmation | `obtcd/.github/workflows/main.yml:237` includes `windows-2025-vs2026` | 需要人工确认 current GitHub-hosted runner availability. |

## Recommended Test Follow-Ups

1. Add an invalid-block REAP coinbase overclaim test.
2. Add a full-node reorg test that crosses expiry commitment activation and
   validates index root/prefix consistency after reorg.
3. Add auto-renew runtime/backoff tests around actual renewal failure and
   scheduler skip behavior.
4. Add a cross-repo parameter consistency checker and wire it into CI.
5. Add release artifact matrix/signed-manifest verification for the final
   mainnet-candidate reviewer packet.
6. Add website public-claim checker to CI/deploy workflows.
