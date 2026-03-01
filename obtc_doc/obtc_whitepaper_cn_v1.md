# 有机比特币（Organic Bitcoin, OBTC）白皮书 V1（实现对齐版）

**版本**：1.0（实现对齐草案）  
**状态**：征求意见稿（RFC）  
**读者**：协议工程师、钱包开发者、矿池、交易所、研究人员、运维团队

---

## 版本说明

本文件满足两条约束：

1. **完整保留 V0 全文**（不删除、不替换原始叙述）。
2. 在 V0 基础上新增 **V1 实现对齐补充**，将 2025-10 至今已经落地的规则、参数与行为补齐到白皮书叙事中。

阅读建议：

- 如果你第一次阅读 OBTC：先看 **Part A（V0 全文）**，再看 **Part B（V1 补充）**。  
- 如果你关注“当前实现实际怎么跑”：直接看 **Part B**。  

---

# Part A — V0 全文（原文完整保留）

# 有机比特币（Organic Bitcoin, OBTC）：一条“会代谢”的货币网络

**版本**：0.1（草案）  
**状态**：征求意见稿（RFC）  
**读者**：协议工程师、钱包开发者、矿池、交易所、研究人员

---

## 摘要（Abstract）
有机比特币（OBTC）是一条从 Bitcoin 分叉而来的区块链，它把币视作**有生命的 UTXO**：必须定期移动以保持有效。每个 UTXO 拥有固定**到期时限 T = 7 年**（以区块高度近似计时）。当 UTXO 到期后，协议允许矿工通过**系统交易**对其进行**收割（reap）**：UTXO 面值的 **30%** 计入本区块的矿工奖励（安全预算），**70%** **返还至该 UTXO 的原始锁定脚本**，并**重置新的 7 年计时**。OBTC 进一步鼓励剪枝：普通全节点仅需保留最近约 7 年的区块，而**归档节点**保存完整历史。目标是：（i）将永久丢失的币渐进式回收为安全预算；（ii）强制性地促进币的**周期性流动（有机代谢）**；（iii）在保持可验证性的前提下降低节点存储负担。

---

## 1. 理念与动机（Philosophy & Rationale）

### 1.1 有机货币观
传统 Bitcoin 把**“不动”**当优点：币可以永远静止。但现实中的长久静止常常意味着**熵增**——遗失密钥、失联所有者、被遗忘的输出。OBTC 倒向相反方向：把长期不动视作**“死亡供给”**，通过**代谢机制**把其逐步回收用于**持续的网络安全**。对活跃用户而言，规则简单：**到期前挪一挪**。

### 1.2 设计目标
- **持续的安全预算**：把死亡/沉睡供给转化为矿工收入，缓释减半后的安全焦虑。
- **活动卫生**：强制定期 UTXO 整理与地址轮换，减少垃圾状态。
- **运行轻量**：允许 7 年滚动剪枝 + 可验证的 UTXO 快照，加速初始同步。
- **可预期性**：简洁、确定性强的规则，尽量少的治理面。
- **脚本中立**：到期后任何脚本一视同仁。

### 1.3 非目标
- 迎合无限期冷存与超长期锁定（>7 年）的用例。
- 要求所有节点永久保存全部历史交易数据（由归档节点承担）。

---

## 2. 货币机制（高层）
设**到期年限** \(T=7\) 年，**返还比例** \(\rho=0.70\)（即税率 \(\tau=1-\rho=0.30\)）。对于**永久不再移动**的 UTXO，到期收割会每隔 \(T\) 年对余额施加一次 30% 的几何衰减，等价为该部分供给的**连续年化衰减率**：
\[ p\;=\; -\frac{\ln \rho}{T}\quad(\text{每年}). \]
代入 \(T=7,\rho=0.7\Rightarrow p\approx 5.1\%/\text{年}\)，**仅作用于“永久丢失”**的币。如果丢失供给占比为 \(L\)，则每年回流到矿工安全预算的大致占比：
\[ B\;\approx\; L\cdot p\;=\;L\cdot(-\ln\rho/T). \]
当 \(L\in[20\%,30\%]\) 时，OBTC 约能提供 **~1.0–1.5%/年** 的持续安全预算。

---

## 3. 共识规则变更（规范性）
**范围**：相对 Bitcoin 的**硬分叉**。

### 3.1 到期定义
- **时间基准**：**区块高度**（确定性强）。定义**每年区块数** \(Y=52{,}560\)（10 分钟目标），则**到期距离** \(E=T\cdot Y=7\cdot52{,}560=367{,}920\) 个区块，自每个 UTXO 的**创建高度**起算。
- 当 \(h\ge h_{create}+E\) 时，UTXO 进入**到期（Expired）**状态。

### 3.2 收割许可（Reaping Permission）
- 到期后，矿工可在区块中加入一种**系统交易**，无需满足原脚本即可**花费该到期 UTXO**。此交易类型称为 **EUTXO‑REAP**。

### 3.3 返还与征税
- 对每个被收割的到期 UTXO，记其面值 \(v\)：
  - **返还额**：\(v_{ret}=\lfloor\rho\cdot v\rfloor\)（向下取整至聪），**发送到原始锁定脚本**（完全相同的 `scriptPubKey`），并**重置 7 年计时**（创建高度=当前区块）。
  - **税额**：\(v_{tax}=v-v_{ret}\) 计入本区块的**矿工奖励**。
- **尘埃返还规则**：若 \(v_{ret}<\texttt{DUST}(h)\)，则**不得**创建返还输出，直接令 \(v_{ret}=0\)、\(v_{tax}=v\)（避免尘埃膨胀）。

### 3.4 Coinbase 限值（奖励核算）
- 令 `BaseSubsidy(h)` 为区块补贴，`Fees(h)` 为手续费总和，`ReapTax(h)` 为本块内所有 EUTXO‑REAP 的税额之和。  
- **Coinbase 输出总额不得超过** `BaseSubsidy(h) + Fees(h) + ReapTax(h)`。
- **成熟期**：`ReapTax` 与 coinbase 同步适用成熟期（如 100 块）后方可花费。

### 3.5 确定性选择与节流（反 MEV）
- **每块上限**：每个区块最多可收割 **N** 个到期 UTXO，且 REAP 交易聚合的总重量**不得超过** **R vbytes**。（`N`、`R` 为共识参数。）
- **全局顺序**：所有到期 UTXO 以**(创建高度升序, txid 升序, vout 升序)** 排序。矿工**必须**按序选取，直至达到上限。  
- **禁止择优**：在仍有容量的情况下跳过应选项以选择后项，视为**区块无效**。

### 3.6 脚本中立
- 到期后不再执行脚本验证。多签、P2SH、P2TR、时间锁、类契约结构一视同仁。

### 3.7 重放保护与链标识
- 使用独立的地址前缀/Bech32 HRP 以及签名域隔离（`chain_id` 风格），避免跨链重放。

### 3.8 激活
- **分叉点**：以 Bitcoin 高度 **600,000** 的 UTXO 状态为基线。自 OBTC 创世块起，适用 OBTC 规则。

---

## 4. 数据结构与节点行为

### 4.1 到期索引（Expiry Index）
全节点维护**到期索引**：以**到期高度** \(h_{exp}=h_{create}+E\) 为键的桶式结构。每块连接流程：
1. 对新产生的 UTXO，计算 \(h_{exp}\) 并登记到对应桶；
2. 在当前高度 \(h\) 取出桶 \(h\) 中项，作为本高度新进入“到期”的集合（再按 §3.5 的全局顺序进入候选队列）。  
这避免了“每块回扫历史”的高成本，使 REAP 组装为 O(容量)。

### 4.2 剪枝与归档
- **可剪枝全节点**：必须持有完整 UTXO 集与最近**约 7 年**的区块及全部区块头。更早区块可剪枝（不再保留交易体）。
- **归档节点**：保存**全部历史**区块与交易。
- **不变式**：历史不可改写；剪枝只是不再**存储**旧区块体，区块头链及其默克尔承诺不被更改。

### 4.3 UTXO 快照承诺（快速同步）
定期（如每月）发布**UTXO 集合承诺**（Merkle/Verkle/Utreexo 风格）。新节点可：
1. 下载最近快照（状态 + 证明）；
2. 校验多个独立来源的承诺（多源一致性）；
3. 自该快照高度向前同步。  
从而降低初始同步成本并弱化单一来源信任。

---

## 5. 内存池与挖矿策略

### 5.1 出块模板
合规矿工按如下步骤组块：
1. 按费率选取普通交易；
2. 根据 §3.5 的全局顺序确定可收割的到期集合，加入 **EUTXO‑REAP**，直至达到 \(N,R\) 上限；
3. 计算 `ReapTax` 并应用 coinbase 限值规则。

### 5.2 EUTXO‑REAP 编码（线协议草图）
系统交易具备：
- `nVersion = 3`（保留），`nType = REAP`；
- **Inputs**：一组到期 UTXO 的 outpoint（`txid,vout`）。**无 scriptsig/witness**。每个输入以“到期且符合 §3.5 选择规则”为有效性条件；
- **Outputs**：对每个输入，若 \(v_{ret}\ge\texttt{DUST}\) 则创建一个回原 `scriptPubKey` 的输出；若不足则不建返还输出。**不**在此交易中显式创建矿工税收输出（税额体现在 coinbase 限值中）；
- **Locktime/sequence**：固定为 0；
- **重量计费**：按常规计入区块重量；通过 \(N,R\) 上限抑制 DOS。

### 5.3 取整规则
`v_ret = floor(rho * v)`（单位聪）。余数计入 `v_tax` 以保持守恒。

---

## 6. 钱包与交互体验（UX）

### 6.1 核心能力
- **到期感知**：对每个 UTXO 显示倒计时；距到期 < 180 天加重提示；
- **一键续期**：合并并发送到**新地址/新脚本**（默认启用隐私卫生），重置 7 年计时；
- **自动续期服务**（可选）：用户设定最大费率，钱包在**到期前 6–12 个月**的随机窗口内自动续期；
- **尘埃预警**：当预测 70% 返还将低于 DUST 时明确提示。

### 6.2 地址/脚本策略
共识层**返还到原脚本**。钱包**应**在返还入账后**自动转发**至**新脚本**，以降低链上关联性（可与续期合并为一笔）。

### 6.3 费率优化
续期应优先选择历史低费时段；钱包可设“当 feerate < 阈值即执行”的触发器。

---

## 7. 动态尘埃阈值（DUST）

### 7.1 定义
记 \(\tilde f(h)\) 为至高度 \(h\) 的**近 30 天中位费率**（sat/vB），记代表性输入虚拟大小 \(v_{in}\)（如 P2TR keypath ~68 vB）。定义：
\[\texttt{DUST}(h)=\alpha\,\tilde f(h)\,v_{in},\quad \alpha\in[1.0,3.0].\]
含义：能否以典型费用合理花费。**建议**：\(\alpha=2\)。

### 7.2 共识 vs 策略
`DUST(h)` 为**共识参数**，由链上费率中位数**机械更新**，更新周期为**2016 块**（一次难度周期），以降低波动与治理争议。

---

## 8. 经济学与博弈

### 8.1 安全预算
来自丢失币的年化安全流入：\(B\approx L\cdot(-\ln\rho/T)\)。当 \(\rho=0.7,T=7,L=25\%\Rightarrow B\approx1.27\%/\text{年}\)。

### 8.2 合规激励
理性用户在**续期费用/UTXO 价值**之比 \(\varphi\) **小于** 税率 \(\tau=30\%\) 时会选择续期。对大多数 UTXO，\(\varphi\ll30\%\)，故合规率高；对极小额输出，DUST 规则避免无意义返还与链上污染。

### 8.3 MEV 抑制
确定性顺序 + 每块上限剥夺矿工择优空间，抑制“只收割肥美 UTXO”的竞赛，也降低 `ReapTax` 的方差与孤块风险。

---

## 9. 隐私考量
- 返还到**同一脚本**可能形成可链接循环；钱包应默认**立即转发**至新脚本。
- 续期应避免大规模、同日同源的聚合；在**到期前 6–12 个月**内随机错峰。
- 快照同步应多源校验，避免单一信任点暴露。

---

## 10. 安全考量与攻防
- **到期集合爆发引发 DOS**：以 \(N,R\) 上限、权重计费与桶式索引缓解；
- **矿工择优**：违反确定性顺序的区块视为无效；
- **时间操纵**：采用高度计时避免 MTP 偏移；
- **跨链重放**：地址前缀/HRP 与签名域隔离；
- **快照投毒**：要求多方承诺、至少 \(k\) 份独立验证；
- **重组安全**：`ReapTax` 与 coinbase 共用成熟期，避免即时抛压与重组套利。

---

## 11. 治理与升级
- **硬编码常量**：\(T=7\) 年；\(\rho=0.70\)；DUST 公式的 \(\alpha=2\)；coinbase 成熟期 100 块；每年区块数 \(Y=52{,}560\)。
- **可调（延迟生效）**：每块 REAP 个数上限 \(N\)、REAP 总重量上限 \(R\)、DUST 的 \(\alpha\)。任何修改需：  
  (i) 客户端 2/3 信号覆盖一个完整难度周期；  
  (ii) 60 天宽限期；  
  (iii) 固定激活高度。  
- 无链上金库与自由裁量委员会。

---

## 12. 实现计划（MVP）
1. **共识层**：实现 EUTXO‑REAP、到期索引、确定性排序、DUST 规则、coinbase 限值；
2. **节点**：7 年剪枝模式 + 归档模式；UTXO 快照导入/导出与承诺校验；
3. **钱包**：到期 UI、一键/自动续期、尘埃预警、返还后默认转发；
4. **挖矿**：出块模板支持；矿池校验 REAP 顺序；`ReapTax` 观测指标；
5. **工具**：公共看板——在险供给（≤90 天）、续期率、`ReapTax` 占比、归档节点分布；
6. **测试网**：加速到期（如 \(T=7\) 天）全链路演练；对 REAP 排序做对抗测试。

---

## 13. 运行指标（KPI）
- `ReapTax / MinerRevenue`（年化）
- 在险供给（≤90 天到期）
- 续期成功率
- 每块平均/95 分位 REAP 数与方差
- 归档节点数量与地域分布
- 钱包自动续期渗透率与投诉率

---

## 14. 参量化附录（数学）
- **永久不动币的年化衰减**：\(p=-\ln\rho/T\)。
- **安全流入**：\(B=L\cdot p\)。
- **等预算变换**：保持 \(p\) 不变，从 \((T,\rho)\) 变到 \((T',\rho')\)：\(\rho'=\exp(-pT')\)。
- **按目标预算反解**：给定 \(B_{target},L,T\)：\(\rho=\exp\!\big(-\tfrac{B_{target}T}{L}\big)\)，税率 \(\tau=1-\rho\)。

---

## 15. 术语表
- **到期（Expiry）**：UTXO 达到 \(T\) 年龄（以区块高度近似），自此可被系统交易收割。
- **EUTXO‑REAP**：系统交易类型；收割到期 UTXO，将 \(\rho\) 返还原脚本、余值计入矿工奖励。
- **`ReapTax`**：区块内收割税额之和，计入 coinbase 限值。
- **可剪枝全节点**：保留最近约 7 年区块 + 全量 UTXO 集。
- **归档节点**：保留全历史区块与交易。

---

## 16. 法务与合规提示
OBTC 是一条**独立链**，并非 Bitcoin。到期回收在不同法域可能被理解为协议费、负利率或对“丢失币”的再分配。交易所与托管机构应据此设计内部政策与客户告知。

---

## 17. 许可
本文档遵循 **CC BY 4.0** 许可；参考实现推荐采用 MIT 许可。

---

*白皮书 v0.1 完*


---

# Part B — V1 实现对齐补充（2025-10 至今）

> 本部分用于把已落地实现与白皮书叙事对齐。  
> 若 Part A 与 Part B 在“当前参数值”上存在差异，以 Part B 的“当前实现参数表”为准。

---

## B1. 目标与边界

V1 补充不改变 OBTC 的核心哲学：

- UTXO 具有时间生命周期；
- 到期资产通过系统路径回收；
- 回收价值在安全预算与返还之间按固定比例分配。

V1 主要做三件事：

1. 把激活时序从概念表述升级为**网络可执行参数**；
2. 把 REAP 与 Replay Protection 从“规则描述”升级为**明确验证行为**；
3. 把 Wallet 侧从“应当如何”升级为**已实现能力与运行约束**。

---

## B2. 当前实现参数与激活时序

## B2.1 三网激活矩阵（当前实现）

| 网络 | Fork Height | Expiry Index Start | Expiry Enable | REAP Hardening | Replay Protection |
|---|---:|---:|---:|---:|---:|
| Mainnet | 950000 | 950000 | 1050000 | 1060000 | 1065000 |
| Testnet | 2800000 | 2800000 | 2800100 | 2800120 | 2800130 |
| Regtest | 100 | 100 | 110 | 112 | 114 |

解释：

- `Fork Height`：链身份分歧点。  
- `Expiry Enable`：普通交易与 REAP 的到期约束开始共识生效。  
- `REAP Hardening`：REAP 输入顺序与输入上限变为强制共识约束。  
- `Replay Protection`：签名域隔离语义进入强制校验。

## B2.2 Expiry 参数（当前实现）

| 网络 | WindowBlocks | ListBatchLimit | ReapMaxInputs |
|---|---:|---:|---:|
| Mainnet | 3,679,200 | 10,000 | 256 |
| Testnet | 1,008 | 5,000 | 500 |
| Regtest | 144 | 1,000 | 200 |

说明：

- Mainnet `WindowBlocks = 3,679,200`（约 7 年）与 V0 核心设计一致。  
- Testnet/Regtest 使用更短窗口用于验证与回归测试加速。

## B2.3 命名空间隔离（当前实现）

OBTC 已在以下域与 Bitcoin 做硬隔离：

- Network magic
- Default port
- Bech32 HRP
- 地址前缀（P2PKH/P2SH/Witness）
- HD 扩展键版本（pub/prv）
- BIP44 coin type

并在启动期执行冲突检测：若 OBTC 命名空间与 BTC 网络发生碰撞，节点会拒绝启动。

---

## B3. 共识规则的实现级细化

## B3.1 Expiry 约束的执行行为

当高度达到 `Expiry Enable` 后：

1. 非 REAP 交易若花费 expired UTXO -> `reject`
2. REAP 交易若花费 non-expired UTXO -> `reject`

这使“到期资产处理路径”被强制收敛到系统交易，不依赖矿工或钱包自觉。

## B3.2 REAP Marker 绑定校验

当前实现对 REAP marker 执行三重一致性检查：

- marker 里的 `height` 与区块上下文一致；
- marker 里的 `count` 与输入个数一致；
- marker 里的 `digest` 与输入序列摘要一致。

任何一项不一致都导致交易无效。

## B3.3 REAP Hardening（顺序与上限）

当高度达到 `REAP Hardening` 后，REAP 必须满足：

1. 输入按 canonical order 排列：`expiry -> amount -> outpoint`
2. 输入数量不超过 `ReapMaxInputs`

意义：

- 避免不同实现出现排序歧义；
- 限制单交易复杂度，降低 DoS 面。

## B3.4 REAP 与脚本执行边界

REAP 作为系统交易走专门共识规则路径，不沿用普通交易脚本执行逻辑；普通交易仍走标准脚本验证路径。边界清晰可降低规则互相污染风险。

---

## B4. REAP 从候选到入块的完整流水线

## B4.1 Candidate Selection

当前实现以 Expiry Index 为入口，扫描 `expiry <= tip` 候选，随后：

1. 用 UTXO 视图剔除已花费或无效项；
2. 对候选做确定性排序；
3. 在 `MaxInputs + WeightBudget` 双约束下截断；
4. 形成可构建 plan。

## B4.2 Tax / Refund / Dust

对输入金额 `v`：

- `tax = floor(v * 30 / 100)`
- `refund = v - tax`

若 `0 < refund < dust_threshold`：

- 不创建 refund 输出；
- 该 refund 折叠进 tax（Dust fold）。

## B4.3 Blueprint 结构

REAP blueprint 的当前实现特征：

- `version = 3`
- 输入：expired outpoints
- 输出：
  - 按 script 聚合后的 refund 输出
  - `OP_RETURN` marker（value = 0）

金额不变量：

`sum(inputs) = sum(refunds) + sum(tax)`

## B4.4 Template 组块策略

当前实现采用两阶段策略：

1. 先评估 REAP 是否可构建；
2. 仅在可构建时为 REAP 预留 weight 头寸；
3. 常规交易选择完成后尝试追加 REAP，并重新做输入可行性检查。

若 REAP 最终追加失败，不影响常规模板有效性。

## B4.5 Mempool 策略

REAP 被定义为 block-internal system transaction，mempool 明确拒收。

---

## B5. Replay Protection 的实现级细化

## B5.1 双层防线

1. Namespace isolation（地址/端口/HD/coin type）
2. Replay-protected sighash domain

## B5.2 Replay-protected sighash 标记

当前实现使用专用 sighash bit（`0x40`）和专用 domain tag：

- `OBTC/SigHashV0/v1`
- `OBTC/SigHashV1/v1`
- `OBTC/TapSighash/v1`

## B5.3 激活门控

Replay protection 不是“全时强制”，而是按网络高度激活：

- 激活前：保留兼容路径；
- 激活后：缺失 replay-protected 语义的签名将验证失败。

## B5.4 三类签名路径

- Legacy：使用 OBTC 域分离标签生成签名消息；
- SegWit v0：同样启用域分离；
- Taproot（key/script path）：激活后要求 sighash type 满足 replay-protected 语义。

---

## B6. Wallet 能力闭环（当前实现）

## B6.1 到期查询：`obtc.getexpiry`

返回典型字段：

- outpoint
- amount
- create/expiry height
- blocks_to_expiry
- days_to_expiry
- status（ok/expiring/expired）
- dust_risk

## B6.2 手动续期：`obtc.renew`

支持：

- 显式指定 outpoints
- 指定续期金额
- 可选目标地址、最大费率、最小确认数

返回：txid、输入输出计数、费率、目标地址等摘要。

## B6.3 批量工具：`renewall`

支持：

- 状态筛选或窗口筛选
- dry-run
- interval/runs 调度执行

## B6.4 Auto-Renew Scheduler（进程内）

当前实现具备：

- 周期调度（Interval）
- 候选窗口过滤（WindowStart/WindowEnd）
- 每轮 UTXO 数量上限（MaxUtxosPerRun）
- 费率上限（MaxFeeRate）
- 两个关键硬化：
  1. **Failure Backoff**：单轮存在失败即延后下一轮
  2. **Per-Run Budget**：限制单轮续期总额（MaxRenewAmountPerRun）

---

## B7. 场景矩阵与运行期行为

## B7.1 Normal

- 到期预警 -> 手动/自动续期 -> 风险下降
- 批量续期在窗口内平滑执行

## B7.2 Boundary

- 激活边界高度切换
- 预算整除/不整除续期金额
- 输入数/weight 命中上限边界

## B7.3 Extreme

- 高频 reorg
- 节点链高暂时不一致
- mempool 逼近 block weight 上限
- 大量 dust-like UTXO

## B7.4 Adversarial

- 伪造 marker
- 重排输入
- 超上限输入集
- 跨链 replay 尝试
- 向 mempool 注入伪系统交易

拦截点分别由 marker 校验、canonical order、input cap、replay-protected sighash、mempool policy 覆盖。

---

## B8. 运行指标与运维建议（实现对齐）

建议最少维护以下指标：

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

告警建议：

- 连续 N 轮 auto-renew 失败
- REAP append success 连续低于阈值
- expiring backlog 持续上升

---

## B9. 术语对照（增补）

| Term | 中文解释 | 在 OBTC 中的作用 |
|---|---|---|
| Expiry | 到期机制 | 为 UTXO 引入生命周期 |
| Expiry Index | 到期索引 | 快速扫描/删除到期候选 |
| REAP | 系统回收机制 | 处理 expired UTXO |
| Marker | REAP 标记输出 | 绑定高度、输入数、摘要 |
| Canonical Order | 规范顺序 | 防歧义、防实现分叉 |
| Replay Protection | 回放保护 | 阻断跨链重放 |
| Domain Separation | 签名域隔离 | 让不同链签名语义不可互换 |
| Auto-Renew | 自动续期 | 钱包后台风险缓释执行器 |
| Failure Backoff | 失败退避 | 避免失败风暴 |
| Per-Run Budget | 单轮预算 | 限制单轮续期总额 |

---

## B10. V1 结语

V1 的意义不是重新定义 OBTC，而是把 V0 的理念与当前落地实现对齐：

- 理念层：保持不变；
- 规则层：更明确、可验证；
- 执行层：形成 Consensus-Mining-Wallet 闭环；
- 运维层：具备可观测与可告警基础。

下一步文档工程重点不在增加概念，而在持续维护“规范、实现、运营数据”的一致性。

---

*白皮书 v1.0（实现对齐版）完*  
