# RAG 质量门禁真实运行证据（2026-09-08）

## Pass

- Run ID：`34238633896`
- URL：https://github.com/Li-QingS/WeKnora/actions/runs/34238633896
- Commit：`d29bc41ac380eb11a9a815de236fcfb161ba8f9b`
- 事件：`push`
- 状态/结论：`completed` / `success`
- Job：`evaluation quality gate` / `success`
- 输入：正式 pass fixture，Recall `1.0000`，比较器退出 `0`。
- 截图：`pass-run.png`。

## Recall 人为退化 Fail

- Run ID：`34239374350`
- URL：https://github.com/Li-QingS/WeKnora/actions/runs/34239374350
- Commit：`3e4ee338a5bc716a856293636a371e8084473fe4`
- 事件：`push`
- 状态/结论：`completed` / `failure`
- Job：`evaluation quality gate` / `failure`
- 输入：runner 临时复制 pass fixture，只把 Recall 从 `1.0000` 降至 `0.6000`。
- 门槛：`min_value=0.8000`、`max_absolute_drop=0.1000`；实测 `delta=0.4000`，比较器退出 `2`。
- 截图：`fail-run.png` 的 GitHub error annotation 直接显示上述 baseline/current/delta/threshold/exit code；`fail-job.png` 展示失败步骤。

原始 `*-run.json` 与 `*-jobs.json` 来自 GitHub 公共 Actions API，PNG 来自对应公开 run/job 页面。仓库不包含凭据和模型 Prompt。
