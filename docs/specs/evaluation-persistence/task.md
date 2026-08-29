# 评测运行持久化（课题三 WP1）Tasks

> 状态：已批准（2026-08-29）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)（均已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/buildinfo/buildinfo.go` | Version/CommitID/GitDirty/GoVersion 注入变量 |
| 修改 | `scripts/get_version.sh` | 增加 GIT_DIRTY 输出 |
| 修改 | `Makefile` | ldflags 增加 buildinfo.* 注入 |
| 修改 | `migrations/versioned/000088_evaluation_runs.{up,down}.sql` | +5 列 |
| 修改 | `migrations/sqlite/000013_evaluation_runs.{up,down}.sql` | 同步 +5 列 |
| 修改 | `internal/database/migration_sqlite_versioned_schema_test.go` | 守卫测试登记表 |
| 修改 | `internal/types/evaluation.go` | EvaluationRun 模型、Interrupted 状态、Snapshot 类型 |
| 修改 | `internal/types/interfaces/evaluation.go` | Repository 接口 + 服务 List 方法 |
| 新建 | `internal/application/repository/evaluation_run.go` | GORM 实现 |
| 新建 | `internal/application/repository/evaluation_run_test.go` | SQLite 单测 |
| 修改 | `internal/application/service/evaluation.go` | 接线 repository、快照、心跳、终态 |
| 新建 | `internal/application/service/evaluation_persist_test.go` | service 单测（mock repo） |
| 修改 | `internal/handler/evaluation.go` | 列表端点 |
| 修改 | `internal/router/routes_infra.go` | 路由注册 |
| 修改 | `internal/container/container.go` | DI 注册 + 启动中断扫描 |

## T1：buildinfo 包与版本注入

**文件：** `internal/buildinfo/buildinfo.go`、`scripts/get_version.sh`、`Makefile`
**依赖：** 无
**步骤：**
1. 新建包，定义 `Version="dev"`、`CommitID="unknown"`、`GitDirty=true`、`GoVersion="unknown"` 四个可注入变量（dev 默认 dirty=true，诚实标记未盖章构建）
2. `get_version.sh` 检测 `git status --porcelain` 输出 GIT_DIRTY
3. Makefile 两处 ldflags 增加 buildinfo.* 的 `-X` 注入

**验证：** `go build ./internal/buildinfo/` 通过；`./scripts/get_version.sh env` 输出含 GIT_DIRTY

## T2：类型定义

**文件：** `internal/types/evaluation.go`
**依赖：** 无
**步骤：**
1. 加 `EvaluationStatueInterrupted = 4`
2. 加 `EvaluationRun` GORM 模型（含 `TableName() = "evaluation_runs"`）与 Snapshot 三个结构体（DatasetSnapshot / ModelSnapshot / VersionSignature / EvaluationConfigSnapshot）

**验证：** `go build ./internal/types/` 通过

## T3：Repository 接口

**文件：** `internal/types/interfaces/evaluation.go`
**依赖：** T2
**步骤：**
1. 按 plan 定义 `EvaluationRunRepository` 七个方法（Create / GetByID / List / UpdateProgress / UpdateHeartbeat / SetDatasetHash / TransitionStatus / MarkStaleInterrupted）
2. 此任务不动 EvaluationService 接口，避免中间态编译断裂

**验证：** `go build ./internal/types/...` 通过

## T4：Repository 实现

**文件：** `internal/application/repository/evaluation_run.go`
**依赖：** T3
**步骤：**
1. GORM 实现全部方法；读方法强制 tenant_id
2. `TransitionStatus` 用 `WHERE id=? AND status IN ?` CAS，并回写 finished_at/err_msg
3. `UpdateProgress` 限定 `status=running`，终态后不生效

**验证：** `go build ./internal/application/repository/` 通过

## T5：Repository 单测

**文件：** `internal/application/repository/evaluation_run_test.go`
**依赖：** T4
**步骤：**
1. SQLite 临时库 AutoMigrate（参照 `task_queue_test.go` 的既有做法）
2. 覆盖：创建/查询、租户隔离（B 查 A 返回不存在）、分页+状态筛选、CAS 终态后拒绝再迁移、UpdateProgress 终态后不生效、MarkStaleInterrupted 只标记过期任务

**验证：** `go test ./internal/application/repository/ -run TestEvaluationRun -race -v` 全过

## T6：迁移与守卫测试

**文件：** `migrations/versioned/000088_evaluation_runs.{up,down}.sql`、`migrations/sqlite/000013_evaluation_runs.{up,down}.sql`、`internal/database/migration_sqlite_versioned_schema_test.go`
**依赖：** T2（列名以模型为准）
**步骤：**
1. PG 000088 up 加 5 列、down 加对应 DROP COLUMN；SQLite 000013 同步
2. 守卫测试 `versionedSQLiteColumns` 登记 `evaluation_runs` 的 5 个新列（版本号预期保持 13/88 不变）

**验证：** `go test ./internal/database/ -v` 全过（含升级路径测试）

## T7：Service 改造（核心任务）

**文件：** `internal/application/service/evaluation.go`、`internal/types/interfaces/evaluation.go`
**依赖：** T4
**步骤：**
1. `NewEvaluationService` 注入 repository；删除 `evaluationMemoryStorage`
2. `Evaluation()`：默认值补全后构建快照 + config_hash → `Create(pending)` → goroutine 内 CAS 转 running、起 10s 心跳 ticker、`EvalDataset` 加载数据集后 `SetDatasetHash`、终态 CAS 落库
3. `EvalDataset()`：每样本完成改调 `UpdateProgress`（替代内存 update）
4. `EvaluationResult()`：改查库，`EvaluationRun` 组装回 `EvaluationDetail`（params/metric JSON 反序列化）
5. 新增 `ListEvaluationRuns`，EvaluationService 接口同步扩展

**验证：** `go build ./...` 通过

## T8：Service 单测

**文件：** `internal/application/service/evaluation_persist_test.go`
**依赖：** T7
**步骤：**
1. mock repository 与周边 service
2. 覆盖：快照不含 API Key/endpoint（N6/AC7）；同配置两次 config_hash 相同（AC7）；创建即落库状态 pending（AC1）；执行失败落 failed+err_msg（AC3/AC11）；启动扫描语义（AC8）

**验证：** `go test ./internal/application/service/ -run Evaluation -race -v` 全过

## T9：列表端点与路由

**文件：** `internal/handler/evaluation.go`、`internal/router/routes_infra.go`
**依赖：** T7
**步骤：**
1. `GetEvaluationRuns` handler：绑定 `types.Pagination` + 可选 `status`，swagger 注释补齐
2. 路由组加 `GET /runs`，Viewer 可读

**验证：** `go build ./...` 通过

## T10：DI 与启动扫描

**文件：** `internal/container/container.go`
**依赖：** T7
**步骤：**
1. `container.Provide(NewEvaluationRunRepository)`
2. 迁移成功后调用 `MarkStaleInterrupted(time.Now().Add(-45*time.Second))`，错误仅记日志

**验证：** `go build ./...` 通过

## T11：全量回归

**依赖：** T1–T10
**步骤：**
1. `go test ./internal/...` 全过；`go vet ./internal/...`；有 golangci-lint 则跑

**验证：** 全绿，无新增警告

## 执行顺序

```
T1 ─ T2 ─ T3 ─ T4 ─ T5 ─┐
      └──── T6 ─────────┼─► T7 ─► T8 ─► T9 ─► T10 ─► T11
```

（T1、T6 与其他任务无依赖，可灵活穿插；T7 是唯一的大任务，预计 15–20 分钟，因为它同时改服务接口。）
