# Embedding 缓存按模型展示与本地开启记录（2026-09-02）

## 背景修正

之前的“Embedding 缓存”是进程内一个 cache manager，但缓存键本身是
`tenant_id + model_id + dimension + text_hash`，即不同模型不会互相复用向量。
页面之前只展示全局 hits / misses / provider_calls，所以看起来像所有模型共用
一个缓存，这是统计粒度造成的误解。

## 本次补充

- `GET /api/v1/embedding-cache/stats` 增加 `enabled` 与 `models`：
  - 每个模型返回 `hits / misses / provider_calls`；
  - `hits + misses` 是文本查找次数，`provider_calls` 是真正发给模型服务的请求数。
- “模型用量”页：
  - 缓存区块显示“已开启 / 未开启”；
  - 开启后按模型列出命中、未命中、模型请求与命中率；
  - 汇总表新增“缓存命中率”列。
- 本地 `.env` 开启 `EMBEDDING_CACHE_ENABLED=true`；默认仍是关闭，
  避免未做缓存实验的环境改变行为。

## 验证

- `go test ./internal/models/embedding/ -run Cache`
- `go test ./internal/handler/ -run EmbeddingCache`
- `cd frontend && npm run type-check`

需要重启 `make dev-app` 后，新进程才会安装 embedding cache 并开始按模型统计。

## 页面口径修正（同日）

模型用量页重新组织为：

- 主表直接展示每个模型的调用次数、成功/失败、输入 Token、输出 Token、
  Chat 缓存命中率、Embedding 向量复用/新增与估算费用；
- “缓存命中率”只用于 Chat 模型，来源是上游返回的
  `cache_read_tokens / (cache_read_tokens + cache_miss_tokens)`；
- Embedding 不再使用“缓存命中率”说法，改为展示“复用次数 / 新增次数”：
  相同租户、相同模型、相同维度的相同文本直接复用已有向量，不触发模型调用；
- 最近调用表展示输入/输出 Token 与缓存读 Token，作为台账明细证据。

## 模型用量筛选（同日补充）

页面顶部增加模型下拉框、日期区间、查询与重置按钮。

- 查询会把 `model_id`、`from`、`to` 传给：
  - `GET /api/v1/model-calls/summary`
  - `GET /api/v1/model-calls`
- 主表与最近调用明细按所选模型和时间区间从 `model_call_records` 重新聚合；
- Chat 缓存命中率来自台账中的 cache token；
- Embedding 向量复用统计来自进程内缓存，不受时间区间筛选影响。

### 分页补充

- “各模型用量”提供独立分页与页码跳转，翻页只切换汇总展示；
- “最近模型调用”同样提供分页与页码跳转，翻页只重新拉取当前筛选条件下
  的调用明细，不再连带重载汇总主表。
