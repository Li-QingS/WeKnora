# Embedding 缓存命中率优化结果

## 结论

本轮保留 PostgreSQL/SQLite 作为持久缓存，在应用进程中加入保守文本规范化、旧键惰性提升、Batch 规范去重和批量感知的并发请求合并。没有新增数据库迁移，也没有把 Embedding 向量写入当前共享 Redis。

在项目内置 EnterpriseRAG 数据的固定格式漂移场景中，命中率从 **33.33% 提高到 66.67%**，增加 **33.33 个百分点**；Provider 输入从 270 降至 135，减少 **50.00%**。32 个并发相同冷请求从 32 次 Provider 调用压缩为 1 次，减少 **96.875%**；16 个并发的双输入 Batch 从 16 次批量调用压缩为 1 次，减少 **93.75%**。

这些数字来自固定 benchmark 和竞态测试，表示对应工作负载的改进，不等同于对未来线上流量的预测。上线后的真实新增收益可以通过模型用量页面的“格式归一命中”和“请求合并”直接观察。

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
