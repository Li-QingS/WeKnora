# CI 定时评测与合并阻断记录（2026-09-02）

## 触发方式

`.github/workflows/rag-quality-gate.yml` 现在支持三种触发：

1. `pull_request`：改动 `internal/**`、`dataset/**`、`migrations/**`、
   `evaluation/**`、`config/**`、eval CLI / evalrunner / Makefile / workflow
   本身时执行；
2. `workflow_dispatch`：任意时刻手动触发；
3. `schedule`：每天 UTC 02:00 自动执行一次。

## 指标退化阻断合并

真实评测分支（配置了 `WEKNORA_EVAL_HOST` / `WEKNORA_EVAL_TOKEN` secrets）执行：

```bash
make eval-ci CONFIG=./evaluation/configs/enterprise.yaml \
  BASELINE=./evaluation/baselines/enterprise_rag.yaml
```

判定规则：

- Runner 先跑评测并生成 `evaluation-result.json` / `evaluation-report.md`；
- 再与基线做 `config_hash + dataset.sha256` 考卷校验和 12 项指标对比；
- 任一指标低于 `min_value`，或绝对/相对下降超过阈值，CLI 退出码 2；
- GitHub Actions 把非零退出码视为任务失败，同时 `if: always()` 上传评测报告；
- 在仓库 Branch Protection 中把 `evaluation quality gate` 设为 required check，
  任务失败时 PR 无法合并。

没有 secrets 时不会静默跳过：workflow 改用仓库内 pass/degraded 夹具执行
比较器自测，保证正常返回 0、退化返回 2。

## 基线更新

基线不能被 PR 悄悄改写。只有显式执行：

```bash
APPROVED_BY=<审批人> APPROVED_COMMIT=<commit> \
  make eval-baseline-generate BASELINE=./evaluation/baselines/<name>.yaml
```

生成后走正常代码评审合入；已有文件必须加 `--force` 才会被覆盖。

## 当前边界

- 未配置 secrets 的仓库只会执行比较器自测，不会做真实模型评测；
- 夜间定时任务同样遵守 secrets guard；没有 secrets 时执行自测；
- 正式门禁已切换到 `evaluation/baselines/enterprise_rag.yaml`，对应真实运行
  `evaluation_10000_1788358086107_16539e92_enterpriser`（50/50 success）；
- `evaluation/baselines/demo.yaml` 保留作比较器自测与旧 demo 回归用。
