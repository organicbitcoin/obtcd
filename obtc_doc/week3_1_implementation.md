# Week 3.1 修订计划（对 Week1-3 的补强）— REAP 工程化落地前置

**时间预算：8–12 小时** ｜ **目标**：在进入 Week4 共识接线前，把 Week3 的“可工作原型”补强为“可被共识/模板稳定复用”的模块。

---

## 为什么要做 Week3.1

你现在的 Week3 代码已经完成核心骨架（选择器/蓝图/测试），但从工程化角度，进入 Week4 前还建议补三类能力：

1. **生产依赖解耦**：选择器需要稳定访问 UTXO 数据与到期索引，避免调用方必须手工拼完整 view。
2. **可审计一致性**：Marker 规范、排序键、税额规则需做“规范化描述 + 固定测试向量”。
3. **集成干跑能力**：需要一个可执行 dry-run 工具或 RPC，便于链上前验证 picked/tax/refund/weight。

---

## 本周（3.1）目标（Definition of Done）

* 在 `mining/reap` 补充“生产可调用接口”：
  * 给 `SelectCandidates` 增加可选 UTXO 预取流程（或由调用方传入统一 fetcher 接口）。
  * 明确 `tip/height` 语义（选择高度 vs 执行高度）并在注释与测试中固定。
* 固定 Marker 规范：
  * 输入序列化字节序、hash 算法、payload 格式写入注释与测试向量。
* 补齐重量与上限行为测试：
  * `MaxInputs`、`WeightBudget`、`Tax cap`（若参数存在）的交互边界。
* 增加一个 dry-run 入口（命令或调试 RPC 二选一）：
  * 输出 `picked/tax/refund/estWeight/markerHash`。
* 文档化：`docs/week3.1-validation.md`。

---

## 建议代码改动

```
mining/reap/
  selector.go            # 补：可插拔 UTXO fetcher / 更清晰高度语义
  packer.go              # 补：marker 序列化规范注释 + 测试向量对齐
  params.go              # 补：与 chaincfg 参数映射（默认值 + 网络覆盖）
  selector_integration_test.go  # 新：模拟真实扫描+view 组合
  marker_vector_test.go          # 新：固定输入->固定 marker hash
cmd/reap-dryrun/                 # 新（可选）
docs/week3.1-validation.md       # 新
```

---

## 测试补强清单

1. **确定性向量测试**（必须）
   * 给定固定 outpoints 与金额，输出顺序、tax、refund、marker hash 固定。
2. **预算截断一致性**（必须）
   * 当 `WeightBudget` 触顶时，截断点稳定且重复运行一致。
3. **模式回归**（建议）
   * `Strict` 与 `Simple` 两模式都保留快照测试，防后续重构破坏。
4. **集成干跑**（必须）
   * 在 simnet/obtcregtest 上跑一次，输出文档化结果。

---

## 交付标准

* `go test ./mining/reap -v` 通过。
* 全仓 `go test ./...` 通过。
* `docs/week3.1-validation.md` 有真实输出样例。
* Week4 可直接复用，无需再改 REAP 核心数据结构。

---

## 风险控制

* 不在 Week3.1 引入新的共识规则（避免范围失控）。
* 若 dry-run 命令来不及，先做 debug RPC 或内部函数打印，保留可审计输出即可。
