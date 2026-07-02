# OBTC Review Fixture Vectors

This file gives copyable concrete fixtures for reviewers. The executable tests
remain the authoritative source; each fixture below points to the test that
checks the result.

## Replay Activation Boundary

| Fixture | Exact Input | Expected Result | Executable Reference |
|---|---|---|---|
| Mainnet candidate replay boundary | `ObtcMainNetForkHeight = 1000000`; `ReplayProtectionAtHeight = 1000001` | inactive at height `1000000`; active at height `1000001` | `chaincfg/params_obtc_test.go:TestOBTCReplayProtectionActivation` |
| Mainnet expiry / REAP boundary | `ObtcMainNetActivationHeight = 1002016` | expiry, REAP, and expiry commitments activate at `1002016` | `chaincfg/params_obtc_test.go:TestGetExpiryParamsDirect` |
| Public testnet replay boundary | `ReplayProtectionAtHeight = 130` | inactive at `129`; active at `130` | `chaincfg/params_obtc_test.go:TestOBTCReplayProtectionActivation` |
| Public testnet expiry / REAP boundary | `EnableAtHeight = 100`; `ReapConsensusAtHeight = 120`; `ExpiryCommitmentEnableAtHeight = 100` | expiry/index commitments start at `100`; REAP consensus starts at `120` | `chaincfg/params_obtc_test.go:TestOBTCTestNetPublicParams` |
| Regtest replay boundary | `ObtcRegTestForkHeight = 100`; `ReplayProtectionAtHeight = 114` | inactive at `113`; active at `114` | `chaincfg/params_obtc_test.go:TestOBTCReplayProtectionActivation` |
| Bitcoin mainnet control | `chaincfg.MainNetParams` | `GetOBTCReplayProtectionHeight` returns `-1`; active is false even at height `1000000` | `chaincfg/params_obtc_test.go:TestOBTCReplayProtectionActivation` |

## Replay-Protected Sighash Examples

Positive fixtures use the OBTC replay bit `0x40` plus the base sighash type.
All rows below are expected to pass after `ScriptVerifyOBTCReplayProtection` is
enabled for the script engine.

| Name | Hash Type Byte | Meaning |
|---|---:|---|
| `all` | `0x41` | `SigHashOBTCReplayProtection | SigHashAll` |
| `none` | `0x42` | `SigHashOBTCReplayProtection | SigHashNone` |
| `single` | `0x43` | `SigHashOBTCReplayProtection | SigHashSingle` |
| `anyonecanpay_all` | `0xc1` | `SigHashOBTCReplayProtection | SigHashAnyOneCanPay | SigHashAll` |
| `anyonecanpay_none` | `0xc2` | `SigHashOBTCReplayProtection | SigHashAnyOneCanPay | SigHashNone` |
| `anyonecanpay_single` | `0xc3` | `SigHashOBTCReplayProtection | SigHashAnyOneCanPay | SigHashSingle` |

Covered spend families:

| Spend Family | Expected Result | Executable Reference |
|---|---|---|
| Legacy P2PKH | all six hash types pass | `txscript/sighash_obtc_replay_test.go:TestLegacyVMOBTCReplayProtectedSigHashMatrix` |
| SegWit v0 P2WPKH | all six hash types pass | `txscript/sighash_obtc_replay_test.go:TestSegWitV0P2WPKHVMOBTCReplayProtectedSigHashMatrix` |
| SegWit v0 P2WSH 1-of-2 multisig | all six hash types pass | `txscript/sighash_obtc_replay_test.go:TestSegWitV0P2WSHMultisigVMOBTCReplayProtectedSigHashMatrix` |
| Taproot key path | all six hash types pass | `txscript/taproot_obtc_replay_test.go:TestTaprootKeyPathVMOBTCReplayProtectedSigHashMatrix` |
| Taproot script path | all six hash types pass | `txscript/taproot_obtc_replay_test.go:TestTaprootScriptPathVMOBTCReplayProtectedSigHashMatrix` |

Negative hash fixtures:

| Exact Input | Expected Result | Executable Reference |
|---|---|---|
| `0x01` (`SigHashAll`, missing replay bit) | rejected after replay activation | `TestLegacyVMRejectsInvalidOBTCReplayHashTypes`, `TestSegWitV0P2WPKHVMRejectsInvalidOBTCReplayHashTypes`, `TestSegWitV0P2WSHMultisigVMRejectsInvalidOBTCReplayHashTypes` |
| `0x61` (`0x40 | 0x20 | SigHashAll`, unknown extra bit) | rejected after replay activation | same invalid-hash tests plus Taproot invalid-hash tests |
| `0x40` (`SigHashOBTCReplayProtection | SigHashDefault`, base type `0`) | rejected after replay activation | same invalid-hash tests plus Taproot invalid-hash tests |
| Taproot 64-byte signature with no sighash byte (`SigHashDefault`) | rejected after replay activation | `txscript/taproot_obtc_replay_test.go:TestTaprootKeyPathVMRejectsInvalidOBTCReplayHashTypes`, `TestTaprootScriptPathVMRejectsInvalidOBTCReplayHashTypes` |

## Expiry Boundary

| Fixture | Exact Input | Expected Result | Executable Reference |
|---|---|---|---|
| Direct expiry formula | `ExpiryParams{WindowBlocks: 144}.CalculateExpiryKey(100)` | `244` | `chaincfg/params_obtc_test.go:TestCalculateExpiryKeyDirect` |
| Public testnet output | `create_height = 100`; `WindowBlocks = 144` | `expiry_height = 244` | `chaincfg/params_obtc_test.go:TestOBTCTestNetPublicParams` |
| Mainnet candidate output | `create_height = 1000001`; `WindowBlocks = 362880` | `expiry_height = 1362881` | `chaincfg/params_obtc.go:GetExpiryParams` |
| Regtest block-1 output | `create_height = 1`; `WindowBlocks = 144` | first expired at height `145`; used by REAP block at `145` | `blockchain/consensus_obtc_edge_test.go:TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim` |

Expiry status is block-height based. Wall-clock waiting without additional
blocks does not change `create_height`, `expiry_height`, or candidate status.

## REAP Marker Payload And Digest

Digest input serialization is, for each ordered input, `txid` bytes in internal
wire order followed by `vout` as uint32 little endian.

| Field | Exact Value |
|---|---|
| Input 1 hash | `0102030000000000000000000000000000000000000000000000000000000000` |
| Input 1 index | `1` (`01000000` little endian) |
| Input 2 hash | `aabbccdd00000000000000000000000000000000000000000000000000000000` |
| Input 2 index | `257` (`01010000` little endian) |
| Expected digest | `eb39b68688466fd7494c455587d1cf7a593137eb305a92b532fb6bb782f8597b` |
| Marker payload at height `321`, count `2` | `REAP:321:2:eb39b68688466fd7494c455587d1cf7a593137eb305a92b532fb6bb782f8597b` |
| OP_RETURN script hex | `6a4b524541503a3332313a323a65623339623638363838343636666437343934633435353538376431636637613539333133376562333035613932623533326662366262373832663835393762` |
| Marker output value | `0` |

Executable references: `mining/reap/marker_vector_test.go:TestMarkerDigestVector`,
`mining/reap/packer.go:markerScript`, and
`mining/reap/reaptx_test.go:TestExtractMarkerPayload`.

## REAP Refund, Tax, And Dust Fold Accounting

Default REAP accounting uses `TaxNum = 30`, `TaxDen = 100`, and
`DustThresholdSat = 720`.

| Input Value | Raw Tax | Raw Refund | Final Tax | Final Refund | Expected Result |
|---:|---:|---:|---:|---:|---|
| `719` | `215` | `504` | `719` | `0` | value below dust threshold folds fully into tax |
| `720` | `216` | `504` | `216` | `504` | threshold value is not folded |
| `1027` | `308` | `719` | `308` | `719` | value above threshold is not folded even if refund is `719` |
| `1100` | `330` | `770` | `330` | `770` | normal proportional split |
| `2001` | `600` | `1401` | `600` | `1401` | normal proportional split |

Concrete blueprint fixture:

| Exact Input | Expected Result | Executable Reference |
|---|---|---|
| Values `1100` and `2001` in one REAP plan at height `123` | `TaxTotal = 930`; `RefundTotal = 2171`; one aggregated refund output plus final zero-value marker | `mining/reap/packer_test.go:TestBuildBlueprintTotalsAndMarker` |
| One value `700` with `DustThresholdSat = 720` | `TaxTotal = 700`; `RefundTotal = 0`; transaction has only the zero-value marker output | `mining/reap/dust_test.go:TestBuildBlueprintDustRefundFoldedToTax` |
| Values `719` and `720` | `TaxTotal = 935`; `RefundTotal = 504` | `mining/reap/dust_extreme_test.go:TestDustExtremeCliff719Vs720` |
| Two values `700` sharing the same refund script | per-input folding gives `TaxTotal = 1400`; `RefundTotal = 0`; aggregate-first behavior would be `TaxTotal = 420`, `RefundTotal = 980`, and is not implemented | `mining/reap/dust_extreme_test.go:TestDustExtremePerInputFoldingDiffersFromAggregate` |

## Canonical REAP Ordering

Canonical REAP order is expiry key, then amount, then OutPoint. The global
prefix rule means a block may include a short prefix, but may not skip an
earlier candidate.

| Fixture | Exact Input | Expected Result | Executable Reference |
|---|---|---|---|
| Global prefix exact | source candidates `opA(amount=1000)`, `opB(amount=2000)`, `opC(amount=3000)` at the same expiry key; REAP inputs `opA, opB` | accepted | `blockchain/validation_reap_test.go:TestCheckReapGlobalPrefix` |
| Global prefix short | same source; REAP inputs `opA` | accepted | `blockchain/validation_reap_test.go:TestCheckReapGlobalPrefix` |
| Skipped first candidate | same source; REAP input `opB` | rejected with `ErrBadReapPrefix` | `blockchain/validation_reap_test.go:TestCheckReapGlobalPrefix` |
| Middle replacement | same source; REAP inputs `opA, opC` | rejected with `ErrBadReapPrefix` | `blockchain/validation_reap_test.go:TestCheckReapGlobalPrefix` |
| Canonical amount order | same expiry key; inputs `2000, 1000` | rejected with `canonical order` error | `blockchain/validation_reap_test.go:TestREAPCanonicalInputOrderEnforced` |
| Canonical amount order accepted | same expiry key; inputs `1000, 2000` | accepted | `blockchain/validation_reap_test.go:TestREAPCanonicalInputOrderAcceptedWhenSorted` |
| Selector tail truncation | five candidates with amounts `1000,1001,1002,1003,1004`; `MaxInputs = 3` | selects first three and reports `Skipped = 2` | `mining/reap/selector_test.go:TestSelectPrefixCandidatesTruncatesTailOnly` |

## Coinbase Overclaim Rejection

| Fixture | Exact Input | Expected Result | Executable Reference |
|---|---|---|---|
| Regtest REAP coinbase overclaim | set coinbase maturity to `1`; build blocks `1..144`; spend the output created at height `1`; include valid REAP at block height `145`; mutate coinbase output `0` by adding `reapFee + 1` | `ProcessBlock` rejects with `ErrBadCoinbaseValue` | `blockchain/consensus_obtc_edge_test.go:TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim` |
| Template accounting control | REAP template includes expired candidates and valid tax | coinbase value equals `subsidy + expectedTax`; it must not equal `subsidy + inputTotal` | `mining/newblocktemplate_reap_template_tests_test.go:TestNewBlockTemplateREAPAppendStructureRefundsAndAccounting` |
