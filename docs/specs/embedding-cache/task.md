# Embedding 缓存（课题三 WP5）Tasks

> 状态：待审批（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）、[plan.md](./plan.md)（已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/types/embedding_cache.go` | Key / Entry / Stats |
| 新建 | `internal/types/interfaces/embedding_cache.go` | Repository 接口 |
| 新建 | `internal/models/embedding/cache.go` | 全局 cache manager + stats |
| 新建 | `internal/models/embedding/cache_wrapper.go` | cachedEmbedder |
| 新建 | `internal/models/embedding/cache_wrapper_test.go` | 包装层单测 |
| 修改 | `internal/models/embedding/embedder.go` | NewEmbedder 安装 cache wrapper |
| 新建 | `internal/application/repository/embedding_cache.go` | GORM 实现 |
| 新建 | `internal/application/repository/embedding_cache_test.go` | Repository 单测 |
| 新建 | `internal/handler/embedding_cache.go` | 统计 API |
| 新建 | `internal/handler/embedding_cache_test.go` | Handler 单测 |
| 修改 | `internal/router/routes_infra.go` | 注册统计路由 |
| 修改 | `internal/container/container.go` | Provide repo + env 开关 |
| 新建 | `migrations/versioned/000091_embedding_cache_entries.up.sql` / `.down.sql` | PG 表 |
| 新建 | `migrations/sqlite/000016_embedding_cache_entries.up.sql` / `.down.sql` | SQLite 表 |
| 修改 | `internal/database/migration_sqlite_versioned_schema_test.go` | 守卫测试 |
| 新建 | `docs/specs/embedding-cache/progress-2026-09-02.md` | 实现与验收记录 |

## 执行顺序

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8
```

T1/T2 是类型与数据，T3/T4 是缓存包装层与接线，T5 是统计 API，T6 是迁移，T7 是端到端，T8 是回归。

---

## T1: 缓存类型与接口

**文件：** 新建 `internal/types/embedding_cache.go`、`internal/types/interfaces/embedding_cache.go`
**依赖：** 无
**步骤：**
1. 定义 `EmbeddingCacheKey`、`EmbeddingCacheEntry`、`EmbeddingCacheStats`，字段与 [plan.md](./plan.md) 一致。
2. `EmbeddingCacheEntry` 加 GORM tag 与唯一约束 `(tenant_id, model_id, dimension, text_hash)`。
3. 定义 `interfaces.EmbeddingCacheRepository`。

**验证：**
```bash
go test ./internal/types/...
```

---

## T2: Repository

**文件：** 新建 `internal/application/repository/embedding_cache.go`、`internal/application/repository/embedding_cache_test.go`
**依赖：** T1
**步骤：**
1. `Get`：按四字段精确查询，JSON 数组 ↔ `[]float32`。
2. `Set`：唯一约束冲突时更新向量与 `updated_at`。
3. `IncrementHit`：`hits + 1`。
4. SQLite 内存库单测：写入/读取/更新/命中计数/唯一约束。

**验证：**
```bash
go test ./internal/application/repository/ -run EmbeddingCache
```

---

## T3: cache manager 与 cachedEmbedder

**文件：** 新建 `internal/models/embedding/cache.go`、`internal/models/embedding/cache_wrapper.go`、`internal/models/embedding/cache_wrapper_test.go`
**依赖：** T1
**步骤：**
1. `SetEmbeddingCache` / `GetEmbeddingCache` / `CacheStats` / `ResetCacheStats`。
2. `cachedEmbedder`：
   - `Embed`：命中计数并返回；未命中调用 inner 后写缓存；
   - `BatchEmbed`：逐条 Get，未命中合并调用，按原顺序合并；
   - `BatchEmbedWithPool`：把自身作为 pool model 传下去；
   - `cacheKey`：tenant 优先 context，缺省用模型 TenantID，TextHash 为 SHA-256。
3. 单测：单条命中、批量部分命中、顺序保持、未安装 cache 时透传。

**验证：**
```bash
go test ./internal/models/embedding/ -run Cache
```

---

## T4: 安装与容器开关

**文件：** 修改 `internal/models/embedding/embedder.go`、`internal/container/container.go`
**依赖：** T2、T3
**步骤：**
1. `NewEmbedder`：若 `GetEmbeddingCache() != nil`，在 concurrency 之前包 `cachedEmbedder`。
2. 容器 Provide `repository.NewEmbeddingCacheRepository`。
3. `EMBEDDING_CACHE_ENABLED=true` 时安装 cache manager；默认关闭；失败只记日志。
4. 容器测试通过。

**验证：**
```bash
go build ./...
go test ./internal/container/
```

---

## T5: 统计 API

**文件：** 新建 `internal/handler/embedding_cache.go`、`internal/handler/embedding_cache_test.go`，修改 `internal/router/routes_infra.go`
**依赖：** T3
**步骤：**
1. `GET /api/v1/embedding-cache/stats` 返回 `CacheStats()`。
2. 路由使用 Admin 权限。
3. Handler 单测：命中/未命中计数正确返回。

**验证：**
```bash
go test ./internal/handler/ -run EmbeddingCache
```

---

## T6: 双库迁移

**文件：** 新建 PG 000091、SQLite 000016，修改守卫测试
**依赖：** T1
**步骤：**
1. 建表：`embedding_cache_entries`，唯一约束四字段，vector 用 JSONB/TEXT。
2. `versionedSQLiteTables` 增加表，`expectedSQLiteMigrationVersion = 16`。
3. 运行数据库测试。

**验证：**
```bash
go test ./internal/database/ -v
```

---

## T7: 端到端验收

**文件：** 新建 `docs/specs/embedding-cache/progress-2026-09-02.md`
**依赖：** T1-T6
**步骤：**
1. 用 mock OpenAI 服务或真实模型，开启 `EMBEDDING_CACHE_ENABLED=true`。
2. 第一次 `make eval-baseline`：Embedding 调用落库；
3. 第二次 `make eval-baseline`：缓存命中，WP4 台账中 Embedding 调用次数下降；
4. 验证 `embedding-cache/stats` 命中/未命中计数；
5. 记录命令、数字与数据库核对到 progress 文档。

**验证：**
```bash
python3 - <<'PY'
import sqlite3
con = sqlite3.connect('data/e2e-wp5.db')
print(con.execute('select count(*) from embedding_cache_entries').fetchone())
print(con.execute('select count(*) from model_call_records where model_type="Embedding"').fetchone())
PY
```

---

## T8: 最终回归

**验证：**
```bash
go build ./...
go test ./internal/...
cd client && go test ./...
cd cli && go test ./... && go vet ./...
cd frontend && npm run type-check
```

并在 `progress-2026-09-02.md` 记录结果。
