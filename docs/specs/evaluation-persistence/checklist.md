# 评测运行持久化（课题三 WP1）Checklist

> 状态：已验收（2026-08-30，主流程完成；AC10/AC11 留待后续）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行代码或观察行为来验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC11）

- [x] AC1 创建即持久：用现有 `POST /evaluation` 发起一次评测，接口返回的任务结构与改动前一致；随后在数据库中查到同 task_id 的记录，状态为 pending，tenant_id / dataset_id / params 与请求一致。（验证：`TestEvaluationPersist_CreatesPendingAndSanitizedSnapshot` + repository 单测）
- [x] AC2 进度持久可见：评测执行期间多次调用 `GET /evaluation?task_id=<id>`，状态显示 running、finished 递增；同一时刻数据库中的 total/finished 与接口一致。（验证：`TestEvaluationRun_ProgressAndHeartbeatStopAtTerminal` + `TestEvaluationPersist_SuccessCompletesWithMetrics`）
- [x] AC3 终态完整落库且不可变：成功记录为 success、12 项指标齐全、finished == total；人为失败记录为 failed 且 err_msg 可定位原因；对已终态记录再次执行进度/心跳/状态更新不会改变原记录。（验证：成功/失败 service 测试 + CAS 单测）
- [x] AC4 重启不丢、中断可解释：评测运行中重启服务后，用同一 task_id 查询返回 interrupted 与中断时间，已写入的进度保留；对同一配置重新发起评测生成新 task_id 与新记录，不覆盖旧记录。（验证：`TestEvaluationRun_MarkStaleInterrupted` + `TestEvaluationPersist_StartupScanMarksStaleRuns`）
- [x] AC5 历史列表可用：`GET /evaluation/runs` 返回当前租户的运行列表，按 created_at 倒序，page/page_size 生效，status 筛选生效，列表项包含 ID、数据集、状态、进度、核心指标、创建/完成时间。（验证：repository List 单测 + service 列表测试）
- [x] AC6 租户隔离：B 租户用 A 租户的 task_id 查询得到"不存在"类错误；B 租户的列表不含 A 租户任何记录。（验证：`TestEvaluationRun_CreateGetTenantIsolation` + `TestEvaluationPersist_EvaluationResultIsTenantScoped`）
- [x] AC7 快照可解释、可对比：成功记录包含 config_snapshot（dataset / models / version 三部分）与 config_hash；同一配置连续两次运行 config_hash 相同；快照中不出现 API Key、endpoint 凭据、Prompt 正文。（验证：`TestEvaluationPersist_CreatesPendingAndSanitizedSnapshot` + `TestEvaluationPersist_ConfigHashStableForSameConfig`）
- [x] AC8 心跳与中断判定：running 记录的 heartbeat_at 随时间更新；把 heartbeat_at 人为改旧后执行启动扫描，记录被标记为 interrupted 并写入 finished_at。（验证：`TestEvaluationRun_MarkStaleInterrupted` + `TestEvaluationRun_ProgressAndHeartbeatStopAtTerminal`）
- [x] AC9 双库迁移一致：PostgreSQL 与 SQLite 两套迁移均包含新增列；从零建库与从旧版本升级两条路径均通过；SQLite 守卫测试登记表已同步。（验证：`go test ./internal/database/ -v`）
- [ ] AC10 性能与并发：相关并发测试带 `-race` 全过；持久化实现相对内存实现的总耗时差异 <= 5%（本地基准，记录在 PR 描述）。（验证：`go test -race` + 基准结果）
- [ ] AC11 存储故障有解释：模拟写入失败时，任务被标记为 failed 且 err_msg 带原因，不出现无解释的 running 悬停；启动扫描遇到数据库不可用时仅记日志，服务仍能启动。（验证：故障注入测试 + 启动日志）

## 集成

- [x] 状态读写出口已切到持久化仓库：重启后旧 task_id 仍可查到结果，证明查询来自数据库而非内存。（验证：`TestEvaluationPersist_SuccessCompletesWithMetrics` + repository 单测）
- [x] 终态保护与租户隔离在仓库层生效：对终态记录的任何更新被拒绝，跨租户读取返回不存在。（验证：`TestEvaluationRun_TransitionStatusCASProtectsTerminal` + 租户隔离单测）
- [x] 启动流程在迁移完成后执行中断扫描，扫描失败不阻断启动。（验证：`TestEvaluationPersist_StartupScanMarksStaleRuns` + `markStaleEvaluationRuns` 仅记日志）
- [x] `GET /evaluation/runs` 已注册，Viewer 可读；现有 `POST /evaluation` 与 `GET /evaluation?task_id=` 请求/响应保持不变。（验证：真实 HTTP 请求 + 路由/单测）
- [x] 版本签名真实可读：成功记录 version 部分包含应用版本、代码提交标识、dirty 标记、Go 版本。（验证：`TestEvaluationPersist_CreatesPendingAndSanitizedSnapshot` 断言 AppVersion/GitCommit；buildinfo 注入脚本输出含 GIT_DIRTY）
- [ ] 写入开销受控：进度更新按样本完成或固定间隔聚合，心跳按 10s 间隔更新，不逐样本高频写库。（验证：AC10 基准 + 日志/代码审查）

## 范围边界

- [x] 中断任务只标记、不续跑：重启后同配置重新发起生成新记录，旧记录保持 interrupted。（验证：`TestEvaluationRun_MarkStaleInterrupted`）
- [x] 不做样本级明细持久化：表中只有聚合结果与配置快照，没有样本级明细表或字段。（验证：迁移 schema 检查）
- [x] 不新增前端/CI/成本功能：历史列表仅后端接口，CI 门禁、成本台账、一键 Runner 不在本工作包交付物中。（验证：PR diff/交付范围检查）

## 编译与测试

- [x] `go build ./...` 通过
- [x] `go vet ./internal/...` 通过
- [x] `go test ./internal/...` 全过
- [x] `go test -race ./internal/application/repository/ -run TestEvaluationRun` 与 `go test -race ./internal/application/service/ -run TestEvaluationPersist` 通过
- [x] `go test ./internal/database/ -v` 全过
- [ ] golangci-lint（如可用）无新增告警

## 端到端场景

- [x] 场景 1（正常完成）：Admin 发起评测 → 立即查库出现 pending → 轮询接口看到 running 且 finished 递增 → 完成后数据库为 success、12 项指标齐全、finished == total、config_snapshot/config_hash 完整；重启服务后同一 task_id 仍返回同样结果。（验证：真实 Lite 服务完整跑一遍）
- [x] 场景 2（运行中重启）：评测运行中重启服务 → 启动后同 task_id 返回 interrupted 与 finished_at，已写入进度保留 → 再次发起同配置评测生成新记录，旧记录不被覆盖。（验证：真实重启 + 合成 running 记录触发启动扫描）
- [x] 场景 3（失败与隔离）：失败路径由自动化测试覆盖（failed + err_msg）；B 租户查询/列表均看不到 A 租户的记录已用真实双租户请求验证。（验证：service 测试 + 真实 HTTP 双租户请求）

## 验收记录（2026-08-30）

- 详细记录见 [progress-2026-08-30.md](./progress-2026-08-30.md)。
- `go build ./...`、`go vet ./internal/...`、`go test ./internal/...` 均通过。
- 本工作包相关单测在 `-race` 下通过：`TestEvaluationRun*`、`TestEvaluationPersist*`。
- 端到端已实测：正常评测落库、历史列表、重启中断扫描、租户隔离。
- 未完成项：AC10 本地基准、AC11 存储故障注入、`golangci-lint`（本机未安装）。
- 已知基线问题：`go test -race ./internal/application/service/` 全量执行时，既有 `TestTenantAPIKeyServiceAuthenticateThrottlesLastUsedUpdates` 存在与本工作包无关的 race。
