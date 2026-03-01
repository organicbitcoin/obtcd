# 有机比特币（Organic Bitcoin, OBTC）白皮书 V1（实现对齐 · 易读扩展版）

**版本**：1.0  
**状态**：征求意见稿（RFC）  
**读者**：协议工程师、钱包开发者、矿池、交易所、研究人员、运维团队、社区建设者

---

## 0. 阅读指南（先看这里）

这份白皮书的目标是：**讲清楚**，而不是“堆术语”。

为了降低阅读门槛，建议按下面顺序读：

- **3 分钟版**：看 Section 1、Section 2、Section 5.1（参数总表）
- **产品/运营版**：再看 Section 6（生命周期）、Section 11（钱包）、Section 14（运维）
- **工程实现版**：重点看 Section 7（索引）、Section 8（REAP）、Section 9（回放保护）、Section 13（安全）
- **经济与治理版**：看 Section 12（经济）、Section 15（治理）

文中会直接使用 English 专业术语（例如 UTXO、Replay Protection、Mempool、Coinbase），
但每个术语都在 **Section 19（术语表）**给出中文解释。

---

## 1. 一句话理解 OBTC

OBTC 是一条从 Bitcoin 分叉演进的链：

- 币仍以 **UTXO** 形式存在；
- 但每个 UTXO 都有“生命周期”；
- 到期后可由系统机制 **REAP** 回收：
  - **70%** 返还到原脚本（Refund）
  - **30%** 进入矿工安全预算（Tax）
- 同时通过 **Namespace Isolation + Replay Protection** 降低跨链重放与误转风险。

换成用户视角就是：

> 你的资产不是“永远静止自动安全”，而是“需要周期性维护、可被系统持续保障”。

---

## 2. 为什么要做这件事（动机）

## 2.1 现实问题

传统 UTXO 模型有一个长期问题：

- 一部分资产会永久沉睡（遗失私钥、遗产失联、备份损坏）；
- 这些资产名义在链上，实际上不再参与经济流通；
- 同时网络安全预算长期依赖补贴递减 + 手续费，不确定性提升。

## 2.2 OBTC 的回答

OBTC 不把“永久静止”当成默认最优，而是引入“有机代谢”模型：

1. **到期机制**让长期不动资产进入可治理状态；
2. **系统回收**把沉睡供给的一部分回流到安全预算；
3. **返还机制**避免“一刀切没收”；
4. **钱包侧续期工具**让活跃持有者以较低成本保持资产活性。

## 2.3 设计目标

- 持续安全预算（Security Budget）
- 明确资产生命周期（Lifecycle Clarity）
- 可预测规则（Deterministic Behavior）
- 可运营系统（Operability）
- 脚本中立（Script Neutrality）

## 2.4 非目标

- 不追求“无限期无需维护”的资产模型；
- 不要求所有节点永久保存完整历史交易体；
- 不引入复杂治理金库或人工裁量分配逻辑。

---

## 3. 系统全景：三层架构

OBTC 实现可以分为三层，每层职责不同。

## 3.1 Consensus Layer（共识层）

负责回答：**什么是合法的交易和区块**。

核心能力：

- Expiry 到期判定
- REAP 合法性判定
- Replay Protection 激活后签名语义强制

## 3.2 Mining & Policy Layer（出块与策略层）

负责回答：**在合法集合里，如何组块更稳**。

核心能力：

- REAP 候选选择与构造
- 模板中 REAP 权重预留
- Mempool 与系统交易隔离

## 3.3 Wallet Execution Layer（钱包执行层）

负责回答：**用户如何知道风险、如何执行动作**。

核心能力：

- `obtc.getexpiry`：看到期风险
- `obtc.renew`：手动续期
- `renewall`：批量续期
- Auto-Renew：自动续期（含失败退避与预算上限）

---

## 4. 核心机制（先讲人话）

先用一个最直观例子。

假设你有一个 1 BTC 的 UTXO：

1. 它在高度 `h_create` 创建；
2. 到高度 `h_create + E`（E 是到期窗口）变为 expired；
3. 若你在此之前续期：资产继续由你主动控制；
4. 若你一直不处理，系统可在区块内执行 REAP：
   - 返还 `0.7 BTC`（到原脚本）
   - 税额 `0.3 BTC` 计入矿工收益
   - 新返还输出重新开始计时

关键点：

- 不是“直接归零”；
- 不是“随意处置”；
- 是规则化、可验证、可预期的链上行为。

---

## 5. 参数与激活时序（当前实现值）

## 5.1 三网激活矩阵

| 网络 | Fork Height | Expiry Index Start | Expiry Enable | REAP Hardening | Replay Protection |
|---|---:|---:|---:|---:|---:|
| Mainnet | 950000 | 950000 | 1050000 | 1060000 | 1065000 |
| Testnet | 2800000 | 2800000 | 2800100 | 2800120 | 2800130 |
| Regtest | 100 | 100 | 110 | 112 | 114 |

解释：

- **Fork Height**：从这一高度开始进入 OBTC 独立规则域。
- **Expiry Enable**：到期花费约束开始生效。
- **REAP Hardening**：REAP 输入顺序与输入上限变为共识强制。
- **Replay Protection**：签名域隔离语义变为共识强制。

## 5.2 Expiry 参数

| 网络 | WindowBlocks | ListBatchLimit | ReapMaxInputs |
|---|---:|---:|---:|
| Mainnet | 3,679,200 | 10,000 | 256 |
| Testnet | 1,008 | 5,000 | 500 |
| Regtest | 144 | 1,000 | 200 |

解释：

- Mainnet `3,679,200` 块约等于 7 年；
- Testnet/Regtest 窗口更短用于快速验证。

## 5.3 Namespace Isolation 参数

- Network magic：
  - Main `0x4F425443`
  - Test `0x4F544553`
  - Reg `0x4F524547`
- Default port：`8555 / 28555 / 28666`
- Bech32 HRP：`obtc / obtct / obtcrt`
- 地址前缀（P2PKH/P2SH/Witness）隔离
- HD key version 隔离
- BIP44 coin type：`20260 / 20261 / 20262`

实现要求：启动阶段会做冲突校验，碰撞即拒绝启动。

## 5.4 REAP 默认策略参数（实现）

- Sort mode：`Strict`
- Mainnet Max inputs：`256`
- Mainnet Weight budget：`200,000`
- Tax ratio：`30%`
- Dust threshold：`546 sat`

## 5.5 Auto-Renew 默认参数（实现）

- Enabled：`false`
- Interval：`30m`
- Failure backoff：`15m`（可为 0）
- Window：`window_end <= blocks_to_expiry <= window_start`
  - `window_start=52560`
  - `window_end=25920`
- MaxUtxosPerRun：`100`
- MaxFeeRate：`5000 sat/KB`
- MaxRenewAmountPerRun：`0`（unlimited）
- `autorenewamount`：启用时必须 > 0

---

## 6. UTXO 生命周期（Lifecycle）

## 6.1 关键公式

设：

- `T = 7` 年
- 每年块数 `Y = 52,560`
- 到期窗口 `E = T × Y = 367,920`

任意 UTXO：

`h_expiry = h_create + E`

当 `h_tip >= h_expiry`，该 UTXO 进入 expired。

## 6.2 状态分层（钱包侧）

为了让用户易懂，钱包通常将状态展示为：

- `ok`：离到期还远
- `expiring`：临近到期，建议处理
- `expired`：已到期

说明：`ok/expiring` 是 UX 分层；真正共识判定是“是否已到期”。

## 6.3 生命周期中的两条路径

1. **主动路径（用户）**：到期前续期，继续保持资产活性；
2. **系统路径（协议）**：到期后可被 REAP 规则化处理。

## 6.4 一个完整示例

- UTXO 金额：`10,000,000 sat`（0.1 BTC）
- 到期后进入 REAP：
  - `tax = floor(10,000,000 × 30%) = 3,000,000 sat`
  - `refund = 7,000,000 sat`
- 若 `refund` 小于 DUST 阈值，则不创建 refund 输出，全部进入 tax。

---

## 7. Expiry Index：如何高效追踪到期资产

## 7.1 为什么必须有索引

如果每个新区块都全历史扫描“谁到期了”，成本不可接受。  
因此需要专门的到期索引。

## 7.2 双向映射结构

实现上可抽象为：

1. `OutPoint -> ExpiryKey`
2. `ExpiryKey -> OutPoint set`

优势：

- 删除快：花费时快速反查并移除；
- 扫描快：按 ExpiryKey 区间顺序遍历。

## 7.3 区块连接与回滚

- ConnectBlock：新增输出入索引，被花费输入出索引；
- DisconnectBlock：反向恢复。

因此在 reorg 下，索引可与主链状态一致。

## 7.4 分页扫描与续扫

支持语义：`(fromKey, toKey, maxResults, startAfter)`

- `maxResults` 控制单页大小；
- `startAfter` 支持断点续扫；
- 适合大规模查询，避免一次性返回过大结果。

---

## 8. REAP：系统回收交易的完整规则

## 8.1 REAP 是什么

REAP 是 block-internal system transaction：

- 目标：处理 expired UTXO；
- 特点：不是普通用户交易模板，不依赖 mempool relay。

## 8.2 合法性前提

REAP 要合法，至少满足：

- 输入必须是 expired UTXO；
- 交易结构满足 REAP 特征；
- marker 一致性校验通过；
- 激活后满足 hardening 约束（顺序与上限）。

## 8.3 Tax / Refund / Dust 规则

对每个输入金额 `v`：

- `tax = floor(v × 30 / 100)`
- `refund = v - tax`

Dust fold：若 `0 < refund < dust_threshold`，则

- `refund -> 0`
- `tax += refund`

## 8.4 Marker 绑定

marker 形如：

`REAP:<height>:<count>:<digest>`

共识校验：

- `height` 一致；
- `count` 一致；
- `digest` 一致。

## 8.5 Hardening（激活后）

REAP 输入必须按 canonical order：

`expiry -> amount -> outpoint`

并满足输入数量上限 `ReapMaxInputs`。

## 8.6 Coinbase Accounting

定义：

- `ReapTax(h)`：当块 REAP 税额总和

coinbase 总额上限：

`BaseSubsidy(h) + Fees(h) + ReapTax(h)`

`ReapTax` 与 coinbase 一样受成熟期约束。

## 8.7 不变量（Invariant）

必须始终成立：

`sum(inputs) = sum(refunds) + sum(tax)`

这是审计和回归验证的核心检查项。

---

## 9. Replay Protection：如何防跨链重放

## 9.1 两层防线

1. **Namespace Isolation**：从地址、端口、HD、coin type 等入口隔离。  
2. **Replay-protected sighash domain**：从签名消息域隔离。

## 9.2 实现要点

- replay bit：`0x40`
- domain tag：分别覆盖 Legacy 路径、Witness 路径、Taproot 路径（三套独立标签）

## 9.3 激活门控

- 激活前：兼容语义仍可运行；
- 激活后：缺失 replay-protected 语义的签名直接失败。

## 9.4 路径覆盖

- Legacy
- SegWit（版本 0）
- Taproot（key path + script path）

---

## 10. 节点运行模型（Node Operation Model）

## 10.1 Pruned Full Node 与 Archive Node

- **Pruned Full Node**：保留完整 UTXO + 滚动窗口区块体 + 全区块头。
- **Archive Node**：保留全历史区块与交易体。

## 10.2 不变量

- 历史不可改写；
- 剪枝只改变“存储保留”，不改变“共识可验证性”。

## 10.3 快照同步（UTXO Commitment）

可通过快照 + 证明 + 多源一致性校验提升初始同步效率。  
建议至少 `k` 个独立来源一致后再信任导入。

---

## 11. 钱包能力闭环（Wallet Capabilities）

## 11.1 `obtc.getexpiry`

输出到期风险关键信息：

- outpoint
- amount
- create/expiry height
- blocks_to_expiry / days_to_expiry
- status
- dust_risk

## 11.2 `obtc.renew`

支持显式 outpoint 续期，参数包含：

- amount
- target address（可选）
- max fee rate（可选）
- minconf（可选）

返回 tx 摘要（txid、输入输出数、费率等）。

## 11.3 `renewall`

支持：

- status 或窗口筛选
- dry-run
- interval/runs 调度

## 11.4 Auto-Renew 调度器

具备：

- 周期执行
- 窗口筛选
- 候选数上限
- 费率上限
- Failure backoff
- Per-run budget

目的：在保证安全边界下实现自动化风险缓释。

---

## 12. 经济与博弈（Economics & Game Theory）

## 12.1 长期安全预算估算

设 `ρ=0.7, T=7`，则：

\[
p = -\ln(\rho)/T \approx 5.1\%/年
\]

若沉睡供给占比 `L`：

\[
B \approx L \cdot p
\]

当 `L ∈ [20%, 30%]` 时，`B` 约为 `~1.0%–1.5%/年`。

## 12.2 用户合规激励

当续期成本比例 `φ` 远小于 `30%` 税率时，理性用户倾向主动续期。  
因此大额 UTXO 通常会主动维护；小额 UTXO 则受 dust 约束。

## 12.3 MEV 抑制

确定性顺序 + 资源上限可压缩“择优收割”空间，降低策略噪声。

---

## 13. 安全模型与威胁分析（Security Model）

## 13.1 典型威胁

- 伪造 marker
- 重排 REAP 输入
- 构造超上限输入集
- 跨链 replay
- 向 mempool 注入伪系统交易
- 快照投毒
- reorg 套利

## 13.2 对应防线

- marker 一致性校验
- canonical order
- input cap + weight budget
- replay-protected sighash
- mempool policy
- 多源快照校验
- coinbase maturity 约束

---

## 14. 运维与可观测性（Operations & Observability）

## 14.1 建议指标

### Consensus

- expired spend rejection count
- REAP non-expired spend rejection count
- replay-protection violation count

### Mining

- template build attempts with REAP
- REAP append success rate
- reserved weight utilization
- REAP tax contributed to coinbase

### Wallet

- auto-renew candidate count / run
- auto-renew success/failure ratio
- backoff activated count
- per-run budget truncation count

## 14.2 告警建议

- 连续 N 轮 auto-renew failure
- REAP append success 连续低于阈值
- expiring backlog 持续上升

## 14.3 实施建议（工程落地）

1. 共识层：Expiry/REAP/Replay Protection 全链路测试  
2. 节点层：剪枝与归档模式双套回归  
3. 钱包层：到期提醒、手动续期、自动续期稳定性测试  
4. 挖矿层：模板组块边界与压力测试  
5. 测试网络：采用加速到期参数（如约 7 天）演练全流程  
6. 工具层：公共看板与异常追踪报表

---

## 15. 治理与升级（Governance）

治理原则：

- 核心常量尽量稳定；
- 可调参数必须有延迟生效与明确激活高度；
- 升级流程尽量机械、透明、可复现；
- 尽量减少自由裁量空间。

---

## 16. 数学附录（Mathematical Appendix）

- 年化衰减：`p = -lnρ / T`
- 安全流入：`B = L × p`
- 等预算变换：`ρ' = exp(-pT')`
- 目标预算反解：`ρ = exp(-(B_target × T)/L)`，`τ = 1 - ρ`

---

## 17. 关键 KPI（KPI）

- `ReapTax / MinerRevenue`（年化）
- 在险供给（≤90 天到期）
- 续期成功率
- 每块 REAP 数量均值与分位波动
- 归档节点数量与分布
- 自动续期渗透率与投诉率

---

## 18. 法务与合规提示

OBTC 是独立链，不等同于 Bitcoin。到期与回收机制在不同法域可能被解释为协议费、负利率或对沉睡资产再分配。交易所与托管机构需据此设计内部流程与用户告知文本。

---

## 19. 术语表（Glossary）

> 这一节专门用于降低阅读门槛。遇到陌生词，优先回看这里。

| 术语 | 英文 | 含义（通俗版） | 在 OBTC 里的作用 |
|---|---|---|---|
| 未花费输出 | UTXO | 你在链上“还能花”的那一笔钱 | OBTC 的基础记账单元 |
| 输出定位符 | OutPoint | `txid:vout`，某个输出的唯一坐标 | 精确定位某个 UTXO |
| 到期 | Expiry | 资产到达规则年龄上限 | 触发续期或系统回收 |
| 到期高度 | Expiry Height | UTXO 变成 expired 的区块高度 | 共识判断的硬阈值 |
| 到期索引 | Expiry Index | 专门记“谁何时到期”的索引结构 | 让扫描与删除高效可行 |
| 系统回收 | REAP | 协议在区块内处理 expired UTXO 的机制 | 把沉睡资产部分回流安全预算 |
| 回收交易 | REAP Tx | 用于执行 REAP 的系统交易 | 非普通用户交易模板 |
| 返还 | Refund | REAP 后回到原脚本的部分（70%） | 保留资产连续性 |
| 税额 | Tax | REAP 后进入矿工收益的部分（30%） | 构成持续安全预算来源 |
| 标记输出 | Marker | `REAP:height:count:digest` 的 OP_RETURN | 绑定输入集与上下文一致性 |
| 规范顺序 | Canonical Order | 约定好的唯一输入排序方式 | 防止实现差异导致分歧 |
| 内存池 | Mempool | 待确认交易暂存区 | REAP 在此被拒收 |
| 区块模板 | Template | 矿工准备打包的候选区块 | REAP 在此被条件追加 |
| 出块奖励交易 | Coinbase | 区块里的奖励结算交易 | 通过上限规则纳入 ReapTax |
| 回放保护 | Replay Protection | 防跨链重放的签名语义约束 | 激活后强制执行 |
| 签名哈希类型 | SigHash | 决定签名覆盖范围和语义 | 引入 replay bit 后隔离域 |
| 域隔离 | Domain Separation | 用不同 tag 区分签名消息空间 | 防止跨链复用签名 |
| 命名空间隔离 | Namespace Isolation | 地址/端口/HD 等命名隔离 | 防止误连与误转 |
| 动态尘埃阈值 | Dynamic Dust | 基于费率变化调整 dust 阈值 | 避免产生不可经济输出 |
| 自动续期 | Auto-Renew | 钱包后台自动执行续期 | 降低用户漏操作风险 |
| 失败退避 | Failure Backoff | 失败后延迟下一轮 | 防止失败风暴 |
| 单轮预算 | Per-Run Budget | 每轮可续期总额上限 | 防止异常放量 |
| 可剪枝全节点 | Pruned Full Node | 只保留必要窗口数据的全节点 | 降低存储压力 |
| 归档节点 | Archive Node | 保留全历史数据的节点 | 提供全历史查询与审计 |
| 链重组 | Reorg | 主链分支切换 | 需要索引与状态回滚一致性 |
| 拒绝服务 | DoS | 用资源耗尽方式拖垮系统 | 通过上限和预算约束缓解 |
| 不变量 | Invariant | 始终必须成立的规则 | 核心正确性校验锚点 |

---

## 20. 常见问题（FAQ）

### Q1：我什么都不做，会发生什么？

如果你的 UTXO 长时间不动并达到到期高度，它会进入 expired 状态。此后它不再按普通交易路径花费，而可能被系统 REAP 处理：70% 返还、30% 计入安全预算。

### Q2：是不是到了到期点就“立刻被拿走”？

不是。到期意味着“进入系统可处理状态”，不是“瞬时扣除”。具体在何时被 REAP，取决于区块内模板是否在该轮选择到该输入。

### Q3：我怎么避免进入 REAP？

最直接方法是到期前主动续期。你可以手动用 `obtc.renew`，也可以使用批量或自动续期策略。

### Q4：为什么要有 30% 税？

这是协议层安全预算设计：把沉睡资产的一部分回流到矿工收益，提升长期安全预算韧性。对活跃用户而言，通常续期成本显著低于 30%。

### Q5：回放保护和地址前缀隔离是不是重复？

不是。两者分别作用在不同层：

- 地址/端口/HD 等命名空间隔离，防误连和误转；
- 签名域隔离，防跨链复用签名语义。

两者叠加才是完整防线。

### Q6：为什么 REAP 不进 mempool？

因为它是系统交易，不是用户普通交易。放进 mempool 会引入伪系统交易污染与资源滥用风险。当前模型是“模板内构造、区块内执行”。

### Q7：小额 UTXO 会怎样？

如果返还额低于 dust 阈值，会触发 dust fold：不创建返还输出，返还额并入 tax。这样可以避免制造链上垃圾输出。

### Q8：我只关心工程落地，最该看哪些章节？

建议顺序：Section 5（参数）-> Section 8（REAP）-> Section 9（Replay Protection）-> Section 11（Wallet）-> Section 14（运维）。

### Q9：这套规则会不会让系统太复杂？

复杂度主要来自“把长期风险显式化”。OBTC 的做法是：把复杂度前置到规则与工具中，减少后期靠人工补救的不可控风险。

### Q10：如果未来参数要调整，怎么避免随意改？

参数升级应遵循“延迟生效 + 固定激活高度 + 透明信号过程”的原则，核心目的是降低治理不确定性与市场预期扰动。

---

## 21. 结语

OBTC 的核心不在“概念新奇”，而在“规则可执行”。

它把三个问题放在同一套机制里解决：

1. 资产长期沉睡如何处理；
2. 安全预算如何在长期保持韧性；
3. 跨层系统如何保持可预测、可运营、可解释。

这份白皮书的价值标准只有一个：

> 规则是否清楚到足以让不同团队独立实现，并在同一链上得到一致结果。

---

## 22. 许可

本文档建议采用 **CC BY 4.0**；参考实现可采用 MIT 等宽松许可。

---

*白皮书 v1.0（实现对齐 · 易读扩展版）完*  
