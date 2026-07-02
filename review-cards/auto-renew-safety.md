# Review Card: Auto-Renew Safety

## Mechanism Summary

Auto-renew is an opt-in wallet process feature that periodically selects
eligible near-expiry UTXOs and renews them under configured limits. It is
disabled by default.

## Core Invariant

Auto-renew must never run unless explicitly enabled, must respect window,
amount, fee-rate, max-UTXO, min-confirmation, and per-run budget limits, and
must not sign while the wallet is locked or unsynced.

## Code Location

- Sibling wallet repo `obtcwallet/config.go`: auto-renew CLI options.
- Sibling wallet repo `obtcwallet/btcwallet.go`: runtime configuration wiring.
- Sibling wallet repo `obtcwallet/wallet/autorenew.go`: policy, scheduler,
  candidate selection, budget limiting, backoff, and run execution.
- Sibling wallet repo `obtcwallet/AUTO_RENEW_SAFETY_NOTES.md`: review notes.

## Test Location

- Sibling wallet repo `obtcwallet/wallet/autorenew_test.go`
- Sibling wallet repo `obtcwallet/wallet/renewal_lifecycle_test.go`
- Sibling wallet repo `obtcwallet/config_autorenew_test.go`
- Sibling wallet repo `obtcwallet/rpc/rpcserver/agentwallet_server_test.go`
- Sibling wallet repo `obtcwallet/WALLET_LIFECYCLE_TESTS.md`

## How To Run Tests

```bash
# From ../obtcwallet:
go test ./wallet -run 'AutoRenew|DefaultAutoRenew|ValidateAutoRenew|LimitAutoRenew|SelectAutoRenew|NormalizeAutoRenew|RenewalSelectedOutpoint' -count=1
go test . -run 'AutoRenew' -count=1
go test ./rpc/rpcserver -run 'ExpiryRisk|PreviewRenewal|SubmitRenewal|Unsynced' -count=1
```

## What To Challenge

- Auto-renew disabled default across config and runtime paths.
- Invalid interval, backoff, amount, fee, window, minconf, and budget values.
- Candidate order, max UTXO cap, and amount budget truncation.
- Locked wallet and unsynced wallet behavior.
- Failure backoff and retry behavior after partial failures.
- Persistence expectations across process restarts.

## Known Limitations

- Tests do not run a 72-hour wall-clock scheduler loop.
- Current code uses the configured maximum fee rate as the fee-rate input; it
  does not independently estimate network fees before deciding to skip.
- Auto-renew state persistence across restarts remains a human review item.
- No formal third-party security audit is recorded for this mechanism.
