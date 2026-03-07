# Expiry Commitment 实现文档（已对齐当前 `master`）

> 文档状态：Synced with `master`
> 对齐日期：2026-03-07
> 目标读者：协议开发、共识实现、挖矿模板维护者

## 1. 目标与范围

本文描述 OBTC 当前已经落地的 `Expiry Commitment` 实现。

它的目标是：

- 不修改区块头结构；
- 在 coinbase `OP_RETURN` 中承诺 expiry state root；
- 让节点在不依赖 `--expiryindex` 可选开关的前提下验证 commitment；
- 保持 reorg 可逆、启动可恢复、模板构造可 fail-fast。

本文只描述**当前代码已经实现的行为**，不再沿用早期设计稿里“单独新建 `blockchain/expirycommit/` 模块”的设想。

## 2. 当前实现落位

当前实现分布在这些文件里：

- `blockchain/expiryindex/commitment.go`
- `blockchain/expiryindex/accumulator.go`
- `blockchain/expiryindex/muhash.go`
- `blockchain/expiryindex/expiryindex.go`
- `mining/mining.go`
- `server.go`
- `config.go`

关键事实：

- `Expiry Commitment` 状态机没有单独拆包，直接并入 `ExpiryIndex`。
- 但在 **OBTC 网络** 上，`ExpiryIndex` 现在是 **always-on 的共识状态源**，不是单纯的可选扫描索引。
- `--expiryindex` 只控制扫描/RPC/REAP 选择功能，不控制 commitment 状态是否维护。

## 3. 协议规则概览

### 3.1 承诺位置

- 承诺只出现在 coinbase 交易输出中；
- 使用专用 `OP_RETURN` 脚本；
- 激活后每个区块必须且只能有一个有效 expiry commitment；
- 承诺的是**前状态**，即区块 `n` 承诺 `Root_{n-1}`。

### 3.2 承诺对象

语义状态定义为集合：

```text
State_h = { (outpoint, expiry_height) }
```

其中：

- `outpoint = txid(32B) + vout(4B)`
- `expiry_height = create_height + WindowBlocks`
- 只纳入进入 expiry 作用域的 UTXO

当前实现的作用域规则：

- 创建高度必须满足 `create_height >= StartScanHeight`
- 区块连接时跳过 `txscript.IsUnspendable(pkScript)` 输出

说明：

- 承诺的是**语义集合**，不是 `ExpiryIndex` 的 bucket 物理布局。
- 双向映射只是查询/扫描用的索引表示；共识承诺依赖的是 MuHash 累加器。

## 4. Coinbase 脚本格式

当前代码在 [`blockchain/expiryindex/commitment.go`](../blockchain/expiryindex/commitment.go) 中定义了固定格式：

```text
OP_RETURN OP_DATA_37 <TAG(4B)> <VERSION(1B)> <ROOT(32B)>
```

具体常量是：

- `TAG = "OEXP"`（4 字节）
- `VERSION = 0x01`
- `ROOT = 32` 字节 MuHash digest

严格约束：

- `pkScript` 总长度必须等于 `39` 字节
- 第二个 opcode 必须是 `OP_DATA_37`
- `TAG` 必须精确匹配 `"OEXP"`
- 同一 coinbase 中出现多个匹配脚本，区块无效
- 激活后缺失该脚本，区块无效

这里已经不是“建议 4~8 字节 tag”的设计稿状态，而是**固定 4 字节 tag** 的已实现协议。

## 5. 状态根算法

### 5.1 Canonical 元素编码

当前实现的单元素编码在 [`blockchain/expiryindex/accumulator.go`](../blockchain/expiryindex/accumulator.go)：

```text
elem = outpoint(36B) || expiry_height_u64_be(8B)
```

总长度固定为 `44` 字节。

### 5.2 累加器

当前使用的是 `MuHash3072`，实现位于 [`blockchain/expiryindex/muhash.go`](../blockchain/expiryindex/muhash.go)。

需要关心的接口只有四个：

- `Add(elem)`
- `Remove(elem)`
- `Digest()`
- `Serialize()/Deserialize()`

这保证了：

- `ConnectBlock` 时可以增量推进
- `DisconnectBlock` 时可以反向回滚
- 不需要每个块都全量重算整个 expiry state

### 5.3 快照模型

挖矿模板不会只拿一个 root，而是拿一个**原子快照**：

```go
type AccumulatorSnapshot struct {
    Root      [32]byte
    TipHash   chainhash.Hash
    TipHeight int32
}
```

这个结构由 `GetAccumulatorSnapshot()` 导出，用来防止“模板父块和 root 不对应”的竞态。

## 6. 激活规则

当前真正控制 commitment 的链参数只有：

- `ExpiryCommitmentEnableAtHeight`

校验规则：

- `height < ExpiryCommitmentEnableAtHeight`：不强制校验 commitment
- `height >= ExpiryCommitmentEnableAtHeight`：必须存在、唯一、格式正确且 root 匹配

需要区分这三个高度：

- `StartScanHeight`：从这里开始维护 expiry index 和 accumulator
- `EnableAtHeight`：从这里开始执行到期花费规则
- `ExpiryCommitmentEnableAtHeight`：从这里开始强制 coinbase expiry commitment

其中 `IsIndexingEnabled()` 看的是 `StartScanHeight`，不是 `EnableAtHeight`。

## 7. 区块连接与断连

### 7.1 ConnectBlock

当前 `ConnectBlock` 逻辑在 [`blockchain/expiryindex/expiryindex.go`](../blockchain/expiryindex/expiryindex.go)。

执行顺序是：

1. 如果 `blockHeight < StartScanHeight`
2. 只更新 `tip-height`
3. 只更新 `accumulator-tip-hash`
4. 不更新双向映射和 accumulator state

如果区块已经进入扫描范围：

1. 读取当前 accumulator state，这就是 `Root_{n-1}`
2. 如果 commitment 已激活，先校验 coinbase commitment
3. 遍历区块交易
4. 对每个非 coinbase 输入：
5. 从 `outpoint -> expiryKey` 映射读出旧条目
6. 先对 MuHash 执行 `Remove`
7. 再从双向映射中删除该 outpoint
8. 对每个新输出：
9. 跳过 `txscript.IsUnspendable(pkScript)`
10. 计算 `expiryKey = blockHeight + WindowBlocks`
11. 对 MuHash 执行 `Add`
12. 写入双向映射
13. 最后一次性持久化：
14. `accumulator-state`
15. `accumulator-tip-hash`
16. `tip-height`

当前实现里，commitment 验证是挂在 `ExpiryIndex.ConnectBlock()` 内完成的，而不是单独放到 `blockchain/validate*.go`。
但由于 `server.go` 会在 OBTC 网络上**无条件创建并启用**这个 indexer，所以它仍然构成共识路径的一部分。

### 7.2 DisconnectBlock

`DisconnectBlock` 是严格的逆操作：

1. 回滚本块新建的输出
2. 对这些输出做 MuHash `Remove`
3. 恢复本块花费的旧输出
4. 对这些旧输出做 MuHash `Add`
5. 回滚 `accumulator-state`
6. 回滚 `accumulator-tip-hash`
7. 回滚 `tip-height`

这保证 reorg 后：

- 双向映射一致
- accumulator root 一致
- 模板读取到的 snapshot 与主链 tip 一致

## 8. Commitment 验证规则

当前验证函数是 `validateExpiryCommitment()`。

激活后它做四件事：

1. 检查 coinbase 中匹配 commitment 的输出数量不能超过 1
2. 提取 commitment
3. 检查 `version == 0x01`
4. 检查 `root == expected Root_{n-1}`

当前错误码是：

- `ErrBadExpiryCommitmentMissing`
- `ErrBadExpiryCommitmentDuplicate`
- `ErrBadExpiryCommitmentFormat`
- `ErrBadExpiryCommitmentMismatch`

注意：

- 当前代码里**没有单独的** `ErrBadExpiryCommitmentVersion`
- 版本错误被归类为 `ErrBadExpiryCommitmentFormat`

## 9. 挖矿侧实现

挖矿路径位于 [`mining/mining.go`](../mining/mining.go)。

### 9.1 状态注入

`BlkTmplGenerator` 现在有两条不同的接线：

- `reapIndex *expiryindex.ExpiryIndex`
- `expiryState expiryCommitmentSource`

对应两个 setter：

- `SetREAPIndex()`
- `SetExpiryCommitmentSource()`

语义区别：

- `SetREAPIndex()`：只给 REAP 交易构造使用，可选
- `SetExpiryCommitmentSource()`：给 coinbase expiry commitment 使用，在 OBTC 网络上始终注入

### 9.2 NewBlockTemplate 中的行为

当 `nextBlockHeight >= ExpiryCommitmentEnableAtHeight` 时：

1. 必须存在 `expiryState`
2. 读取 `snapshot := GetAccumulatorSnapshot()`
3. 校验 `snapshot.TipHeight == best.Height`
4. 校验 `snapshot.TipHash == best.Hash`
5. 只有完全一致，才把 `snapshot.Root` 写入 coinbase

如果任一步失败，模板构造直接返回错误，不继续生成块模板。

这就是当前代码的 fail-fast 行为。

## 10. 启动接线与开关语义

当前接线在 [`server.go`](../server.go)。

### 10.1 OBTC 网络上的默认行为

只要是 OBTC 网络：

1. 无条件创建 `ExpiryIndex`
2. 无条件把它加入 `IndexManager`
3. 无条件在 blockchain 创建后注入 `ChainAccessor`
4. 无条件把它作为 `SetExpiryCommitmentSource()` 注入模板生成器

### 10.2 `--expiryindex` 的真实含义

`config.go` 当前说明已经改成：

```text
Enable ExpiryIndex scan/RPC features on OBTC networks.
Expiry commitment consensus state is maintained regardless.
```

也就是说这个开关只影响：

- 扫描/RPC 能力
- 挖矿时是否使用扫描索引去构造 REAP 交易

它**不影响**：

- accumulator 是否维护
- coinbase commitment 是否验证
- 区块有效性判断

## 11. 存储与恢复

当前实现没有新建独立的 `expirycommit` bucket，而是复用 [`blockchain/expiryindex/buckets.go`](../blockchain/expiryindex/buckets.go) 里的 `expiry-meta`。

当前元数据键是：

- `tip-height`
- `version`
- `accumulator-state`
- `accumulator-tip-hash`

当前索引版本：

- `CurrentIndexVersion = 2`

恢复路径：

1. `Init()` 读取当前 index version 和 tip height
2. 如果 `ChainAccessor` 还没注入，先延迟 rebuild
3. `SetChainAccessor()` 后运行 `smartRebuild()`
4. `smartRebuild()` 根据落后程度选择：
5. `fastRebuildFromUTXO()`
6. 或 `incrementalCatchUp()`

### 11.1 Fast rebuild

`fastRebuildFromUTXO()` 当前会：

1. 清空现有 expiry index buckets
2. 遍历 UTXO 集
3. 对每个符合 `StartScanHeight` 的 UTXO：
4. 重建双向映射
5. 在内存里同步重建 MuHash
6. 完成后一次性写回：
7. `accumulator-state`
8. `accumulator-tip-hash`
9. `tip-height`

如果中途失败，会重新清空 index，避免留下“看起来有效但实际半重建”的脏状态。

## 12. 与早期设计稿的差异

为了避免误读，这里明确列出当前实现和早期设计稿的差异：

### 12.1 已经落地的部分

- 使用 coinbase `OP_RETURN` 做前状态承诺
- 使用 MuHash3072 做增量 accumulator
- 使用 `AccumulatorSnapshot` 解决 tip/root 竞态
- 在 OBTC 网络上 always-on 维护 commitment 状态

### 12.2 当前没有按设计稿那样实现的部分

- 没有新建 `blockchain/expirycommit/` 目录
- `TAG` 和 `VERSION` 不是链参数，而是 `commitment.go` 中的固定常量
- validation 没有独立拆到新的 `blockchain/validate*.go` commitment 模块
- 存储没有单独的 `expiry-commit-meta` bucket，而是并入现有 `expiry-meta`

## 13. 测试覆盖

当前相关测试主要在：

- `blockchain/expiryindex/commitment_test.go`
- `blockchain/expiryindex/accumulator_test.go`
- `blockchain/expiryindex/muhash_test.go`
- `blockchain/expiryindex/rebuild_test.go`
- `mining/...` 中的模板相关测试

当前特别重要的一条回归测试是：

- 当 `StartScanHeight < EnableAtHeight` 时，live `ConnectBlock()` 维护出来的 accumulator root，必须与 `fastRebuildFromUTXO()` 一致

这条测试锁住的是“扫描起点”和“规则激活点”错位时不会出现状态根漂移。

## 14. 当前实现的约束与结论

可以把当前实现概括成一句话：

> `Expiry Commitment` 已经落在 `ExpiryIndex` 内部实现，但在 `server.go` 的接线语义上，它已经是 OBTC 网络上的 always-on 共识状态，而不是可选功能。

这也是阅读和后续维护时最需要记住的边界：

- `ExpiryIndex` 有两层职责：扫描索引 + commitment 状态机
- `--expiryindex` 只关扫描索引那一层
- commitment 验证和模板注入必须按 always-on 路径理解
