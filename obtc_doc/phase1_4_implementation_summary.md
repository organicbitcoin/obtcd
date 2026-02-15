# OBTC 项目实现总结（面向新读者）

> 本文档是独立的项目总结与阅读导航。  
> 目标：让第一次接触 OBTC 的读者，快速理解系统目标、核心机制、代码结构与当前进展。

---

## 1. 项目一句话说明

OBTC 是基于 btcd 的链实现，核心机制是：
- 为 UTXO 引入到期语义；
- 对到期 UTXO 通过系统交易（REAP）进行回收；
- 回收金额分为返还（refund）与税（tax），税进入矿工收益；
- 整个流程强调可索引、可验证、可审计与可复现。

---

## 2. REAP 是什么（先读这个）

REAP（**Reclaim Expired Assets Protocol**）可以理解为 OBTC 的“到期资产清算机制”。当某些 UTXO 满足到期条件后，系统会构造一笔 REAP 交易来处理它们。

它的核心目标有三个：
- **状态清理**：把长期沉睡、已到期的 UTXO 从可花费集合中有序处理掉；
- **价值分配**：按协议规则把金额拆成 refund（返还）和 tax（税）；
- **可审计**：通过 marker（标记输出）让节点和外部工具可以复核“这笔 REAP 处理了哪些输入、金额是否一致”。

一个简化流程是：
1. ExpiryIndex 提供到期候选集合；
2. Selector 按确定性规则选择本块要处理的输入；
3. Packer 构造 REAP 蓝图交易（refund 输出 + marker 输出）；
4. （进行中）在共识与挖矿模板层完成链上闭环验证与出块接线。

换句话说：
- **ExpiryIndex** 负责“找谁到期”；
- **REAP** 负责“怎么处理到期”；
- **共识/模板接线** 负责“把处理规则变成链上强约束”。

---

## 3. 系统分层（从下到上）

### A. 基础网络与参数层（已完成）
负责网络参数、节点启动与基础运行能力。

### B. 到期索引层 ExpiryIndex（已完成）
负责跟踪“哪些 UTXO 已到期/即将到期”，提供扫描与查询能力。

### C. REAP 选择与交易蓝图层（已完成）
负责从索引中确定性选择候选，并构造 REAP 系统交易蓝图。

### D. 共识验证与模板集成层（进行中）
负责将 REAP 真正接入区块验证与挖矿模板，形成链上闭环。

---

## 4. 目前已经实现了什么

### A. 基础网络与参数层
- OBTC 独立网络参数与识别逻辑已具备；
- 本地双节点开发脚本可用；
- 可完成基础启动、互联、出块与转账验证。

关键入口：
- `chaincfg/params_obtc.go`
- `scripts/devnet-up.sh`

### B. 到期索引层 ExpiryIndex
- 已实现索引核心：连接区块、回滚区块、范围扫描、分页读取；
- 已实现索引参数与编码层；
- 支持重组场景下的一致性处理。

关键入口：
- `blockchain/expiryindex/expiryindex.go`
- `blockchain/expiryindex/buckets.go`
- `blockchain/expiryindex/encode.go`
- `blockchain/expiryindex/params.go`

### C. REAP 选择与交易蓝图层
- 已实现确定性选择器与交易蓝图构造；
- 已实现权重估算、参数校验、marker 规则与 dry-run 摘要；
- 已实现策略级 REAP 交易识别工具。

关键入口：
- `mining/reap/selector.go`
- `mining/reap/packer.go`
- `mining/reap/weight.go`
- `mining/reap/params.go`
- `mining/reap/types.go`
- `mining/reap/marker.go`
- `mining/reap/dryrun.go`
- `mining/reap/reaptx.go`

---

## 5. 当前进行中的关键工作

### D. 共识验证与模板集成层
目标是把“可构造 REAP 蓝图”升级为“链上可验证、可出块”的完整闭环。

当前状态：
- 已有基础：
  - `mining/reap/reaptx.go`
  - `mempool/policy.go`（已有相关策略改动）
- 仍待补齐：
  - `blockchain/validation_reap.go`
  - `mining/template_reap.go`
  - 对应集成验证文档与案例

结论：
- 底层能力（A/B/C）已具备；
- 系统闭环（D）是当前主线任务。

---

## 6. 新读者建议阅读路径

1. 先读本文件第 1、2 节，建立全局概念。  
2. 阅读 `blockchain/expiryindex/expiryindex.go`，理解到期索引。  
3. 阅读 `mining/reap/selector.go` + `mining/reap/packer.go`，理解 REAP 从“选择”到“成型”。  
4. 阅读 `mining/reap/marker.go` + `mining/reap/dryrun.go`，理解可审计输出。  
5. 最后看第 4 节，理解系统闭环还差哪些接线。

---

## 7. 代码索引（按职责，含关键逻辑说明）

> 这部分做两件事：
> 1) 告诉你“应该看哪个文件”；
> 2) 告诉你“看这个文件时重点盯哪段逻辑”。

### 网络与参数
- `chaincfg/params_obtc.go`  
  核心是 `ObtcMainNetParams / ObtcTestNetParams / ObtcRegTestParams` 与 `ExpiryParams`。它定义网络魔数、端口、地址前缀、分叉高度与到期窗口，是运行时行为的“全局配置根”。  
  **重点看**：
  - `IsOBTC`：决定哪些路径走 OBTC 专属逻辑；
  - `GetExpiryParams`：决定是否启用到期索引与到期规则；
  - `CalculateExpiryKey`：到期高度计算入口（高度制）。

- `config.go`  
  核心是命令行参数到运行时配置对象的映射。它决定节点到底跑在哪个网络、启用哪些服务。  
  **重点看**：网络选择参数与默认值，确认 obtcmainnet/obtctestnet/obtcregtest 的落地路径。

- `params.go`  
  核心是把配置层选择映射到 chain params 实例，并把同一份参数注入给区块链验证、索引、RPC。  
  **重点看**：参数注入顺序，避免“验证层和索引层读到不同网络参数”的隐性配置错误。

- `cmd/btcctl/config.go`  
  核心是 CLI 侧网络与 RPC 参数解析，让工具请求与节点网络配置一致。  
  **重点看**：端口和网络前缀是否匹配，避免调试时“命令发到错误网络”。

### 到期索引（ExpiryIndex）
- `blockchain/expiryindex/expiryindex.go`  
  核心函数：`NewExpiryIndex`、`Init`、`ConnectBlock`、`DisconnectBlock`、`ScanExpiringUTXOs`。  
  核心机制：维护双向映射并支持分页扫描。
  - 正向映射：`OutPoint -> ExpiryKey`（便于花费时快速删除）；
  - 反向映射：`ExpiryKey -> []OutPoint`（便于按到期顺序扫描）。  
  **重点看**：
  - `ConnectBlock/DisconnectBlock` 的对称性（重组一致性关键）；
  - `ScanExpiringUTXOs` 的分页游标逻辑（`fromKey/toKey/startAfter`）；
  - `smartRebuild` 的重建策略选择（初次构建、落后追赶）。

- `blockchain/expiryindex/buckets.go`  
  核心是 bucket 元数据与版本管理：`bktOutpoint2Expiry`、`bktExpiry2Outpoints`、`bktExpiryMeta`。  
  **重点看**：
  - `dbPutTipHeightIndexed / dbGetTipHeightIndexed`（索引进度）；
  - 版本字段与初始化路径（升级兼容点）。

- `blockchain/expiryindex/encode.go`  
  核心是确定性编码：`encodeOutPoint`、`encodeExpiryKey`、`encodeOutPointList`。  
  **重点看**：
  - `ExpiryKey` 使用大端编码带来自然有序扫描；
  - outpoint 列表编码/删除逻辑确保跨节点一致。

- `blockchain/expiryindex/params.go`  
  核心是把链参数转换成索引参数，并提供启用/校验辅助函数。  
  **重点看**：`IsExpiryEnabled`、`IsIndexingEnabled`、`ValidateListParams`。

### 查询接口（RPC）
- `rpcserver.go`  
  核心是 `listexpiring` 等 handler，把外部请求映射到索引扫描参数并返回分页结果。  
  **重点看**：参数约束、分页 continuation 的返回字段。

- `btcjson/obtcextcmds.go` / `btcjson/obtcextresults.go`  
  核心是命令与结果结构定义（协议契约层）。  
  **重点看**：字段命名稳定性与向后兼容性。

- `rpcserverhelp.go`  
  核心是对外帮助文本。  
  **重点看**：参数含义与示例是否和真实行为一致。

### REAP 核心（选择 + 构造）
- `mining/reap/selector.go`  
  核心函数：`SelectCandidates`、`SelectCandidatesWithScanner`。  
  核心算法：扫描候选 -> 过滤不可用 UTXO -> 稳定排序 -> 按 `MaxInputs/WeightBudget` 截断。  
  **重点看**：
  - `sortCandidates` 的排序键（expiry/amount/hash/index）；
  - `taxForValue` 的整数税额计算与不变量；
  - 分页扫描 + `startAfter` 续扫正确性。

- `mining/reap/packer.go`  
  核心函数：`BuildBlueprint`。  
  核心逻辑：
  - 把选中输入组装成系统交易输入；
  - 按 `PkScript` 聚合 refund 输出；
  - 追加 marker 输出；
  - 校验 `sum(inputs) = refund + tax`。  
  **重点看**：`markerScript` 负责编码可审计 payload。

- `mining/reap/marker.go`  
  核心函数：`MarkerDigest`。  
  核心作用：把输入 outpoint 序列化后做哈希，用于 marker 校验与可复核性。

- `mining/reap/dryrun.go`  
  核心函数：`BuildDryRunSummary`。  
  核心作用：输出 `picked/tax/refund/estWeight/markerHash`，用于上线前比对和调试。

- `mining/reap/params.go` / `mining/reap/types.go` / `mining/reap/weight.go` / `mining/reap/reaptx.go`  
  分别负责参数默认值与校验、数据结构契约、估重、策略级 REAP 识别。  
  **重点看**：
  - 参数跨网络默认值；
  - 估重是否偏保守（避免模板超重）；
  - `IsLikelyREAPTx` 仅为识别，不等于完整共识验证。

### 共识验证接线（已实现）
- `blockchain/validation_reap.go`  
  已实现 REAP 专用验证组件：
  - `isLikelyReapTx`：识别 REAP 交易形态；
  - `checkReapMarker`：校验 marker 的高度、输入个数、digest 一致性；
  - `checkExpirySpendRules`：约束“普通交易不得花费过期 UTXO，REAP 不得花费未过期 UTXO”。  
  **重点看**：错误分支是否返回一致规则错误类型。

- `blockchain/validate.go`  
  `CheckTransactionInputs` 已挂入 REAP 校验流程。  
  **重点看**：REAP 检查在常规输入校验中的调用顺序和失败短路行为。

- `blockchain/scriptval.go`  
  对 REAP 交易路径做脚本校验分流（避免按普通签名脚本路径处理）。  
  **重点看**：分流条件是否与 `isLikelyReapTx` 保持一致。

### 挖矿模板接线（已实现）
- `mining/template_reap.go`  
  核心函数：`maybeBuildREAPTx`、`collectExpiredOutpoints`。  
  核心逻辑：
  - 在满足网络/高度条件时扫描过期候选；
  - 拉取 UTXO 视图并构造 REAP 蓝图；
  - 返回系统交易与税额供模板计费。  
  **重点看**：早退条件（非 OBTC、未到启用高度、空候选）与分页扫描上限。

- `mining/mining.go`  
  已在 `NewBlockTemplate` 尝试注入 REAP 交易，并把税额计入总 fee，影响 coinbase。  
  **重点看**：
  - 注入时机与重量/sigop 约束检查；
  - `SetREAPIndex` wiring 是否在 server 启动阶段完成。

- `server.go`  
  负责把 expiry index 实例注入模板生成器（`SetREAPIndex`）。  
  **重点看**：节点启动初始化顺序，避免模板路径拿到 nil index。

### 还可继续补强的点（建议）
- `mining/template_reap.go` 的深路径集成测试（含更多候选、截断、异常注入）；
- REAP 交易构造与校验在高负载/并发下的压力回归；
- 端到端文档样例补充更多“失败案例 -> 预期错误”。

---

## 8. 完成度总览

| 模块 | 状态 | 说明 |
|---|---|---|
| 基础网络与参数 | ✅ 完成 | 可运行、可验证 |
| 到期索引 | ✅ 完成 | 可索引、可扫描（含重组与重建路径测试） |
| REAP 选择与蓝图 | ✅ 完成 | 可选择、可构造、可审计（含排序/税额/marker 直接单测） |
| 共识验证 | ✅ 已接线 | `CheckTransactionInputs` 已接入 REAP marker 与过期花费规则校验 |
| 挖矿模板集成 | ✅ 已接线（持续增强中） | `NewBlockTemplate` 已尝试注入 REAP 系统交易并计入税费 |
| 测试完备度（Phase1-4） | ⚠️ 持续提升中 | 主要函数已补直接单测；模板集成深路径与高强度场景仍在扩展 |

---

## 9. 术语与缩写对照（全部展开）

- **OBTC (Organic Bitcoin)**：本项目链实现名称。  
- **BTC (Bitcoin)**：比特币主链。  
- **UTXO (Unspent Transaction Output)**：未花费交易输出。  
- **REAP (Reclaim Expired Assets Protocol)**：到期资产回收协议/流程；在实现上体现为处理到期 UTXO 的系统交易机制。  
- **Expiry**：到期状态，表示 UTXO 超过规则窗口后不可按普通交易继续使用。  
- **ExpiryIndex**：维护到期状态的索引系统。  
- **Refund**：返还给原锁定脚本的金额部分。  
- **Tax**：回收过程中的协议税额，进入矿工收益。  
- **Marker**：REAP 交易中的标记输出，用于识别与审计。  
- **RPC (Remote Procedure Call)**：远程过程调用接口（节点对外的命令调用方式）。  
- **CLI (Command-Line Interface)**：命令行交互方式。  
- **Tx (Transaction)**：交易。  
- **TxID (Transaction ID)**：交易哈希标识。  
- **PkScript (Public Key Script)**：交易输出锁定脚本。  
- **Reorg (Chain Reorganization)**：链重组，要求索引与规则保持一致。
