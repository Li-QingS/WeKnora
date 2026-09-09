# 上游同步与评测功能 Review（2026-09-10）

## 结论

- 当前开发分支 `feat/evaluation-persistence` 已合并腾讯 `upstream/main` 的最新提交 `b60351f8`。合并提交为 `9008586f`，祖先检查成功，合并时相对上游为 ahead 46、behind 0。
- RAG 评测、Wiki 评测、模型调用台账、模型价格、日期筛选和 Embedding 缓存的前端、API、服务、仓储及迁移接线仍完整；本轮合并后的全仓测试、静态检查和生产构建全部通过。
- Wiki 评测保持实验功能：Wiki 数据生成会调用 Chat 模型；评分只做名称精确匹配、Embedding 语义匹配和确定性有向图比较，不调用生成模型裁判，也没有加入 CI 门禁。
- Embedding 缓存继续使用 SQL 是当前阶段的合理选择。它兼容 PostgreSQL 与 Lite/SQLite，能跨重启和多实例复用。当前实现默认关闭，适合评测和中等规模重复任务；在没有容量治理前不应默认作为高并发生产缓存长期启用。

## 上游合并检查

| 检查项 | 结果 | 证据 |
|---|---|---|
| 腾讯远端 | 通过 | `upstream = git@github.com:Tencent/WeKnora.git` |
| 最新 main 已合并 | 通过 | `git merge-base --is-ancestor upstream/main HEAD` 返回 0 |
| ahead / behind | 通过 | 合并后 `HEAD...upstream/main = 46 / 0` |
| 文本冲突 | 通过 | Git 自动合并完成，无未解决冲突 |
| 迁移版本 | 已修复 | 上游 MCP 迁移与本分支 `000092_evaluation_runs` 重号，已将 MCP 迁移顺延为 PostgreSQL `000098` |
| SQLite 对齐 | 已修复 | 新增 SQLite `000020_mcp_metadata`，补齐 `usage_instructions` 和 `mcp_metadata` |
| 迁移防回归 | 已增加 | 自动检查每个目录的版本唯一性及 up/down 配对 |

实际开发 PostgreSQL 已从版本 97 升级到 98，状态为 `dirty=false`；全新临时 PostgreSQL 也从 0 成功迁移到 98。两次均确认 `mcp_metadata` 表和 `mcp_services.usage_instructions` 字段存在。SQLite 新建库和从 v4 升级的测试均迁移到版本 20。

## 功能完整性矩阵

| 功能 | 前端与入口 | 后端与持久化 | 结果与测试 | 结论 |
|---|---|---|---|---|
| RAG 问答评测 | 评测中心 RAG 页签，可配置数据集、模型和分块 | `/evaluation` 路由、后台运行、心跳、恢复、运行记录和配置快照 | Recall/MRR/ROUGE-L、成本和耗时可查看；Go/CLI 测试通过 | 完整 |
| RAG CI 门禁 | GitHub Actions 与 CLI `eval` 命令 | 基线比较、退出码和报告 | 已有通过、退化失败、恢复通过的真实证据 | 完整 |
| Wiki 评测 | 评测中心独立 Wiki 页签，包含说明、运行、历史、详情和报告下载 | 隔离临时知识库、生成、冻结、评分、持久化、失败恢复和清理 | 实体/概念覆盖率；图 Precision/Recall/F1；节点与边明细 | 代码链路完整，仍为手动实验 |
| Wiki 模型边界 | 页面明确区分生成与评分 | `:generation` 归属 Chat 调用；`:scoring` 只调用 Embedding | 调用组与评分单元测试通过 | 完整 |
| 模型用量与价格 | 模型用量页、筛选、汇总、明细和价格编辑 | SQL 台账、价格快照、按租户查询和聚合 | 同日日期区间测试覆盖 9 月 8 日整天 | 完整 |
| Embedding 缓存 | 模型用量页显示当前进程命中、未命中和 Provider 调用 | SQL 持久缓存，按租户/模型/维度/配置/文本隔离 | 单条、批量、重复输入、无效向量、并发 upsert、故障回退测试通过 | 实验范围完整 |

本轮没有重新发起需要付费模型的 EnterpriseRAG 85 文档 Wiki 全量运行，因此“最新合并版本上的真实全量生成质量”仍属于运行验收项。之前的真实模型、解析器和 CI 证据仍保存在 `docs/evidence/`。

## Review 发现与处理

| 级别 | 发现 | 处理 |
|---|---|---|
| 高 | 上游新增 `000092_mcp_metadata` 与本分支评测迁移重号，Git 无文本冲突但 golang-migrate 会在运行时拒绝目录 | 顺延为 `000098`，并增加迁移编号唯一和配对测试 |
| 高 | 上游 MCP 元数据只提供 PostgreSQL 迁移，Lite 代码访问时会缺表和字段 | 新增 SQLite `000020`，新建及旧库升级测试均通过 |
| 中 | 未持久化的临时模型可能以空 model ID、0 维度写入共享缓存，无法证明向量空间身份稳定 | 缺租户、模型 ID 或有效维度时直接绕过持久缓存 |
| 中 | 保留的鉴权类 `CustomHeaders` 实际不会发给 Provider，却会改变缓存命名空间，导致无意义的缓存分裂 | 命名空间只纳入真正会发送的自定义 Header；仍保留有效路由 Header 的隔离 |
| 低 | Wiki 历史删除缺少确认，失败提示在成功重试后仍可能残留 | 增加删除确认，并在重新加载、查看、删除和下载前清除旧错误 |

没有发现上游本次 Agent、MCP、沙箱和引用显示改动破坏评测路由、依赖注入或前端入口的情况。

## Embedding 缓存技术选型

当前的 SQL 缓存应保留，原因如下：

1. Lite 模式可以只使用 SQLite；改成 Redis 会让缓存功能依赖额外基础设施。
2. SQL 数据跨进程重启保存，多副本读取同一结果，适合重复评测和可复现实验。
3. 缓存键是精确主键查询，远程 Embedding 网络耗时通常明显高于一次索引查询，当前负载下 SQL 已能减少 Provider 调用。
4. 项目 Redis 同时服务流管理、Asynq 任务和 Langfuse。Compose 中没有给 Embedding 向量设置独立容量和淘汰边界，把大量向量直接写入同一 Redis 会放大内存及队列稳定性风险。
5. 模型调用台账、价格快照和评测结果需要历史查询、聚合和审计，也应继续使用 SQL，Redis 不适合作为这些业务记录的主存储。

真实开发库当前有 11,119 条 Embedding 缓存记录、25,323 次累计使用，其中 14,204 次是持久记录复用；表及索引、TOAST 合计约 94 MB，约 8.9 KB/条。这证明缓存已经产生复用，同时也证明无界增长不能忽略。

当前实现的生产限制：

- 没有 TTL、租户配额或总容量清理；
- 向量以 JSONB/TEXT 保存，空间效率一般；
- Batch 查询仍逐键读取，命中计数逐次更新数据库；
- 并发冷请求没有 singleflight，可能同时调用 Provider；
- 页面统计是当前进程累计，多副本之间不聚合；
- 缓存读写失败会正确回退 Provider，但目前没有限频的错误指标或告警。

如果后续把它升级为长期生产能力，建议按下面顺序实施：

1. 先增加按 `updated_at` 的保留期、每租户/模型容量上限和后台分批清理；
2. 再实现批量 Get/Set、命中计数异步聚合和并发请求合并；
3. 增加有界进程内 L1；只有实际压测证明数据库读延迟成为瓶颈时，再引入带 TTL 和内存上限的专用 Redis L1；
4. Redis L1 与任务队列使用独立实例或至少独立容量预算，SQL 继续作为可选持久 L2。

## 验证结果

2026-09-10 在合并后的代码上通过：

- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `client`: `go test ./...`、`go vet ./...`
- `cli`: `go test ./...`、`go vet ./...`
- `frontend`: `npm test`（781 项）、`npm run type-check`、`npm run build`、`npm run check-i18n`
- PostgreSQL：全新库 0 → 98、开发库 97 → 98
- SQLite：全新库和 v4 升级至 20
- `git diff --check`

Vite 仍有大于 500 kB 的既有 chunk 提示，但构建成功。评测页面目前使用中文硬编码，后续若要求完整多语言，需要单独迁移到 i18n；两项都不影响本次功能正确性。
