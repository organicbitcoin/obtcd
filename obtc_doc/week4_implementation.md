# Week 4 计划（共识验证 & 挖矿集成）— Go + btcd + Cursor

> 修订（2026-02-11）：本周前置依赖增加 `week3_1_implementation.md`。建议先完成 Week3.1 的 marker 向量测试、dry-run 与参数映射，再进行共识接线。

**时间预算：18–20 小时** ｜ **目标**：把第 3 周产出的 **REAP 选择器 + 蓝图**接入**共识验证路径**与**挖矿模板**，实现“到期→系统交易→矿工收税”的端到端闭环，并跑通 reorg/压力用例。

---

## 🎯 本周目标（Definition of Done）

* **共识验证**：区块中允许**至多 1 笔 REAP 交易**；对其执行**专用验证规则**（无需签名脚本），并强制**确定性选择**与**税/返还**一致。
* **挖矿模板**：在 `NewBlockTemplate` 中自动插入 REAP 蓝图，**税额以交易费形式**并入 coinbase（含**每块税上限**）。
* **策略/中继**：mempool **拒绝** REAP 交易（只允许矿工内部构造），普通交易**不得花费已过期 UTXO**。
* **端到端**：simnet 上演示“到期→REAP→coinbase 收税”≥48 小时稳定出块。
* **测试**：单元 + 集成 + reorg 场景全部通过；CI 绿。

---

## 🗂️ 代码组织（新增/修改）

```
btcd/
  blockchain/
    validation_reap.go     # ✅ 新：REAP 专用共识验证（IsREAP、CheckReapTx、CheckExpiredSpends）
    validate.go            # ♻️ 挂钩：在常规交易验证前后调用 REAP 检查
    spendcheck.go          # ♻️ 增加“过期 UTXO 只能被 REAP 花费”的检查
  mining/
    reap/
      selector.go          # Week3
      packer.go            # Week3
      params.go            # Week3（+ 本周补充 cap 参数）
    template_reap.go       # ✅ 新：把 REAP 蓝图注入挖矿模板、计算税费与 cap
    template.go            # ♻️ 调用点
  mempool/
    policy.go              # ♻️ 非标准：拒收 REAP 交易 & 禁止花费已过期 UTXO
  rpc/
    miningrpc.go           # ✅ 可选：debug RPC getreaptemplate（只读）
  chaincfg/
    params_obtc.go         # ♻️ 网络参数补充：MaxREAPInputsPerBlock、MaxReapTaxPerBlock、TaxRate
```

---

## 🔧 共识规则（本周落地为代码的“硬规定”）

### 1) REAP 交易识别（`IsREAP(tx)`）

* 满足全部条件即视为 **REAP**：

  * `nVersion == REAP_VERSION`（常量）；
  * **无签名数据**（`ScriptSig/Witness` 为空）；
  * **恰有 2 个输出**：

    * **Refund 输出**：金额为 `Σinputs - TaxTotal`，并返还至输入原始 `scriptPubKey`（允许按脚本聚合）；
    * **Marker 输出**：`OP_RETURN "REAP:<height>:<count>:<sha256(inputs)>"`，金额=0；
  * **无锁时/序号**要求（可将 `nLockTime = height` 作为审计信息，不参与规则判定）。

> 备注：REAP 交易**不可进入 mempool**，只作为“系统交易”随区块出现。

### 2) 过期 UTXO 花费规则

* **任何非 REAP 交易**一旦花费**过期** UTXO → **区块无效**。
* REAP 交易的每个输入必须满足：在执行高度 `H` 时 `utxo.IsExpired(H)` 为真（依据 Week2 Index）。
* 如果在一个区块里既有普通交易又有 REAP，顺序按正常拓扑；但**普通交易**不得花费**已过期**的 UTXO（未过期的照常）。

### 3) 确定性选择 & 数额校验

* 节点在验证时调用与挖矿相同的 `SelectCandidates(H, view, params)` 得到**期望计划** `Plan*`。
* 区块若含 REAP：

  * **输入集合**必须等于 `Plan.Inputs[:k]`（`k ≤ MaxInputs`，与蓝图一致）；
  * **税额** `TaxTotal = Σ floor(value_i * r.num / r.den)`；
  * **Refund** = `Σvalue_i - TaxTotal`；
  * 交易内 **Refund/Marker 输出**金额与脚本必须精确匹配；
  * **不允许额外输出**。
* 区块**不含** REAP：

  * **阶段化建议**：先采用“允许跳过但记录告警/统计”的软约束（Week4），避免上线初期因实现差异影响活性；
  * 在 Week7 硬化阶段再评估是否升级为“强制必须包含 REAP”的硬共识约束。

### 4) 税收入账（fee 化）与上限

* REAP 交易的 **交易费 = TaxTotal**（返还部分由交易输出显式支付）；
* coinbase 总额 = **区块补贴 + 全部交易费**（包括 REAP 税）。
* 增加网络参数：`MaxReapTaxPerBlock`（例如对主网设置一个安全上限），若 `TaxTotal > cap` → **区块无效**（或截断选择器以不超 cap）。
* 为防极端体积，保留 `MaxREAPInputsPerBlock` 与模板侧的**估重**限制。

---

## ⚙️ 挖矿模板集成（`template_reap.go`）

* 在组装模板开始阶段：

  1. 基于 `tip` 和 `UtxoView` 调用 `SelectCandidates`；
  2. 得到 `Plan` 后调用 `BuildBlueprint`；
  3. 将蓝图交易插入模板 **tx 列表的第一位**（紧随 coinbase 之后）；
  4. 更新模板的费统计，使 coinbase 反映 `TaxTotal`；
  5. 如 `TaxTotal > MaxReapTaxPerBlock` 或重量预算达不到 → **截断输入**并重建蓝图，直至满足限制（或不插入）。
* 若 `Plan.Picked == 0` 则不插入 REAP 交易（模板保持原样）。

---

## 🚦 策略/中继（`mempool/policy.go`）

* **拒收 REAP 交易**（`IsREAP(tx) => reject non-standard`）。
* 普通交易若尝试花费**已过期** UTXO：在策略层即可 `reject`（双保险，真正的判定仍在共识层）。

---

## 🧪 测试计划

### A. 单元测试

1. **验证规则**（`validation_reap_test.go`）

   * 合法 REAP：输入为到期集合，Refund/Marker 正确，区块有效。
   * **错序/漏选/多选**：与 `SelectCandidates` 结果不一致 → 无效。
   * **税额/四舍五入**：逐输入 `floor` 累加一致，任何偏差 → 无效。
   * **超 cap**：`TaxTotal > MaxReapTaxPerBlock` → 无效。
   * **非 REAP 花费过期**：存在则区块无效。
2. **模板集成**（`template_reap_test.go`）

   * 无到期 → 不插 REAP；
   * 有到期 → 插入且 coinbase 费计入；
   * 多到期 → 受 `MaxInputs`/重量预算截断；
   * 超 cap → 截断后有效。

### B. 集成测试（simnet）

* 构造一批“将到期”UTXO；推进高度触发 REAP：

  * 观察：区块含 1 笔 REAP、coinbase 费用增长=税额；
  * 导出 Refund/Marker 输出并校验与计划一致。
* **reorg**：

  * 分叉两条链（分别含/不含 REAP 或不同选择），以更长链取胜；
  * 断言：最终链索引与余额一致，没有“双 REAP”或“漏 REAP”。

### C. 压力/边界

* 小额 UTXO 洪水（靠近 DUST 阈值）→ 选择器 & 模板仍能稳定构造蓝图；
* 大量到期（> `MaxInputs` 数倍）→ 连续多块逐步清理，coinbase 费稳定；
* 随机金额集合的**税额幂等**（多次构建蓝图结果稳定）。

---

## 🕒 时间分配（≤ 20h）

| 任务                                      |        预估 |
| --------------------------------------- | --------: |
| 规则梳理 & 接口挂钩（validate.go / spendcheck）   |      2.0h |
| `IsREAP` + `CheckReapTx`（识别/税/返还/集合一致性） |      5.0h |
| `CheckExpiredSpends`（非 REAP 禁止花费过期）     |      1.0h |
| 模板集成（构造/插入/费统计/截断 & cap）                |      4.0h |
| 策略/中继（mempool 拒收 REAP & 过期花费）           |      1.0h |
| 单元测试（规则 + 模板）                           |      4.0h |
| 集成/冒烟（simnet + reorg）                   |      2.0h |
| **合计**                                  | **19.0h** |

> **缓冲**：若 `CheckReapTx` 实现复杂超时，先只做**集合/税额/输出**一致性校验，把 **cap** 检查放在模板侧，下一周再把 cap 提升到共识层（若需要）。

---

## 🧱 常见坑 & 规避

* **脚本验证绕过范围**：仅对 **REAP** 交易跳过签名验证，且 **仅允许花费已过期 UTXO**；否则可能被滥用。
* **与 mempool 交互**：REAP 不入池；普通交易在策略层预拒绝过期花费，但最终判定在共识层。
* **选择器一致性**：验证端与模板端**调用同一实现**，避免双份逻辑偏差。
* **税额舍入**：务必按“逐输入 `floor`”累加；任何全局比例乘法都会在跨实现时跑偏。
* **cap 与体积**：先在模板侧截断，再在共识层做**硬约束**（如来不及，本周先模板侧，Week5 再上链约束）。

---

## 📑 本周交付物（Deliverables）

* `validation_reap.go` + 测试
* `template_reap.go` + 测试
* `mempool/policy.go` 小改
* `chaincfg/params_obtc.go` 增：`MaxREAPInputsPerBlock`、`MaxReapTaxPerBlock`、`TaxRate`
* `docs/week4-validation.md`：包含 txid、区块哈希、coinbase 费差、reorg 结果
