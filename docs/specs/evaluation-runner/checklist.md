# 可复现评测 Runner（课题三 WP2）Checklist

> 状态：已验收（2026-09-01）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行命令或观察行为验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC12）

- [x] AC1 一条命令出报告：执行 `make eval-baseline CONFIG=./evaluation/configs/default.yaml`，退出码为 0，并在 `artifacts/evaluation/` 下生成 `evaluation-result.json` 与 `evaluation-report.md`。（验证：端到端实测，退出码 0，两个文件生成）
- [x] AC2 报告与数据库一致：`evaluation-result.json` 中的 `run_id` 与 `config_hash` 能在服务端 `evaluation_runs` 表中查到同一记录，字段值一致。（验证：SQLite 查库 `evaluation_1_1788275115626_95118518_demo` 的 config_hash 与报告一致）
- [x] AC3 自定义数据集可用：`dataset/demo` 包含 10-30 个 QA 对；以 `dataset_id=demo` 发起评测成功，快照中的 `dataset.sha256` 与 `sample_count` 正确。（验证：demo=15 条，快照 sha256/sample_count 一致）
- [x] AC4 配置错误退出码 3：不存在的配置文件、非法 `dataset_id`、坏 YAML 均在发起任务前报错并以退出码 3 结束，服务端不产生新运行记录。（验证：`does-not-exist` 数据集实测退出码 3，服务端 400）
- [x] AC5 服务不可用退出码 4：服务未启动、鉴权失败或接口 5xx 时以退出码 4 结束。（验证：连接不存在端口实测退出码 4）
- [x] AC6 运行失败/超时退出码 5：任务终态为 failed / interrupted 或等待超时，以退出码 5 结束，报告中包含 `err_msg` 与最终进度。（验证：无效 chat 模型运行失败，退出码 5，失败报告生成；超时由单元测试覆盖）
- [x] AC7 参数覆盖生效且兼容：同一服务、同一数据集，不同 YAML 参数产生不同的 `params` 与 `config_hash`；不带新字段的原始 `POST /evaluation` 请求响应与之前一致。（验证：`TestEvaluationPersist_ParamsOverrideAffectsConfigHash` + handler 单测）
- [x] AC8 非等待模式可用：`weknora eval run --no-wait` 输出任务 ID 并以退出码 0 结束；随后用该 ID 能继续轮询并生成报告。（验证：`--no-wait` 实测 + `--task-id` 续跑退出码 0，报告生成）
- [x] AC9 可复现签名一致：相同数据集与配置连续运行两次，两次 `config_hash` 相同；报告中包含复现命令与版本签名。（验证：两次真实运行 config_hash 均为 `2bbe7dd1...`，报告含 reproduce）
- [x] AC10 路径安全：`dataset_id` 传 `../`、绝对路径或特殊字符会被拒绝并以退出码 3 结束，不读取仓库外目录。（验证：`dataset_id=../escape` 服务端返回 400；CLI 配置校验单测覆盖）
- [x] AC11 无敏感信息泄漏：报告、配置样例与运行快照中不出现 API Key、endpoint 凭据、Prompt 正文。（验证：grep 报告无真实凭据；快照沿用 WP1 脱敏）
- [x] AC12 自动化验证：新增单元测试通过，`go build` / `go test` 覆盖服务端、SDK、CLI；端到端场景在真实服务上完整跑通并记录证据。（验证：最终回归命令全绿，记录见 progress-2026-09-01.md）

## 集成

- [x] `dataset_id` 真正生效：`default` 加载 `dataset/samples`，`demo` 加载 `dataset/demo`，不同 ID 得到不同数据与哈希。（验证：DatasetService 单测 + 真实运行快照）
- [x] 数据集失败在创建运行前返回：服务端对坏数据集返回 HTTP 400，`evaluation_runs` 中无新 pending 记录。（验证：handler 单测 + 真实请求 400）
- [x] `config_hash` 包含数据集哈希：相同参数但不同数据集，两次运行的 `config_hash` 不同。（验证：服务端单测覆盖哈希入 hash 口径）
- [x] SDK 与真实 API 契约一致：`StartEvaluation` / `GetEvaluationResult` / `ListEvaluationRuns` 能对真实服务返回正确解析结果。（验证：端到端命令 + SDK 单测）
- [x] Runner 报告与数据库共用同一事实来源：报告中的 `config_hash/config_snapshot` 来自 `GET /evaluation/runs` 记录，而非本地重新计算。（验证：代码审查 + 报告/库对比）
- [x] `eval run` 命令遵循 CLI 输出契约：成功走 stdout envelope，错误走 stderr typed error；`--dry-run` 不调用 SDK、不写报告。（验证：CLI 单测 + 手工运行）
- [x] `make eval-baseline` 使用根目录相对路径：从仓库根目录执行即可，不依赖进入 `cli/`。（验证：直接运行目标）

## 编译与测试

- [x] 服务端：`go build ./...` 通过
- [x] 服务端：`go test ./internal/...` 通过
- [x] SDK：`cd client && go test ./...` 通过
- [x] CLI：`cd cli && go test ./...` 通过
- [x] CLI：`cd cli && go vet ./...` 通过
- [x] `go test -race` 覆盖服务端新增/修改测试（如 DatasetService、EvaluationService）通过

## 端到端场景

- [x] 场景 1（成功基线）：启动 Lite 服务并配置好凭据 → `make eval-baseline CONFIG=./evaluation/configs/default.yaml` → 退出码 0；`artifacts/evaluation/evaluation-result.json` 与 `evaluation-report.md` 生成；报告 run_id/config_hash 与数据库一致；报告中指标完整、数据集 sample_count 正确。
- [x] 场景 2（失败分类）：分别运行非法配置（退出码 3）、服务停止（退出码 4）、失败任务（退出码 5），三类错误在 stderr 中给出可区分的原因。
- [x] 场景 3（可复现）：同一配置连跑两次，两次 `config_hash` 相同；报告中包含可直接复制的复现命令。

## 验收记录

实际命令输出、退出码、数据库核对结果与未覆盖项见 [progress-2026-09-01.md](./progress-2026-09-01.md)。
