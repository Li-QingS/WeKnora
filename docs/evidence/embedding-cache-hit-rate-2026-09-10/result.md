# Embedding 缓存命中率优化结果

## 结论

本轮保留 PostgreSQL/SQLite 作为持久缓存，在应用进程中加入保守文本规范化、旧键惰性提升、Batch 规范去重和批量感知的并发请求合并。没有新增数据库迁移，也没有把 Embedding 向量写入当前共享 Redis。

在项目内置 EnterpriseRAG 数据的固定格式漂移场景中，命中率从 **33.33% 提高到 66.67%**，增加 **33.33 个百分点**；Provider 输入从 270 降至 135，减少 **50.00%**。32 个并发相同冷请求从 32 次 Provider 调用压缩为 1 次，减少 **96.875%**；16 个并发的双输入 Batch 从 16 次批量调用压缩为 1 次，减少 **93.75%**。

这些数字来自固定 benchmark 和竞态测试，表示对应工作负载的改进，不等同于对未来线上流量的预测。上线后的真实新增收益可以通过模型用量页面的“格式归一命中”和“请求合并”直接观察。

## 优化作用在哪个阶段

这一轮提升的是 **Embedding 向量生成阶段**。全局缓存包装器安装在配置模型的 `Embed`/`BatchEmbed` 调用外层，因此覆盖：

- 文档入库、重建索引时的 Chunk 向量化；
- RAG 查询时的 Query 向量化；
- Wiki 分类、Wiki 评测等复用同一 Embedding 模型的向量计算。

它不缓存大模型生成的 Wiki 页面、实体/概念候选或引用抽取结果。Wiki LLM 生成阶段的 Prompt Cache 是后一项独立优化，读取复用率按 `cache_read_tokens / prompt_tokens` 统计；显式缓存写入也包含在 `prompt_tokens` 中。

## 模型用量页面的统计口径

模型用量页面主指标现为 **文档向量化复用（进程内累计）**，只统计文档上传、重解析、重建索引以及 FAQ 入库时的 Chunk/FAQ Embedding。Wiki 评测用于实体与概念语义对齐的标签 Embedding、RAG 查询向量和模型连接测试不计入该复用率，避免一次性评测标签稀释文档缓存效果。

缓存能力仍对所有阶段生效，分类只影响观测统计，不参与缓存键计算，因此不会阻止不同阶段复用同一个向量。分用途计数从本次版本启动后开始累计，服务重启后清零；SQL 中的向量和历史 `hits` 继续持久保存。本文“开发前真实数据基线”是 SQL 全阶段历史数据，不能与页面的文档向量化进程指标直接比较。

## 修改位置

| 位置 | 修改内容 |
|---|---|
| `internal/models/embedding/cache_text.go` | 定义保守的文本规范化规则 |
| `internal/models/embedding/cache_wrapper.go` | 规范键查询、旧键惰性提升、Batch 去重、Provider 回填和异常降级 |
| `internal/models/embedding/cache_flight.go` | 合并同进程并发冷请求，并让等待者各自响应 Context 取消 |
| `internal/models/embedding/cache.go`、`internal/types/embedding_cache.go` | 增加格式归一命中、合并请求和 Provider 调用统计 |
| `frontend/src/views/settings/ModelUsageSettings.vue` | 在模型用量页面展示新增缓存指标 |
| `internal/models/embedding/cache_workload.go` | 标记文档索引、Wiki 评测和其他 Embedding 调用用途 |
| `internal/application/service/retriever/keywords_vector_hybrid_indexer.go` | 将索引阶段归入文档向量化统计 |
| `internal/application/service/wiki_evaluation_embedding.go` | 将语义标签向量归入 Wiki 评测统计并从页面主指标排除 |
| `cmd/embedding-cache-benchmark/main.go` | 固定数据集复现工具 |

本轮核心代码提交为 `13ea76a3`，没有数据库迁移。
后续的统计口径修正同样没有数据库迁移，只增加进程内用途计数与页面筛选。

## 修改前后对比

| 场景 | 修改前 | 修改后 | 提升 |
|---|---:|---:|---:|
| 固定格式漂移数据命中率 | 33.33% | 66.67% | +33.33 个百分点 |
| 同场景 Provider 输入数 | 270 | 135 | -50.00% |
| 32 个并发相同冷请求的 Provider 调用 | 32 | 1 | -96.875% |
| 16 个并发双输入 Batch 的 Provider 调用 | 16 | 1 | -93.75% |

这里的“修改前”由 benchmark 关闭新增规范化、Batch 去重和并发合并能力后运行同一批输入得到；“修改后”开启这些能力。开发库 56.09% 的历史持久复用占比是长期 SQL 基线，不应与这组受控对照直接相减。

## 开发前真实数据基线

2026-09-10 对本机开发 PostgreSQL 执行只读统计：

| 指标 | 数值 |
|---|---:|
| 缓存记录 | 11,119 |
| 累计使用次数 `sum(hits)` | 25,323 |
| 已持久化复用次数 `sum(max(hits-1, 0))` | 14,204 |
| 历史持久复用占比 | 56.09% |
| 仅使用一次的记录 | 7,227（65.00%） |
| 表与索引总大小 | 98,508,800 bytes（约 93.95 MiB） |

历史数据证明 SQL 持久缓存已有实际收益，同时也说明存在大量低复用键。格式差异和并发冷启动是本轮直接处理的两个漏命中来源。

## 方案

1. **保守规范化**：统一 UTF-8 BOM、Unicode NFC、CRLF/CR、文本首尾空白和行尾水平空白；保留正文内部空格、空行数量、Markdown 与代码缩进。
2. **键和值一致**：新生成的向量统一使用规范文本作为 Provider 输入；缓存键表示一个保守的格式等价类。
3. **兼容旧缓存**：规范键未命中时查询原始精确键；旧向量只在原文与规范文本属于上述格式等价类时提升到规范键，逐次完成惰性迁移，无需清库或离线迁移。
4. **Batch 去重**：一个 Batch 内的相同文本和格式变体只保留一个 Provider 输入，结果按原始顺序复制回各位置。
5. **并发合并**：用进程级、批量感知的 flight registry 原子认领一组键。同一进程内同时到达的相同冷请求共享一次 Provider 结果，同时仍能把多个不同缺失键放在一个 Provider Batch 中。
6. **可观测性**：接口和模型用量页面新增“格式归一命中”“请求合并”“有效复用率”，可区分 SQL 命中、规范化收益和并发收益。
7. **故障降级**：缓存读写错误不返回给业务调用方，仍调用 Provider；等待者尊重自己的 Context 取消；各调用方获得独立向量切片。

## 为什么继续使用 SQL

当前缓存需要跨重启持久化，并已经保存约 94 MiB 数据。现有 Redis 同时承载队列、流和 Langfuse，未设置 Embedding 专用容量、淘汰策略与 TTL。直接迁移到共享 Redis 会增加内存挤压和不可预测淘汰风险，也不会从根本上提高由键格式差异造成的命中率。

因此本轮采用“SQL 持久层 + 进程内瞬时合并层”。后续数据量继续增长时，应先增加 SQL 配额/保留策略和批量读写；只有在监控证明数据库读延迟成为主要瓶颈后，再增加有独立内存预算的 Redis L1，而不是替换 SQL。

## 复现

```bash
go run ./cmd/embedding-cache-benchmark \
  -dataset dataset/enterprise_rag \
  -output-dir docs/evidence/embedding-cache-hit-rate-2026-09-10

go test -race ./internal/models/embedding \
  -run 'TestCachedEmbedderCoalescesConcurrent(ColdRequests|Batches)$' \
  -count=1 -v
```

固定数据结果见 `benchmark.json`、`benchmark.md` 和 `concurrency-test.txt`。

## 验证

- 后端：`go test ./...`、`go vet ./...`、`go build ./...`。
- 并发：`go test -race ./internal/models/embedding -count=1`。
- Client 与 CLI：各自执行 `go test -count=1 ./...` 和 `go vet ./...`。
- 前端：781 项测试、`npm run type-check`、`npm run build`、`npm run check-i18n`。

## 回退

远端 annotated tag `cache-hit-baseline-20260910` 指向优化前提交 `e4c51bf8`。查看或建立回退分支：

```bash
git switch --detach cache-hit-baseline-20260910
git switch -c rollback/cache-hit-baseline cache-hit-baseline-20260910
```

本轮没有数据库迁移。若只撤销本轮代码而保留后续提交，可执行：

```bash
git revert 13ea76a3
```
