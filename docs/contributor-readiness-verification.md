# Contributor Build Verification

This record captures clean-clone verification of the public contributor path.
It is evidence for the listed commits and environment only, not a claim that a
public mainnet is ready.

## 2026-11-02

Environment:

- macOS 26.5.1, arm64;
- Go 1.25.3;
- `obtcd` commit `ed926d74c60a917534f3b2c9ba11eedc16655651`;
- `obtcwallet` commit `9372e1dbc87feba57bbde32a398ee9366c140052`.

Commands and results:

| Repository | Command | Result |
| --- | --- | --- |
| `obtcd` | `go build -o ./btcd .` | passed |
| `obtcd` | `go build -o ./btcctl ./cmd/btcctl` | passed |
| `obtcd` | `go build -o ./obtc-status ./cmd/obtc-status` | passed |
| `obtcd` | `go test ./...` | passed |
| `obtcwallet` | `go build -o ./btcwallet .` | passed |
| `obtcwallet` | `go build -o ./renewall ./cmd/renewall` | passed |
| `obtcwallet` | `go test ./...` | one failure in `chain/TestJitterTicker` |
| `obtcwallet` | `go test ./chain -run '^TestJitterTicker$' -count=10 -v` | passed 10 of 10 reruns |

The wallet result is recorded as a possible timing-sensitive test failure, not
as a successful full-suite verification. This pass did not attempt a public
testnet sync, request test coins, or exercise wallet renewal because those
steps require live network coordination and manually issued testnet coins.
