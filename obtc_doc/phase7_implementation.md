# Phase 7 计划（硬化、恢复、发布流程）— 按当前代码基线对齐

> 对齐日期：2026-03-07
> 目标：在当前 `master` 已有 REAP / Replay Protection / Expiry Commitment 基础上，补齐真正还缺的硬化项、恢复流程和发布流程。

## 1. 当前已实现的基线

这部分内容已经在当前代码里，不应再写成“未来才开始做”：

### 1.1 共识规则

- 普通交易不能花费已过期 UTXO
- REAP 只能花费已过期 UTXO
- REAP Marker 校验
- REAP 输入 canonical 顺序校验
- `ReapMaxInputs` 共识上限校验
- Replay Protection 激活与脚本验证
- Expiry Commitment（coinbase `OP_RETURN` + MuHash 前状态承诺）

对应代码：

- [`blockchain/validation_reap.go`](../blockchain/validation_reap.go)
- [`blockchain/validation_obtc_replay.go`](../blockchain/validation_obtc_replay.go)
- [`blockchain/expiryindex/expiryindex.go`](../blockchain/expiryindex/expiryindex.go)
- [`mining/mining.go`](../mining/mining.go)

### 1.2 ExpiryIndex 恢复基线

当前已存在：

- `CurrentIndexVersion = 2`
- version mismatch 检查
- `smartRebuild()`
- `fastRebuildFromUTXO()`
- `incrementalCatchUp()`

当前已经具备“已有自动恢复基线 + 显式 `--reindex-expiry` 入口”的 operator 恢复能力。
但这不等于发布、观测和故障演练层已经收口。

### 1.3 发布脚本基线

仓库当前已有：

- [`release/release.sh`](../release/release.sh)
- [`release/README.md`](../release/README.md)

它们当前的语义是：

- 走 `shasum -a 256`
- 生成 `manifest-<TAG>.txt`
- 发布链路默认基于 **GPG/manifest** 思路

当前**没有**：

- `build/release.sh`
- `Dockerfile.release`
- `minisign` 集成

## 2. 当前仍待补齐的硬化项

### 2.1 已冻结的 REAP 共识口径

这几项现在已经不再悬而未决：

- **不引入 `MaxReapTaxPerBlock`**
  - 原因：异常大的单个过期 UTXO 不应因为块级税帽而变成“永远无法合法处理”的卡死输入
- **每块最多 1 笔 REAP**
  - 从 `ReapConsensusAtHeight` 开始，区块级显式检查生效
- **不再使用 `BurnPolicy` 一词**
  - 协议口径改为：所有未返还部分都作为矿工收入，通过 REAP 交易隐含 fee 并入 coinbase
- **`REAP_VERSION` 冻结为 `3`**
  - 当前实现已按交易版本 `3` 识别和构造 REAP 系统交易

因此 Phase 7 的重点不再是重新讨论这些规则，而是：

1. 验证冻结后的规则与实现一致
2. 补齐运维与恢复能力
3. 完成故障注入和发布流程演练

### 2.2 仍未落地的运维/恢复能力

- chaos / fault injection 脚本
- 发布说明文档
- 可复现构建验证文档

## 3. Phase 7 目标

Phase 7 应改成四件事：

1. **硬化缺口审计**
2. **恢复流程补齐**
3. **chaos / failure 注入**
4. **发布流程标准化**

而不是继续把“已实现能力”当成待办。

## 4. 建议交付物

- `docs/phase7-validation.md`
  - 列出已实现 vs 未实现硬化项
- `docs/repro-build.md`
  - 基于 `release/release.sh` 的实际发布流程
- `scripts/validation/chaos/`
  - reorg、kill -9、I/O 干扰、无效 REAP 注入

这里建议把原文档里不存在的 `tools/chaos/` 改为贴近当前仓库结构的 `scripts/validation/chaos/`。

## 5. 任务拆解

### 5.1 共识硬化差距清单

先明确三类状态：

#### 已实现

- `ReapMaxInputs` 共识校验
- 过期花费规则
- REAP 输入顺序
- Replay Protection
- Expiry Commitment

#### 待确认

- 模板侧与验证侧对 REAP 识别是否完全一致

#### 未实现

- 完整的 chaos / failure 注入脚本集

### 5.2 索引恢复与版本迁移

当前代码的真实口径应写成：

- `indexVersion = 2` 已存在
- mismatch 会报错退出
- 启动后依赖 `smartRebuild()` 自动追平

如果希望更强的 operator 体验，Phase 7 应继续补：

- 运维文档：什么情况下依赖自动 rebuild，什么情况下执行 `--reindex-expiry`

### 5.3 故障注入

建议最小矩阵：

1. reorg 回放
2. `kill -9` 重启恢复
3. 无效 REAP/mempool 对抗
4. I/O 干扰

关注结果：

- 不错误分叉
- 索引与 accumulator 不漂移
- 重启后能继续同步

### 5.4 发布流程标准化

当前最务实的做法不是新造一套 `build/release.sh`，而是：

1. 基于现有 `release/release.sh`
2. 明确支持的目标平台
3. 明确 `btcd` / `btcctl` 是本仓当前产物
4. 如果后续要引入 `minisign`，作为**新增工作项**，不是当前基线

当前 repo 里没有 `btcwallet`，所以 Phase 7 文档不应再把 `btcwallet` 写成本仓直接发布产物。

## 6. 验证命令

### 测试

```bash
go test ./...
go test ./blockchain/... -count=1
go test ./mining/... -count=1
```

### 发布脚本基线

```bash
./release/release.sh <TAG>
```

产物和校验当前按 `release/README.md` 理解，应先基于：

- `manifest-<TAG>.txt`
- `shasum -a 256`
- GPG 签名

如果项目后续确定切到 `minisign`，应在文档里单独作为切换项记录。

## 7. 完成标准（DoD）

- [ ] `docs/phase7-validation.md` 明确列出已实现与未实现硬化项
- [ ] 至少完成一轮 reorg / kill -9 / 无效 REAP 对抗验证
- [ ] 发布流程文档与当前 `release/release.sh` 对齐
- [ ] 文档不再引用不存在的 `build/release.sh`、`Dockerfile.release`、`btcwallet`

## 8. 风险与约束

- **把已实现能力继续当待办**：会掩盖真正缺口
- **把不存在的发布链路写成默认流程**：到发布阶段会直接卡住
- **自动 rebuild 与 `--reindex-expiry` 的使用场景写不清**：operator 容易误操作
- **没有 chaos 注入记录**：发布前对恢复能力的把握仍然偏弱
