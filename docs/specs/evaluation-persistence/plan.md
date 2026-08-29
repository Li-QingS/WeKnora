# 评测运行持久化（课题三 WP1）Plan

> 状态：已批准（2026-08-29，逐段确认）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

把 `evaluation_runs` 表变成评测状态的唯一事实来源，内存 map 退役；服务启动时做一次"中断扫描"，运行期间由后台心跳维持活性证据。现有评测执行链路（goroutine + errgroup 并发跑样本）保持不动，只换掉它的状态读写出口。

```
POST /evaluation ──► EvaluationService.Evaluation()
                          │
                          ├─► 构建有效配置 + 版本签名 ──► 规范化 ──► config 快照 + 哈希
                          ├─► EvaluationRunRepository.Create()     ──► evaluation_runs（status=pending）
                          └─► 后台 goroutine：EvalDataset()
                                   │  每样本完成 ──► UpdateProgress()（状态/进度/指标落库）
                                   │  心跳 ticker ──► UpdateHeartbeat()（每 10s 一次）
                                   └─► 终态 ──► TransitionStatus(running→success/failed)

服务启动 ──► 迁移完成 ──► MarkStaleInterrupted()
                               （status∈{pending,running} 且心跳过期 → interrupted）

GET /evaluation?task_id=   ──► 查库（重启后仍可查）
GET /evaluation/runs（新） ──► 按租户分页列表，支持状态筛选
```

## 核心数据结构

### 表结构变更（`evaluation_runs` 扩列，方案 A：扩充现有未提交的 000088/000013）

| 列 | 类型（PG / SQLite） | 说明 |
|---|---|---|
| `heartbeat_at` | `TIMESTAMP` / `DATETIME`，可空 | 运行中心跳，启动扫描的判死依据 |
| `finished_at` | `TIMESTAMP` / `DATETIME`，可空 | 到达任一终态（success/failed/interrupted）的时间 |
| `config_hash` | `VARCHAR(64)` / `TEXT`，默认 `''` | 规范化配置的 SHA-256 hex |
| `config_snapshot` | `JSONB` / `TEXT`，默认 `'{}'` | 有效配置 + 版本签名（JSON） |
| `temporary_kb_id` | `VARCHAR(128)` / `TEXT`，默认 `''` | 评测创建的临时知识库，中断后便于追踪残留 |

本地开发库一次性修复：`DROP TABLE evaluation_runs` + migrate force 87，重启让迁移重跑（表为空，零数据风险）。SQLite lite 库同理。

### Go 类型（internal/types/evaluation.go 扩展）

```go
const (
    // ...现有 0-3 不变
    EvaluationStatueInterrupted EvaluationStatue = 4  // 服务重启导致的中断
)

type EvaluationRun struct {
    ID             string             `gorm:"primaryKey;type:varchar(128)"`
    TenantID       uint64             `gorm:"index"`
    DatasetID      string             `gorm:"type:varchar(128)"`
    Status         EvaluationStatue
    StartTime      time.Time
    ErrMsg         string
    Total          int
    Finished       int
    Params         json.RawMessage    `gorm:"type:jsonb;default:'{}'"`
    Metric         json.RawMessage    `gorm:"type:jsonb"`
    HeartbeatAt    *time.Time
    FinishedAt     *time.Time
    ConfigHash     string             `gorm:"type:varchar(64)"`
    ConfigSnapshot json.RawMessage    `gorm:"type:jsonb;default:'{}'"`
    TemporaryKBID  string             `gorm:"type:varchar(128)"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type EvaluationConfigSnapshot struct {
    Dataset  DatasetSnapshot   `json:"dataset"`
    Models   []ModelSnapshot   `json:"models"`
    Version  VersionSignature  `json:"version"`
}
type DatasetSnapshot  struct{ ID, SHA256 string; SampleCount int }
type ModelSnapshot    struct{ ID, Name, Provider, Type string }  // 无密钥/endpoint
type VersionSignature struct{ AppVersion, GitCommit string; GitDirty bool; GoVersion string }
```

config_hash 口径：`SHA-256(json.Marshal(有效ChatManage + Dataset快照 + Models快照))`，不含版本签名；struct 序列化字段序固定，天然规范化。

### Repository 接口（internal/types/interfaces/evaluation.go 扩展）

```go
type EvaluationRunRepository interface {
    Create(ctx context.Context, run *types.EvaluationRun) error
    GetByID(ctx context.Context, tenantID uint64, id string) (*types.EvaluationRun, error)
    List(ctx context.Context, tenantID uint64, status *types.EvaluationStatue,
         p *types.Pagination) ([]*types.EvaluationRun, int64, error)

    // 仅 status=running 时生效；终态后拒绝（保护历史不可变）
    UpdateProgress(ctx context.Context, id string, finished, total int,
                   metric json.RawMessage) error
    UpdateHeartbeat(ctx context.Context, id string, at time.Time) error
    SetDatasetHash(ctx context.Context, id string, sha256 string, samples int) error

    // CAS 状态迁移：仅当当前状态在 from 集合中才更新，返回是否生效
    TransitionStatus(ctx context.Context, id string, from []types.EvaluationStatue,
                     to types.EvaluationStatue, errMsg string) (bool, error)

    // 启动扫描：pending/running 且 heartbeat_at 过期或为 NULL → interrupted
    MarkStaleInterrupted(ctx context.Context, cutoff time.Time) (int64, error)
}
```

## 模块设计

### EvaluationRunRepository（新建，internal/application/repository/evaluation_run.go）

**职责：** `evaluation_runs` 表的全部读写，GORM 实现，双库兼容（JSON 字段沿用 `json.RawMessage` + `type:jsonb` 惯例，SQLite 自动落 TEXT）。
**依赖：** `*gorm.DB`。

### EvaluationService 改造（internal/application/service/evaluation.go）

**职责变化：** 状态读写出口从 `evaluationMemoryStorage` 换成 repository；`Evaluation()` 在默认值补全完成后构建配置快照并随创建一并落库；后台 goroutine 增加心跳 ticker 和终态落库；`EvaluationResult()` 改为查库（含租户校验，repo 层强制）；新增 `ListEvaluationRuns`。

### 中断恢复（container 启动流程）

**职责：** 迁移完成后，把 pending/running 且心跳过期（或为 NULL）的记录批量标记为 interrupted + finished_at。失败仅记日志，不阻断启动。
**挂载点：** `internal/container/container.go` 迁移成功之后。

### EvaluationHandler 扩展（internal/handler/evaluation.go）

新增 `GET /evaluation/runs`：复用 `types.Pagination`（page/page_size，默认 20）+ `status` 筛选。旧端点签名不动。RBAC 沿用现有 `/evaluation` 路由组（Admin 发起、Viewer 查询）。

### buildinfo（新建，internal/buildinfo/）

Version/CommitID/GitDirty/GoVersion 四个 ldflags 注入变量；dev 模式无注入时默认 `dev/unknown/true/unknown`。`scripts/get_version.sh` 增加 GIT_DIRTY 输出，Makefile ldflags 增加 buildinfo.* 注入（handler.* 保持不动）。

## 模块交互

发起评测：

```
handler.Evaluation
  → service.Evaluation()
      1. 现有逻辑不变：补全 KB / rerank / chat 默认值 → 有效 ChatManage
      2. 经 modelService 取三个模型的 name/provider/type（脱敏）→ ConfigSnapshot（dataset.hash 暂空）→ config_hash
      3. repository.Create(status=pending)              ← 先落库，再启 goroutine
      4. goroutine：
           TransitionStatus(pending→running)            ← CAS，失败则退出
           心跳 ticker（每 10s UpdateHeartbeat）
           EvalDataset()：
             加载数据集 → SetDatasetHash(sha256, 样本数)
             …现有并发评测逻辑不变…
             每样本完成 → UpdateProgress(finished, total, 实时指标)
           成功 → TransitionStatus(running→success, metric + finished_at)
           失败 → TransitionStatus(running→failed, err_msg + finished_at)
```

查询：`GET /evaluation?task_id` → `repo.GetByID(当前租户, id)` → 组装为现有 `EvaluationDetail` 响应形状；`GET /evaluation/runs` → `repo.List(当前租户, status?, pagination)`。

启动恢复：container 迁移成功后 `MarkStaleInterrupted(now - 45s)`。阈值 = 心跳间隔 10s 的 4 倍以上，容忍 GC/调度抖动。

## 文件组织

```
migrations/versioned/000088_evaluation_runs.{up,down}.sql   — 扩充：+5 列（修改现有未提交文件）
migrations/sqlite/000013_evaluation_runs.{up,down}.sql      — 同步扩充
internal/database/migration_sqlite_versioned_schema_test.go — 守卫测试登记表更新

internal/types/evaluation.go                 — EvaluationRun 模型、Interrupted 状态、Snapshot 类型
internal/types/interfaces/evaluation.go      — EvaluationRunRepository 接口 + 服务接口加 List
internal/application/repository/evaluation_run.go       — 新建：GORM 实现
internal/application/repository/evaluation_run_test.go  — 新建：SQLite 内存库单测（含租户隔离、CAS）
internal/application/service/evaluation.go   — 改造：接线 repository、快照、心跳、终态落库
internal/handler/evaluation.go               — 新增 GET /evaluation/runs handler
internal/router/routes_infra.go              — 注册列表路由（Viewer 可读）
internal/container/container.go              — 注册 repository + 迁移后中断扫描

internal/buildinfo/buildinfo.go              — 新建：版本信息变量
scripts/get_version.sh                       — 增加 GIT_DIRTY 输出
Makefile                                     — ldflags 增加 buildinfo.* 注入

docs/specs/evaluation-persistence/           — spec.md / plan.md / task.md / checklist.md
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 迁移方式 | 扩充现有 000088/000013 | 未提交未推送，仅本地库应用过；保持"一张表一个迁移" |
| 状态存储 | DB 为唯一事实来源，删除内存 map | 进度写频率低（每样本一次），直写开销可忽略；避免双写一致性问题 |
| 终态保护 | repo 层 CAS（`WHERE status IN (...)`） | 数据库层面保证不可变，不依赖应用层自觉 |
| 版本信息来源 | 新建 `internal/buildinfo` 包 + ldflags | 避免 service→handler 反向依赖；Makefile 双份注入成本极低 |
| config_hash 口径 | sha256(有效 ChatManage + 数据集 + 模型快照)，不含版本签名 | 代码升级不改变"同配置"的判定 |
| 心跳参数 | 间隔 10s、过期阈值 45s，代码常量 | spec 已定"不对用户暴露"；45s 容忍长时间 GC/调度抖动 |
| 列表分页 | 复用 `types.Pagination`（page/page_size，默认 20） | 与现有列表接口惯例一致 |
