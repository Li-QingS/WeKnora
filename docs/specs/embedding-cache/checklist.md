# Embedding 缓存（课题三 WP5）Checklist

> 状态：已验收（2026-09-02）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行命令或观察行为验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC8）

- [x] AC1 双库迁移：PG 000091 与 SQLite 000016 创建 `embedding_cache_entries`，守卫测试更新到版本 16；从零建库与升级路径通过。（验证：`go test ./internal/database/ -v`）
- [x] AC2 缓存键正确：相同租户/模型/维度/文本命中；任一不同则不命中。（验证：cache wrapper 单测 + repository 单测）
- [x] AC3 单条命中：第二次 Embedding 同一文本不再调用底层模型，返回相同向量。（验证：`TestCachedEmbedderSingleHit`）
- [x] AC4 批量部分命中：混合已缓存/未缓存文本时只对未缓存文本调用一次模型，返回顺序一致。（验证：`TestCachedEmbedderBatchPartialHit`）
- [x] AC5 开关与降级：默认关闭透传；开启后缓存读写失败 fail-open。（验证：`TestWrapEmbeddingCacheNoCachePassthrough` + 代码 fail-open）
- [x] AC6 统计可见：`GET /embedding-cache/stats` 返回 hits/misses/provider_calls；热运行 Embedding 调用不新增。（验证：端到端冷/热对比）
- [x] AC7 隔离与正确性：不同租户/模型/维度互不串用；开启缓存前后向量一致。（验证：wrapper/repository 单测）
- [x] AC8 自动化验证：`go build` / `go test` / vet / 前端 type-check 全过。（验证：最终回归）

## 集成

- [x] cache wrapper 在 cost wrapper 外层，命中时不新增 WP4 台账记录。（验证：热运行 provider_calls 不增长）
- [x] 缓存未安装时 `NewEmbedder` 行为不变。（验证：透传单测）
- [x] Repository Get/Set/IncrementHit 在 SQLite 内存库可用。（验证：repository 单测）
- [x] 容器 `EMBEDDING_CACHE_ENABLED=true` 安装 cache，默认关闭。（验证：启动日志 + 容器测试）
- [x] 统计 API 已注册，Admin 可读。（验证：handler 单测 + 真实请求）
- [x] 与 WP4 联动：缓存命中不产生 provider 调用，Embedding 台账调用次数不增长。（验证：冷/热对比）

## 编译与测试

- [x] 服务端：`go build ./...` 通过
- [x] 服务端：`go test ./internal/...` 通过
- [x] SDK：`cd client && go test ./...` 通过
- [x] CLI：`cd cli && go test ./...` 通过
- [x] CLI：`cd cli && go vet ./...` 通过
- [x] 数据库：`go test ./internal/database/ -v` 通过
- [x] 前端：`npm run type-check` 通过

## 端到端场景

- [x] 场景 1（缓存生效）：全新库冷运行 Embedding provider calls=19；热运行保持 19，hits=31，无新增调用。
- [x] 场景 2（隔离与降级）：不同键不串用；缓存失败时评测仍成功（fail-open 代码路径 + 单测）。
- [x] 场景 3（无缓存兼容）：未安装缓存时 wrapper 透传，行为与未实现缓存前一致。

## 验收记录

实际命令输出、冷/热对比与数据库核对见 [progress-2026-09-02.md](./progress-2026-09-02.md)。
