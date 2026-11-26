## Summary

Describe the problem, the change, and the linked issue.

Related issue: <!-- Link an existing issue, or explain why none is needed. -->

## Scope and risk

- Risk classification: `docs/tests/tooling` / `RPC-wallet` / `consensus-mining-security`
  (test-only changes in consensus-sensitive paths still need expert review)
- Network: `none` / `obtcmainnet` / `obtctestnet` / `obtcregtest` / devnet
- [ ] Consensus or activation heights
- [ ] Serialization or network compatibility
- [ ] REAP or expiry
- [ ] Replay Protection or signatures
- [ ] Mining or coinbase commitments
- [ ] Miner rewards or economic parameters
- [ ] Wallet key management or funds
- [ ] No item above is affected

For every checked risk area, explain the positive, negative, and boundary cases
and any activation-height or compatibility effect.

## How to test

List the exact commands you actually ran and their results. Separate proposed
checks from completed checks and explain failures or omissions. Include the OS,
Go version, and commit. Describe positive, negative, and boundary coverage, or
explain why it is not applicable for a documentation-only change.

## AI assistance

State whether and how AI tools helped. Authors are responsible for reviewing
generated changes, explaining them, and verifying correctness; never include
private keys, passwords, or production logs in AI conversations or PR evidence.

## Checklist

- [ ] I followed [CONTRIBUTING.md](https://github.com/organicbitcoin/obtcd/blob/master/CONTRIBUTING.md).
- [ ] The change is focused and contains no unrelated consensus or parameter changes.
- [ ] Tests cover relevant success, rejection, and boundary behavior, or the PR explains why tests are not applicable.
- [ ] A bug fix includes a regression test when practical.
- [ ] Documentation matches the current code and does not claim unverified network or release readiness.
- [ ] Logs, examples, screenshots, and fixtures contain no secrets or private identity data.
- [ ] Go changes are formatted with `gofmt`, and CI passes.
- [ ] Any new logging statements use an appropriate subsystem and logging level.
