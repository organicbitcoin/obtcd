# OBTC Parameter References

Audit date: 2026-06-30

Scope: code and documentation references for OBTC parameters. Code source of
truth remains `obtcd/chaincfg/params_obtc.go` unless explicitly noted.

## Code Source Map

| Area | Source |
|---|---|
| OBTC chain parameters and expiry parameters | `chaincfg/params_obtc.go:18` |
| Network magic constants | `wire/protocol.go:199` |
| `obtcd` node RPC ports | `params.go:74` |
| `btcctl` node/wallet RPC defaults | `cmd/btcctl/config.go:153` |
| Wallet node/wallet RPC ports | `obtcwallet/netparams/params.go:44` |
| Wallet selected network flags | `obtcwallet/config.go:71` |
| Wallet auto-renew CLI flags | `obtcwallet/config.go:88` |

## Cross-Network Parameter Table

| Parameter | Mainnet | Testnet | Regtest | Code reference |
|---|---:|---:|---:|---|
| Fork height | `1000000` | `0` | `100` | `chaincfg/params_obtc.go:40` |
| First independent block | `1000001` | `1` by fork semantics | `101` by fork semantics | `chaincfg/params_obtc.go:50` |
| Replay protection activation | `1000001` | `130` | `114` | `chaincfg/params_obtc.go:392`, `:409`, `:425` |
| Expiry enable height | `1002016` | `100` | `110` | `chaincfg/params_obtc.go:390`, `:407`, `:423` |
| Expiry commitment activation | `1002016` | `100` | `110` | `chaincfg/params_obtc.go:399`, `:415`, `:431` |
| REAP hardening height | `1002016` | `120` | `112` | `chaincfg/params_obtc.go:391`, `:408`, `:424` |
| `WindowBlocks` | `362880` | `144` | `144` | `chaincfg/params_obtc.go:387`, `:404`, `:420` |
| `ListBatchLimit` | `10000` | `5000` | `1000` | `chaincfg/params_obtc.go:388`, `:405`, `:421` |
| `StartScanHeight` | `0` | `0` | `0` | `chaincfg/params_obtc.go:389`, `:406`, `:422` |
| Tax ratio | `30 / 100` | `30 / 100` | `30 / 100` | `chaincfg/params_obtc.go:396`, `:412`, `:428` |
| Refund ratio | `70 / 100` derived | `70 / 100` derived | `70 / 100` derived | `blockchain/validation_reap.go:491` |
| Dust threshold | `720 sat` | `720 sat` | `720 sat` | `chaincfg/params_obtc.go:398`, `:414`, `:430` |
| REAP normal input cap | `256` | `500` | `200` | `chaincfg/params_obtc.go:393`, `:410`, `:426` |
| REAP dust input cap | `1024` | `1000` | `400` | `chaincfg/params_obtc.go:394`, `:411`, `:427` |
| REAP max weight | `400000` | `0` / disabled by omitted field | `0` / disabled by omitted field | `chaincfg/params_obtc.go:395`; no field at `:402` or `:418` |
| Network magic | `0x4F425443` | `0x4F544553` | `0x4F524547` | `wire/protocol.go:199` |
| P2P port | `9527` | `19527` | `29527` | `chaincfg/params_obtc.go:71`, `:168`, `:247` |
| Node RPC port | `9528` | `19528` | `29528` | `params.go:74`, `:80`, `:86` |
| Wallet RPC port | `9554` | `19554` | `29554` | `obtcwallet/netparams/params.go:44`, `:52`, `:96` |
| Bech32 HRP | `obtc` | `obtct` | `obtcrt` | `chaincfg/params_obtc.go:149`, `:189`, `:311` |
| P2PKH prefix | `0x47` | `0x71` | `0x72` | `chaincfg/params_obtc.go:150`, `:190`, `:312` |
| P2SH prefix | `0x32` | `0xD1` | `0xD2` | `chaincfg/params_obtc.go:151`, `:191`, `:313` |
| WIF/private key prefix | `0x9A` | `0xF1` | `0xF2` | `chaincfg/params_obtc.go:152`, `:192`, `:314` |
| Witness pubkey hash prefix | `0x2A` | `0x2C` | `0x2E` | `chaincfg/params_obtc.go:153`, `:193`, `:315` |
| Witness script hash prefix | `0x2B` | `0x2D` | `0x2F` | `chaincfg/params_obtc.go:154`, `:194`, `:316` |
| HD private version | `0B47B01E` | `0B48B01E` | `0B49B01E` | `chaincfg/params_obtc.go:156`, `:195`, `:317` |
| HD public version | `0B47B5D4` | `0B48B5D4` | `0B49B5D4` | `chaincfg/params_obtc.go:157`, `:196`, `:318` |
| BIP44 coin type | `20260` | `20261` | `20262` | `chaincfg/params_obtc.go:160`, `:197`, `:319` |

需要人工确认:

- `ReapMaxWeight` is explicitly set for mainnet and omitted for testnet/regtest,
  so the Go zero value disables the weight cap there. Confirm that this is the
  intended test/regtest behavior before external review.
- Testnet and regtest "first independent block" values above are inferred from
  fork semantics; only mainnet has a named `ObtcMainNetFirstBlockHeight`
  constant.

## Mainnet Parameter Details

| Parameter | Value | Source | Notes |
|---|---:|---|---|
| Chaincfg name | `obtcmainnet` | `chaincfg/params_obtc.go:69` | Used for active network identity. |
| CLI flag | `--obtcmainnet` | `config.go:158` | Wallet has matching flag at `obtcwallet/config.go:71`. |
| DNS seed | `seed.obtc.example.com` | `chaincfg/params_obtc.go:73` | Placeholder; release blocker in docs. |
| Genesis block/hash | Bitcoin mainnet genesis | `chaincfg/params_obtc.go:77` | Shared-history fork model. |
| Pow limit bits | `0x1d00ffff` | `chaincfg/params_obtc.go:81` | Mainnet candidate baseline. |
| Fork DAA start | `1000001` | `chaincfg/params_obtc.go:95` | `ObtcMainNetFirstBlockHeight`. |
| Fork DAA bootstrap end | `1002016` | `chaincfg/params_obtc.go:96` | `ObtcMainNetActivationHeight`. |
| Fork DAA reset bits | `0x1d00ffff` | `chaincfg/params_obtc.go:97` | Needs final review with release parameters. |
| Fork DAA bootstrap half-life | `1h` | `chaincfg/params_obtc.go:98` | Mainnet candidate. |
| Fork DAA normal half-life | `48h` | `chaincfg/params_obtc.go:99` | Mainnet candidate. |
| Versionbits CSV/Segwit/Taproot | always active at height `1` | `chaincfg/params_obtc.go:120`, `:126`, `:132` | Documented in `docs/mainnet-params.md:71`. |
| Relay non-standard txs | `false` | `chaincfg/params_obtc.go:146` | Mainnet policy. |

## Wallet And RPC Port References

| Parameter | Mainnet | Testnet | Regtest | Source |
|---|---:|---:|---:|---|
| `obtcd` node RPC | `9528` | `19528` | `29528` | `params.go:74` |
| `btcctl` node RPC default | `9528` | `19528` | `29528` | `cmd/btcctl/config.go:153` |
| `btcctl -wallet` RPC default | `9554` | `19554` | `29554` | `cmd/btcctl/config.go:153` |
| `obtcwallet` RPC client port | `9528` | `19528` | `29528` | `obtcwallet/netparams/params.go:44` |
| `obtcwallet` legacy RPC server port | `9554` | `19554` | `29554` | `obtcwallet/netparams/params.go:44` |

Risk:

- `obtcd/config.go:131` and `obtcd/config.go:168` still describe inherited
  Bitcoin defaults in help text. Runtime default selection uses
  `activeNetParams.rpcPort` at `config.go:820`, but operator help can mislead.

## Expiry, REAP, Replay, And Commitment Implementation References

| Area | Reference | What it controls |
|---|---|---|
| Expiry parameter type | `chaincfg/params_obtc.go:18` | Field definitions for expiry, REAP, replay, commitment. |
| Expiry parameter resolver | `chaincfg/params_obtc.go:378` | Network-specific expiry parameter values. |
| Replay activation resolver | `chaincfg/params_obtc.go:355` | Activation height lookup. |
| Replay script flag | `blockchain/validation_obtc_replay.go:12` | Height-gated script flag activation. |
| Replay sighash bit/tag | `txscript/sighash.go:32`, `txscript/sighash.go:43` | OBTC replay-protected signature domains. |
| Replay enforcement | `txscript/engine.go:1177` | Requires OBTC replay bit when active. |
| REAP marker | `blockchain/validation_reap.go:113` | Height/count/digest marker validation. |
| REAP block hardening | `blockchain/validation_reap.go:159` | At most one REAP transaction after hardening. |
| REAP global prefix | `blockchain/validation_reap.go:198` | Canonical global prefix validation. |
| REAP canonical order and caps | `blockchain/validation_reap.go:333` | Order, tier caps, max weight. |
| REAP tax/refund/dust | `blockchain/validation_reap.go:446`, `:454`, `:466` | Tax, dust fold, refund distribution. |
| Expiry spend rejection | `blockchain/validation_reap.go:547` | Ordinary expired spend rejection and REAP non-expired rejection. |
| Transaction validation wiring | `blockchain/validate.go:973` | Calls REAP/expiry checks before normal input accounting. |
| Block validation wiring | `blockchain/validate.go:1139` | Calls REAP block hardening/global prefix path. |
| Expiry commitment format | `blockchain/expiryindex/commitment.go:14` | `OEXP` OP_RETURN commitment format. |
| Commitment validation | `blockchain/expiryindex/expiryindex.go:608` | Missing/duplicate/version/mismatch checks. |
| Mining REAP template path | `mining/template_reap.go:17` | Builds REAP tx after expiry enable height. |
| REAP candidate selection | `mining/reap/selector.go:36`, `:207` | Prefix selection and strict ordering. |

## Wallet Implementation References

| Area | Reference | What it controls |
|---|---|---|
| Expiry height/status helpers | `obtcwallet/wallet/expiry.go:51` | Wallet-side expiry status. |
| Expiry policy from chaincfg | `obtcwallet/wallet/expiry_policy.go:39` | Window and dust threshold resolution. |
| `obtc.getexpiry` registration | `obtcwallet/rpc/legacyrpc/obtc_methods.go:70` | Legacy RPC method exposure. |
| `obtc.getexpiry` implementation | `obtcwallet/rpc/legacyrpc/obtc_methods.go:126` | Expiry query, sorting, limit/filter. |
| `obtc.renew` implementation | `obtcwallet/rpc/legacyrpc/obtc_methods.go:315` | Explicit outpoint renewal transaction. |
| `renewall` options | `obtcwallet/cmd/renewall/main.go:31` | Batch renewal CLI flags. |
| `renewall --dry-run` | `obtcwallet/cmd/renewall/main.go:514` | Preview without signing/publishing. |
| Auto-renew defaults | `obtcwallet/wallet/autorenew.go:14`, `:51` | Default-off policy, intervals, fee caps. |
| Auto-renew config wiring | `obtcwallet/btcwallet.go:92` | Enables scheduler after wallet load. |
| Auto-renew runtime | `obtcwallet/wallet/autorenew.go:187`, `:282`, `:333` | Configure, loop, single-run execution. |

Risk:

- `obtcwallet/rpc/legacyrpc/obtc_methods.go:85` and
  `obtcwallet/wallet/autorenew.go:409` hardcode a projected reclaim value of
  `70%`. This matches current `30/100` tax, but is a drift point unless
  protected by tests or derived from chaincfg.

## Documentation References

| Document | Reference | Parameters mentioned |
|---|---|---|
| `obtcd/README.md` | `README.md:96` | Network magic, P2P/RPC ports, fork heights, HRP, prefixes, BIP44. |
| `obtcd/README.md` | `README.md:107` | Mainnet fork provisional, replay `1000001`, expiry/REAP/commitment `1002016`. |
| `obtcd/docs/mainnet-params.md` | `docs/mainnet-params.md:17` | Chain name, flag, magic, ports, wallet ports, HRP, DNS seed. |
| `obtcd/docs/mainnet-params.md` | `docs/mainnet-params.md:36` | Address/key namespaces and BIP44. |
| `obtcd/docs/mainnet-params.md` | `docs/mainnet-params.md:52` | Consensus baseline and versionbits. |
| `obtcd/docs/mainnet-params.md` | `docs/mainnet-params.md:83` | Fork, replay, expiry, REAP, commitment, window, batch, caps, tax, dust, weight. |
| `obtcd/docs/mainnet-join.md` | `docs/mainnet-join.md:14` | Mainnet baseline: flag, ports, HRP, magic, fork, replay, activation. |
| `obtcd/docs/mainnet-join.md` | `docs/mainnet-join.md:160` | DNS seed/open release checklist. |
| `obtcwallet/README.md` | `README.md:28` | Wallet flags, node RPC ports, wallet RPC ports, renewal methods. |
| `obtcwallet/docs/mainnet-readiness.md` | `docs/mainnet-readiness.md:24` | Mainnet wallet network selection and ports. |
| `obtcwallet/docs/releases/obtcwallet-testnet-v0.1.0.md` | `docs/releases/obtcwallet-testnet-v0.1.0.md:11` | Wallet network flags and ports. |
| `obtc-website/mainnet-candidate.html` | `mainnet-candidate.html:97` | Candidate fork, replay, expiry, REAP, commitment, window, split, dust, weight. |
| `obtc-website/docs.html` | `docs.html:44`, `docs.html:107` | Candidate parameter summary and table. |
| `obtc-website/whitepaper.md` | `whitepaper.md:8` | Mainnet fork/replay/activation notice. |
| `obtc-website/whitepaper.md` | `whitepaper.md:264` | Activation matrix across mainnet/testnet/regtest. |
| `obtc-website/whitepaper.md` | `whitepaper.md:287` | Window and batch limits. |
| `obtc-website/whitepaper.md` | `whitepaper.md:322` | Network magic, ports, HRP, BIP44. |
| `obtc-website/whitepaper.md` | `whitepaper.md:347` | REAP caps, tax, refund, dust, max weight. |
| `obtc-website/versions.html` | `versions.html:48` | Current candidate values and source-of-truth link. |
| `obtc-website/versions.html` | `versions.html:74` | Old draft values explicitly marked superseded. |
| `obtc-website/wallet.md` | `wallet.md:21` | Wallet preview ports and evidence-gated renewal claims. |

## Parameter Consistency Notes

No direct value mismatch was found between the current `obtcd` mainnet
candidate parameters and the primary public parameter summaries listed above.

Items needing human confirmation:

- Website whitepaper provenance is stale: `obtc-website/whitepaper.md:16`,
  `whitepaper.html:229`, and `content/whitepaper-v1.md:16` cite
  `cd9ae639500bbffd82a8b42b1c6ca1c0152c629d`, while this audit used
  `36e94c508ea35d1e9d36e992c5d3efa23f5b6ee4`.
- `seed.obtc.example.com` must either be replaced or intentionally documented
  as unused before external mainnet-candidate review artifacts are cut.
- Cross-repo CI should compare these values automatically instead of relying on
  manual documentation review.
