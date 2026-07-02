# Review Card: Expiry Formula

## Mechanism Summary

Each confirmed UTXO receives an expiry height derived from its creation height
and the network expiry window:

```text
expiry_height = create_height + WindowBlocks
```

After expiry activation, normal spends of expired UTXOs are rejected. REAP
spends are only valid for expired UTXOs.

## Core Invariant

For every UTXO, all node, index, mining, and wallet code must compute the same
expiry height from the same creation height and network parameters.

## Code Location

- `chaincfg/params_obtc.go`: `ExpiryParams`, network values, and
  `CalculateExpiryKey`.
- `blockchain/validation_reap.go`: `checkExpirySpendRules`.
- `blockchain/expiryindex/expiryindex.go`: persisted expiry key use.
- Sibling wallet repo `obtcwallet/wallet/expiry.go`: wallet expiry status.
- Sibling wallet repo `obtcwallet/wallet/expiry_policy.go`: wallet policy
  resolution from chaincfg.

## Test Location

- `chaincfg/params_obtc_test.go`
- `blockchain/consensus_obtc_edge_test.go`
- `blockchain/validation_reap_test.go`
- `blockchain/expiryindex/expiryindex_test.go`
- Sibling wallet repo `obtcwallet/wallet/expiry_test.go`
- Sibling wallet repo `obtcwallet/wallet/expiry_policy_test.go`

## How To Run Tests

```bash
go test ./chaincfg -run 'Expiry|OBTC' -count=1
go test ./blockchain -run 'Expiry|REAPInputValidity' -count=1
go test ./blockchain/expiryindex -run 'ExpiryCalculation|ExpiryParams' -count=1

# From ../obtcwallet:
go test ./wallet -run 'Expiry|RenewalRisk|ResolveExpiryPolicy' -count=1
```

## What To Challenge

- Off-by-one at `txHeight == expiry_height`.
- Different behavior before and after `EnableAtHeight`.
- Overflow or negative-height behavior in wallet-side helpers.
- Whether unconfirmed UTXOs are excluded from expiry decisions until confirmed.
- Drift between `obtcd` chain parameters and wallet policy resolution.

## Known Limitations

- Testnet and regtest use accelerated windows and do not demonstrate mainnet
  wall-clock duration.
- Wallet expiry status is advisory until a transaction is mined; consensus
  evaluates the inclusion height.
- No formal third-party security audit is recorded for this mechanism.
