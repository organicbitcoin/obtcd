# REAP Dust 行为说明（实现口径）

> 目的：统一钱包侧、RPC 展示侧与共识/打包侧对 `dust` 语义的理解，避免出现“看起来税率正常，但实际返还为 0”的误判。

## 1) 当前实现规则

REAP 在每个输入上先计算税，再应用 dust 规则：

- `tax = floor(value * TaxNum / TaxDen)`
- `refund = value - tax`
- 若 `refund > 0 && refund < DustThresholdSat`，则：
  - `refund = 0`
  - `tax += refund_before_fold`

默认参数（当前）：

- `TaxNum/TaxDen = 30/100`
- `DustThresholdSat = 720`（6! = 720）

## 2) 1027/1028 cliff（边界突变）

在默认参数下：

- `value=1027`：`tax=308`，`refund=719`（<720）→ 折叠后 `refund=0`、`tax=1027`
- `value=1028`：`tax=308`，`refund=720`（==720）→ 不折叠，`refund=720`、`tax=308`

结论：**只差 1 sat，结果从”无返还”跳到”返还 720”**。

## 3) 逐笔判定，不做“先聚合后判定”

Dust 判定是按 **每个输入** 执行，然后才会按脚本聚合输出。

示例（同脚本两笔输入，各 700 sat）：

- 每笔：`tax=210`，`refund=490`（<720）→ 两笔都折叠
- 最终：`TaxTotal=1400`，`RefundTotal=0`

如果误按“先聚合后判定”（**当前实现不是这样**）：

- 总额 1400：`tax=420`，`refund=980`（>=720）
- 会得到完全不同的结果。

## 4) TaxNum=0 也不等于“绝对零损失”

当 `TaxNum=0` 时，名义税为 0，但 dust 规则仍生效：

- `value=719`：`refund=719`（<720）→ 折叠为 tax，最终 `refund=0`、`tax=719`
- `value=720`：`refund=720`（==720）→ 不折叠，`refund=720`

结论：**TaxNum=0 不会自动关闭 dust 折叠**。

## 5) 钱包侧建议（避免理解偏差）

1. 风险评估按“输入级”计算，不要只按总额估算。
2. 对 `refund` 接近阈值（如 710~730 sat）做高亮提示，说明存在 cliff。
3. 在 UI/文档明确区分：
   - 名义税率（TaxNum/TaxDen）
   - dust 折叠导致的额外有效损失
4. 若提供“零税率模式”说明，需明确 dust 规则是否仍开启。

## 6) 回归测试覆盖

对应测试文件：`mining/reap/dust_extreme_test.go`

- `TestDustExtremeCliff1027Vs1028`
- `TestDustExtremePerInputFoldingDiffersFromAggregate`
- `TestDustExtremeTaxNumZeroStillFoldsSubDustRefund`
