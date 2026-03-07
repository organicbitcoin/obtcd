# Phase 8 计划（主网候选发布）— 按当前代码基线对齐

> 对齐日期：2026-03-07
> 目标：在当前 `master` 已有主网参数与共识规则基础上，完成真正需要的主网候选发布准备，而不是按“新创世链上线”去规划。

## 1. 当前主网基线

当前代码已经定义了 [`ObtcMainNetParams`](../chaincfg/params_obtc.go)。

最重要的事实：

- OBTC Mainnet 是 **Bitcoin mainnet 的硬分叉**
- 当前代码复用 **Bitcoin genesis**
- 真正需要冻结的是：
  - fork height
  - 网络命名空间
  - OBTC 共识参数
  - 种子节点

不是去“重新生成主网创世块”。

当前主网关键参数：

| 项目 | 当前值 |
|------|--------|
| 网络名 | `obtcmainnet` |
| 网络标志 | `--obtcmainnet` |
| P2P 默认端口 | `9527` |
| Bech32 HRP | `obtc` |
| Fork Height | `950000` |
| `WindowBlocks` | `362880` |
| `EnableAtHeight` | `ObtcMainNetForkHeight + 100000` |
| `ReapConsensusAtHeight` | `ObtcMainNetForkHeight + 110000` |
| `ReplayProtectionAtHeight` | `ObtcMainNetForkHeight + 115000` |
| `ReapMaxInputs` | `256` |
| `ExpiryCommitmentEnableAtHeight` | `ObtcMainNetForkHeight + 100000` |

当前还有一个现实问题：

- `DNSSeeds` 仍是 placeholder：`seed.obtc.example.com`

这意味着 Phase 8 的重点首先应该是**替换 placeholder seeds**，而不是创世生成器。

## 2. 当前缺口

主网候选发布文档里原先写的很多产物，仓库里还没有：

- 没有 `cmd/gengenesis/`
- 没有 `cmd/checkgenesis/`
- 没有 `cmd/obtc-status/`
- 没有 `docs/mainnet-join.md`
- 没有 `docs/mainnet-params.md`
- 没有 `docs/phase8-validation.md`
- 没有 `build/release.sh`
- 没有 `Dockerfile.release`

当前可直接复用的是：

- [`release/release.sh`](../release/release.sh)
- [`release/README.md`](../release/README.md)
- 根目录 [`Dockerfile`](../Dockerfile)

## 3. Phase 8 目标

主网候选发布现在应收敛成这几件事：

1. 冻结并审计主网参数表
2. 替换真实种子节点
3. 用现有发布链路产出 `btcd` / `btcctl`
4. 补主网接入文档
5. 完成 72h 节点与同步观察

不要再把“生成主网创世块”作为 Phase 8 的默认前提。

## 4. 建议交付物

- `docs/mainnet-params.md`
  - 从 `chaincfg/params_obtc.go` 导出主网参数表
- `docs/mainnet-join.md`
  - 启动、校验、连接种子、排错
- `docs/phase8-validation.md`
  - 72h 观察记录
- `infra/`
  - 主网 seed 节点部署脚本
- `release/`
  - 基于现有 `release/release.sh` 的发布说明补充

## 5. 任务拆解

### 5.1 参数冻结

需要冻结并二次审计的项目应改成：

- `Name = obtcmainnet`
- `DefaultPort = 9527`
- `Bech32HRPSegwit = "obtc"`
- 地址 / WIF / HD namespace
- `ForkHeight = 950000`
- `WindowBlocks = 362880`
- `EnableAtHeight`
- `ReapConsensusAtHeight`
- `ReplayProtectionAtHeight`
- `ExpiryCommitmentEnableAtHeight`
- `ReapMaxInputs = 256`

这里不应再写：

- “主网创世生成”
- “创世哈希/nonce 双人校验”

因为当前链模型不是这样。

### 5.2 种子节点

当前最实际的主网候选工作是：

1. 准备真实 seed 节点
2. 替换 `seed.obtc.example.com`
3. 提供接入说明

建议最小化配置：

```ini
obtcmainnet=1
listen=0.0.0.0:9527
rpclisten=127.0.0.1:9528
txindex=1
notls=1
rpcuser=<user>
rpcpass=<pass>
addpeer=<seed1>
addpeer=<seed2>
```

### 5.3 发布产物

当前本仓可以直接发布的主要是：

- `btcd`
- `btcctl`

当前不应再把以下内容写成默认产物：

- `btcwallet`
- `obtc-status`

建议基于现有链路：

```bash
./release/release.sh <TAG>
```

当前 release 流程按 `release/README.md` 理解，更偏向：

- `manifest-<TAG>.txt`
- `shasum -a 256`
- GPG 签名

如果项目后续要改成 `SHA256SUMS + minisign`，那是额外发布流程改造，不是当前既有能力。

### 5.4 72h 观察窗口

主网候选初期的观察重点应该按**当前激活时序**理解：

- REAP 在 `EnableAtHeight` 之前不会进入真实运行阶段
- 因此早期 72h 更关注：
  - 出块连续性
  - peer 连通性
  - 同步耗时
  - replay protection 路径
  - expiry commitment 区块接受路径
  - 深度重组情况

REAP 覆盖率 / 积压这类指标，在未到激活窗口前应标成：

- `N/A (pre-activation)`

### 5.5 观测方式

如果还没有状态页，主网候选阶段可以先基于：

- `getblockchaininfo`
- `getpeerinfo`
- `getmempoolinfo`
- `getchaintips`
- 节点日志

如果要观测 `listexpiring` / `getexpiryindexstats`：

- 至少准备 1 个带 `--expiryindex` 的观测节点
- 不要把 scan/RPC 是否启用，误写成 commitment 是否维护

## 6. 验证命令

### 节点启动

```bash
go build ./...

./btcd --obtcmainnet --datadir=.obtc/mainnet-node \
  --listen=0.0.0.0:9527 \
  --rpclisten=127.0.0.1:9528 \
  --txindex --notls --rpcuser=u --rpcpass=p
```

### 发布脚本

```bash
./release/release.sh <TAG>
```

### 基本观测

```bash
./cmd/btcctl/btcctl --obtcmainnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:9528 getblockchaininfo
./cmd/btcctl/btcctl --obtcmainnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:9528 getpeerinfo
./cmd/btcctl/btcctl --obtcmainnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:9528 getchaintips
```

## 7. 完成标准（DoD）

- [ ] 主网参数表与 `chaincfg/params_obtc.go` 一致
- [ ] placeholder DNS seeds 被替换
- [ ] `docs/mainnet-join.md` 落位
- [ ] 发布流程明确基于现有 `release/release.sh`
- [ ] 文档不再要求“新主网创世生成”
- [ ] 72h 观察模板与指标口径明确区分 pre-activation / post-activation

## 8. 风险与约束

- **继续按新创世链规划主网**：会把整个发布流程带偏
- **placeholder seed 不替换**：候选主网无法稳定接入
- **把本仓不产出的工具写进发布清单**：到发布时会直接缺件
- **在 pre-activation 阶段强行要求 REAP 指标达标**：会让观察口径失真
