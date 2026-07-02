---
name: Wallet renewal issue
about: Report expiry visibility, obtc.renew, renewall, or wallet renewal safety behavior.
title: "[wallet-renewal]: "
labels: ["bug", "wallet renewal", "needs triage"]
assignees: ""
---

## Summary

Describe the wallet renewal behavior that looks wrong.

## Repository

- [ ] `organicbitcoin/obtcd`
- [ ] `organicbitcoin/obtcwallet`
- [ ] Unsure

## Environment

- `obtcd` commit or tag:
- `obtcwallet` commit or tag:
- Network flag:
- Node height:
- Wallet synced height:
- Wallet locked: yes / no
- Command used: `obtc.getexpiry` / `obtc.renew` / `renewall` / auto-renew
- OS / architecture:

## Reproduction

1.
2.
3.

Command or RPC request:

```text

```

## Expected Result

What renewal status or transaction behavior did you expect?

## Actual Result

What happened instead?

## Evidence

- Redacted `obtc.getexpiry` output:
- Renewal txid, if broadcast:
- Selected OutPoints:
- Fee rate / minconf / target amount:
- Error message:
- Redacted logs:

## Safety Check

- [ ] I removed seed phrases, private keys, wallet private passphrases, RPC passwords, and sensitive screenshots.
- [ ] I did not use Bitcoin mainnet private keys or real wallet backups in OBTC software.
