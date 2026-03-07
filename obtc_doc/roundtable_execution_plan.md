# OBTC 圆桌共识纪要（V1.3.2）执行计划

> 文档状态：已对齐当前代码与当前工作区草稿
> 对齐日期：2026-03-07（Europe/Stockholm）

## Context

圆桌纪要草稿 `obtc_doc/OBTC_圆桌最终共识纪要_2026-03-04.md` 定义了 F1-F4 四项交付和 Gate A / Gate B 双闸门。

这份文档不再复述“理想应然”，而是按 **2026-03-07 当前仓库与工作区状态** 给出执行口径：

- 哪些项已经有代码或脚本草稿；
- 哪些项仍未落地；
- 哪些术语和实现语义已经变化，必须按最新代码理解。

## 一、现状总览（按 2026-03-07）

| 项目 | 圆桌要求 | 当前状态 | 结论 |
|------|----------|----------|------|
| F1 | REAP WeightBudget 双档走廊与自动回滚 | 未在当前 `master` 落地 | 仍待实现 |
| F2 | `MaxReapTaxPerBlock` + `H_taxcap` 锁定 | 已决定不纳入 v1 共识 | 从当前执行范围移除 |
| F3 | Kill 指标脚本化与 Gate A 观测 | 当前工作区已有 `scripts/gate/` 草稿 | 可继续收口与演练 |
| F4 | 冻结包最小三件套 | 当前未见 `obtc_doc/freeze_pack/` 落位 | 仍待补齐 |

直接结论：

1. **Gate A 截至 2026-03-07 仍不能视为已满足前置条件**。
2. 当前最接近可执行状态的是 **F3**，因为脚本草稿已经存在。
3. **F4 仍是文档交付缺口**。
4. F1 仍停留在纪要要求层；F2 已被当前 v1 冻结口径移出执行范围。

## 二、必须先统一的最新口径

### 2.1 `ExpiryIndex` 语义已经变化

当前 `master` 上：

- `ExpiryIndex` 不再只是可选扫描索引；
- 在 **OBTC 网络** 上，它是 **always-on 的 expiry commitment 状态源**；
- `--expiryindex` 只控制 scan/RPC/REAP 选择功能。

这对 F3 有直接影响：

- Gate 脚本里通过 `getexpiryindexstats` 看到的，是**扫描/RPC 层状态**；
- 它**不是** expiry commitment 共识状态是否维护的直接等价物；
- 如果节点没开 `--expiryindex`，`getexpiryindexstats` 可能返回 disabled，但这不表示 commitment 共识状态被关闭。

因此，F3 文档与脚本说明必须避免把：

```text
expiry index disabled = 共识状态关闭
```

这种旧语义继续写进去。

### 2.2 当前没有 `getreapstats`

当前仓库里：

- 有 `listexpiring`
- 有 `getexpiryindexstats`
- 没有 `getreapstats`

所以 F3 里所有“REAP 覆盖率 / 积压 / 孤块率”的自动化统计，当前都还不是完整的 RPC 化能力。

最新口径应是：

- **前激活阶段**：只做早期代理指标
- **后激活阶段**：再补 REAP 专用统计与硬判定

### 2.3 Dust cliff 口径必须改成 `719/720`

当前实现与文档基线 [`reap_dust_behavior.md`](../obtc_doc/reap_dust_behavior.md) 已明确：

- Dust 阈值：`720 sat`
- cliff：`719/720`

因此 F4 钱包告知文档不应继续写旧的：

- `778/779`

而应统一写成：

- `719/720 cliff`

### 2.4 参数名要按当前代码理解

圆桌纪要和脚本里常写：

- `MaxREAPInputs`

当前代码链参数实际名是：

- `ReapMaxInputs`

在执行计划里可以保留圆桌术语，但要注明：

```text
圆桌术语 MaxREAPInputs = 当前代码中的 ReapMaxInputs
```

## 三、F3 — Kill 脚本（当前工作区已有草稿）

### 3.1 当前已有内容

当前工作区已经有：

- `scripts/gate/metrics.sh`
- `scripts/gate/kill_check.sh`
- `scripts/gate/gate_a_report.sh`
- `scripts/gate/README.md`

它们已经覆盖了 Gate A 的“早期主网/演练期代理指标”。

### 3.2 当前脚本实际采集的指标

`metrics.sh` 当前实际依赖的 RPC 是：

- `getblockchaininfo`
- `getinfo`
- `getchaintips`
- `getmempoolinfo`
- `getpeerinfo`
- `getexpiryindexstats`

当前没有使用也无法使用：

- `getreapstats`

### 3.3 当前脚本实际判定逻辑

#### 硬 Kill

- `max_fork_depth > 1`
- replay violation 日志扫描命中数 `> 0`

注意：

- replay violation 目前不是专用 RPC 计数器；
- 当前实现依赖日志关键字扫描，是**代理信号**，不是共识级计数接口。

#### 软 Kill

- `connections < 1`
- `expiry index lag > 100`（仅在 scan/RPC 打开且能拿到 stats 时有效）
- `non_active_tips > 10`

#### REAP 专用指标

当前脚本已经把以下指标列为 **post-activation placeholders**：

- REAP coverage ≥ 95%
- backlog < `3 × ReapMaxInputs`
- orphan rate ≤ 3%（峰值 ≤ 5%）

这部分口径是对的，但必须在计划里明确：

- 当前只是占位；
- 还没有完整的数据源和自动核算链路。

### 3.4 F3 还需要收口的地方

1. **README 和执行文档补充最新语义**
   - 明确 `getexpiryindexstats` 反映的是 scan/RPC 状态，不是 commitment 开关
   - 明确 `--expiryindex` 仅影响 scan/RPC

2. **devnet / regtest 演练留痕**
   - 连续跑 `kill_check.sh --watch`
   - 产出 `metrics.jsonl`
   - 产出 `kill_events.jsonl`
   - 生成 `gate_a_report.md`

3. **REAP 后激活指标待后补**
   - 覆盖率
   - 积压
   - 孤块率
   - 对应的自动化采集接口/离线审计器

### 3.5 F3 当前执行建议

当前最合理的执行顺序是：

1. 用现有 `scripts/gate/` 完成 **前激活阶段演练**
2. 保留硬 Kill 与早期软 Kill 代理指标
3. 不虚构 `getreapstats`
4. 等 REAP 激活窗口接近时，再补 REAP 专用统计

## 四、F4 — 冻结包 v1（三件套）

### 4.1 当前状态

当前未见以下文件落位：

- `obtc_doc/freeze_pack/wallet_notice_v1.md`
- `obtc_doc/freeze_pack/compliance_trail_v1.md`
- `obtc_doc/freeze_pack/exchange_runbook_v1.md`

所以 F4 目前仍是**未完成项**。

### 4.2 三件套的最新口径

#### 1) 钱包告知

应覆盖：

- 链识别方法
- 到期 / 续期说明
- replay protection 基本提示
- dust cliff 风险

这里必须对齐当前技术文档：

- dust cliff 写 `719/720`
- dust threshold 写 `720 sat`
- REAP 尚未激活时，要把“当前阶段无 REAP 运行场景”写清楚

#### 2) 合规留痕

应至少包含：

- 文案哈希
- 时间戳
- 操作者
- 版本号
- 证据链接模板

#### 3) 交易所 Runbook

应至少包含：

- 停充提流程
- 回滚操作
- 客服分流
- 复盘模板

### 4.3 F4 当前执行建议

F4 是纯文档项，不依赖新的共识代码，因此应该直接落文件，不必继续停留在“计划态”。

当前建议目录仍然是：

- `obtc_doc/freeze_pack/`

## 五、Gate A / Gate B 的当前判断

### 5.1 Gate A（目标日期：2026-03-10）

根据圆桌纪要，Gate A 要求同时满足：

1. F2 完成并锁定
2. F3 自动判定与 Kill 脚本上线并留痕
3. F1 走廊控制与自动回滚链路演练通过
4. 72h 观测窗指标达标

按 2026-03-07 当前状态判断：

- F2：未落地
- F1：未落地
- F3：有脚本草稿，但仍需演练与留痕
- F4：未完成

因此当前结论只能是：

> **截至 2026-03-07，Gate A 仍应视为未满足前置条件。**

### 5.2 Gate B（目标日期：2026-03-17）

Gate B 建立在 Gate A 通过基础上。

由于 Gate A 当前还未就绪，Gate B 也不能被视为进入执行完成态，只能保留为后续闸门。

## 六、按当前状态重排的执行顺序

### 立即可做

1. 收口 `scripts/gate/` 文案与说明
2. 在 devnet/regtest 上跑 F3 演练并生成留痕
3. 落 F4 三件套文档

### 仍待协议/实现补齐

1. F1 走廊切换与自动回滚
2. REAP 后激活阶段专用统计链路

## 七、验证方案

### F3 验证

```bash
./scripts/devnet-up.sh start
./scripts/gate/kill_check.sh --watch --interval=60
./scripts/gate/gate_a_report.sh --out=gate_a_report.md
```

补充说明：

- 若需要 `expiry index lag` 指标，节点要启用 `--expiryindex`
- 若未启用 `--expiryindex`，该项应视为 scan/RPC 不可用，而不是 commitment 状态关闭

### F4 验证

1. 三件套文件全部落位
2. 钱包告知中的 dust cliff 对齐 `719/720`
3. 技术描述与以下文档一致：
   - [`reap_dust_behavior.md`](../obtc_doc/reap_dust_behavior.md)
   - [`newcomer_reading_guide.md`](../obtc_doc/newcomer_reading_guide.md)
   - [`expiry_commitment_implementation.md`](../obtc_doc/expiry_commitment_implementation.md)

## 八、结论

这份计划按最新口径收束后，可以归纳为三句话：

1. F3 已有可运行草稿，但当前只覆盖**前激活阶段代理指标**。
2. F4 仍是明确缺口，而且 dust cliff 文案必须统一改成 `719/720`。
3. 截至 2026-03-07，Gate A / Gate B 都不应在文档里被写成“已具备放行条件”。
