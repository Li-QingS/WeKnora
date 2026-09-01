# 模型调用台账与费用估算（课题三 WP4）Plan

> 状态：待审批（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

WP4 由五部分组成：

1. **全局记账 Recorder**：镜像 Langfuse manager 的全局单例模式，模型包装层统一调用 `costledger.Record(...)`，业务代码零改动。
2. **模型包装层**：在 Chat / Embedding / Rerank / VLM / ASR 五个模型客户端外各加一层 cost wrapper，调用结束后收集 Token/状态/耗时并上报。
3. **Repository**：`model_call_records` 与 `model_prices` 两张表的 GORM 实现，负责写入、筛选、分页、汇总。
4. **费用计算**：调用时读取价格配置，生成价格快照并计算估算费用；未知价格记为 `null`。
5. **API + 最小页面**：明细/汇总/价格 API 与一个“模型用量”页面。

```
模型业务调用
  → cost wrapper（外层）
      → 真实模型调用
      → costledger.Record(ctx, ModelCallInfo)
          → ModelCallRecorder
              → model_prices 取价格
              → 计算费用 + 价格快照
              → model_call_records 落库
```

## 核心数据结构

### types.ModelCallInfo（包装层上报）

```go
type ModelCallInfo struct {
    TenantID         uint64
    ModelID          string
    ModelName        string
    ModelType        string // string(types.ModelType)
    Purpose          string
    Status           ModelCallStatus
    StartedAt        time.Time
    FinishedAt       time.Time
    DurationMS       int64
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CacheReadTokens  int
    CacheWriteTokens int
    CacheMissTokens  int
    UnitType         string
    UnitCount        int64
    ErrorType        string
    ErrorMessage     string
    SessionID        string
    UserID           string
    PrincipalType    string
    PrincipalID      string
    RequestGroupID   string
    TraceID          string
}
```

### types.ModelCallRecord（DB）

```go
type ModelCallRecord struct {
    ID               string          `gorm:"primaryKey"`
    TenantID         uint64          `gorm:"index"`
    ModelID          string          `gorm:"index"`
    ModelName        string
    ModelType        string
    Purpose          string
    Status           string
    StartedAt        time.Time
    FinishedAt       time.Time
    DurationMS       int64
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CacheReadTokens  int
    CacheWriteTokens int
    CacheMissTokens  int
    UnitType         string
    UnitCount        int64
    ErrorType        string
    ErrorMessage     string
    SessionID        string
    UserID           string
    PrincipalType    string
    PrincipalID      string
    RequestGroupID   string
    TraceID          string
    EstimatedCostUSD *float64         `gorm:"type:decimal(20,8)"`
    PriceSnapshot    json.RawMessage  `gorm:"type:jsonb"`
    CreatedAt        time.Time
}
```

### types.ModelPrice（DB）

```go
type ModelPrice struct {
    ID                        string    `gorm:"primaryKey"`
    TenantID                  uint64    `gorm:"index"` // 0 = 全局默认
    ModelID                   string    `gorm:"index"`
    InputPricePerMillion      *float64
    OutputPricePerMillion     *float64
    CacheReadPricePerMillion  *float64
    CacheWritePricePerMillion *float64
    UnitType                  string
    UnitPrice                 *float64
    Currency                  string
    UpdatedBy                 string
    CreatedAt                 time.Time
    UpdatedAt                 time.Time
}

// 唯一约束：(tenant_id, model_id)
```

### types.ModelCallSummaryItem（API）

```go
type ModelCallSummaryItem struct {
    ModelID            string   `json:"model_id"`
    ModelName          string   `json:"model_name"`
    ModelType          string   `json:"model_type"`
    Calls              int64    `json:"calls"`
    SuccessCount       int64    `json:"success_count"`
    FailedCount        int64    `json:"failed_count"`
    PromptTokens       int64    `json:"prompt_tokens"`
    CompletionTokens   int64    `json:"completion_tokens"`
    TotalTokens        int64    `json:"total_tokens"`
    CacheReadTokens    int64    `json:"cache_read_tokens"`
    CacheWriteTokens   int64    `json:"cache_write_tokens"`
    EstimatedCostUSD   *float64 `json:"estimated_cost_usd"`
}
```

## 接口变更

### interfaces.ModelCallRepository

```go
type ModelCallRepository interface {
    Create(ctx context.Context, record *types.ModelCallRecord) error
    List(ctx context.Context, tenantID uint64, filter *types.ModelCallFilter,
        p *types.Pagination) ([]*types.ModelCallRecord, int64, error)
    Summary(ctx context.Context, tenantID uint64,
        filter *types.ModelCallFilter) ([]*types.ModelCallSummaryItem, error)
}

type ModelPriceRepository interface {
    Get(ctx context.Context, tenantID uint64, modelID string) (*types.ModelPrice, error)
    Upsert(ctx context.Context, price *types.ModelPrice) error
    List(ctx context.Context, tenantID uint64) ([]*types.ModelPrice, error)
}
```

### interfaces.ModelCallService

```go
type ModelCallService interface {
    List(ctx context.Context, filter *types.ModelCallFilter,
        p *types.Pagination) (*types.PageResult, error)
    Summary(ctx context.Context, filter *types.ModelCallFilter) ([]*types.ModelCallSummaryItem, error)
    UpsertPrice(ctx context.Context, price *types.ModelPrice) error
    ListPrices(ctx context.Context) ([]*types.ModelPrice, error)
}
```

## 模块设计

### costledger.Recorder

**职责：** 全局记账入口。
**文件：** `internal/costledger/recorder.go`

```go
type Recorder interface {
    Record(ctx context.Context, info *types.ModelCallInfo) error
}

func SetRecorder(r Recorder)
func GetRecorder() Recorder
func Record(ctx context.Context, info *types.ModelCallInfo) error // recorder nil 时 no-op
```

包级互斥锁保护单例，与 Langfuse `GetManager()` 模式一致；测试可替换/重置。

### 模型 cost wrapper

每个模型包新增一个 cost wrapper 并在构造时安装：

| 包 | 文件 | 包装接口 | 采集内容 |
|---|---|---|---|
| chat | `cost_wrapper.go` | Chat / ChatStream | 最终 Usage，流式结束时取最后一个 usage |
| embedding | `cost_wrapper.go` | Embed / BatchEmbed | 近似 Token（沿用 Langfuse 估算口径） |
| rerank | `cost_wrapper.go` | Rerank | 近似 Token + `unit_type=documents, unit_count=len(documents)` |
| vlm | `cost_wrapper.go` | Predict | `unit_type=requests, unit_count=1` |
| asr | `cost_wrapper.go` | Transcribe | `unit_type=requests, unit_count=1` |

wrapper 内统一：

- 读取 `TenantIDFromContext` / `SessionIDFromContext` / `PrincipalFromContext` / `LLMCallMetadataFromContext`；
- 缺少租户时回退到模型配置里的 `TenantID`（Config 结构新增字段，`ConfigFromModel` 填充）；
- 调用 `costledger.Record`，失败只记日志，不影响模型调用结果。

安装顺序：真实 provider → concurrency → debug → langfuse → **cost（最外层）**，使 cost 覆盖完整调用与流式消费。

### Repository

**文件：** `internal/application/repository/model_call.go`

- `List`：`tenant_id` 必带，支持 `model_id`、`model_type`、`status`、`created_at >= from`、`created_at <= to`，按 `created_at DESC` 分页。
- `Summary`：按 `model_id/model_name/model_type` 分组，聚合调用次数、成功/失败数、Token、cache token、估算费用。
- `Upsert`：`(tenant_id, model_id)` 冲突时更新。

### ModelCallRecorder 与费用计算

**文件：** `internal/application/service/model_cost.go`

```go
type modelCallRecorder struct {
    calls  interfaces.ModelCallRepository
    prices interfaces.ModelPriceRepository
}

func (r *modelCallRecorder) Record(ctx context.Context, info *types.ModelCallInfo) error
```

流程：

1. 查租户价格，查不到再查全局价格（tenant_id=0）；
2. 组装 `PriceSnapshot`；
3. 按公式计算估算费用；价格缺失则 `EstimatedCostUSD=nil`；
4. 组装 `ModelCallRecord` 并落库。

### Handler 与路由

**文件：** `internal/handler/model_call.go`、`internal/router/routes_infra.go`

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/model-calls` | 明细分页，支持筛选 |
| GET | `/api/v1/model-calls/summary` | 汇总 |
| GET | `/api/v1/model-prices` | 价格列表 |
| GET | `/api/v1/model-prices/:modelId` | 单模型价格 |
| PUT | `/api/v1/model-prices/:modelId` | 写入/更新价格 |

权限：Admin（模型管理区域）。

### 前端最小页面

**文件：**

```text
frontend/src/api/model/usage.ts
frontend/src/views/system/ModelUsage.vue
frontend/src/router/...（注册路由）
frontend/src/components/settings/...（入口菜单）
```

页面内容：

- 汇总表：按模型显示调用次数、成功/失败、Token、估算费用；
- 明细表：最近调用，分页；
- 费用为空显示 `unknown`。

### 双库迁移

```text
migrations/versioned/000089_model_call_records.{up,down}.sql
migrations/versioned/000090_model_prices.{up,down}.sql
migrations/sqlite/000014_model_call_records.{up,down}.sql
migrations/sqlite/000015_model_prices.{up,down}.sql
```

更新 `internal/database/migration_sqlite_versioned_schema_test.go`：

- `versionedSQLiteTables` 增加 `model_call_records`、`model_prices`；
- `expectedSQLiteMigrationVersion = 15`。

### 容器接线

`internal/container/container.go`：

- Provide `repository.NewModelCallRepository`、`repository.NewModelPriceRepository`、`service.NewModelCallService`；
- 启动时 `Invoke` 构造 `ModelCallRecorder` 并 `costledger.SetRecorder(...)`；
- 接线失败只记日志，不阻断启动。

## 模块交互

```
NewChat / NewEmbedder / NewReranker / NewVLM / NewASR
  → cost wrapper
      → inner model call
      → costledger.Record(ctx, info)
          → ModelCallRecorder
              → ModelPriceRepository.Get (tenant → global)
              → PriceSnapshot + cost
              → ModelCallRepository.Create

GET /api/v1/model-calls
  → ModelCallService.List
      → ModelCallRepository.List(tenant scoped)

GET /api/v1/model-calls/summary
  → ModelCallService.Summary
      → ModelCallRepository.Summary(tenant scoped)
```

## 文件组织

```text
internal/types/model_call.go
internal/types/interfaces/model_call.go
internal/costledger/recorder.go
internal/models/chat/cost_wrapper.go
internal/models/embedding/cost_wrapper.go
internal/models/rerank/cost_wrapper.go
internal/models/vlm/cost_wrapper.go
internal/models/asr/cost_wrapper.go
internal/application/repository/model_call.go
internal/application/repository/model_call_test.go
internal/application/service/model_cost.go
internal/application/service/model_cost_test.go
internal/handler/model_call.go
internal/handler/model_call_test.go
internal/router/routes_infra.go
internal/container/container.go
migrations/versioned/000089_model_call_records.*
migrations/versioned/000090_model_prices.*
migrations/sqlite/000014_model_call_records.*
migrations/sqlite/000015_model_prices.*
frontend/src/api/model/usage.ts
frontend/src/views/system/ModelUsage.vue
docs/specs/cost-ledger/...
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 记账入口 | 全局 `costledger.Recorder` | 镜像 Langfuse manager，模型包装层与业务代码解耦 |
| 包装层位置 | cost wrapper 最外层 | 覆盖完整调用与流式消费，记录真实端到端耗时 |
| 价格 | `model_prices` 表 + 调用时快照 | 历史费用不随改价变化 |
| 未知费用 | `EstimatedCostUSD=nil` | 避免把 unknown 误显示为 0 |
| 写入时机 | 调用结束后同步 insert | 先保证正确与可测；基准显示明显开销再异步化 |
| 租户回退 | ctx tenant 优先，模型 TenantID 兜底 | 后台任务缺少租户上下文时仍能归属 |
| 权限 | Admin 可读可配 | 与模型管理区域一致 |
| 页面 | 汇总表 + 明细表 | 满足材料验收，不引入图表复杂度 |

## Spec 覆盖自检

| Spec 需求 | Plan 归属 |
|-----------|-----------|
| F1 调用记录 | `ModelCallRecord` + Repository |
| F2 统一记账 | cost wrapper × 5 + `costledger.Record` |
| F3 价格配置 | `ModelPrice` + 价格 API |
| F4 费用估算 | `ModelCallRecorder` + PriceSnapshot |
| F5 明细 API | `GET /model-calls` |
| F6 汇总 API | `GET /model-calls/summary` |
| F7 价格 API | `GET/PUT /model-prices` |
| F8 最小页面 | `ModelUsage.vue` |
| F9 双库迁移 | PG 000089/000090 + SQLite 000014/000015 |
| N1-N6 | 租户过滤、无 Prompt/Key、同步落库、双库、测试、no-op 降级 |
