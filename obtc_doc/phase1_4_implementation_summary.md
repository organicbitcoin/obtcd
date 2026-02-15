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

### 网络与参数
- `chaincfg/params_obtc.go`  
  核心是 `ObtcMainNetParams / ObtcTestNetParams / ObtcRegTestParams` 与 `ExpiryParams`。它定义网络魔数、端口、地址前缀、分叉高度与到期窗口，是所有运行时行为的“总开关配置源”。

- `config.go`  
  核心是节点启动参数解析与网络模式选择（例如 obtcmainnet/obtctestnet/obtcregtest），把命令行配置映射到运行时配置对象。

- `params.go`  
  核心是把配置层选择映射到 chain params 实例，并在启动路径中统一注入，保证共识层、索引层、RPC 层读取到一致的网络参数。

- `cmd/btcctl/config.go`  
  核心是 CLI 侧网络与 RPC 参数解析，让工具请求与节点网络配置保持一致，避免“连错网/端口不匹配”。

### 到期索引
- `blockchain/expiryindex/expiryindex.go`  
  核心函数是 `NewExpiryIndex`、`Init`、`ConnectBlock`、`DisconnectBlock`、`ScanExpiringUTXOs`。核心算法是：在区块连接/回滚时维护双向索引，并按到期键范围做分页扫描。

- `blockchain/expiryindex/buckets.go`  
  核心是三桶结构：`bktOutpoint2Expiry`（正向映射）、`bktExpiry2Outpoints`（反向映射）、`bktExpiryMeta`（版本/进度）。关键逻辑是元数据版本化与 tip height 持久化，保证可恢复与可升级。

- `blockchain/expiryindex/encode.go`  
  核心算法是确定性编码：`encodeOutPoint`（36字节）、`encodeExpiryKey`（8字节大端，天然按时间序排序）、`encodeOutPointList`（稳定排序后编码），确保跨节点一致性。

- `blockchain/expiryindex/params.go`  
  核心是把链参数转换成索引运行参数（窗口、批大小、启用高度），并统一提供“索引是否启用”的判断逻辑。

### 查询接口
- `rpcserver.go`  
  核心是 RPC handler（如 `listexpiring`），把外部请求转换成索引扫描条件，并返回分页结果。

- `btcjson/obtcextcmds.go`  
  核心是扩展命令定义（请求结构），决定 RPC 参数格式与兼容行为。

- `btcjson/obtcextresults.go`  
  核心是返回结构定义（响应 schema），约束客户端可依赖的数据字段。

- `rpcserverhelp.go`  
  核心是 RPC 文档与 help 文本，确保接口可发现、可调试、可自解释。

### REAP 核心
- `mining/reap/selector.go`  
  核心函数是 `SelectCandidates`。核心算法：扫描到期集合 → 过滤已花费 UTXO → 按稳定排序（expiry/amount/txid/vout）→ 在 `MaxInputs` 与 `WeightBudget` 下截断，计算 tax/refund。

- `mining/reap/packer.go`  
  核心函数是 `BuildBlueprint`。核心逻辑：把计划输入组装成系统交易，按 `pkScript` 聚合 refund 输出，追加 marker 输出，并校验 `input = refund + tax` 不变量。

- `mining/reap/weight.go`  
  核心是蓝图交易权重估算函数，采用保守估计给选择器做预算截断，避免构造超重交易。

- `mining/reap/params.go`  
  核心是 `DefaultREAPParams`、`DefaultREAPParamsForNet`、`Validate`，把算法参数（税率、批量、上限）标准化并做合法性校验。

- `mining/reap/types.go`  
  核心是 `REAPPlan`、统计字段与错误类型定义，统一模块内外的数据契约。

- `mining/reap/marker.go`  
  核心函数是 `MarkerDigest`。核心算法：按固定字节序序列化 input outpoints 并计算 sha256，确保 marker 可重复验证。

- `mining/reap/dryrun.go`  
  核心函数是 `BuildDryRunSummary`，输出 `picked/tax/refund/estWeight/markerHash`，用于上线前或调试时的可审计预览。

- `mining/reap/reaptx.go`  
  核心函数是 `IsLikelyREAPTx`、`ExtractMarkerPayload`，通过版本号 + OP_RETURN marker 形态做策略级识别（非完整共识校验）。

### 闭环接线（待补齐）
- `blockchain/validation_reap.go`  
  预期核心是 REAP 专用区块验证：输入集合一致性、税额与输出约束、普通交易禁止花费过期 UTXO。

- `mining/template_reap.go`  
  预期核心是挖矿模板接线：自动生成并注入 REAP 交易，税额并入模板 fee/coinbase 统计。

---

## 8. 完成度总览

| 模块 | 状态 | 说明 |
|---|---|---|
| 基础网络与参数 | ✅ 完成 | 可运行、可验证 |
| 到期索引 | ✅ 完成 | 可索引、可扫描 |
| REAP 选择与蓝图 | ✅ 完成 | 可选择、可构造、可审计 |
| 共识验证与模板集成 | ⚠️ 进行中 | 闭环接线待完成 |

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
