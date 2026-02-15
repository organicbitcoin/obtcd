# Week4–8 可执行任务清单（Execution Checklist）

> 说明：按你的要求，这份文档只做“可执行化”，**不启动 Phase3.1 实施**。
> 约定：每周都以 `go test ./...` 通过作为收尾门槛之一。

---

## Phase 4（共识验证 & 挖矿模板集成）

### 目标
把 Week3 的 REAP 选择器/蓝图接入到区块验证与模板生成，先达到“端到端可跑 + 稳定”。

### 任务清单（按顺序）
1. 新增/接入 REAP 验证入口
   - [ ] 在 `blockchain` 增加 `IsREAP` 与 `CheckReapTx` 调用链
   - [ ] 在常规交易验证路径增加“非 REAP 不得花费过期 UTXO”检查
2. 挖矿模板注入
   - [ ] 在 `NewBlockTemplate` 流程中调用 `SelectCandidates` + `BuildBlueprint`
   - [ ] REAP 交易插入 coinbase 后第一位
   - [ ] REAP 税额并入模板 fee 统计
3. mempool 策略
   - [ ] 拒收 REAP 交易（仅区块内系统交易）
   - [ ] 普通交易若花费已过期 UTXO，策略层拒绝
4. 测试
   - [ ] `validation_reap_test.go`：合法/非法 REAP、错序、税额不一致、超上限
   - [ ] `template_reap_test.go`：有/无到期、截断、税额入账
   - [ ] reorg 集成用例
5. 文档
   - [ ] `docs/phase4-validation.md` 写入 txid、块高、税额差值、reorg 结果

### 验证命令
```bash
go test ./blockchain -run Reap -v
go test ./mining -run Reap -v
go test ./mempool -v
go test ./...
```

### 周结束 DoD
- [ ] 区块验证和模板构造都可处理 REAP
- [ ] simnet 上“到期→REAP→coinbase 收税”跑通
- [ ] 全仓测试通过

---

## Phase 5（钱包续期 RPC，建议先做 5A）

### 5A（本周建议范围）
1. `obtc.getexpiry`
   - [ ] 输出 expiry_height / blocks_to_expiry / status / dust_risk
2. `obtc.renew`（手动）
   - [ ] 支持 outpoint 或 before_days 筛选
   - [ ] 默认续期到新地址
3. 批量续期入口
   - [ ] CLI: `renew-all --before <days>`
4. 测试与文档
   - [ ] `docs/phase5-validation.md` 记录批量续期样例

### 5B（可并入 Week6）
- [ ] 自动续期（随机窗口 + max feerate + 每日预算）

### 验证命令
```bash
go test ./... 
# 另补钱包仓测试命令（按你实际 btcwallet fork 路径）
```

### 周结束 DoD
- [ ] 手动与批量续期可用
- [ ] 成功率达到目标（>=99%）
- [ ] 自动续期可延后不阻塞主线

---

## Phase 6（网络部署与观测，先稳后广）

### 目标
先做“可复现部署 + 小规模公网验证”，再扩展到三地域。

### 任务清单
1. 参数与创世（Testnet）
   - [ ] 固化测试网参数（魔数/端口/HRP/WIF/BIP32）
   - [ ] 创世生成与校验工具可复现
2. 部署（阶段一）
   - [ ] 单地域双节点 + 1 个观察节点
   - [ ] systemd + UFW + 日志轮转脚本
3. 状态页/审计
   - [ ] `obtc-status` 输出高度、节点数、mempool、REAP 汇总
   - [ ] 审计脚本导出近 288 块统计
4. 扩容（阶段二，可选）
   - [ ] 三地域种子（EU/US/AS）

### 验证命令（示例）
```bash
open ports / service status checks
curl http://<node>:<status-port>/status.json
# 审计脚本
# go run tools/reap-audit.go --rpc=... --window=288
```

### 周结束 DoD
- [ ] 新节点可在目标时间内同步到头
- [ ] 状态页稳定可用
- [ ] REAP 指标可观测

---

## Phase 7（硬化、故障注入、可复现构建）

### 目标
把关键限制上升到共识硬约束，完成 RC 级质量门槛。

### 任务清单
1. 共识硬化
   - [ ] `MaxREAPInputsPerBlock`、`MaxReapTaxPerBlock` 进入硬校验
   - [ ] REAP 每块唯一性硬校验
2. 索引健壮性
   - [ ] `indexVersion` 升级与迁移/重建策略
3. 压测与故障注入
   - [ ] 小额洪水
   - [ ] reorg 回放
   - [ ] kill -9 恢复
   - [ ] 无效 REAP 注入对抗
4. 可复现构建
   - [ ] 固定依赖与镜像 digest
   - [ ] 产物哈希与 minisign 签名
5. 文档
   - [ ] `docs/phase7-validation.md`
   - [ ] `docs/repro-build.md`

### 验证命令（示例）
```bash
go test ./...
./build/release.sh
sha256sum -c SHA256SUMS
minisign -Vm SHA256SUMS -P <pubkey>
```

### 周结束 DoD
- [ ] 共识硬化到位
- [ ] 压测门槛达标
- [ ] 可复现构建流程可被他人执行

---

## Phase 8（发布候选里程碑）

### 建议目标
优先“公开候选测试网里程碑”；若坚持主网候选，必须增加独立外部审计门槛。

### 任务清单
1. 冻结参数表
   - [ ] `docs/mainnet-params.md` 与代码一致
2. 发布资产
   - [ ] 三平台二进制 + Docker + SHA256SUMS + 签名
3. 节点部署
   - [ ] 至少三地域种子可互连
4. 文档
   - [ ] `docs/mainnet-join.md`
   - [ ] `docs/release-notes-*.md`
5. 观察窗
   - [ ] 连续 72h 指标记录

### 周结束 DoD
- [ ] 门槛指标达标（出块、孤块率、同步、REAP 覆盖率）
- [ ] 可宣布候选里程碑

---

## 全阶段统一收尾检查
- [ ] 分支干净（仅本周变更）
- [ ] 全仓测试通过：`go test ./...`
- [ ] 周验证文档更新
- [ ] commit message 使用英文
