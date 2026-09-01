# Embedding 缓存（课题三 WP5）Plan

> 状态：待审批（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

WP5 在 Embedding 客户端层插入一个 cache wrapper：

```
业务调用
  → cost wrapper
  → langfuse/debug
  → cache wrapper（命中直接返回）
  → concurrency + provider（未命中才走）
```

缓存数据落在数据库表 `embedding_cache_entries`，键为“租户 + 模型 + 维度 + 文本 SHA-256”。容器启动时根据环境变量决定是否安装缓存；未安装时 wrapper 不存在，行为与现在完全一致。

## 核心数据结构

### types.EmbeddingCacheKey

```go
type EmbeddingCacheKey struct {
    TenantID  uint64
    ModelID   string
    Dimension int
    TextHash  string
}
```

### types.EmbeddingCacheEntry（DB）

```go
type EmbeddingCacheEntry struct {
    ID        string          `gorm:"primaryKey;type:varchar(64)"`
    TenantID  uint64          `gorm:"index"`
    ModelID   string          `gorm:"type:varchar(128);index"`
    Dimension int             `gorm:"index"`
    TextHash  string          `gorm:"type:varchar(64)"`
    Vector    json.RawMessage `gorm:"type:jsonb"`
    Hits      int64           `gorm:"default:1"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

// 唯一约束：(tenant_id, model_id, dimension, text_hash)
```

### types.EmbeddingCacheStats

```go
type EmbeddingCacheStats struct {
    Hits   int64 `json:"hits"`
    Misses int64 `json:"misses"`
}
```

## 接口变更

### interfaces.EmbeddingCacheRepository

```go
type EmbeddingCacheRepository interface {
    Get(ctx context.Context, key *types.EmbeddingCacheKey) ([]float32, bool, error)
    Set(ctx context.Context, key *types.EmbeddingCacheKey, vector []float32) error
    IncrementHit(ctx context.Context, key *types.EmbeddingCacheKey) error
}
```

### embedding.EmbeddingCache（包装层使用的窄接口）

```go
type EmbeddingCache interface {
    Get(ctx context.Context, key types.EmbeddingCacheKey) ([]float32, bool, error)
    Set(ctx context.Context, key types.EmbeddingCacheKey, vector []float32) error
    IncrementHit(ctx context.Context, key types.EmbeddingCacheKey) error
}
```

## 模块设计

### embedding cache manager

**文件：** `internal/models/embedding/cache.go`

```go
func SetEmbeddingCache(c EmbeddingCache)
func GetEmbeddingCache() EmbeddingCache
func CacheStats() types.EmbeddingCacheStats
func ResetCacheStats()
```

全局单例 + 互斥锁；命中/未命中用 `atomic.Int64` 计数。

### cachedEmbedder

**文件：** `internal/models/embedding/cache_wrapper.go`

```go
type cachedEmbedder struct {
    inner    Embedder
    cache    EmbeddingCache
    tenantID uint64
}
```

- `Embed`：构造 key → Get；命中计数并返回；未命中调用 inner 后 Set。
- `BatchEmbed`：逐条 Get；未命中文本合并调用 inner.BatchEmbed；按原顺序合并；写缓存。
- `BatchEmbedWithPool`：把自身作为 pool model 传下去，让子批次也走缓存。
- `cacheKey(ctx, modelID, dimension, text)`：tenant 优先 context，缺省用模型 TenantID；TextHash 为 SHA-256 hex。

### Repository

**文件：** `internal/application/repository/embedding_cache.go`

- `Get`：按四字段精确查；向量以 JSON 数组读写。
- `Set`：`(tenant_id, model_id, dimension, text_hash)` 冲突时更新向量与时间。
- `IncrementHit`：`hits = hits + 1`。

### Stats API

**文件：** `internal/handler/embedding_cache.go`

```text
GET /api/v1/embedding-cache/stats
→ {"success":true,"data":{"hits":N,"misses":M}}
```

路由使用 Admin 权限。

### 容器与开关

`internal/container/container.go`：

- Provide `repository.NewEmbeddingCacheRepository`；
- `EMBEDDING_CACHE_ENABLED=true` 时 `embedding.SetEmbeddingCache(repo)`；
- 默认关闭；接线失败只记日志。

## 模块交互

```
NewEmbedder
  → 若全局 cache 存在：cachedEmbedder
  → wrapEmbeddingConcurrency
  → debug / langfuse / cost wrapper

Embed(text)
  → cache.Get(key)
      ├─ hit → 返回缓存向量
      └─ miss → provider.Embed → cache.Set → 返回

BatchEmbed(texts)
  → 逐条 cache.Get
  → 只对 miss 集合调用 provider.BatchEmbed
  → 合并结果并写缓存

GET /embedding-cache/stats → CacheStats()
```

## 文件组织

```text
internal/types/embedding_cache.go
internal/types/interfaces/embedding_cache.go
internal/models/embedding/cache.go
internal/models/embedding/cache_wrapper.go
internal/models/embedding/cache_wrapper_test.go
internal/application/repository/embedding_cache.go
internal/application/repository/embedding_cache_test.go
internal/handler/embedding_cache.go
internal/handler/embedding_cache_test.go
internal/router/routes_infra.go
internal/container/container.go
migrations/versioned/000091_embedding_cache_entries.up.sql / .down.sql
migrations/sqlite/000016_embedding_cache_entries.up.sql / .down.sql
internal/database/migration_sqlite_versioned_schema_test.go
docs/specs/embedding-cache/...
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 缓存后端 | 数据库表 | Lite/标准模式一致，可持久，复用双库迁移体系 |
| 缓存键 | tenant + model + dimension + text_hash | 满足租户隔离与模型维度隔离 |
| 安装位置 | NewEmbedder 内、concurrency 之前 | 命中时连并发闸都不占用，收益最大 |
| 默认开关 | 关闭，`EMBEDDING_CACHE_ENABLED=true` 开启 | 避免未评估前改变行为 |
| 失败策略 | fail-open，只记日志 | 缓存故障不影响业务 |
| 统计 | 进程级 atomic 计数 + API | 简单可演示，不引入趋势表 |
| 批量 | 部分命中合并调用 | 命中越多，provider 调用越少 |

## Spec 覆盖自检

| Spec 需求 | Plan 归属 |
|-----------|-----------|
| F1 缓存键 | `EmbeddingCacheKey` + `cacheKey` |
| F2 缓存读写 | `EmbeddingCacheRepository` + 表 |
| F3 包装层 | `cachedEmbedder` |
| F4 开关与降级 | 容器 env + fail-open |
| F5 可观测统计 | `CacheStats` + API |
| F6 双库迁移 | PG 000091 + SQLite 000016 |
| N1-N5 | 键隔离、租户过滤、低侵入、测试、无清理 |
