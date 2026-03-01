# 有机比特币（Organic Bitcoin, OBTC）白皮书 V1（实现对齐融合版）

**版本**：1.0（实现对齐草案）  
**状态**：征求意见稿（RFC）  
**读者**：协议工程师、钱包开发者、矿池、交易所、研究人员、运维团队

---

## 版本说明

本版本在不删除既有设计细节的前提下，将“V0 设计叙述”与“2025-10 至今已落地实现”融合为一体化文档：

- 保留 V0 的核心理念、数学框架、治理边界与路线图；
- 纳入当前实现中的参数、激活时序、校验行为与运行约束；
- 对存在“提案值 vs 当前实现值”的条目明确并列说明。

本文档目标不是宣传，而是作为“可讨论、可实现、可验证”的技术规范底稿。

---

## 摘要（Abstract）

有机比特币（OBTC）是一条从 Bitcoin 分叉演进的区块链，它将币视作**有生命周期的 UTXO**：UTXO 必须在规则窗口内被重新激活（移动/续期），否则进入到期状态。核心机制包括：

1. **Expiry（到期机制）**：每个 UTXO 基于创建高度计算到期高度。  
2. **REAP（系统回收）**：到期后，协议允许系统交易回收该 UTXO；其中 **30%** 计入矿工安全预算，**70%** 返还至原始锁定脚本，并重置计时。  
3. **Replay Protection（回放保护）**：通过命名空间隔离与签名域隔离双层防护，降低跨链重放风险。  
4. **轻节点运行导向**：普通全节点可采用滚动窗口剪枝；归档节点保留全历史。  

OBTC 的目标是把“长期不动资产”从被动沉淀转化为可度量的安全预算来源，同时保持协议可验证性、可预测性与工程可运营性。

---

## 1. 理念与动机（Philosophy & Rationale）

### 1.1 有机货币观

传统 Bitcoin 强调“币可无限静止”。OBTC 引入另一种视角：

- 长期静止常对应熵增（私钥遗失、主体失联、输出遗忘）；
- 协议可以把这类“死亡供给”逐步代谢为网络安全预算；
- 对活跃持有者而言，规则可简化为：**到期前主动续期**。

### 1.2 设计目标

- **持续安全预算**：把沉睡/丢失供给部分转化为矿工收益。  
- **状态卫生（State Hygiene）**：推动 UTXO 周期性整理，抑制长期僵尸状态。  
- **运行轻量化**：支持滚动窗口剪枝与可验证快照同步。  
- **规则可预期**：用确定性排序、硬边界参数降低实现分歧。  
- **脚本中立**：到期后对脚本类型一视同仁。

### 1.3 非目标

- 不追求无限期冷存与超长期锁定（>7 年）兼容。  
- 不要求所有节点永久保存完整历史交易体（归档节点承担该职责）。

---

## 2. 货币机制（Monetary Mechanism）

设到期年限 `T = 7` 年，返还比例 `ρ = 0.70`（税率 `τ = 1 - ρ = 0.30`）。

对“永久不再移动”的 UTXO，其余额在每个到期周期经历几何衰减，对应连续年化衰减率：

\[
p = -\frac{\ln \rho}{T}
\]

代入 `T=7, ρ=0.7`，得 `p ≈ 5.1%/年`（仅作用于永久丢失部分）。

若丢失供给占比为 `L`，则每年回流到安全预算的大致比例：

\[
B \approx L \cdot p = L \cdot \left(-\ln\rho/T\right)
\]

当 `L ∈ [20%, 30%]` 时，长期安全预算可约为 `~1.0%–1.5%/年`。

---

## 3. 协议参数与激活时序（Constants & Activation）

参数是行为开关，不是注释。以下为当前实现对齐值。

### 3.1 三网激活矩阵（当前实现）

| 网络 | Fork Height | Expiry Index Start | Expiry Enable | REAP Hardening | Replay Protection |
|---|---:|---:|---:|---:|---:|
| Mainnet | 950000 | 950000 | 1050000 | 1060000 | 1065000 |
| Testnet | 2800000 | 2800000 | 2800100 | 2800120 | 2800130 |
| Regtest | 100 | 100 | 110 | 112 | 114 |

说明：

- `Fork Height`：链身份规则分歧点；此前与 Bitcoin 保持一致。  
- `Expiry Enable`：普通交易与 REAP 的到期花费约束开始生效。  
- `REAP Hardening`：REAP 输入顺序与上限变为共识强制。  
- `Replay Protection`：签名域隔离语义变为共识强制。

> 历史说明：V0 草案中出现过以高度 `600,000` 为分叉基线的提案表达；当前实现采用上表参数。

### 3.2 Expiry 参数（当前实现）

| 网络 | WindowBlocks | ListBatchLimit | ReapMaxInputs |
|---|---:|---:|---:|
| Mainnet | 3,679,200 | 10,000 | 256 |
| Testnet | 1,008 | 5,000 | 500 |
| Regtest | 144 | 1,000 | 200 |

说明：

- Mainnet `WindowBlocks = 3,679,200` 对应约 7 年窗口；
- Testnet/Regtest 缩短窗口以提升验证和回归效率。

### 3.3 Namespace Isolation（当前实现）

OBTC 已与 Bitcoin 在以下维度做硬隔离：

- Network magic：
  - Main `0x4F425443`
  - Test `0x4F544553`
  - Reg `0x4F524547`
- Default port：`8555 / 28555 / 28666`
- Bech32 HRP：`obtc / obtct / obtcrt`
- 地址前缀（P2PKH/P2SH/Witness）
- HD 扩展键版本（pub/prv）
- BIP44 coin type：`20260 / 20261 / 20262`

实现要求：启动期执行冲突校验，若与 BTC 命名空间碰撞则拒绝启动。

### 3.4 REAP 策略默认值（当前实现）

- Sort mode：`Strict`
- Max inputs（Mainnet）：`256`
- Weight budget（Mainnet）：`200,000`
- Tax ratio：`30%`
- Dust threshold：`546 sat`

### 3.5 Auto-Renew 默认值（当前实现）

- Enabled：`false`
- Interval：`30m`
- Failure backoff：`15m`（可设 `0` 关闭）
- Window：`window_end <= blocks_to_expiry <= window_start`
  - `window_start=52560`
  - `window_end=25920`
- MaxUtxosPerRun：`100`
- MaxFeeRate：`5000 sat/KB`
- MaxRenewAmountPerRun：`0`（unlimited）
- `autorenewamount`：启用时必须 > 0

---

## 4. 共识规则（Consensus Rules，规范性）

**范围**：相对 Bitcoin 的硬分叉规则集。

### 4.1 到期定义（Expiry Definition）

时间基准采用区块高度。定义每年区块数：

`Y = 52,560`（10 分钟目标）

到期距离：

`E = T × Y = 7 × 52,560 = 367,920`

对任一 UTXO：

- `h_exp = h_create + E`
- 当 `h_tip >= h_exp` 时，该 UTXO 进入 expired 状态。

### 4.2 收割许可（Reaping Permission）

到期后，矿工可在区块中加入系统交易花费该 UTXO，无需满足原脚本常规执行路径。该系统交易类型称为 **EUTXO-REAP**。

### 4.3 返还与征税（Refund & Tax）

对每个被收割 UTXO（面值 `v`）：

- `v_ret = floor(ρ × v)`：返还额，回原始 `scriptPubKey`，并重置计时；
- `v_tax = v - v_ret`：计入当块矿工奖励池。

尘埃规则：若 `v_ret < DUST(h)`，不创建返还输出，令 `v_ret=0, v_tax=v`。

### 4.4 Coinbase 限值与成熟期

令：

- `BaseSubsidy(h)`：区块补贴
- `Fees(h)`：手续费总和
- `ReapTax(h)`：本块所有 REAP 的税额总和

则 coinbase 总额上限：

`BaseSubsidy(h) + Fees(h) + ReapTax(h)`

`ReapTax` 与 coinbase 同步适用成熟期（如 100 块）后可花费。

### 4.5 确定性选择与节流

V0 设计要求每块受 `N` 与 `R` 双上限约束，并按全局顺序选取到期 UTXO：

- V0 提案顺序：`(create_height asc, txid asc, vout asc)`；
- 在仍有容量时跳过应选项去选后项，区块应视为无效（反择优约束）。

当前实现在工程上把“确定性 + 资源上限”具体化为：

- 确定性排序（落地到 canonical order 约束链路）；
- 输入上限（`ReapMaxInputs`）；
- 交易权重预算（weight budget）；
- marker 一致性校验（高度/计数/摘要）。

### 4.6 REAP Hardening（当前实现增强）

在 `REAP Hardening` 激活后，REAP 额外满足：

1. 输入顺序必须是 canonical order（`expiry -> amount -> outpoint`）；
2. 输入数量不得超过 `ReapMaxInputs`。

### 4.7 REAP Marker 绑定校验

REAP marker 形如：

`REAP:<height>:<count>:<digest>`

共识校验以下一致性：

- marker 高度与区块上下文一致；
- marker count 与输入个数一致；
- marker digest 与输入序列摘要一致。

### 4.8 Expiry 生效后的花费约束（当前实现）

`Expiry Enable` 后：

- 非 REAP 交易花费 expired UTXO -> reject
- REAP 交易花费 non-expired UTXO -> reject

### 4.9 脚本中立

到期后脚本类型不构成特权：多签、P2SH、P2TR、时间锁等在到期语义下同等处理。

### 4.10 Replay Protection（规范与实现）

为避免跨链重放，采用双层防线：

1. Namespace isolation（地址/端口/HD/coin type）；
2. Replay-protected sighash domain（可理解为 `chain_id` 风格的签名域隔离）。

当前实现中：

- replay bit：`0x40`
- domain tag：
  - `OBTC/SigHashV0/v1`
  - `OBTC/SigHashV1/v1`
  - `OBTC/TapSighash/v1`
- 激活门控：
  - 激活前：兼容路径可用
  - 激活后：不满足 replay-protected 语义的签名将失败
- 路径覆盖：Legacy / SegWit v0 / Taproot（key path + script path）

---

## 5. 数据结构与节点行为（Node Behavior）

### 5.1 Expiry Index

全节点维护到期索引，核心映射可抽象为：

1. `OutPoint -> ExpiryKey`
2. `ExpiryKey -> OutPoint set`

连接区块时：

- 新输出写入对应到期桶；
- 被花费输入从索引移除。

### 5.2 Connect/Disconnect 与 Reorg

- Connect：正向更新索引；
- Disconnect：逆向回滚索引；

以保证 reorg 下索引与主链状态一致。

### 5.3 分页扫描与续扫

扫描接口可支持 `(fromKey, toKey, maxResults, startAfter)` 语义，避免全量拉取导致的大事务与大内存占用。

### 5.4 剪枝与归档

- 可剪枝全节点：保留完整 UTXO + 最近滚动窗口区块（及全区块头）；
- 归档节点：保留全历史区块与交易体。

不变量：历史不可改写；剪枝仅改变存储保留策略。

### 5.5 UTXO 快照承诺

可周期发布 UTXO commitment（Merkle/Verkle/Utreexo 风格），新节点可通过“快照 + 证明 + 多源一致性校验”加速同步并降低单一信任源风险。

---

## 6. REAP 交易结构与挖矿执行（Mining & Policy）

### 6.1 出块模板流程

合规矿工可按以下步骤组块：

1. 选择常规交易；
2. 按确定性规则构造 REAP 候选并受上限约束；
3. 计算 `ReapTax` 并应用 coinbase 上限。

当前实现采用“预评估 + 条件预留 + 追加校验”的两阶段模板策略：

- 先判断 REAP 是否可构建；
- 可构建才预留 REAP weight；
- 常规交易选完后再尝试追加 REAP，并做输入可行性复检。

### 6.2 Mempool 策略

REAP 被定义为 block-internal system transaction，mempool 不接收。

### 6.3 EUTXO-REAP 编码（设计与实现对齐）

V0 设计草图中给出以下协议特征：

- `nVersion = 3`
- `nType = REAP`
- 输入为到期 outpoints
- 输出为返还输出 + marker 输出
- 输入不依赖常规 `scriptsig/witness` 路径
- `locktime/sequence` 固定化（提案中为 0）
- REAP 交易重量纳入区块 weight 计量，并受 `N/R` 上限约束

当前实现中可观察到的关键特征与之保持一致：

- 交易版本使用 `version=3`
- 末尾含 marker 输出（`OP_RETURN`）
- refund 按 script 聚合，tax 通过 coinbase accounting 体现
- REAP 不走常规 mempool relay，而是区块模板内系统追加路径

### 6.4 取整与金额守恒

- `v_ret = floor(ρ × v)`
- `v_tax = v - v_ret`

并维持守恒不变量：

`sum(inputs) = sum(refunds) + sum(tax)`

### 6.5 资源边界控制

通过以下组合边界降低资源攻击面：

- 输入数上限
- 交易权重预算
- 确定性排序
- marker 一致性校验

---

## 7. 钱包与交互体验（Wallet & UX）

### 7.1 核心能力

#### `obtc.getexpiry`

面向用户和自动化流程输出到期风险字段：

- outpoint / amount
- create_height / expiry_height
- blocks_to_expiry / days_to_expiry
- status（ok/expiring/expired）
- dust_risk

#### `obtc.renew`

支持显式 outpoint 续期，允许设置：

- amount
- target address（可选）
- max fee rate（可选）
- minconf（可选）

返回 tx 摘要信息（txid、输入输出计数、费率等）。

#### `renewall`

支持：

- status 或窗口过滤
- dry-run
- interval/runs 调度

### 7.2 Auto-Renew（当前实现）

调度器具备：

- 周期执行（Interval）
- 窗口筛选（WindowStart / WindowEnd）
- 每轮候选数上限（MaxUtxosPerRun）
- 费率上限（MaxFeeRate）
- 两项关键硬化：
  1. Failure backoff（失败后延迟下一轮）
  2. Per-run budget（单轮续期总额上限）

与 V0 的 UX 建议保持一致的方向包括：

- 在到期前约 **6–12 个月**窗口内错峰执行，避免集中续期；
- 在可接受费率阈值下触发续期，降低长期维护成本。

### 7.3 地址与脚本策略

共识层返还到原脚本；钱包可在返还后主动转发到新脚本，降低长期可链接性。

### 7.4 费率策略

续期交易可结合费率阈值与低费窗口执行，以降低长期维护成本。

---

## 8. DUST 机制（Dust Threshold）

V0 给出的动态阈值定义：

\[
DUST(h)=\alpha\,\tilde f(h)\,v_{in},\quad \alpha\in[1.0,3.0]
\]

其中：

- `\tilde f(h)`：近 30 天中位费率（sat/vB）
- `v_in`：代表性输入虚拟大小
- 建议 `α=2`

并建议按 2016 块周期机械更新，抑制波动。

当前实现中的 REAP 默认 `dust threshold` 为 `546 sat`。动态化可作为后续演进方向。

---

## 9. 场景矩阵（Behavior Matrix）

### 9.1 Normal

- 到期预警 -> 续期执行 -> 风险下降
- 批量续期在窗口内平滑执行

### 9.2 Boundary

- 激活高度前后切换
- 输入上限与 weight 上限边界
- 预算整除/不整除续期金额

### 9.3 Extreme

- 高频 reorg
- chain client 暂时滞后
- mempool 逼近 block weight 上限
- 大量 dust-like UTXO

### 9.4 Adversarial

- 伪造 marker
- 重排输入
- 构造超大输入集
- 跨链 replay 尝试
- 向 mempool 注入伪系统交易

预期拦截点由 marker 校验、canonical order、输入/权重上限、replay-protected sighash 与 mempool policy 协同覆盖。

---

## 10. 经济学与博弈（Economics & Game Theory）

### 10.1 安全预算

长期近似：

`B ≈ L × (-lnρ/T)`

在 `ρ=0.7, T=7, L=25%` 时，`B≈1.27%/年`。

### 10.2 合规激励

当续期成本占比 `φ` 显著低于税率 `τ=30%` 时，理性用户倾向续期。大额 UTXO 通常满足该条件；小额 UTXO 则受 DUST 机制约束。

### 10.3 MEV 抑制

确定性顺序 + 上限约束抑制矿工“择优收割”空间，降低收益方差与策略噪声。

---

## 11. 隐私考量（Privacy）

- 返还到同脚本可能形成可链接循环；
- 钱包应默认在可接受费率下错峰续期并优先新脚本承接；
- 快照同步应采用多源承诺校验，降低单点信任暴露。

---

## 12. 安全考量与攻防（Security）

- 到期集合爆发导致资源挤压：由输入上限与 weight 预算控制；
- 矿工择优：确定性顺序约束；
- 时间操纵：高度计时避免 MTP 偏移问题；
- 跨链重放：namespace + replay-protected sighash 双层防线；
- 快照投毒：要求多源校验，建议至少 `k` 份独立来源一致后再信任导入；
- 重组套利：`ReapTax` 服从 coinbase 成熟期约束。

---

## 13. 治理与升级（Governance）

V0 设计提出：

- 硬编码核心常量：`T=7, ρ=0.70, Y=52,560, maturity=100` 等；
- 可调参数（延迟生效）：`N, R, α`；
- 建议升级流程：
  - 一个完整窗口内达成约定信号覆盖（例如 2/3）；
  - 预留宽限期（例如 60 天）；
  - 固定激活高度。

原则：减少自由裁量面，降低治理不确定性。

---

## 14. 实施与运维（Implementation & Operations）

### 14.1 MVP 路线（继承 V0）

1. 共识层：Expiry + REAP + coinbase accounting + replay protection
2. 节点层：剪枝模式/归档模式 + 快照导入导出
3. 钱包层：到期显示、手动续期、自动续期、尘埃预警
4. 挖矿层：模板支持 + REAP 顺序校验 + 税额观测
5. 工具层：公共看板、统计、回归工具
6. 测试网络：采用加速到期参数（示例：`T≈7天`）做全链路演练，并补充 REAP 排序/边界/对抗测试

### 14.2 运行期观测指标（实现对齐）

#### Consensus

- expired spend rejection count
- REAP non-expired spend rejection count
- replay-protection violation count

#### Mining

- template build attempts with REAP
- REAP append success rate
- reserved weight utilization
- REAP tax contributed to coinbase

#### Wallet

- auto-renew candidate count / run
- auto-renew success/failure ratio
- backoff activated count
- per-run budget truncation count

### 14.3 告警建议

- 连续 N 轮 auto-renew failure
- REAP append success 连续低于阈值
- expiring backlog 持续上升

---

## 15. 关键 KPI（KPI）

- `ReapTax / MinerRevenue`（年化）
- 在险供给（≤90 天到期）
- 续期成功率
- 每块 REAP 数量均值与分位波动
- 归档节点数量与分布
- 自动续期渗透率与投诉率

---

## 16. 数学附录（Mathematical Appendix）

- 永久不动币年化衰减：
  `p = -lnρ / T`
- 安全流入：
  `B = L × p`
- 等预算变换：
  `ρ' = exp(-pT')`
- 目标预算反解：
  `ρ = exp(-(B_target × T)/L)`，`τ = 1 - ρ`

---

## 17. 术语表（Glossary）

- **Expiry**：UTXO 到期机制。  
- **Expired UTXO**：达到到期高度的 UTXO。  
- **EUTXO-REAP**：系统回收交易类型。  
- **ReapTax**：区块内 REAP 税额总和。  
- **OutPoint**：`txid:vout`，UTXO 唯一定位符。  
- **Canonical order**：确定性输入排序规则。  
- **Replay protection**：签名域隔离防重放机制。  
- **Domain separation**：用独立 tag/语义隔离签名消息空间。  
- **Namespace isolation**：链参数与地址/密钥命名空间隔离。  
- **Auto-Renew**：钱包后台自动续期流程。  
- **Failure backoff**：失败后延迟重试机制。  
- **Per-run budget**：单轮续期总额上限机制。  
- **Mempool**：未确认交易池。  
- **Template**：矿工组块候选模板。  
- **Coinbase maturity**：coinbase 可花费成熟期。  
- **Pruned full node**：可剪枝全节点，保留滚动窗口区块体与完整 UTXO 集。  
- **Archive node**：归档节点，保留全历史区块与交易体。  
- **Dust fold**：当返还额低于尘埃阈值时并入 tax，不创建返还输出。  
- **ListBatchLimit**：到期扫描单页上限，用于控制查询内存与时延。

---

## 18. 法务与合规提示

OBTC 是独立链，不等同于 Bitcoin。到期与回收机制在不同法域可能被解释为协议费、负利率或对沉睡资产的再分配。交易所与托管机构需据此设计内部流程与用户告知文本。

---

## 19. 许可

本文档建议采用 **CC BY 4.0**；参考实现可采用 MIT 等宽松许可。

---

*白皮书 v1.0（实现对齐融合版）完*  
