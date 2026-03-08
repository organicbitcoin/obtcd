# OBTC × AI 战略方向（草案）

> 目标：把“OBTC 是为 AI agent 设计的货币”从一句叙事，整理成可执行的产品方向。

---

## 1. 核心判断

OBTC 最值得做的 AI 方向，不是“让 AI 也能调用钱包”，而是把它做成一种**面向 AI agent 的资金操作系统**。

真正有辨识度的不是聊天入口，也不是“接个大模型”；而是让 OBTC 同时具备这四种能力：

- **可持有**：agent 不依赖银行账户、不依赖 KYC 也能原生持有和转移价值
- **可授权**：不同 agent 拿到的是不同能力，而不是同一个全能钱包
- **可续期**：生命周期资产能被持续监控、自动续期、批量调度
- **可审计**：所有机器动作都有明确的操作 ID、策略依据和责任边界

如果只做“余额、地址、转账”，那只是传统钱包加了一层 AI 外壳。  
如果做到“观察、规划、授权、签名、续期、审计、恢复”这一整套闭环，OBTC 才真的更贴近 AI。

---

## 2. 为什么 OBTC 天然适合 AI

### 2.1 AI agent 需要一种不依赖银行的货币

未来很多 agent 会自己购买：

- 算力和推理服务
- 数据和 API
- 自动化工具和机器人服务
- 外包任务和链上执行能力

但 agent 不能自行完成 KYC，也不能走传统银行开户流程。  
这意味着它们需要一种**无需许可、原生互联网、机器可调用**的结算资产。

### 2.2 OBTC 比传统钱包更像“生命周期资产系统”

BTC 钱包默认假设是：

- 人类持有者偶尔打开钱包
- 看余额
- 发起少量转账

而 OBTC 的默认假设不同：

- 资产有 `expiry -> expiring -> renew / REAP` 生命周期
- 钱包要持续感知风险窗口
- 资产需要策略化续期和批量调度

这种工作模式恰好更适合 AI agent，而不是更适合人类手工操作员。

### 2.3 AI 擅长的，正是 OBTC 需要的

AI agent 的优势是：

- 持续观察
- 按规则执行
- 事件驱动调度
- 多账户一致性管理
- 预算控制和记录留痕

这些能力和 OBTC 的生命周期模型天然吻合。

---

## 3. 北极星定义

建议把 OBTC 的 AI 方向定义为：

> **Lifecycle Money for Autonomous Agents**  
> 一种为自主软件主体设计的、可授权、可续期、可审计的货币系统。

对应到产品形态，北极星不是“AI 钱包 App”，而是：

- 一个共享钱包内核
- 一套面向 agent 的程序接口
- 一套面向人类操作员的 CLI
- 一套可验证的授权、签名、续期、审计模型

换句话说，OBTC 不该拆成“AI 钱包”和“人类钱包”两套系统。  
正确方向是：**一个内核，两类入口，统一语义。**

---

## 4. 产品原则

### 4.1 Capability First

不要再以“全局解锁的钱包”作为默认模型。  
默认模型应该是：

- `renewal-agent` 只能看 expiring UTXO 并请求续期
- `payments-agent` 只能在预算内付款
- `treasury-agent` 只能做资金分配和归集
- `recovery-agent` 只能处理恢复和冻结类操作

也就是说，授权单位不再是 wallet passphrase，而是：

- `capability`
- `policy`
- `session`

### 4.2 Intent First

不要让 agent 直接拼原始交易。  
应该先表达意图，再由钱包系统落地成 plan / PSBT / tx。

典型意图包括：

- `renew_utxos`
- `pay_invoice`
- `allocate_budget`
- `sweep_change`
- `consolidate_dust`

### 4.3 Event Driven

AI 最适合消费事件，而不是反复轮询原始 RPC。  
OBTC 钱包应当提供一等公民级事件流：

- `utxo.expiring`
- `renewal.succeeded`
- `renewal.failed`
- `budget.low`
- `policy.blocked`
- `reap_risk.elevated`

### 4.4 Audit Native

每个 agent 动作都应默认携带：

- `operation_id`
- `idempotency_key`
- `requested_by`
- `capability_id`
- `policy_version`
- `decision_reason`

对 AI 钱包来说，审计不是附加功能，而是核心能力。

### 4.5 One Core, Two Surfaces

人类入口和 AI 入口必须共用同一套核心语义：

- 人类：CLI
- 程序：gRPC / HTTP / PSBT / SDK

不要出现“CLI 能做、AI 不能做”或“AI 有审计、CLI 没审计”的分裂实现。

---

## 5. 最值得做的能力方向

### 5.1 Capability Wallet

把钱包从“一个口令解开全部权限”改成“能力令牌钱包”。  
这是 AI 化最重要的基础设施，没有它，后面的 agent 场景都不稳。

### 5.2 Preview -> Approve -> Sign -> Publish

所有 agent 操作默认先预演，再审批，再签名，再发布。  
这样可以：

- 降低误操作风险
- 强化机器可读错误
- 保证审计可回放
- 让人工审批自然插入流程

### 5.3 Watch-Only Planner + Isolated Signer

推荐拆成两层：

- `planner`：同步链、观察 UTXO、判断 expiry、生成 plan
- `signer`：只负责批准后的签名，不参与复杂业务编排

这样可以天然降低 agent 接触密钥材料的范围。

### 5.4 Agent Subaccounts

不要让多个 agent 共用一个默认账户。  
建议每个 agent 使用独立账户或独立 scope：

- `treasury`
- `renewal-agent`
- `payments-agent`
- `market-agent`
- `recovery-agent`

至少做到：

- 预算隔离
- 地址空间隔离
- capability 隔离
- 审计隔离

### 5.5 Renewal Optimizer

这是 OBTC 最有特色的 AI 能力之一。  
不只是“自动续期”，而是让 agent 自动判断：

- 哪些 UTXO 应优先续期
- 哪些 UTXO 应该顺手合并
- 哪些 dust 值得保留
- 哪些金额可以直接放弃给 REAP

这比“让 AI 发转账”更体现 OBTC 的独特性。

### 5.6 Policy DSL

为钱包定义一套机器可读的策略语言，例如：

```text
max_daily_spend = 10 OBTC
renew_before = 30d
require_human_approval_above = 1 OBTC
forbid_new_counterparty = true
```

这会让 OBTC 从“可转移的币”变成“可治理的资金系统”。

### 5.7 Machine-Readable Invoice / Quote

让 agent 可以直接理解付款请求，而不是依赖人类读文本。  
一个机器可读报价至少应包括：

- 金额
- 过期时间
- 服务类型
- 付款条件
- 退款规则
- 对方收款公钥或地址

这样才可能支撑算力、数据、API 等自动化采购场景。

### 5.8 Agent-to-Agent Escrow

两个 agent 之间应能做条件式结算，而不是只能“先打钱再信任”。  
典型模式：

- A agent 提交任务
- B agent 交付结果摘要 / 签名证明
- 钱包系统按条件释放资金

这会直接打开数据交易、推理服务、自动化外包等场景。

---

## 6. 高价值场景

### 6.1 算力与推理服务结算

最直观的 AI 原生场景。  
agent 需要自己购买：

- GPU 时间
- 推理调用额度
- 模型托管资源

OBTC 在这里适合作为小额、高频、机器可触发的结算层。

### 6.2 数据与 API 采购

agent 不只是买算力，也会买：

- 新闻数据
- 结构化数据库查询
- 私有 API 调用额度
- 插件或工具使用权

机器可读 invoice + 小额支付 + 自动审计，是这个场景的关键。

### 6.3 自动续期运维

OBTC 自身的生命周期机制就适合长出一个新市场：

- 钱包托管方
- 自动续期服务商
- 企业 treasury 运维机器人
- 批量 wallet maintenance agent

这是其他大多数币没有的内生 AI 运维场景。

### 6.4 Treasury Agent

企业或 DAO 可以让多个 agent 分工管理资金：

- 收入归集
- 周期预算分配
- 续期保障
- 费用结算
- 风险冻结

OBTC 的生命周期模型会迫使 treasury 变得更加主动和自动化。

### 6.5 Task-Bound Money

未来可以给 agent 发放“任务型资金”，而不是无限制转账能力。  
例如：

- 这笔钱只能用于 API 调用
- 只能在 30 天内花完
- 超过预算自动冻结
- 任务结束 capability 自动失效

这非常适合临时 worker agent 或项目型 agent。

---

## 7. 路线图建议

### P0：先把 AI 基础设施打透

P0 目标不是做复杂生态，而是先把钱包能力模型立住。

- 正式化 `AgentWalletService`
- CLI 尽量走统一程序接口
- 引入 `capability` 和 agent 账户隔离
- 补齐 `preview -> approve -> sign -> publish`
- 建立统一 operation / audit 模型
- 输出机器可读错误码与失败原因
- 把 expiry 风险做成结构化查询和事件流

### P1：把 AI 路线从“能用”推进到“好用”

- 增加 reservation / lease 机制，避免多 agent 抢同一 UTXO
- 增加 policy DSL
- 支持 watch-only planner + remote signer
- 增加 renewal optimizer
- 增加机器可读 invoice / quote
- 增加 task-bound subwallet / subaccount

### P2：把单钱包扩展成 agent 经济基础设施

- agent-to-agent escrow
- 第三方 renewal service 市场
- 基于签名历史的 agent reputation
- 标准化服务交付凭证和结算证明
- 面向企业 treasury 的多 agent 协同框架

---

## 8. 不建议优先做的事

- 不要急着把 AI 逻辑写进共识层
- 不要做“把 prompt 或模型输出上链”的噱头
- 不要先做花哨 GUI，再补核心授权和审计
- 不要让 agent 直接保存 seed、mnemonic 或长期口令
- 不要把项目分裂成“给 AI 的钱包”和“给人类的钱包”两套系统

这些方向要么风险高，要么辨识度低，要么会稀释真正的产品重点。

---

## 9. 对外叙事建议

如果要把 OBTC 的 AI 方向讲清楚，建议统一使用下面这类表达：

- **Bank accounts are for humans. OBTC is for software agents.**
- **BTC 适合存储价值，OBTC 更适合管理会行动的资金。**
- **OBTC 不是把 AI 接进钱包，而是把钱包重做成 agent 的资金操作系统。**
- **OBTC 是 lifecycle money，不是静态沉睡的钱。**

核心不是“AI 很火，所以蹭 AI”。  
核心是：**OBTC 的生命周期模型，本来就比传统加密货币更适合机器经济。**

---

## 10. 与现有文档的关系

这份文档定位是**战略方向稿**，回答“为什么做、优先做什么、哪些值得押注”。

相关文档分工如下：

- `obtc_innovation_analysis_cn.md`
  - 解释 OBTC 的经济模型创新与 AI 叙事位置
- `obtc_popular_science_cn.md`
  - 用通俗语言向非技术读者解释 AI 方向
- `obtcwallet/docs/developer/ai_agent_wallet_interface_zh.md`
  - 具体展开钱包接口、权限模型与工程落地方式

---

## 11. 结论

OBTC 真正贴近 AI 的方式，不是“支持一个 AI 钱包按钮”，而是把货币、钱包、权限、续期和审计重新组织成一套**为 agent 工作方式而设计的系统**。

如果做对了，OBTC 的差异化将不只是“回收沉睡资产”，还会变成：

- 一种适合 agent 持有的货币
- 一种适合 agent 执行预算的资金系统
- 一种适合机器经济长期运转的 lifecycle money

这条路线比“更大的区块”更难，但也更有独特性。

---

*文档日期：2026-03-08*
*状态：草案*
