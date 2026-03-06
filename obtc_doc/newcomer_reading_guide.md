# OBTC 新人代码阅读指南（2025-10 至 2026-03）

> **目标读者**：初级程序员、刚加入 OBTC 项目的开发者
> **目标**：快速理解 OBTC 相对于上游 btcd 的全部新增代码，掌握核心概念、文件结构、变量含义、算法原理和阅读重点
> **语言**：中文为主，专业术语保留英文

---

## 目录

- [第一章：项目全局认知](#第一章项目全局认知)
- [第二章：开发时间线与里程碑](#第二章开发时间线与里程碑)
- [第三章：核心概念速查表](#第三章核心概念速查表)
- [第四章：网络参数层 chaincfg](#第四章网络参数层-chaincfg)
- [第五章：到期索引层 ExpiryIndex](#第五章到期索引层-expiryindex)
- [第六章：REAP 选择与交易构造层 mining/reap](#第六章reap-选择与交易构造层-miningreap)
- [第七章：共识验证层 blockchain/validation](#第七章共识验证层-blockchainvalidation)
- [第八章：重放保护 Replay Protection](#第八章重放保护-replay-protection)
- [第九章：挖矿模板集成 Mining Template](#第九章挖矿模板集成-mining-template)
- [第十章：RPC 扩展与内存池策略](#第十章rpc-扩展与内存池策略)
- [第十一章：基础设施与 DevNet](#第十一章基础设施与-devnet)
- [第十二章：测试体系全景](#第十二章测试体系全景)
- [第十三章：推荐阅读路径](#第十三章推荐阅读路径)
- [附录 A：完整新增文件清单](#附录-a完整新增文件清单)
- [附录 B：术语表](#附录-b术语表)
- [附录 C：常见问题](#附录-c常见问题)

---

## 第一章：项目全局认知

### 1.1 OBTC 是什么

OBTC（Organic Bitcoin）是基于 btcd（Go 语言实现的 Bitcoin 全节点）的**硬分叉（hard fork）**项目。它在比特币的基础架构上引入了一套叫做 **REAP（Resource Expiration and Allocation Protocol，资源到期与分配协议）** 的新机制。

**一句话概括**：OBTC 让长时间不动的 UTXO（未花费交易输出）自动"到期"，系统通过 REAP 交易回收这些到期资产，其中 70% 退还原持有者，30% 作为税收分配给矿工。

### 1.2 与上游 btcd 的关系

OBTC 保留了 btcd 的模块路径（`github.com/btcsuite/btcd`），这意味着：
- 大部分比特币核心代码**不需要修改**
- OBTC 新增的代码集中在**特定文件和特定包**中
- 本指南只关注 2025 年 10 月以来**新增或修改**的代码

### 1.3 架构分层（从下到上）

```
┌─────────────────────────────────────────────────┐
│  D. 挖矿模板集成 (mining/mining.go,             │
│     mining/template_reap.go)                     │
├─────────────────────────────────────────────────┤
│  C. REAP 选择与交易构造 (mining/reap/*.go)       │
├─────────────────────────────────────────────────┤
│  B. 到期索引 (blockchain/expiryindex/*.go)       │
├─────────────────────────────────────────────────┤
│  A. 网络参数与基础设施 (chaincfg/params_obtc.go, │
│     wire/protocol.go, scripts/)                  │
├─────────────────────────────────────────────────┤
│  横切：共识验证 + 重放保护 + RPC + 内存池策略      │
└─────────────────────────────────────────────────┘
```

### 1.4 数据流全景

一个完整的 REAP 处理流程如下：

```
1. 新区块连接 ──→ ExpiryIndex.ConnectBlock()
                    记录 "哪些 UTXO 在哪个高度到期"

2. 矿工构建模板 ──→ maybeBuildREAPTx()
                    ├─ collectExpiredOutpoints(): 从索引扫描到期候选
                    ├─ reap.SelectCandidates():   确定性选择+排序+截断
                    └─ reap.BuildBlueprint():     构造 REAP 系统交易

3. 区块验证 ──→ CheckTransactionInputs()
                ├─ checkReapMarker():            验证 Marker 完整性
                ├─ checkExpirySpendRules():      验证到期花费规则
                └─ checkReapConsensusHardening(): 验证输入排序和数量限制
```

---

## 第二章：开发时间线与里程碑

项目从 2025 年 10 月开始，至今共 **136 次提交**，分 8 个阶段推进：

| 时间 | 阶段 | 关键交付 | 对应 PR |
|------|------|---------|---------|
| 2025-10-07 | Phase 1 | OBTC 网络参数骨架、命名规范 | #2 |
| 2025-10-12~13 | Phase 1 | 硬分叉高度、CI/CD、DevNet 脚本 | #3 |
| 2025-10-18~19 | Phase 2 | ExpiryIndex 核心实现 | #4 |
| 2025-10-26 | Phase 2 | ExpiryIndex 测试完善 | #5 |
| 2025-11-02 | Phase 2 | RPC 集成（listexpiring, getexpiryindexstats） | #6 |
| 2026-01-30~31 | Phase 2 | 验证工具、分页扫描 | #7, #8 |
| 2026-02-04~05 | Phase 3 | REAP Selector 蓝图构造器 | #10 |
| 2026-02-10~13 | Phase 3.1 | Marker、DryRun、Dust 规则 | #11~#18 |
| 2026-02-15 | Phase 4 | 共识验证 + 模板接线（31 次提交） | #19~#27 |
| 2026-02-27~28 | Phase 4+ | 测试加固、Staircase 压力、并发回归 | #28~#38 |
| 2026-03-01~03 | Phase 5 | 重放保护、命名空间隔离、ExpiryIndex 修复 | #39~#44 |

---

## 第三章：核心概念速查表

在阅读代码之前，请确保理解以下概念：

### 3.1 UTXO（Unspent Transaction Output）

比特币的"账户余额"模型。每一笔交易消耗一些旧的 UTXO（作为输入），产生新的 UTXO（作为输出）。OBTC 的核心改动就是给每个 UTXO 加了一个"到期时间"。

### 3.2 ExpiryKey（到期键）

**公式**：`ExpiryKey = CreateHeight + WindowBlocks`

- `CreateHeight`：UTXO 被创建时所在的区块高度（block height）
- `WindowBlocks`：到期窗口，主网为 368,880 块（约 7 年）
- `ExpiryKey`：UTXO 到期的区块高度

**举例**：一个在高度 100,000 创建的 UTXO，其 ExpiryKey = 100,000 + 368,880 = 468,880。当链到达高度 468,880 时，这个 UTXO 就"到期"了。

### 3.3 REAP 交易（系统交易）

矿工在出块时自动生成的特殊交易，用于回收到期 UTXO。特征：
- **交易版本号（Version）**= 3（普通比特币交易是 1 或 2）
- **输入**：若干到期 UTXO
- **输出**：退款输出（refund output）+ 一个 Marker 输出
- **Marker**：OP_RETURN 输出，值为 0，payload 格式 `REAP:<height>:<count>:<digest>`

### 3.4 税收和退款

对每个被 REAP 回收的 UTXO：
- **Tax（税）**= `floor(value × TaxNum / TaxDen)` = `floor(value × 30 / 100)`
- **Refund（退款）**= `value - tax`
- 如果 Refund 低于 Dust 阈值（546 satoshi），则整个金额归为 Tax

税收不会出现在 REAP 交易的输出中——它作为隐含费用（implicit fee）加入 coinbase 奖励。

### 3.5 Marker（标记输出）

REAP 交易的最后一个输出，是一个 **OP_RETURN** 脚本，内容格式：
```
REAP:<区块高度>:<输入数量>:<摘要哈希>
```
例如：`REAP:950100:256:a3f2b8d9e1c4f7...`

Marker 的作用是让任何节点都可以独立验证："这笔 REAP 交易是否正确处理了指定数量的到期 UTXO"。

### 3.6 重放保护（Replay Protection）

由于 OBTC 从比特币硬分叉，同一笔签名在两条链上都可能有效（即"重放攻击"）。OBTC 通过在签名哈希（sighash）前加上域分离标签（domain separation tag）来解决：
- Legacy 脚本：`"OBTC/SigHashV0/v1"`
- SegWit v0：`"OBTC/SigHashV1/v1"`
- Taproot：`"OBTC/TapSighash/v1"`

---

## 第四章：网络参数层 chaincfg

### 4.1 文件：`chaincfg/params_obtc.go`

**阅读重点**：这是整个 OBTC 项目的"配置根"，几乎所有其他模块都从这里读取网络参数。

#### 4.1.1 ExpiryParams 结构体（到期参数）

```go
type ExpiryParams struct {
    WindowBlocks             uint64  // UTXO 到期窗口（块数）
    ListBatchLimit           int     // RPC 单次返回上限
    StartScanHeight          int32   // 开始构建索引的块高
    EnableAtHeight           int32   // 开始强制到期规则的块高
    ReapConsensusAtHeight    int32   // 启用规范 REAP 排序/限制的块高
    ReplayProtectionAtHeight int32   // 启用重放保护的块高
    ReapMaxInputs            int     // 共识级别的 REAP 最大输入数
}
```

**各字段含义详解**：

| 字段 | 含义 | 主网值 | 为什么重要 |
|------|------|--------|-----------|
| `WindowBlocks` | UTXO 从创建到到期经过的区块数 | 368,880（≈7年） | 决定"到期"的定义 |
| `ListBatchLimit` | RPC `listexpiring` 命令一次最多返回多少条 | 1000 | 防止 RPC 拒绝服务 |
| `StartScanHeight` | ExpiryIndex 从哪个高度开始索引 | 分叉高度 | 分叉前的 UTXO 不需要索引 |
| `EnableAtHeight` | 到期规则从哪个高度生效 | 分叉高度+100,000 | 给用户缓冲期 |
| `ReapConsensusAtHeight` | 强制 REAP 输入排序规范的高度 | 分叉高度+110,000 | 渐进激活共识规则 |
| `ReplayProtectionAtHeight` | 重放保护激活高度 | 分叉高度+115,000 | 签名域分离 |
| `ReapMaxInputs` | 单个 REAP 交易最多包含的输入数 | 256 | 防止交易过大 |

#### 4.1.2 三个 OBTC 网络

```
┌──────────┬──────────────┬───────┬──────────┬────────────────┐
│ 网络      │ Magic Number │ 端口   │ Bech32   │ WindowBlocks   │
├──────────┼──────────────┼───────┼──────────┼────────────────┤
│ MainNet  │ 0x4F425443   │ 8555  │ "obtc"   │ 368,880 (7年)  │
│ TestNet  │ 0x4F544553   │ 28555 │ "obtct"  │ 1,008 (1周)    │
│ RegTest  │ 0x4F524547   │ 28666 │ "obtcrt" │ 144 (1天)      │
└──────────┴──────────────┴───────┴──────────┴────────────────┘
```

- **Magic Number**：4 字节网络标识，用于 P2P 消息头，防止不同网络的消息混淆
- **Bech32 HRP**：地址前缀，如 `obtc1qxyz...`，与比特币的 `bc1` 区分
- **WindowBlocks**：测试网设得小是为了快速验证到期逻辑

#### 4.1.3 核心辅助函数

```go
IsOBTC(params *Params) bool
  // 判断当前网络是否为 OBTC 网络
  // 内部实现：检查 Net 字段是否为 ObtcMainNet/ObtcTestNet/ObtcRegNet 之一

IsPostOBTCFork(params *Params, height int32) bool
  // 判断给定高度是否在硬分叉之后
  // 用途：在此高度之后才应用 OBTC 特有的共识规则

GetExpiryParams(params *Params) *ExpiryParams
  // 获取当前网络的到期参数
  // 非 OBTC 网络返回 nil

IsOBTCReplayProtectionActive(params *Params, height int32) bool
  // 检查重放保护是否已激活
```

**阅读提示**：先看 `ObtcMainNetParams` 变量的完整定义，理解它继承了哪些比特币参数、修改了哪些。

#### 4.1.4 命名空间隔离验证

`init()` 函数中调用 `validateOBTCNamespaceIsolation()`，该函数在**程序启动时**自动检查：
- OBTC 的地址前缀不与 Bitcoin 冲突
- Bech32 HRP 唯一
- HD（分层确定性钱包）的 CoinType 唯一

**为什么重要**：如果地址前缀重复，用户可能把 OBTC 的币误发到比特币地址，造成资产丢失。

### 4.2 文件：`chaincfg/params_obtc_test.go`

测试网络参数的正确性，包括：
- 地址编码/解码
- 网络识别（`IsOBTC`）
- 分叉高度计算
- 命名空间隔离

### 4.3 文件：`wire/protocol.go`（修改）

新增三个网络常量：
```go
ObtcMainNet BitcoinNet = 0x4F425443  // "OBTC" 的 ASCII
ObtcTestNet BitcoinNet = 0x4F544553  // "OTES"
ObtcRegNet  BitcoinNet = 0x4F524547  // "OREG"
```

---

## 第五章：到期索引层 ExpiryIndex

ExpiryIndex 是 OBTC 最核心的基础设施之一，它回答一个核心问题：**"当前有哪些 UTXO 已经到期？"**

### 5.1 包结构概览

```
blockchain/expiryindex/
├── doc.go                  # 包文档（设计原则说明）
├── expiryindex.go          # 核心索引实现 ★★★ 最重要
├── buckets.go              # 数据库桶管理
├── encode.go               # 编码/解码（确定性序列化）
├── params.go               # 过期参数适配
├── log.go                  # 日志配置
├── benchmark_test.go       # 性能基准测试
├── buckets_test.go         # 桶操作测试
├── database_test.go        # 数据库集成测试
├── encode_test.go          # 编码单元测试
├── encode_extra_test.go    # 编码边界测试
├── expiryindex_test.go     # 核心索引测试
├── helpers_extra_test.go   # 辅助函数测试
├── params_extra_test.go    # 参数测试
├── rebuild_test.go         # 重建策略测试
├── scan_extra_test.go      # 扫描边界测试
├── scan_staircase_test.go  # Staircase 分页测试 ★★ 重要
├── sequence_fuzz_test.go   # Fuzz 测试
└── utxo_test.go            # UTXO 操作测试
```

### 5.2 文件：`expiryindex.go` ★★★

这是本包最核心的文件，必须仔细阅读。

#### 5.2.1 ChainAccessor 接口

```go
type ChainAccessor interface {
    BestHeight() int32
    BlockByHeight(height int32) (*btcutil.Block, error)
    FetchSpendJournal(block *btcutil.Block) ([]SpentTxOut, error)
    ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error
}
```

**为什么需要这个接口**？因为 btcd 的初始化顺序是：先创建 IndexManager（含 ExpiryIndex），再创建 BlockChain。所以 ExpiryIndex 创建时拿不到 BlockChain 引用，需要后续通过 `SetChainAccessor()` 注入。这个设计模式叫**延迟注入（deferred injection）**。

#### 5.2.2 ExpiryIndex 结构体

```go
type ExpiryIndex struct {
    db              database.DB        // bbolt 数据库实例
    params          *chaincfg.Params   // 链参数（用于判断网络）
    expiryParams    *ExpiryParams      // 过期参数（WindowBlocks 等）
    curTipHeight    int32              // 当前已索引到的最高块高度
    disabled        bool               // 是否禁用（非 OBTC 网络时为 true）
    chain           ChainAccessor      // 区块链访问器（延迟设置）
}
```

#### 5.2.3 双向映射——核心数据结构

ExpiryIndex 在数据库中维护**两个桶（bucket）**，构成双向映射：

```
正向映射（用于花费时快速删除）：
  bktOutpoint2Expiry: OutPoint(36字节) → ExpiryKey(8字节)

  例如: {txid:abc..., vout:0} → 468880

反向映射（用于按到期顺序扫描）：
  bktExpiry2Outpoints: ExpiryKey(8字节) → [OutPoint列表]

  例如: 468880 → [{txid:abc...,vout:0}, {txid:def...,vout:1}, ...]
```

**为什么需要双向映射**？
- 当一个 UTXO 被花费时，需要从索引中删除它。此时只知道 OutPoint，需要正向映射找到它的 ExpiryKey，然后从反向映射中移除。
- 当矿工构建 REAP 交易时，需要找到所有"在某个高度范围内到期"的 UTXO。此时用反向映射按 ExpiryKey 范围扫描。

#### 5.2.4 ConnectBlock——区块连接

```go
func (idx *ExpiryIndex) ConnectBlock(dbTx database.Tx,
    block *btcutil.Block, stxos []blockchain.SpentTxOut) error
```

当一个新区块被添加到主链时调用。核心流程：

```
1. 如果当前高度 < StartScanHeight，仅更新 tipHeight，返回
2. 遍历区块中的所有交易：
   a. 对于每个输入（除 coinbase 外）：
      调用 disconnectTxOut(outpoint)  // 被花费的 UTXO 从索引中移除
   b. 对于每个输出：
      调用 connectTxOut(outpoint, blockHeight)  // 新 UTXO 加入索引
3. 更新 tipHeight
```

**关键函数 connectTxOut**：
```go
func (idx *ExpiryIndex) connectTxOut(dbTx, outpoint, createHeight) error {
    expiryKey := createHeight + WindowBlocks   // 计算到期高度
    // 正向映射：存储 outpoint → expiryKey
    bktOutpoint2Expiry.Put(encodeOutPoint(outpoint), encodeExpiryKey(expiryKey))
    // 反向映射：将 outpoint 追加到 expiryKey 的列表中
    appendOutPointToList(bktExpiry2Outpoints, expiryKey, outpoint)
}
```

#### 5.2.5 DisconnectBlock——区块断开（链重组）

```go
func (idx *ExpiryIndex) DisconnectBlock(dbTx database.Tx,
    block *btcutil.Block, stxos []blockchain.SpentTxOut) error
```

当发生链重组（reorg）时调用，执行 ConnectBlock 的**反向操作**：
- 移除该区块创建的 UTXO
- 恢复该区块花费的 UTXO

**重要**：ConnectBlock 和 DisconnectBlock 的对称性是**数据一致性的关键**。如果这两个函数不完全对称，链重组后索引数据会不一致。

#### 5.2.6 ScanExpiringUTXOs——Staircase 分页扫描

```go
func (idx *ExpiryIndex) ScanExpiringUTXOs(
    fromKey, toKey uint64,     // ExpiryKey 范围 [fromKey, toKey]
    maxResults int,            // 最多返回多少条
    startAfter *wire.OutPoint, // 分页游标（上一页的最后一项）
) ([]*ExpiringUTXO, bool, error)
```

**Staircase（阶梯式）分页的含义**：

想象 ExpiryKey 是"楼层"，同一楼层有多个 UTXO（"房间"）：

```
ExpiryKey=100: [utxo_a, utxo_b, utxo_c]     ← 第一级阶梯
ExpiryKey=200: [utxo_d, utxo_e]              ← 第二级阶梯
ExpiryKey=300: [utxo_f, utxo_g, utxo_h, ...] ← 第三级阶梯
```

分页时，游标可能落在某个"楼层"的中间位置。比如上一页返回到 `utxo_b`，下一页需要：
1. 在 ExpiryKey=100 这个楼层，从 `utxo_b` 之后继续（`utxo_c`）
2. 遍历完这个楼层后，上到下一个楼层（ExpiryKey=200）
3. 依此类推

**关键实现细节**：
- 使用 `findOutPointStartIndex()` 做二分查找，定位"上一页最后一项"在当前 ExpiryKey 列表中的位置
- `compareOutPoint()` 比较两个 OutPoint：先按 Hash 字典序，再按 Index 数值

#### 5.2.7 索引重建策略（smartRebuild）

```
smartRebuild(indexTipHeight):
├── indexTipHeight == -1（首次初始化）
│   └─ tryFastRebuildOrFallback()
│      ├─ 尝试 fastRebuildFromUTXO()  // 遍历全量 UTXO 集重建
│      └─ 失败则 incrementalCatchUp() // 逐块追赶
├── lag > 1000 blocks（显著落后）
│   └─ tryFastRebuildOrFallback()     // 全量重建更高效
├── 0 < lag ≤ 1000 blocks（轻微落后）
│   └─ incrementalCatchUp()           // 增量追赶更快
└── lag == 0 → 已最新，无操作
```

**fastRebuildFromUTXO**：直接遍历数据库中的全部 UTXO 集，重新计算每个 UTXO 的 ExpiryKey 并填充索引。适用于首次启动或大幅落后。

**incrementalCatchUp**：逐块处理从 fromHeight+1 到 toHeight 的每个区块，模拟 ConnectBlock 操作。适用于轻微落后。

### 5.3 文件：`encode.go`

定义三种确定性编码格式：

#### OutPoint 编码（固定 36 字节）
```
[0:32]  交易哈希（TxID），32 字节，原始字节序
[32:36] 输出索引（Vout），4 字节，小端序（Little-Endian）
```

#### ExpiryKey 编码（固定 8 字节）
```
8 字节，大端序（Big-Endian）的 uint64
```

**为什么用大端序**？因为数据库（bbolt）按键的字节序排序。大端序保证数值小的 ExpiryKey 排在前面，从而实现**自然有序扫描**——直接遍历数据库键就是按到期时间从早到晚。

#### OutPointList 编码（变长）
```
[0:4]    列表长度 N，4 字节小端序
[4:40]   第一个 OutPoint（36 字节）
[40:76]  第二个 OutPoint（36 字节）
...
```
列表中的 OutPoint 按字典序排列，保证编码的确定性。

**关键函数**：
- `appendOutPointToList()` —— 追加新 OutPoint 到已编码的列表，自动去重
- `removeOutPointFromList()` —— 从列表中删除指定 OutPoint，列表为空时返回 nil

### 5.4 文件：`buckets.go`

管理三个数据库桶和元数据：

```go
bktExpiryMeta        = "expiry-meta"          // 元数据（版本号、已索引高度）
bktOutpoint2Expiry   = "outpoint-to-expiry"   // 正向映射
bktExpiry2Outpoints  = "expiry-to-outpoints"  // 反向映射
```

元数据键：
```go
keyTipHeightIndexed = "tip-height"  // 已索引到的最高块高度
keyIndexVersion     = "version"     // 索引版本号（当前为 1）
```

重要常量：
```go
MaxOutpointsPerKey = 10000   // 单个 ExpiryKey 下最多存多少个 OutPoint
DefaultBatchSize   = 1000    // 默认事务批处理大小
```

### 5.5 文件：`params.go`

将 `chaincfg.ExpiryParams` 适配为 `expiryindex.ExpiryParams`，并提供辅助函数：

```go
CalculateExpiryKey(createHeight int32) uint64
  // 核心公式：return uint64(createHeight) + WindowBlocks

IsExpiryEnabled(height int32) bool
  // height >= EnableAtHeight

IsIndexingEnabled(height int32) bool
  // height >= StartScanHeight

CalculateExpiryRange(fromHeight int32, horizonBlocks uint64) (fromKey, toKey uint64)
  // 计算 RPC 扫描范围

GetDefaultHorizon() uint64
  // 默认扫描视域：144 块（约 1 天）
```

### 5.6 文件：`blockchain/expiry_chain_accessor.go`

**适配器模式**：将 `*blockchain.BlockChain` 包装为 `expiryindex.ChainAccessor` 接口。

```go
type ExpiryChainAccessor struct {
    chain *BlockChain
}
```

实现的方法：
- `BestHeight()` → 返回当前最佳链高度
- `BlockByHeight()` → 按高度获取区块
- `FetchSpendJournal()` → 获取区块的花费日志
- `ForEachUTXO()` → 遍历全部 UTXO（用于快速重建）

### 5.7 文件：`blockchain/utxo_iter.go`

提供底层 UTXO 遍历方法，供 ExpiryIndex 快速重建使用：

```go
func (b *BlockChain) ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error
```

实现细节：
1. 获取读锁（防止并发修改）
2. 打开数据库只读事务
3. 遍历 `utxoSetBucketName` 桶的所有键值对
4. 从键中解析 OutPoint（32 字节哈希 + VLQ 编码的 vout）
5. 从值中反序列化 UTXOEntry 获取区块高度
6. 对每个条目调用回调函数 `fn`

---

## 第六章：REAP 选择与交易构造层 mining/reap

这一层回答："到期的 UTXO 怎么处理？"

### 6.1 包结构概览

```
mining/reap/
├── types.go         # 核心数据类型定义 ★ 先读这个
├── params.go        # REAP 运行参数配置
├── selector.go      # 候选选择算法 ★★★ 最核心
├── packer.go        # 蓝图交易构造
├── reaptx.go        # REAP 交易构建与识别 ★★★
├── dust.go          # Dust 折叠规则
├── marker.go        # Marker 摘要计算
├── weight.go        # 交易权重估算
├── dryrun.go        # Dry-run 摘要生成
├── bench_test.go            # 性能测试
├── dryrun_test.go           # DryRun 测试
├── dust_test.go             # Dust 规则测试
├── dust_extreme_test.go     # Dust 极端边界测试
├── marker_vector_test.go    # Marker 向量测试
├── packer_test.go           # Packer 测试
├── params_test.go           # 参数验证测试
├── reaptx_test.go           # REAP 交易测试
├── reap_extra_test.go       # 额外回归测试
├── selector_test.go         # Selector 测试
├── staircase_pressure_test.go  # 阶梯压力测试
├── stress_regression_test.go   # 压力回归测试
└── week3_extra_test.go      # 历史遗留测试（已重命名）
```

### 6.2 文件：`types.go` ★ 先读

定义核心数据类型，理解它们是理解整个 REAP 流程的基础。

#### SortMode（排序模式）

```go
type SortMode int

const (
    SortModeStrict SortMode = iota  // 严格排序：按 ExpiryKey → 金额 → TxID → Vout
    SortModeSimple                  // 简单排序：按 ExpiryKey → TxID → Vout（忽略金额）
)
```

**为什么有两种模式**？`SortModeStrict` 用于共识验证（所有节点必须产生相同结果），`SortModeSimple` 用于早期测试。

#### REAPPlan（REAP 执行计划）

```go
type REAPPlan struct {
    Inputs      []wire.OutPoint  // 选中的输入列表（到期 UTXO 的引用）
    TaxTotal    int64            // 总税收金额（单位：satoshi）
    RefundTotal int64            // 总退款金额（单位：satoshi）
    Height      int32            // 当前区块高度
    Stats       REAPStats        // 选择统计信息
}
```

#### REAPStats（选择统计）

```go
type REAPStats struct {
    Candidates int    // 扫描到的候选 UTXO 总数
    Picked     int    // 实际选中的输入数量
    Skipped    int    // 因配额限制被跳过的数量
    EstWeight  int64  // 估算的交易权重（vBytes）
}
```

#### 错误变量

```go
var (
    ErrNilView  = fmt.Errorf("nil utxo view")    // UTXO 视图为空
    ErrNilIndex = fmt.Errorf("nil expiry index")  // 到期索引为空
)
```

### 6.3 文件：`params.go`

#### REAPParams 结构体

```go
type REAPParams struct {
    Sort             SortMode  // 候选排序模式
    MaxInputs        int       // 单个 REAP 交易最大输入数
    WeightBudget     int64     // 权重预算（vBytes）
    ScanBatch        int       // 每批次从索引扫描的数量
    TaxNum           int64     // 税率分子（默认 30）
    TaxDen           int64     // 税率分母（默认 100）
    DustThresholdSat int64     // Dust 阈值（默认 546 satoshi）
}
```

**各参数在不同网络的默认值**：

| 参数 | 全局默认 | 主网 | 测试网 | 回归测试 |
|------|---------|------|--------|---------|
| MaxInputs | 1000 | **256** | 500 | 200 |
| WeightBudget | 400,000 | **200,000** | 400,000 | 400,000 |
| ScanBatch | 10,000 | 10,000 | 5,000 | 2,000 |
| TaxNum/TaxDen | 30/100 | 30/100 | 30/100 | 30/100 |
| DustThresholdSat | 546 | 546 | 546 | 546 |

**主网为什么更保守？**
- `MaxInputs=256`（全局默认 1000）：限制单个 REAP 交易大小，避免占用过多区块空间
- `WeightBudget=200,000`（全局默认 400,000）：为正常交易留出更多空间

#### Validate() 方法

```go
func (p REAPParams) Validate() error
```
检查参数合法性：MaxInputs > 0、ScanBatch > 0、TaxDen > 0、TaxNum >= 0、DustThresholdSat >= 0。

### 6.4 文件：`selector.go` ★★★ 核心文件

这是 REAP 的"大脑"——决定每个区块要回收哪些到期 UTXO。

#### 主入口：SelectCandidates

```go
func SelectCandidates(
    ctx context.Context,
    tip int32,                              // 当前链尖高度
    idx *expiryindex.ExpiryIndex,           // 到期索引实例
    view *blockchain.UtxoViewpoint,         // UTXO 视图
    p REAPParams,                           // REAP 参数
) (REAPPlan, error)
```

#### 算法流程（selectCandidatesWithScanner）

```
步骤 1：参数校验
  - view 不为空
  - scanner 不为空
  - MaxInputs > 0

步骤 2：批量扫描过期 UTXO
  扫描范围：[fromKey=0, toKey=uint64(tip)]
  循环：
    results = scanner.ScanExpiringUTXOs(fromKey, toKey, ScanBatch, startAfter)
    对于每个 result：
      entry = view.LookupEntry(outpoint)
      如果 entry == nil 或已花费 → 跳过（UTXO 不存在或已花费）
      加入 candidates 列表
    如果 !hasMore → 退出循环
    更新 startAfter 为最后一条结果

步骤 3：排序
  调用 sortCandidates(candidates, p.Sort)

步骤 4：贪心选择
  for each candidate（按排序后的顺序）：
    如果已选数量 >= MaxInputs → 停止
    如果 EstimateBlueprintWeight(已选数量+1) > WeightBudget → 停止
    计算 tax = taxForValue(amount, p)
    计算 refund = amount - tax
    应用 dust 规则：(refund, tax) = applyDustRule(refund, tax, DustThresholdSat)
    加入选中列表
    累计 TaxTotal 和 RefundTotal

步骤 5：返回 REAPPlan
```

#### 排序函数：sortCandidates

```go
func sortCandidates(cs []candidate, mode SortMode)
```

排序优先级（从高到低）：
1. **ExpiryKey 升序**——最先到期的 UTXO 优先处理
2. **金额升序**（仅 SortModeStrict）——小额 UTXO 优先消化
3. **TxID 字典序**——确保跨节点一致
4. **Vout 升序**——同一交易的不同输出的确定性顺序

**为什么要这样排序**？所有节点必须对"选哪些 UTXO"产生**完全相同的结果**，否则共识会分裂。确定性排序是共识安全的基础。

#### 税率计算：taxForValue

```go
func taxForValue(v int64, p REAPParams) int64 {
    return (v * p.TaxNum) / p.TaxDen  // 默认：(value * 30) / 100
}
```

### 6.5 文件：`reaptx.go` ★★★

#### IsLikelyREAPTx——REAP 交易识别

```go
func IsLikelyREAPTx(tx *wire.MsgTx) bool
```

判断标准：
1. `tx.Version == 3`（REAPTxVersion）
2. 至少有 1 个输出
3. 最后一个输出的 Value == 0
4. 最后一个输出是有效的 OP_RETURN
5. Payload 以 `"REAP:"` 开头

**注意**：这是**策略级识别**（heuristic），不是完整的共识验证。共识验证在 `validation_reap.go` 中完成。

#### BuildBlueprint——构建 REAP 交易

```go
func BuildBlueprint(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (*wire.MsgTx, error)
```

构建流程：

```
1. 创建交易：Version=3, LockTime=plan.Height-1

2. 处理输入：
   refundByScript = map[string]int64{}  // 按脚本聚合退款

   for each outpoint in plan.Inputs：
     entry = view.LookupEntry(outpoint)
     amount = entry.Amount()
     pkScript = entry.PkScript()

     tax = taxForValue(amount, p)
     refund = amount - tax
     (refund, tax) = applyDustRule(refund, tax, DustThresholdSat)

     tx.AddTxIn(TxIn{
         PreviousOutPoint: outpoint,
         Sequence: 0xFFFFFFFE,  // 非最终但接近
     })

     if refund > 0:
         refundByScript[pkScript] += refund

3. 生成退款输出（按 pkScript 字典序排序，确保确定性）：
   for each (pkScript, value) in sorted(refundByScript)：
     tx.AddTxOut(TxOut{Value: value, PkScript: pkScript})

4. 生成 Marker 输出：
   markerPkScript = OP_RETURN <"REAP:height:count:digest">
   tx.AddTxOut(TxOut{Value: 0, PkScript: markerPkScript})

5. 验证不变量：
   assert sum(inputs) == refundTotal + taxTotal
```

**为什么退款输出要按脚本排序**？确保相同的输入集合在任何节点上都生成完全相同的输出顺序——这是共识确定性的要求。

#### ExtractMarkerPayload——提取 Marker 内容

```go
func ExtractMarkerPayload(pkScript []byte) (string, bool)
```

从 OP_RETURN 脚本中提取 payload 字符串。

### 6.6 文件：`dust.go`

#### applyDustRule——Dust 折叠规则

```go
func applyDustRule(refund, tax, dustThresholdSat int64) (adjRefund, adjTax int64)
```

规则非常简单：
```
如果 dustThresholdSat <= 0（禁用）：返回原值
如果 0 < refund < dustThresholdSat：refund → 0, tax += refund
否则：返回原值
```

**为什么需要 Dust 规则**？
- 如果退款金额只有几百 satoshi，它在链上的存储成本可能超过其价值
- 这些"灰尘"输出会永久占用 UTXO 集，增加节点负担
- 把它们合并到税收（矿工奖励），既清理了状态，又避免了浪费

**778/779 突变边界（cliff）**：

```
value=778: tax=233, refund=545 (< 546) → 折叠！refund=0, tax=778
value=779: tax=233, refund=546 (= 546) → 不折叠，refund=546, tax=233
```

只差 1 satoshi，结果从"全额充公"变为"返还 546"。这是一个设计上已知的特性（feature），详见 `obtc_doc/reap_dust_behavior.md`。

### 6.7 文件：`marker.go`

```go
func MarkerDigest(inputs []wire.OutPoint) string
```

计算 REAP 交易输入的 SHA-256 摘要：
1. 创建 SHA-256 哈希器
2. 对每个输入写入 TxID（32 字节）+ Vout（4 字节小端序）
3. 返回十六进制编码的哈希值（64 字符字符串）

**用途**：嵌入 Marker 输出中，供验证节点校验"REAP 交易是否真的包含了声称的那些输入"。

### 6.8 文件：`weight.go`

```go
func EstimateBlueprintWeight(numInputs int) int64
```

保守估算 REAP 交易的权重：

```
weight = 40（基础）+ numInputs × 772（每输入+对应输出）+ 120（Marker）
```

各部分估算值：
- 基础权重（版本号+LockTime 等）：40 字节
- 每个输入的保守上界：600 字节
- 每个退款输出：172 字节
- Marker 输出：120 字节

**为什么偏保守（偏大）**？因为实际退款输出会按脚本聚合（多个输入可能退给同一脚本），实际权重通常小于估算值。偏大确保不会超出区块权重限制。

### 6.9 文件：`dryrun.go`

```go
func BuildDryRunSummary(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (DryRunSummary, error)
```

不真正构建交易，只快速计算统计摘要：

```go
type DryRunSummary struct {
    Picked      int     // 选中数
    TaxTotal    int64   // 总税收
    RefundTotal int64   // 总退款
    EstWeight   int64   // 估算权重
    MarkerHash  string  // Marker 摘要哈希
}
```

**使用场景**：矿工快速评估、RPC 返回估算值、测试验证。

---

## 第七章：共识验证层 blockchain/validation

### 7.1 文件：`validation_reap.go` ★★★

这是 REAP 的**共识安全守门员**，确保链上的每一笔 REAP 交易都是合法的。

#### 7.1.1 isLikelyReapTx——REAP 交易识别

```go
func isLikelyReapTx(tx *wire.MsgTx) bool
```

与 `mining/reap` 包中的 `IsLikelyREAPTx` 功能相同，但在 blockchain 包内定义以避免循环导入。

#### 7.1.2 checkReapMarker——验证 Marker 完整性

```go
func checkReapMarker(tx *wire.MsgTx, txHeight int32) error
```

验证步骤：
1. 解析 Marker payload：`"REAP:<height>:<count>:<digest>"`
2. 检查 `height == txHeight`（区块高度匹配）
3. 检查 `count == len(tx.TxIn)`（输入数量匹配）
4. 检查 `digest == reapInputDigest(tx)`（摘要匹配）

#### 7.1.3 checkReapConsensusHardening——规范化硬化

```go
func checkReapConsensusHardening(tx *wire.MsgTx, txHeight int32,
    utxoView *UtxoViewpoint, chainParams *chaincfg.Params) error
```

从 `ReapConsensusAtHeight` 起执行：

**规则 1：输入数量限制**
```
len(tx.TxIn) <= ReapMaxInputs
```

**规则 2：输入规范排序**

REAP 交易的输入必须按照以下优先级排序：
```
1. ExpiryKey（到期高度）—— 升序
2. Amount（金额）—— 升序
3. TxID Hash（交易哈希）—— 字典序
4. Vout Index（输出索引）—— 升序
```

通过 `compareReapInputOrderKey` 函数比较相邻输入，验证它们严格有序。

#### 7.1.4 checkExpirySpendRules——到期花费规则

```go
func checkExpirySpendRules(tx *wire.MsgTx, txHeight int32,
    utxoView *UtxoViewpoint, chainParams *chaincfg.Params) error
```

核心规则：

```
对于交易中的每个输入：
  expiryHeight = createHeight + WindowBlocks
  expired = (txHeight >= expiryHeight)

  如果是 REAP 交易 且 UTXO 未到期 → 错误！REAP 只能花费已到期的 UTXO
  如果不是 REAP 交易 且 UTXO 已到期 → 错误！普通交易不能花费已到期的 UTXO
```

**这条规则的重要性**：它实现了 OBTC 的核心安全保证——到期 UTXO 只能通过系统的 REAP 流程回收，而不能被任何人随意花费。

### 7.2 文件：`validate.go`（修改部分）

在 `CheckTransactionInputs` 函数中集成了 REAP 验证：

```go
// 在常规输入校验中的调用位置
if err := checkReapMarker(tx.MsgTx(), txHeight); err != nil {
    return 0, err
}
if err := checkExpirySpendRules(tx.MsgTx(), txHeight, utxoView, chainParams); err != nil {
    return 0, err
}
if err := checkReapConsensusHardening(tx.MsgTx(), txHeight, utxoView, chainParams); err != nil {
    return 0, err
}
```

### 7.3 文件：`scriptval.go`（修改部分）

REAP 交易使用专有验证路径，**跳过常规脚本验证**：

```go
// ValidateTransactionScripts() 中
if isLikelyReapTx(tx.MsgTx()) {
    return nil  // REAP 交易不走标准脚本引擎
}

// checkBlockScripts() 中
for _, tx := range block.Transactions() {
    if isLikelyReapTx(tx.MsgTx()) {
        continue  // 跳过 REAP 交易的脚本收集
    }
}
```

**为什么跳过**？REAP 交易是系统交易，没有用户签名，不能用标准脚本验证。它有自己的验证路径（`validation_reap.go`）。

---

## 第八章：重放保护 Replay Protection

### 8.1 问题背景

OBTC 是比特币的硬分叉。分叉后，同一个私钥同时控制 OBTC 和 BTC 上的资产。如果没有重放保护，用户在 OBTC 上签名的交易可以被"重放"到 BTC 网络上（反之亦然），导致资产在两条链上同时被转移。

### 8.2 文件：`txscript/sighash.go`（修改部分）

#### 域分离标签

```go
SigHashOBTCReplayProtection SigHashType = 0x40  // 重放保护标志位

var (
    obtcReplaySighashTagV0  = []byte("OBTC/SigHashV0/v1")   // Legacy 脚本
    obtcReplaySighashTagV1  = []byte("OBTC/SigHashV1/v1")   // SegWit v0
    obtcReplayTapSighashTag = []byte("OBTC/TapSighash/v1")  // Taproot
)
```

#### 实现原理

在计算签名哈希时，如果启用了重放保护，在哈希数据前加上域分离标签：

```
原始 sighash: hash = SHA256(serialized_data)
OBTC sighash: hash = SHA256("OBTC/SigHashV0/v1" || serialized_data)
```

由于标签不同，相同的交易在 OBTC 和 BTC 上会产生不同的 sighash，因此签名不可互换。

#### 新增辅助函数

```go
isOBTCReplayProtectedSigHashType(hashType SigHashType) bool
  // 检查 SigHashType 是否包含 0x40 标志位

stripOBTCReplayProtection(hashType SigHashType) SigHashType
  // 移除 0x40 标志位，得到底层的 sighash 类型

shouldUseOBTCReplayProtectionDomain(hashType SigHashType) bool
  // 是否应该使用 OBTC 域分离标签
```

### 8.3 文件：`txscript/engine.go`（修改部分）

新增脚本验证标志：

```go
ScriptVerifyOBTCReplayProtection  // 强制使用 OBTC 重放保护 sighash 验证
```

在 Taproot Keyspend 验证中应用：

```go
if vm.hasFlag(ScriptVerifyOBTCReplayProtection) {
    // 强制要求签名必须使用重放保护 sighash 类型
    if !isOBTCReplayProtectedSigHashType(sigHashType) {
        return error  // 拒绝不带重放保护的签名
    }
}
```

### 8.4 文件：`blockchain/validation_obtc_replay.go`

条件激活函数：

```go
func ApplyOBTCReplayProtectionScriptFlag(
    base txscript.ScriptFlags,
    params *chaincfg.Params,
    height int32,
) txscript.ScriptFlags {
    if chaincfg.IsOBTCReplayProtectionActive(params, height) {
        return base | txscript.ScriptVerifyOBTCReplayProtection
    }
    return base
}
```

在 `validate.go` 的 `CheckConnectBlock` 中被调用，确保只在激活高度之后才强制要求重放保护。

### 8.5 文件：`txscript/sighash_obtc_replay_test.go`

测试覆盖：
- 默认情况下拒绝重放保护 sighash（需要显式启用）
- 启用和禁用状态下 hash 值完全不同
- Legacy 和 Witness 使用各自独立的标签

---

## 第九章：挖矿模板集成 Mining Template

### 9.1 文件：`mining/template_reap.go` ★★

这是将 REAP 从"可以构造交易"升级到"实际出块"的关键文件。

#### 9.1.1 maybeBuildREAPTx

```go
func (g *BlkTmplGenerator) maybeBuildREAPTx(nextBlockHeight int32) (*btcutil.Tx, int64, error)
```

**早退条件**（任一满足则跳过 REAP）：
1. `reapIndex == nil`
2. 不是 OBTC 网络
3. 高度 < EnableAtHeight

**正常流程**：
1. 获取网络特定的 REAP 参数（SortModeStrict）
2. 验证参数
3. 调用 `collectExpiredOutpoints()` 获取过期候选
4. 如果无候选 → 返回 nil
5. 创建 dummy 交易获取 UTXO 视图
6. 调用 `reap.SelectCandidates()` 选择输入
7. 如果无选中 → 返回 nil
8. 调用 `reap.BuildBlueprint()` 构建交易
9. 返回 (交易, 税收总额, nil)

#### 9.1.2 collectExpiredOutpoints

```go
func (g *BlkTmplGenerator) collectExpiredOutpoints(
    nextBlockHeight int32, p reap.REAPParams) ([]wire.OutPoint, error)
```

从 ExpiryIndex 分批扫描到期 UTXO：
```
maxCollect = min(MaxInputs * 20, 50000)
扫描范围：[0, nextBlockHeight)
循环调用 ScanExpiringUTXOs()，每次最多 ScanBatch 条
累积结果直到达到 maxCollect 或无更多结果
```

#### 9.1.3 权重预留策略

```go
func (g *BlkTmplGenerator) normalTxWeightLimit(nextBlockHeight int32, reserveForREAP bool) uint32
```

为 REAP 交易预留权重空间：
```
如果 reserveForREAP = true：
  limit = BlockMaxWeight - REAPWeightBudget
否则：
  limit = BlockMaxWeight
```

这确保区块中有足够空间放入 REAP 交易，同时不会饿死正常交易。

### 9.2 文件：`mining/mining.go`（修改部分）

#### BlkTmplGenerator 新增字段

```go
type BlkTmplGenerator struct {
    // ...原有字段...
    reapIndex   *expiryindex.ExpiryIndex  // REAP 到期索引

    // 可选测试钩子
    reapSigOpCostFn      func(tx, view, segwit) (int, error)
    reapFetchInputViewFn func(tx) (*UtxoViewpoint, error)
}
```

#### SetREAPIndex 方法

```go
func (g *BlkTmplGenerator) SetREAPIndex(idx *expiryindex.ExpiryIndex)
```

在 `server.go` 的节点启动阶段调用，将 ExpiryIndex 注入模板生成器。

#### NewBlockTemplate 中的 REAP 集成

```
1. 规划 REAP 交易：
   plannedREAPTx, plannedREAPFee := maybeBuildREAPTx(nextBlockHeight)

2. 计算 normal 交易的权重限制（预留 REAP 空间）

3. 从 mempool 选择交易...

4. 尝试添加 REAP 交易：
   ├─ 检查权重限制（不超过 BlockMaxWeight）
   ├─ 检查 sigop 限制（不超过 MaxBlockSigOpsCost）
   ├─ 获取 REAP 输入的 UTXO 视图
   ├─ 调用 CheckTransactionInputs() 验证
   ├─ 更新 blockUtxos 反映花费
   ├─ 添加到区块交易列表
   └─ 税收加入 totalFees → 影响 coinbase 奖励
```

### 9.3 文件：`server.go`（修改部分）

负责启动初始化顺序：

```
节点启动
  → 创建 BlockChain
  → 创建 ExpiryIndex（通过 IndexManager）
  → ExpiryIndex.SetChainAccessor(chain)  // 注入链访问器
  → BlkTmplGenerator.SetREAPIndex(idx)   // 注入到模板生成器
```

**初始化顺序至关重要**：如果顺序错误，模板生成器可能拿到 nil 的索引，导致 REAP 功能静默失效。

---

## 第十章：RPC 扩展与内存池策略

### 10.1 文件：`btcjson/obtcextcmds.go`

新增两个 RPC 命令：

#### listexpiring 命令
```go
type ListExpiringCmd struct {
    StartHeight *int32  `json:"startheight"`   // 扫描起始高度
    EndHeight   *int32  `json:"endheight"`     // 扫描结束高度
    MaxResults  *int    `json:"maxresults"`    // 最大返回数量
    StartAfter  *string `json:"startafter"`    // 分页游标（"txid:vout"）
}
```

**用途**：查询即将到期或已到期的 UTXO 列表。

#### getexpiryindexstats 命令
```go
type GetExpiryIndexStatsCmd struct{}  // 无参数
```

**用途**：获取到期索引的统计信息（总 UTXO 数、总 ExpiryKey 数等）。

### 10.2 文件：`btcjson/obtcextresults.go`

RPC 返回结构体：

```go
type ExpiringUTXOResult struct {
    TxID          string  `json:"txid"`           // 交易 ID
    Vout          uint32  `json:"vout"`           // 输出索引
    ExpiryHeight  uint64  `json:"expiryheight"`   // 到期块高
    CreateHeight  uint64  `json:"createheight"`   // 创建块高
    BlocksToExpiry int64  `json:"blockstoexpiry"` // 剩余块数
}

type ListExpiringResult struct {
    ExpiringUTXOs []ExpiringUTXOResult  // 结果列表
    TotalResults  int                    // 总数
    NextHeight    *int32                 // 下一页起始高度
    NextOutpoint  *string                // 下一页起始 outpoint
}

type ExpiryIndexStatsResult struct {
    Disabled        bool                 // 是否禁用
    TipHeight       int32                // 已索引高度
    TotalUTXOs      int                  // 总 UTXO 数
    TotalExpiryKeys int                  // 总 ExpiryKey 数
    NetworkParams   *ExpiryParamsResult  // 网络参数
}
```

### 10.3 文件：`rpcserver.go`（修改部分）

注册了 `listexpiring` 和 `getexpiryindexstats` 的处理函数，将 RPC 请求映射到 ExpiryIndex 的扫描操作。

### 10.4 文件：`mempool/mempool.go`（修改部分）

在交易进入内存池前增加检查：

```go
// REAP 系统交易禁止进入内存池
if reap.IsLikelyREAPTx(tx.MsgTx()) {
    return nil, txRuleError(wire.RejectNonstandard,
        "reap system transaction is not accepted in mempool")
}
```

**为什么禁止**？REAP 交易是矿工在出块时自动生成的系统交易，不应该在 P2P 网络中传播。如果允许，可能导致：
- 不同矿工生成冲突的 REAP 交易
- 网络带宽浪费
- 潜在的 DoS 攻击向量

### 10.5 文件：`mempool/reap_policy_test.go`

测试内存池确实拒绝 REAP 交易：创建一个 Version=3 的 REAP 交易并验证被拒绝。

---

## 第十一章：基础设施与 DevNet

### 11.1 文件：`scripts/devnet-up.sh`

一键启动双节点 simnet 开发环境：

```bash
./scripts/devnet-up.sh start    # 启动 2 节点 DevNet
./scripts/devnet-up.sh demo     # 运行演示交易
./scripts/devnet-up.sh status   # 查看网络状态
./scripts/devnet-up.sh stop     # 停止 DevNet
```

**节点配置**：
- Node 1（矿工）：RPC 18556, P2P 18555
- Node 2（对等）：RPC 18557, P2P 18558
- 网络：simnet（本地模拟网络）

### 11.2 文件：`scripts/ci-validate.sh`

CI 验证脚本，运行 lint + 单元测试 + 构建检查。

### 11.3 文件：`scripts/validation/`

验证工具目录：
- `utxo_expiry_validator.go`：独立的 UTXO 到期验证器
- `quick_validate.sh`：快速验证脚本
- `demo.sh`：演示脚本
- `config_examples.conf`：配置示例

### 11.4 文件：`.githooks/`

Git 钩子：
- `pre-commit`：提交前检查（lint、fmt）
- `pre-push`：推送前运行完整测试

---

## 第十二章：测试体系全景

### 12.1 ExpiryIndex 测试矩阵

| 测试文件 | 覆盖内容 |
|---------|---------|
| `expiryindex_test.go` | ConnectBlock/DisconnectBlock、基本 CRUD |
| `buckets_test.go` | 桶操作、元数据读写 |
| `encode_test.go` | 编码/解码正确性 |
| `encode_extra_test.go` | 编码边界条件 |
| `database_test.go` | 数据库集成 |
| `rebuild_test.go` | 重建策略（smart/fast/incremental） |
| `scan_staircase_test.go` | **Staircase 分页**——多 ExpiryKey 跨越 |
| `scan_extra_test.go` | 扫描边界条件 |
| `sequence_fuzz_test.go` | 随机操作序列的一致性 |
| `utxo_test.go` | UTXO 操作 |
| `helpers_extra_test.go` | 辅助函数 |
| `params_extra_test.go` | 参数计算 |
| `benchmark_test.go` | 性能基准 |

### 12.2 REAP 测试矩阵

| 测试文件 | 覆盖内容 |
|---------|---------|
| `selector_test.go` | 候选选择算法、排序、截断 |
| `packer_test.go` | Blueprint 构造、refund 聚合 |
| `reaptx_test.go` | REAP 交易识别、Marker 解析 |
| `dust_test.go` | Dust 折叠基本规则 |
| `dust_extreme_test.go` | **778/779 cliff**、零税率+Dust |
| `marker_vector_test.go` | Marker 摘要向量测试 |
| `params_test.go` | 参数验证 |
| `dryrun_test.go` | DryRun 摘要生成 |
| `staircase_pressure_test.go` | 阶梯式扫描压力测试 |
| `stress_regression_test.go` | 高负载/并发回归测试 |
| `bench_test.go` | 性能基准 |

### 12.3 集成与边界测试

| 测试文件 | 覆盖内容 |
|---------|---------|
| `mining/template_reap_test.go` | 模板生成中的 REAP 注入 |
| `mining/newblocktemplate_p0_coverage_test.go` | P0 边界场景 |
| `mining/newblocktemplate_p1_test.go` | P1 费率切换矩阵 |
| `mining/newblocktemplate_p2_test.go` | P2 会计和辅助 |
| `mining/newblocktemplate_reap_boundary_test.go` | REAP 边界 E2E |
| `blockchain/validation_reap_test.go` | REAP 共识验证 |
| `blockchain/validation_obtc_replay_test.go` | 重放保护验证 |
| `txscript/sighash_obtc_replay_test.go` | Sighash 域分离 |
| `mempool/reap_policy_test.go` | 内存池拒绝 REAP |
| `chaincfg/params_obtc_test.go` | 网络参数 |

### 12.4 运行测试

```bash
# 运行全部单元测试
make unit

# 运行带竞态检测的测试
make unit-race

# 运行特定包的测试
go test ./blockchain/expiryindex/... -v -count=1
go test ./mining/reap/... -v -count=1

# 运行集成测试（需要 rpctest tag）
go test -tags=rpctest ./integration/... -count=1 -v
```

---

## 第十三章：推荐阅读路径

### 13.1 入门路线（建议 3-5 天）

```
Day 1：全局概念
├── 阅读本文档第一章~第三章
├── 阅读 chaincfg/params_obtc.go（重点看 ExpiryParams 和三个网络定义）
└── 理解 ExpiryKey 的计算公式

Day 2：到期索引
├── 阅读 blockchain/expiryindex/doc.go（设计原则）
├── 阅读 blockchain/expiryindex/expiryindex.go（核心 500 行）
│   ├── 重点：ConnectBlock / DisconnectBlock 的对称性
│   ├── 重点：ScanExpiringUTXOs 的分页逻辑
│   └── 重点：smartRebuild 的策略选择
├── 阅读 blockchain/expiryindex/encode.go（大端序设计）
└── 运行 go test ./blockchain/expiryindex/... -v -run TestScanExpiringUTXOsStaircasePressure

Day 3：REAP 选择与构造
├── 阅读 mining/reap/types.go（数据类型先行）
├── 阅读 mining/reap/selector.go（核心选择算法）
│   ├── 重点：sortCandidates 的四级排序
│   └── 重点：贪心选择的截断条件
├── 阅读 mining/reap/reaptx.go（Blueprint 构造）
│   ├── 重点：refund 按脚本聚合
│   └── 重点：Marker 输出生成
├── 阅读 mining/reap/dust.go（只有 10 行核心逻辑）
└── 阅读 mining/reap/marker.go（SHA-256 摘要）

Day 4：共识验证 + 模板集成
├── 阅读 blockchain/validation_reap.go
│   ├── 重点：checkExpirySpendRules（到期花费二元规则）
│   ├── 重点：checkReapConsensusHardening（输入排序规范）
│   └── 重点：checkReapMarker（Marker 完整性）
├── 阅读 mining/template_reap.go
│   ├── 重点：maybeBuildREAPTx 的早退条件
│   └── 重点：权重预留策略
└── 快速浏览 mining/mining.go 中 REAP 相关部分

Day 5：重放保护 + 测试验证
├── 阅读 txscript/sighash.go 中的 OBTC 修改
├── 阅读 blockchain/validation_obtc_replay.go
├── 运行完整测试：make unit
└── 尝试启动 DevNet：./scripts/devnet-up.sh start
```

### 13.2 速查：按功能定位代码

| 我想了解... | 看哪个文件 | 看哪个函数/结构体 |
|------------|-----------|-------------------|
| OBTC 和 BTC 的区别 | `chaincfg/params_obtc.go` | `ObtcMainNetParams` |
| UTXO 什么时候到期 | `blockchain/expiryindex/params.go` | `CalculateExpiryKey()` |
| 到期索引怎么工作 | `blockchain/expiryindex/expiryindex.go` | `ConnectBlock()`, `ScanExpiringUTXOs()` |
| 矿工怎么选择 REAP 输入 | `mining/reap/selector.go` | `SelectCandidates()` |
| REAP 交易长什么样 | `mining/reap/reaptx.go` | `BuildBlueprint()` |
| 税率怎么算 | `mining/reap/selector.go` | `taxForValue()` |
| Dust 折叠规则 | `mining/reap/dust.go` | `applyDustRule()` |
| Marker 怎么验证 | `blockchain/validation_reap.go` | `checkReapMarker()` |
| 到期 UTXO 能被谁花 | `blockchain/validation_reap.go` | `checkExpirySpendRules()` |
| 重放保护怎么实现 | `txscript/sighash.go` | `obtcReplaySighashTagV0` 等 |
| REAP 交易怎么进区块 | `mining/template_reap.go` | `maybeBuildREAPTx()` |
| RPC 怎么查到期信息 | `btcjson/obtcextcmds.go` | `ListExpiringCmd` |
| 内存池为什么拒绝 REAP | `mempool/mempool.go` | `IsLikelyREAPTx()` 检查 |

---

## 附录 A：完整新增文件清单

按模块分组，标注重要程度（★=必读，☆=了解即可）：

### 核心源文件

```
★★★ chaincfg/params_obtc.go               # OBTC 网络参数定义
★★★ blockchain/expiryindex/expiryindex.go  # 到期索引核心
★★  blockchain/expiryindex/encode.go       # 确定性编码
★★  blockchain/expiryindex/buckets.go      # 数据库桶管理
★   blockchain/expiryindex/params.go       # 过期参数适配
★   blockchain/expiryindex/doc.go          # 包文档
☆   blockchain/expiryindex/log.go          # 日志配置
★★  blockchain/expiry_chain_accessor.go    # 链访问适配器
★   blockchain/utxo_iter.go               # UTXO 迭代器
★★★ blockchain/validation_reap.go          # REAP 共识验证
★   blockchain/validation_obtc_replay.go   # 重放保护激活
★★★ mining/reap/selector.go               # 候选选择算法
★★★ mining/reap/reaptx.go                 # REAP 交易构建
★★  mining/reap/packer.go                 # Blueprint 构造
★   mining/reap/types.go                  # 核心数据类型
★   mining/reap/params.go                 # REAP 参数
★   mining/reap/dust.go                   # Dust 折叠规则
★   mining/reap/marker.go                 # Marker 摘要
★   mining/reap/weight.go                 # 权重估算
★   mining/reap/dryrun.go                 # DryRun 摘要
★★  mining/template_reap.go               # REAP 模板集成
★   btcjson/obtcextcmds.go                # RPC 命令定义
★   btcjson/obtcextresults.go             # RPC 结果定义
☆   txscript/sighash_obtc_replay_test.go  # 重放保护测试
```

### 修改的上游文件

```
★   mining/mining.go          # NewBlockTemplate REAP 注入
★   blockchain/validate.go    # REAP 验证挂接点
★   blockchain/scriptval.go   # REAP 脚本验证分流
★   mempool/mempool.go        # REAP 交易拒绝策略
★   txscript/sighash.go       # 重放保护域分离
★   txscript/engine.go        # 重放保护标志
☆   wire/protocol.go          # OBTC 网络常量
☆   config.go                 # OBTC 网络命令行参数
☆   params.go                 # 参数注入映射
☆   server.go                 # SetREAPIndex 初始化
☆   rpcserver.go              # RPC handler
☆   rpcserverhelp.go          # RPC 帮助文本
```

### 基础设施

```
☆   scripts/devnet-up.sh              # DevNet 启动脚本
☆   scripts/ci-validate.sh            # CI 验证脚本
☆   scripts/validation/               # 验证工具目录
☆   .githooks/pre-commit              # 提交前钩子
☆   .githooks/pre-push                # 推送前钩子
```

---

## 附录 B：术语表

| 术语 | 英文全称 | 含义 |
|------|---------|------|
| OBTC | Organic Bitcoin | 本项目链实现名称 |
| REAP | Resource Expiration and Allocation Protocol | 到期资产回收协议 |
| UTXO | Unspent Transaction Output | 未花费交易输出——比特币的"余额"单元 |
| ExpiryKey | Expiry Key | 到期键——UTXO 到期的区块高度 |
| ExpiryIndex | Expiry Index | 到期索引——跟踪哪些 UTXO 何时到期 |
| WindowBlocks | Window Blocks | 到期窗口——从创建到到期的区块数 |
| Refund | Refund | 退款——REAP 回收中退还原持有者的部分（70%） |
| Tax | Tax | 税收——REAP 回收中分配给矿工的部分（30%） |
| Dust | Dust | 灰尘——金额过小的输出（< 546 satoshi） |
| Marker | Marker | 标记输出——REAP 交易中的 OP_RETURN 审计标记 |
| Blueprint | Blueprint | 蓝图——REAP 交易的完整构造结果 |
| DryRun | Dry Run | 干运行——不构造交易只计算统计 |
| Staircase | Staircase | 阶梯式——ExpiryIndex 的分页扫描模式 |
| Fork Height | Fork Height | 分叉高度——OBTC 从 BTC 分叉的区块高度 |
| Reorg | Chain Reorganization | 链重组——主链切换到更长的分叉链 |
| Sighash | Signature Hash | 签名哈希——交易签名时的哈希值 |
| OP_RETURN | OP_RETURN | 比特币脚本操作码，创建可证明不可花费的输出 |
| Coinbase | Coinbase | 区块中的第一笔交易，包含矿工奖励 |
| PkScript | Public Key Script | 锁定脚本——定义花费条件的脚本 |
| Bech32 | Bech32 | 比特币地址编码格式（如 bc1... 或 obtc1...） |
| HRP | Human-Readable Part | 人类可读前缀——Bech32 地址的网络标识部分 |
| Magic Number | Magic Number | 魔数——P2P 网络消息头的 4 字节网络标识 |
| bbolt | bbolt | Go 语言的嵌入式 KV 数据库（btcd 使用） |
| VLQ | Variable-Length Quantity | 变长整数编码 |
| vBytes | Virtual Bytes | 虚拟字节——比特币的交易权重单位 |

---

## 附录 C：常见问题

### Q1：OBTC 和 btcd 的代码怎么区分？

看文件名：
- `*_obtc_*` 或 `*_reap*` 后缀的文件是 OBTC 新增的
- `chaincfg/params_obtc.go`、`blockchain/expiryindex/`、`mining/reap/` 整个目录都是新的
- 修改的上游文件可以通过 `git log --since="2025-10-01" -- <file>` 查看变更历史

### Q2：为什么 REAP 交易的版本是 3？

比特币交易版本 1 和 2 已被使用（版本 2 启用 BIP 68 相对锁定时间）。OBTC 选择版本 3 来标识系统交易，便于快速识别。

### Q3：为什么税率是 30%？

这是协议设计决策，详见白皮书。30% 在激励矿工处理到期 UTXO 和保护持有者权益之间取得平衡。

### Q4：如果到期 UTXO 的持有者在到期前花费了它会怎样？

完全正常。`checkExpirySpendRules` 检查的是"在 EnableAtHeight 之后的区块中"：
- 如果 UTXO 还没到期（`txHeight < expiryHeight`），普通交易可以正常花费它
- 一旦被花费，ExpiryIndex 会在 `ConnectBlock` 中把它从索引中移除

### Q5：链重组时 REAP 交易怎么办？

和普通交易一样：
- `DisconnectBlock` 会恢复被 REAP 花费的 UTXO 到索引中
- 新的最长链可能包含不同的 REAP 交易（因为候选集可能不同）
- ExpiryIndex 的双向映射保证了重组后的一致性

### Q6：怎么在本地测试 REAP 功能？

```bash
# 1. 构建
make build

# 2. 启动 DevNet
./scripts/devnet-up.sh start

# 3. 运行 REAP 相关测试
go test ./mining/reap/... -v -count=1
go test ./blockchain/expiryindex/... -v -count=1
go test ./blockchain/ -v -run TestReap -count=1
```

### Q7：我想修改税率或到期窗口怎么办？

修改 `chaincfg/params_obtc.go` 中对应网络的 `ExpiryParams`。注意：
- 这些是**共识参数**，修改后需要所有节点同步更新
- 测试网和回归测试网的参数可以随意调整用于测试
- 主网参数的修改需要硬分叉

---

> 文档版本：v1.0
> 生成日期：2026-03-06
> 覆盖代码范围：2025-10-07 ~ 2026-03-03（136 次提交）
