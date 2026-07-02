# Review Card: Wallet Renewal

## Mechanism Summary

Wallet renewal is the holder-side path for creating a fresh UTXO before an old
UTXO expires. The wallet reports expiry status through `obtc.getexpiry` and can
renew selected OutPoints through `obtc.renew` or the `renewall` helper.

## Core Invariant

Wallet renewal must only spend selected, still-renewable UTXOs under explicit
fee, amount, confirmation, target-address, and signing controls. Dry-run paths
must not sign or broadcast.

## Code Location

- Sibling wallet repo `obtcwallet/wallet/expiry.go`: expiry and renewal-risk
  helpers.
- Sibling wallet repo `obtcwallet/wallet/expiry_policy.go`: chaincfg-derived
  wallet expiry policy.
- Sibling wallet repo `obtcwallet/rpc/legacyrpc/obtc_methods.go`:
  `obtc.getexpiry` and `obtc.renew`.
- Sibling wallet repo `obtcwallet/cmd/renewall/main.go`: batch renewal CLI.
- This repo `integration/rpctest/memwallet.go`: test wallet replay and expiry
  behavior.

## Test Location

- Sibling wallet repo `obtcwallet/wallet/expiry_test.go`
- Sibling wallet repo `obtcwallet/wallet/expiry_policy_test.go`
- Sibling wallet repo `obtcwallet/wallet/renewal_lifecycle_test.go`
- Sibling wallet repo `obtcwallet/rpc/legacyrpc/obtc_methods_test.go`
- Sibling wallet repo `obtcwallet/cmd/renewall/main_test.go`
- This repo `integration/rpctest/memwallet_unit_test.go`

## How To Run Tests

```bash
# From ../obtcwallet:
go test ./wallet -run 'Expiry|RenewalSelectedOutpoint|RenewalRisk' -count=1
go test ./rpc/legacyrpc -run 'MakeGetExpiryResult|ParseRenew|GetRenew' -count=1
go test ./cmd/renewall -run 'SelectOutpoints|RunRenewAllOnce|RenewFilter' -count=1

# From this repo:
go test ./integration/rpctest -run 'MemWallet.*OBTC|ExpiredUTXO|ReplayProtected' -count=1
```

## What To Challenge

- Renewal at the last safe height before expiry.
- Expired UTXOs being selected for normal renewal.
- Dry-run accidentally signing, previewing, submitting, or broadcasting.
- Fee-rate and amount parameter validation.
- Selected OutPoint mismatch or unintended coin selection.
- Wallet signing without OBTC replay protection.

## Known Limitations

- Funded renewal broadcast evidence is release-scope dependent and is not a
  production wallet claim.
- The legacy `obtc.renew` RPC success path is harder to isolate than lower-level
  wallet construction tests.
- No formal third-party security audit is recorded for this mechanism.
