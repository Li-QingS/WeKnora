# CI 质量回归门禁（课题三 WP3）Checklist

> 状态：已验收（2026-09-01 / 2026-09-02）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行命令或观察行为验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC11）

- [x] AC1 基线文件存在且可解释：`evaluation/baselines/demo.yaml` 的 `config_hash` 与真实 demo 运行一致，包含检索/生成指标阈值和批准元数据。（验证：`cat` + 与 `evaluation-result.json` 对比）
- [x] AC2 通过场景退出码 0：`weknora eval compare --result evaluation/fixtures/evaluation-result-pass.json --baseline evaluation/baselines/demo.yaml` 输出全过，退出码 0。（验证：命令输出 `pass:true` + `echo $?`）
- [x] AC3 退化场景退出码 2：对 `evaluation-result-degraded.json` 执行比较，输出包含基线值、当前值、delta 与失败指标，退出码 2。（验证：命令输出 6 个 FAIL + `echo $?`）
- [x] AC4 考卷不匹配退出码 3：修改夹具的 `config_hash` 或 dataset 哈希后比较，返回 `eval.config_error` 且退出码 3，不输出“通过”。（验证：compare 单测 `TestCompareResultMismatch` + 命令映射）
- [x] AC5 Runner 集成：`weknora eval run --baseline <path>` 在退化时退出码 2，报告文件仍生成且包含对比小节。（验证：runner 单测 `TestRunWithBaselineRegression` + `TestRunCommandBaselineRegression`）
- [x] AC6 基线显式更新：`weknora eval baseline create` 缺 `--approved-by`/`--approved-commit` 时报错；文件已存在且无 `--force` 时拒绝覆盖；`--force` 可覆盖。（验证：baseline 单测 + 命令测试）
- [x] AC7 workflow 存在且可执行：`.github/workflows/rag-quality-gate.yml` 已提交，包含 secrets guard、真实评测分支与比较器自测分支。（验证：文件 review + 无 secrets 自测命令本地可跑）
- [x] AC8 退化演示可复现：正常 pass 夹具退出 0，degraded 夹具退出 2，文档记录两条命令。（验证：T6 命令输出）
- [x] AC9 无敏感信息泄漏：基线文件、比较输出、夹具和报告中不出现 API Key、endpoint 凭据、Prompt 正文。（验证：内容 review + 沿用 WP2 脱敏）
- [x] AC10 自动化验证：比较器与基线生成单测通过；`go build ./...`、服务端/客户端/CLI 测试与 vet 通过。（验证：最终回归命令全绿）
- [x] AC11 无迁移、指标算法不变：本次不新增数据库迁移，不修改 12 项指标实现，`evaluation-result.json` 结构保持兼容。（验证：git diff 检查）

## 集成

- [x] `config_hash + dataset.sha256` 双字段校验生效：任一不匹配即拒绝比较，错误信息可读。（验证：compare 单测 + `ErrBaselineMismatch` 映射）
- [x] 阈值规则三种类型生效：`min_value`、`max_absolute_drop`、`max_relative_drop` 各自有单测覆盖。（验证：`go test ./internal/evalrunner/ -run Compare`）
- [x] 基线生成与比较器共用同一指标口径：生成基线后立即用同一结果比较，全部通过。（验证：baseline 生成单测 + compare pass）
- [x] Runner 无 baseline 时行为不变：不带 `--baseline` 的报告 JSON 不含 `comparison` 字段。（验证：普通 `make eval-baseline` 报告）
- [x] CLI 输出契约保持：`eval compare` 成功走 stdout envelope，失败走 stderr typed error；`baseline create --dry-run` 不写文件。（验证：CLI 单测 + 手工运行）
- [x] `make eval-ci` 使用根目录相对路径，从仓库根目录直接执行。（验证：`make -n` 与实际运行）

## 编译与测试

- [x] 服务端：`go build ./...` 通过
- [x] 服务端：`go test ./internal/...` 通过
- [x] SDK：`cd client && go test ./...` 通过
- [x] CLI：`cd cli && go test ./...` 通过
- [x] CLI：`cd cli && go vet ./...` 通过
- [x] 比较器/基线相关测试在 `-race` 下通过（如适用）

## 端到端场景

- [x] 场景 1（正常门禁）：使用现有 Lite 服务 → `make eval-ci CONFIG=./evaluation/configs/default.yaml BASELINE=./evaluation/baselines/demo.yaml` → 退出码 0；报告含“Baseline comparison”且全部通过。
- [x] 场景 2（退化拦截）：用 degraded 夹具触发比较器 → 退出码 2；单测覆盖 `weknora eval run --baseline` 的退化路径与报告对比。
- [x] 场景 3（考卷保护）：改掉夹具的 dataset 哈希后比较 → 退出码 3，明确提示“考卷不匹配”，不会误判通过或退化。

## 验收记录

实际命令输出、退出码、报告对比内容与未覆盖项见 [progress-2026-09-01.md](./progress-2026-09-01.md)。
