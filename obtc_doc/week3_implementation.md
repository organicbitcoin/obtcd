# Week 3 计划（REAP 选择器 & 系统交易构造）— Go + btcd + Cursor

**时间预算：18–20 小时** ｜ **目标**：在不改共识验证与挖矿模板的前提下，完成 **确定性 REAP 选择算法** 与 **系统交易（Blueprint）构造**，并通过属性测试/基准测试，为第 4 周“验证规则 & 挖矿集成”做好准备。

---

## 🎯 本周目标（Definition of Done）

* 在 `mining/reap/` 实现 **REAP 选择器**（按到期索引输出**确定性**候选集合；支持每块上限与估算重量/容量）。
* 产出**系统交易蓝图**（`wire.MsgTx`），包含：

  * 多个 **“到期 UTXO” 输入**（暂不校验脚本，本周仅构造）。
  * **Refund 输出**（将 70% 返还至原始 `scriptPubKey`；相同脚本可做确定性聚合）。
  * **REAP 标记输出**（`OP_RETURN "REAP:<height>:<count>:<hash>"`，便于识别与审计）。
  * 费用=**30% 税**（由系统交易“少付输出”让渡为 Miner fee；第 4 周由 coinbase 收取）。
* 属性测试：**排序稳定性/幂等性/舍入一致性**；基准：10k 候选的选择耗时。
* 暴露内部 API，供第 4 周验证路径/挖矿模板调用。
* 在 simnet 做“干跑”（不广播）：验证蓝图数额与统计正确。

---

## 🧭 设计决策（拍板）

1. **排序键（默认严格模式）**
   `expiryHeight → amount → txid → vout`（全升序）

   * **简化模式（降级开关）**：`expiryHeight → txid → vout`（去掉 `amount`，发生性能/一致性问题时可一键切换）。
   * 两种模式均做属性测试，默认使用**严格模式**。

2. **税与返还（整数舍入规则）**

   * 对每个输入 **单独** 计算税：`tax_i = floor(value_i * 30 / 100)`；
   * `refund = Σvalue_i - Σtax_i`；
   * 这样避免累计浮点/跨实现差异，**确定性强**。

3. **返还脚本策略（按原脚本返还）**

   * 不再使用 BurnPolicy；返还输出直接使用输入 UTXO 的原始 `scriptPubKey`。
   * 若用 `OP_RETURN`，只**占位**并在第 4 周的策略/共识里允许其携带金额（或改为 P2WSH\_Zero）。

4. **容量与上限（不触链上限制）**

   * 选择器仅根据 **每块 REAP 输入上限**（如 `MaxREAPInputsPerBlock`）、**系统交易重量估算** 来截断候选集合；
   * 真实重量/区块大小校验在第 4 周的挖矿模板里生效。

---

## 🗂️ 目录/文件组织

```
btcd/
  mining/
    reap/
      selector.go          # SelectCandidates() - 读 ExpiryIndex + UTXO view，输出 REAPPlan
      packer.go            # BuildBlueprint() - 构造 MsgTx（refund 输出 + 标记输出）
      params.go            # 选择/打包参数（排序模式、上限、税率）
      weight.go            # 交易重量估算（保守估计）
      types.go             # REAPPlan/Stats/Errors
      selector_test.go     # 属性测试（排序稳定/幂等/舍入）
      packer_test.go       # 数量/金额/重量一致性测试
      bench_test.go        # 基准：1k/10k 候选
```

---

## 🔧 数据类型与 API（草案）

```go
// 排序/模式/策略
type SortMode int    // Strict, Simple

type REAPParams struct {
  Sort        SortMode
  MaxInputs   int        // 每块最多纳入多少个到期输入
  WeightBudget int64     // 预估系统交易重量上限（可选，0=忽略）
  TaxNum, TaxDen int64   // 默认 30/100
}

// 计划与统计
type REAPPlan struct {
  Inputs    []wire.OutPoint // 已按最终顺序排好
  TaxTotal  int64           // Σ floor(value_i*TaxNum/TaxDen)
  RefundTotal int64         // Σ value_i - TaxTotal
  Height    int32           // 计划针对的 tip height（或执行高度）
  Stats     struct {
    Candidates int
    Picked     int
    Skipped    int
    EstWeight  int64
  }
}

// 选择器：从 ExpiryIndex + UTXO 视图挑选到期 UTXO
func SelectCandidates(ctx context.Context, tip int32, idx *expiryindex.Index,
  view *blockchain.UtxoViewpoint, p REAPParams) (REAPPlan, error)

// 打包器：把计划构造成系统交易蓝图（不入 mempool，本周仅构造）
func BuildBlueprint(plan REAPPlan, view *blockchain.UtxoViewpoint, p REAPParams) (*wire.MsgTx, error)
```

---

## ⚙️ 关键实现要点

### A. 选择器（`selector.go`）

* 从 `ExpiryIndex.ScanRange(tipHeight)` 取出 “≤ tipHeight” 的到期 outpoints（分页迭代，避免一次性拉爆内存）。
* 过滤：

  * 已花费/不可见（`view.LookupEntry` == spent）→ **跳过**。
  * 未成熟 coinbase（极小概率）→ **跳过**。
* 排序：按 `SortMode` 进行稳定排序（使用 `slices.SortFunc`，确保比较器无非决定性来源）。
* 逐个累加：

  * 估算重量（`weight.go`，按 P2WPKH/P2WSH 的**保守上界**估计输入权重），叠加直到 `MaxInputs` 或 `WeightBudget` 打满即止。
* 统计：记录 `Candidates/Skipped/Picked/EstWeight`。
* 输出 `REAPPlan`。

### B. 打包器（`packer.go`）

* 创建 `MsgTx`：

  * `Version = REAP_VERSION`（常量占位，例如 `3` 或 `0x4F42`）
  * `LockTime = plan.Height`（利于后续审计；不用于共识）
  * 输入：每个 outpoint 的 `TxIn{SignatureScript: nil, Witness: nil, Sequence: 0xFFFFFFFE}`（占位；第 4 周验证规则中绕过脚本校验）
  * 输出1..N（**Refund**）：将 `plan.RefundTotal` 按原 `scriptPubKey` 聚合后输出
  * 输出2（**Marker**）：`value = 0`，`scriptPubKey = OP_RETURN "REAP:<height>:<count>:<sha256(inputs)>"`
* 计算 `TaxTotal`、`RefundTotal` 校验一致（断言：`Σinputs = TaxTotal + RefundTotal`）。
* 返回 `*MsgTx`（蓝图），**不入 mempool**，仅供第 4 周的模板/验证使用。

### C. 重量估算（`weight.go`）

* 输入估算：按 P2WSH 上界计（比实际更重，保证安全）。
* 输出估算：保守按 `N(Refund)+1(Marker)` 估算。
* 暴露 `EstimateBlueprintWeight(plan)`，供选择器“预算截断”。

### D. 参数注入（`params.go`）

* 暴露默认参数集 `DefaultREAPParams(Strict|Simple)`；
* 支持从 `chaincfg.Params` 读取“每块上限 / 策略 / 税率”的网络级默认值；
* 单测可重写参数，便于属性测试。

---

## 🧪 测试计划

### 1) 属性测试（`selector_test.go`）

* **排序稳定性**：同一批输入重复运行 100 次，输出顺序完全一致。
* **幂等性**：`Select` → `BuildBlueprint` → 重新基于蓝图还原输入集合，等价于原集合。
* **舍入一致性**：构造随机金额集合，验证 `Σfloor(value_i*30/100) + refund == Σvalue_i`。
* **降级一致性**（Simple 模式）：移除 `amount` 参与排序后，若输入金额互异，顺序不同但**蓝图哈希**在 Simple 模式内稳定。

### 2) 单元测试（`packer_test.go`）

* **两端极值**：最小/最大金额、数量（1、MaxInputs）。
* **容量截断**：给大于 `MaxInputs` 的候选，确保截断位置正确。
* **重量截断**：设置紧张预算，确保估算重量触顶时停止。
* **返还聚合**：相同 `scriptPubKey` 的返还输出会聚合且结果稳定。

### 3) 基准（`bench_test.go`）

* 1k/10k 候选输入，测 `SelectCandidates` 耗时与分配；
* 目标：10k 候选排序 + 选择 < 150ms（本地）。

### 4) 集成“干跑”（simnet）

* 用脚本批量造 UTXO（地址轮换、金额分布）；
* 推进高度使其“到期”（利用 Week 2 的 ExpiryIndex 加速窗口）；
* 调 `Select` + `BuildBlueprint`，打印统计：`picked/estWeight/tax/refund`；
* 验证 Marker 输出里 `sha256(inputs)` 与计划一致。

---

## 🕒 时间分配（≤ 20h）

| 任务                             |        预估 |
| ------------------------------ | --------: |
| 需求定稿 & 框架（types/params/weight） |      1.5h |
| 选择器实现（分页读取/过滤/排序/截断）           |      6.0h |
| 打包器实现（Refund/Marker/金额校验）      |      4.5h |
| 属性测试与单测（稳定/幂等/舍入/容量）           |      4.0h |
| 基准测试与优化                        |      1.0h |
| 集成“干跑”脚本 & 文档（简要）              |      1.0h |
| **合计**                         | **18.0h** |

> **缓冲**（2h）：若排序或重量估算踩坑，先切 **Simple 模式**，把严格模式留到第 4 周修正。

---

## 🧱 常见坑 & 规避

* **不确定排序来源**：不要使用 map 迭代或非稳定排序；比较器必须只依赖（高度/金额/txid/vout）。
* **金额来源**：从 `UtxoViewpoint` 读取，**不可**塞到 ExpiryIndex（避免双写与一致性问题）。
* **OP\_RETURN 金额标准性**：本周仅蓝图；标准性与共识/策略放到第 4 周统一处理。
* **重量估算偏差**：宁可保守（高估），以免第 4 周模板阶段被区块重量拒绝。
* **性能**：对 ExpiryIndex 的读取要分批（如每批 10k），避免一次性加载过多候选。

---

## 📑 本周交付物（Deliverables）

* `mining/reap/`: `selector.go`, `packer.go`, `params.go`, `weight.go`, `types.go`
* 测试：`selector_test.go`, `packer_test.go`, `bench_test.go`
* `docs/week3-validation.md`：干跑结果（picked/tax/refund/estWeight/marker 哈希）
* （可选）`cmd/reap-dryrun/`：命令行小工具，便于在 simnet 交互查看计划与蓝图摘要

---