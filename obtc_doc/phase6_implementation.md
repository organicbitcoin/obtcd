# Phase 6 计划（OBTC Testnet 部署与观测）— 按当前代码基线对齐

> 对齐日期：2026-03-07
> 目标：把 OBTC Testnet 以**当前 `master` 已有实现**为基线部署起来，并补足最小观测与接入文档。

## 1. 当前基线

Phase 6 现在不能再按“新链创世生成”去理解。当前代码明确表明：

- OBTC 是 **Bitcoin 的硬分叉**，不是独立新创世链；
- `ObtcTestNetParams` 已经存在于 [`chaincfg/params_obtc.go`](/Users/pengyu/src/obtcd/chaincfg/params_obtc.go)；
- Testnet 复用 **Bitcoin testnet3 genesis**，在 `ObtcTestNetForkHeight` 之后分叉；
- 部署时应使用实际网络标志 `--obtctestnet`，不是文档里常见的 `--network=obtc-testnet` 写法。

当前代码里的 Testnet 关键参数：

| 项目 | 当前值 |
|------|--------|
| 网络名 | `obtctestnet` |
| P2P 默认端口 | `19527` |
| Bech32 HRP | `obtct` |
| Fork Height | `2800000` |
| `WindowBlocks` | `1008` |
| `EnableAtHeight` | `ObtcTestNetForkHeight + 100` |
| `ReapConsensusAtHeight` | `ObtcTestNetForkHeight + 120` |
| `ReplayProtectionAtHeight` | `ObtcTestNetForkHeight + 130` |
| `ReapMaxInputs` | `500` |
| `ExpiryCommitmentEnableAtHeight` | `ObtcTestNetForkHeight + 100` |

## 2. 当前缺口

这几个能力在仓库里**还没有现成实现**，文档不能再写成既成事实：

- 没有 `cmd/gengenesis/`
- 没有 `cmd/checkgenesis/`
- 没有 `cmd/obtc-status/`
- 没有 `docs/testnet-join.md`
- 没有专用 `tools/reap-audit.go`

当前可直接复用的只有：

- 节点二进制 `btcd`
- 命令行 `btcctl`
- 根目录 [`Dockerfile`](/Users/pengyu/src/obtcd/Dockerfile)
- RPC：`getblockchaininfo`、`getpeerinfo`、`getmempoolinfo`、`getchaintips`、`listexpiring`、`getexpiryindexstats`

## 3. Phase 6 目标

本阶段的目标应收敛为：

1. 用现有 `ObtcTestNetParams` 跑起可连通的 Testnet 节点
2. 把 placeholder DNS seeds 替换成真实种子
3. 用现有 RPC 和日志提供最小观测
4. 补齐接入文档和部署脚本

不应再把“新创世生成器”作为 Phase 6 前提。

## 4. 交付物

建议交付物改为：

- `chaincfg/params_obtc.go`
  - 核对并冻结 Testnet 参数
  - 替换 placeholder DNS seeds
- `docs/testnet-join.md`
  - 节点接入、端口、校验、排错
- `docs/phase6-validation.md`
  - 同步耗时、互联情况、观测截图或日志摘要
- `infra/` 或 `scripts/`
  - 最小部署脚本
  - `systemd` 示例
  - 防火墙/端口说明

如果后续要做状态页，再单独作为新增项，不在本阶段默认前提里写死 `obtc-status`。

## 5. 任务拆解

### 5.1 Testnet 参数冻结

重点核对：

- `DefaultPort = 19527`
- `Bech32HRPSegwit = "obtct"`
- `GenesisBlock = testNet3GenesisBlock`
- `GenesisHash = testNet3GenesisHash`
- `WindowBlocks = 1008`
- `ReapMaxInputs = 500`

这里要特别注意：

- 当前文档不能再写“生成新的 Testnet 创世”
- 真正需要冻结的是 **fork height 之后的 OBTC 参数**，不是 genesis 本身

### 5.2 种子节点部署

建议目标：

- 至少 2~3 台长期在线节点
- 可先单地域双节点，再扩到 EU / US / AS

建议最小配置项：

```ini
obtctestnet=1
listen=0.0.0.0:19527
rpclisten=127.0.0.1:19528
txindex=1
notls=1
rpcuser=<user>
rpcpass=<pass>
addpeer=<seed1>
addpeer=<seed2>
```

### 5.3 最小观测

当前最现实的做法是基于现有 RPC 拼出最小观测面：

- `getblockchaininfo`：块高、best hash、difficulty
- `getpeerinfo`：连接数
- `getmempoolinfo`：mempool 规模
- `getchaintips`：分叉/非 active tip
- `listexpiring`：即将到期 UTXO 扫描
- `getexpiryindexstats`：索引 tip 与规模

注意：

- `listexpiring` / `getexpiryindexstats` 属于 **scan/RPC 能力**
- 节点如果没开 `--expiryindex`，这些 RPC 不可用
- 但 expiry commitment 共识状态仍然会维护，这两件事不能混为一谈

因此：

- 至少要有一个“观测节点”显式开启 `--expiryindex`
- 不是所有节点都必须开

### 5.4 接入文档

`docs/testnet-join.md` 至少应包含：

- `--obtctestnet` 启动方式
- 默认 P2P 端口 `19527`
- 地址前缀 `obtct`
- RPC 示例
- 常见错误
  - 没连上 peers
  - 忘了加 `--obtctestnet`
  - 没开 `--expiryindex` 导致观测 RPC 不可用

## 6. 验证命令

### 本地双节点

```bash
go build ./...

./btcd --obtctestnet --datadir=.obtc/node1 \
  --listen=127.0.0.1:19527 \
  --rpclisten=127.0.0.1:19528 \
  --txindex --notls --rpcuser=u --rpcpass=p &

./btcd --obtctestnet --datadir=.obtc/node2 \
  --listen=127.0.0.1:19529 \
  --rpclisten=127.0.0.1:19530 \
  --txindex --notls --rpcuser=u --rpcpass=p \
  --connect=127.0.0.1:19527 &
```

### 观测节点 RPC

```bash
./cmd/btcctl/btcctl --obtctestnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:19528 getblockchaininfo
./cmd/btcctl/btcctl --obtctestnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:19528 getpeerinfo
./cmd/btcctl/btcctl --obtctestnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:19528 getchaintips
./cmd/btcctl/btcctl --obtctestnet --rpcuser=u --rpcpass=p --rpcserver=127.0.0.1:19528 getexpiryindexstats
```

## 7. 完成标准（DoD）

- [ ] Testnet 节点可使用 `--obtctestnet` 正常启动并互连
- [ ] 文档不再要求“新创世生成器”作为前提
- [ ] 至少 1 个观测节点开启 `--expiryindex` 并能提供 `getexpiryindexstats`
- [ ] `docs/testnet-join.md` 落位
- [ ] `docs/phase6-validation.md` 记录同步与互联结果

## 8. 风险与约束

- **placeholder seed 仍未替换**：外部接入会失败或不稳定
- **把 scan/RPC 和 commitment 状态混淆**：会误判节点健康
- **继续按新创世链理解 Testnet**：后续部署脚本和文档都会跑偏
- **没有专用状态页**：本阶段先接受“RPC + 日志”的最小观测方案
