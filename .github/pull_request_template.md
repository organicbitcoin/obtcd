## Purpose

Describe the problem, the change, and the linked issue.

## Scope and risk

- Network: `none` / `obtcmainnet` / `obtctestnet` / `obtcregtest` / devnet
- [ ] Consensus or activation heights
- [ ] REAP or expiry
- [ ] Replay Protection or signatures
- [ ] Mining or coinbase commitments
- [ ] Wallet key management or funds
- [ ] No item above is affected

For every checked risk area, explain the positive, negative, and boundary cases
and any activation-height or compatibility effect.

## Verification

List the exact commands and results reviewers can reproduce. Include the OS,
Go version, and commit when the result depends on the environment.

## Checklist

- [ ] I followed [CONTRIBUTING.md](https://github.com/organicbitcoin/obtcd/blob/master/CONTRIBUTING.md).
- [ ] The change is focused and contains no unrelated consensus or parameter changes.
- [ ] Tests cover relevant success, rejection, and boundary behavior, or the PR explains why tests are not applicable.
- [ ] A bug fix includes a regression test when practical.
- [ ] Documentation matches the current code and does not claim unverified network or release readiness.
- [ ] Logs, examples, screenshots, and fixtures contain no secrets or private identity data.
- [ ] Go changes are formatted with `gofmt`, and CI passes.
- [ ] Any new logging statements use an appropriate subsystem and logging level.
