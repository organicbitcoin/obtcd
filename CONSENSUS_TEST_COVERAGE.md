# OBTC Consensus Test Coverage

This report maps OBTC consensus-critical behavior to current tests. It is a
review aid, not a protocol specification.

## How To Run

Focused consensus coverage:

```bash
go test ./blockchain -run 'Test(OBTC|REAP|NonREAP|CheckExpiry|ReapBlock|ValidateTransactionScripts)' -count=1
go test ./blockchain/expiryindex -run 'Test.*(Commitment|Connect|Disconnect)' -count=1
go test ./txscript -run 'Test.*OBTC.*Replay|Test(LegacyVMRequires|SegWitV0VMRequires)' -count=1
go test ./mining ./mining/reap -run 'Test.*REAP|Test.*Reap|Test.*Dust|Test.*Template|TestNewBlockTemplateFeeAccountingConsistency' -count=1
```

Full local validation:

```bash
make unit
make unit-race
go test -p 1 -tags=rpctest ./integration/... -count=1 -v
```

## Test Files

| Path | Purpose |
|---|---|
| `blockchain/consensus_obtc_edge_test.go` | Added focused expiry, REAP input, marker, tax/refund/dust, and coinbase overclaim tests. |
| `blockchain/validation_reap_test.go` | Existing REAP consensus validation: expired/non-expired spends, marker digest/count, canonical order, caps, weight, dust, prefix, tax/refund. |
| `blockchain/validation_reap_extra_test.go` | Existing helper and edge-case branch coverage for REAP validation helpers. |
| `blockchain/fullblocks_obtc_test.go` | Existing full block acceptance/rejection for REAP prefix, expired normal spends, and multiple REAP txs. |
| `blockchain/scriptval_obtc_test.go` | Existing proof that REAP system txs bypass script execution while non-REAP txs do not. |
| `blockchain/validation_obtc_replay_test.go` | Existing activation flag application for replay protection at chain height. |
| `blockchain/expiryindex/commitment_test.go` | Existing expiry commitment extraction, activation, missing, duplicate, mismatch, matching-root tests. |
| `blockchain/expiryindex/commitment_edge_test.go` | Added unsupported commitment version rejection. |
| `blockchain/expiryindex/expiryindex_test.go` | Existing connect/disconnect, missing/duplicate commitment, and index round-trip coverage. |
| `txscript/sighash_obtc_replay_test.go` | Existing replay-domain hash tests plus added legacy and segwit-v0 VM enforcement tests. |
| `txscript/taproot_obtc_replay_test.go` | Existing taproot key path and script path replay-protected sighash tests. |
| `mining/newblocktemplate_accounting_and_helpers_test.go` | Existing block-template fee and coinbase accounting with and without REAP. |
| `mining/reap/*.go` tests | Existing selector, packer, marker vector, dust, budget, and dry-run coverage. |

## Rule Coverage

| Rule | Positive Tests | Negative Tests |
|---|---|---|
| Expiry ordinary spend before expiry | `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries` | `TestNonREAPExpiredSpendRejected`, `TestOBTCFullBlockRejectsNonREAPExpiredSpend` |
| Expiry activation boundary | `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries`, `TestCheckExpirySpendRulesEnableBoundaryAndIdempotent` | `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries` |
| Network-specific expiry windows | `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries` | `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries` |
| REAP spends only expired UTXOs | `TestREAPExpiredSpendWithExpectedRefundAccepted`, `TestOBTCFullBlockAcceptsValidREAPSpend` | `TestREAPNonExpiredSpendRejected` |
| REAP missing/empty/duplicate inputs | None for empty input because it is invalid by sanity | `TestOBTCREAPInputValidityEdgeCases` |
| Ordinary transaction spoofing REAP | None | `TestOBTCREAPInputValidityEdgeCases`, `TestOBTCREAPMarkerMissingMultipleAndPlacement` |
| REAP marker valid format | `TestCheckReapMarkerDirect`, `TestValidationREAPHelpersDirect`, `TestIsLikelyREAPTx` | `TestParseReapMarkerPayloadEdgeCases` |
| REAP marker height/count/digest mismatch | `TestCheckReapMarkerDirect` | `TestCheckReapMarkerDirect`, `TestCheckReapMarkerCountMismatch`, `TestREAPMarkerDigestMismatchRejected` |
| REAP marker missing/multiple/wrong placement | None | `TestOBTCREAPMarkerMissingMultipleAndPlacement` |
| REAP canonical order | `TestREAPCanonicalInputOrderAcceptedWhenSorted`, `TestCompareReapInputOrderKey` | `TestREAPCanonicalInputOrderEnforced` |
| REAP global prefix selection | `TestCheckReapGlobalPrefix`, `TestOBTCFullBlockAcceptsShortREAPPrefix` | `TestCheckReapGlobalPrefix`, `TestOBTCFullBlockRejectsREAPCherryPickNonPrefix`, `TestOBTCFullBlockRejectsREAPWithoutPrefixSource`, `TestOBTCFullBlockRejectsREAPPrefixSourceTipMismatch` |
| REAP input caps and weight budget | `TestREAPTwoTierDustCapAcceptedAtLimit`, `TestREAPTwoTierNormalCapAcceptedAtLimit`, `TestREAPNearFullTierCapsWithUniqueRefundScriptsFitsMaxWeight`, `TestREAPSingleMaxScriptSizeUTXOFitsMaxWeight` | `TestREAPTwoTierDustCapExceededRejected`, `TestREAPTwoTierNormalCapExceededRejected`, `TestREAPTwoTierInputAfterDustCapRejected`, `TestREAPTwoTierInputAfterNormalCapRejected`, `TestREAPMaxWeightExceededRejected` |
| Tax/refund 30/70 accounting | `TestReapTaxForValue`, `TestOBTCREAPTaxRefundDustAccountingMatrix`, `TestREAPExpiredSpendWithExpectedRefundAccepted` | `TestREAPExpiredSpendRequiresExpectedRefundDistribution`, `TestOBTCREAPTaxRefundDustAccountingMatrix` |
| Dust fold and dust threshold boundary | `TestApplyReapDustRule`, `TestREAPExpiredDustOnlyInputAcceptedWithoutRefundOutput`, `TestOBTCREAPTaxRefundDustAccountingMatrix`, `mining/reap` dust tests | `TestOBTCREAPTaxRefundDustAccountingMatrix` |
| Replay activation flag | `TestApplyOBTCReplayProtectionScriptFlag`, `TestProcessTransactionOBTCReplayProtectionActivation` | `TestApplyOBTCReplayProtectionScriptFlag`, `TestProcessTransactionOBTCReplayProtectionActivation` |
| Legacy replay-protected VM path | `TestLegacyVMRequiresOBTCReplayProtectedHashType` | `TestLegacyVMRequiresOBTCReplayProtectedHashType` |
| Segwit v0 replay-protected VM path | `TestSegWitV0VMRequiresOBTCReplayProtectedHashType`, `TestCalcWitnessSigHashOBTCReplayProtectionWrapper` | `TestSegWitV0VMRequiresOBTCReplayProtectedHashType` |
| Taproot replay-protected key/script paths | `TestRawTxInTaprootSignatureOBTCReplayProtectionOption`, `TestRawTxInTapscriptSignatureOBTCReplayProtectionOption` | `TestCalcTaprootSignatureHashRawOBTCReplayProtectionGated`, `TestCalcTaprootSignatureHashOBTCReplayProtectionOption`, `TestCalcTapscriptSignatureHashOBTCReplayProtectionOption` |
| Expiry commitment activation and exact-one rule | `TestValidateExpiryCommitmentBeforeActivationIsNoop`, `TestValidateExpiryCommitmentAcceptsMatchingRootAfterActivation` | `TestValidateExpiryCommitmentRejectsMissingAfterActivation`, `TestValidateExpiryCommitmentRejectsDuplicateAfterActivation`, `TestValidateExpiryCommitmentRejectsMismatchedRootAfterActivation`, `TestValidateExpiryCommitmentRejectsUnsupportedVersionAfterActivation` |
| Expiry index connect/disconnect and reorg recovery | `TestConnectDisconnectBlock`, `TestConnectDisconnectBlockRoundTripWithMixedInputs`, `TestConnectDisconnectLongSequenceRollback`, recovery/rebuild tests | `TestDisconnectBlockErrorsOnShortSpentJournal`, bucket/inconsistency failure tests |
| Coinbase accounting with REAP tax | `TestNewBlockTemplateFeeAccountingConsistency` | `TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim` |
| Coinbase accounting without REAP | `TestNewBlockTemplateFeeAccountingConsistency`, legacy full block tests | Legacy `ErrBadCoinbaseValue` fullblock tests |

## Newly Added Coverage

- `TestOBTCExpiryOrdinarySpendActivationAndWindowBoundaries`
- `TestOBTCREAPInputValidityEdgeCases`
- `TestOBTCREAPMarkerMissingMultipleAndPlacement`
- `TestOBTCREAPTaxRefundDustAccountingMatrix`
- `TestOBTCFullBlockRejectsREAPTaxCoinbaseOverclaim`
- `TestValidateExpiryCommitmentRejectsUnsupportedVersionAfterActivation`
- `TestLegacyVMRequiresOBTCReplayProtectedHashType`
- `TestSegWitV0VMRequiresOBTCReplayProtectedHashType`

## Implementation Bugs Found

No implementation bug was confirmed by these tests.

One current-behavior note: zero-input REAP-like transactions are rejected by
`CheckTransactionSanity` as `ErrNoTxInputs`, before REAP-specific validation.
That matches the current validation pipeline and should be treated as consensus
behavior unless the protocol is explicitly changed in a separate plan.
