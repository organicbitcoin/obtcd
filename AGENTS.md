# AGENTS.md

本文档用于约定 OBTC 仓库的推荐目录结构与职责边界，方便快速定位模块与后续扩展。

## 结构总览（建议）

```
obtc/ (repo root)
  btcd/                # upstream 代码（如有单独目录）
  blockchain/
    expiryindex/       # Week2: 到期索引（已实现）
    validation_reap.go # Week4+: REAP 共识验证（规划）
  chaincfg/
    params_obtc.go     # OBTC 网络参数
  mining/
    reap/              # Week3: 选择器/蓝图构造（规划）
    template_reap.go   # Week4: 模板注入（规划）
  mempool/
    policy.go          # Week4: REAP 策略限制（规划）
  rpc/
    rpcserver.go       # RPC 接入点
  cmd/
    gengenesis/        # Week6/8: 创世生成器（规划）
    checkgenesis/      # Week6/8: 创世校验器（规划）
    obtc-status/       # Week6: 最小状态页（规划）
  scripts/
    devnet-up.sh       # Week1: devnet 一键脚本
    validation/        # Week2: 验证脚本与工具
  docs/
    phase1-validation.md
    phase2-summary.md
    phase2-validation.md    # Week2: 验证记录（建议补齐）
    phase3-validation.md    # Week3: 计划/验证（规划）
    phase4-validation.md    # Week4: 计划/验证（规划）
    testnet-join.md         # Week6: Testnet 接入指南（规划）
    mainnet-join.md         # Week8: Mainnet 接入指南（规划）
  obtc_doc/
    AGENTS.md
    obtc_roadmap_plan.md
    newcomer_reading_guide.md
    expiry_commitment_implementation.md
    reap_dust_behavior.md
    phase5_implementation.md
    phase6_implementation.md
    phase7_implementation.md
    phase8_implementation.md
```

## 说明

- 上述结构为“建议落位”，已实现与规划项混合在一起，便于对照周计划。
- 若实际目录不同，以仓库现状为准，可在此文件同步更新。
- 新增模块尽量按功能归类，避免在顶层堆积零散文件。

## 交互约束

- 对话回复一律中文。
- 提交记录（commit message）使用英文。
