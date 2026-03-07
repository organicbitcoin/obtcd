# Phase 6 Validation Report (OBTC Testnet)

> Purpose: record objective evidence for Phase 6 readiness based on current implementation baseline.

## 1. Validation metadata

- Date:
- Operator:
- Repository commit:
- Node role: `seed` / `observer` / `candidate`
- Region:
- Host spec (CPU/RAM/Disk):

## 2. Node configuration snapshot

- Network flag: `--obtctestnet`
- P2P listen:
- RPC listen:
- `txindex`: enabled/disabled
- `expiryindex`: enabled/disabled
- `addpeer` list:

## 3. Connectivity and sync checks

| Check | Command | Expected | Actual | Result |
|---|---|---|---|---|
| Node starts successfully | `scripts/phase6/run_testnet_node.sh start` | process running |  |  |
| Basic chain info | `btcctl ... getblockchaininfo` | valid response |  |  |
| Peers connected | `btcctl ... getpeerinfo` | >=1 peer |  |  |
| Chain tips healthy | `btcctl ... getchaintips` | active tip present |  |  |
| Mempool accessible | `btcctl ... getmempoolinfo` | valid response |  |  |

## 4. Expiry observability checks (observer node)

| Check | Command | Expected | Actual | Result |
|---|---|---|---|---|
| Expiry index stats | `btcctl ... getexpiryindexstats` | valid response |  |  |
| Expiring scan | `btcctl ... listexpiring 0 99999999 100` | valid response |  |  |

## 5. Runtime evidence

- Start time:
- Observation window:
- Highest observed height:
- Peer count range:
- Notable warnings/errors from logs:

## 6. Issues found and mitigations

| ID | Symptom | Root cause | Mitigation | Status |
|---|---|---|---|---|
| P6- |  |  |  |  |

## 7. DoD checklist

- [ ] Node starts with `--obtctestnet` and remains stable
- [ ] Connectivity verified (peer discovery and active tip)
- [ ] At least one observer node serves expiry RPC (`--expiryindex` enabled)
- [ ] Testnet join documentation validated by dry run
- [ ] Evidence/logs archived for review

## 8. Appendix: command log

```bash
# paste exact commands and key outputs here
```

## 9. Optional automation

You can append a structured snapshot directly into this file:

```bash
scripts/phase6/collect_validation_snapshot.sh \
  --rpcuser=<u> \
  --rpcpass=<p> \
  --rpcserver=127.0.0.1:19528 \
  --append docs/phase6-validation.md
```
