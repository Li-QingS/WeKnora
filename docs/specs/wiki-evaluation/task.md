# Wiki 评测与统一评测中心 Tasks

> 状态：已批准（2026-09-07，最终审批）
> 输入：已批准的 `spec.md` 与 `plan.md`
> 执行原则：每个任务完成验证后再进入依赖它的任务；实现期间按阶段提交代码。

## 文件清单

| 操作 | 文件 | 职责 |
|---|---|---|
| 新建 | `dataset/enterprise_rag/wiki_gold.json` | Wiki 实体、概念、别名和有向边 Gold |
| 新建 | `migrations/versioned/000096_wiki_evaluation_runs.up.sql` | PostgreSQL 运行表升级 |
| 新建 | `migrations/versioned/000096_wiki_evaluation_runs.down.sql` | PostgreSQL 回滚 |
| 新建 | `migrations/sqlite/000018_wiki_evaluation_runs.up.sql` | SQLite 运行表升级 |
| 新建 | `migrations/sqlite/000018_wiki_evaluation_runs.down.sql` | SQLite 回滚 |
| 修改 | `internal/database/migration_sqlite_versioned_schema_test.go` | SQLite 最终 schema 断言 |
| 修改 | `internal/types/evaluation.go` | 共享运行类型、阶段和数据集文档 |
| 新建 | `internal/types/wiki_evaluation.go` | Wiki Gold、请求、快照、指标和详情类型 |
| 修改 | `internal/types/tracing.go` | 跨异步边界的 request group |
| 修改 | `internal/types/interfaces/evaluation.go` | Wiki 服务、评分器和仓储接口 |
| 修改 | `internal/application/repository/evaluation_run.go` | 类型过滤、阶段、失败和结果持久化 |
| 修改 | `internal/application/repository/evaluation_run_test.go` | 运行仓储回归与 Wiki 测试 |
| 修改 | `internal/application/service/evaluation.go` | 显式 RAG 类型与默认列表兼容 |
| 修改 | `internal/application/service/evaluation_persist_test.go` | RAG 持久化兼容测试 |
| 修改 | `internal/application/service/dataset.go` | 保留 85 篇有序去重语料 |
| 修改 | `internal/application/service/dataset_test.go` | 数据集文档和哈希兼容测试 |
| 修改 | `internal/application/service/knowledge_create.go` | 带稳定标题的 passage 创建入口 |
| 修改 | `internal/application/service/knowledge_create_test.go` | 新入口兼容测试 |
| 修改 | `internal/application/service/wiki_ingest.go` | Wiki pending-op request group 传播 |
| 修改 | `internal/application/service/wiki_ingest_test.go` | pending-op 传播测试 |
| 新建 | `internal/application/service/wiki_evaluation.go` | Wiki 运行协调器 |
| 新建 | `internal/application/service/wiki_evaluation_gold.go` | Gold 加载、校验和签名 |
| 新建 | `internal/application/service/wiki_evaluation_import.go` | 临时 KB 创建和语料导入 |
| 新建 | `internal/application/service/wiki_evaluation_wait.go` | Wiki 稳定完成监控 |
| 新建 | `internal/application/service/wiki_evaluation_freeze.go` | Wiki 页面和链接冻结 |
| 新建 | `internal/application/service/wiki_evaluation_match.go` | 节点匹配和覆盖率 |
| 新建 | `internal/application/service/wiki_evaluation_graph.go` | 有向图结构评分 |
| 新建 | `internal/application/service/wiki_evaluation_report.go` | JSON/Markdown 报告 |
| 新建 | `internal/application/service/wiki_evaluation_test.go` | 协调器成功、失败和清理测试 |
| 新建 | `internal/application/service/wiki_evaluation_gold_test.go` | Gold 严格校验测试 |
| 新建 | `internal/application/service/wiki_evaluation_import_test.go` | 临时 KB 和导入测试 |
| 新建 | `internal/application/service/wiki_evaluation_wait_test.go` | 完成监控测试 |
| 新建 | `internal/application/service/wiki_evaluation_freeze_test.go` | 页面冻结测试 |
| 新建 | `internal/application/service/wiki_evaluation_match_test.go` | 精确、语义和覆盖率测试 |
| 新建 | `internal/application/service/wiki_evaluation_graph_test.go` | 图指标测试 |
| 新建 | `internal/application/service/wiki_evaluation_report_test.go` | 报告一致性测试 |
| 修改 | `internal/tracing/langfuse/asynq.go` | request group 注入和恢复 |
| 修改 | `internal/tracing/langfuse/asynq_test.go` | Langfuse 开关两种场景测试 |
| 修改 | `internal/handler/evaluation.go` | 共享历史列表类型分发 |
| 修改 | `internal/handler/evaluation_test.go` | RAG 默认列表与 Wiki 分发测试 |
| 新建 | `internal/handler/wiki_evaluation.go` | Wiki HTTP 接口 |
| 新建 | `internal/handler/wiki_evaluation_test.go` | Wiki HTTP 行为测试 |
| 修改 | `internal/container/container.go` | 服务与 handler 注入 |
| 新建 | `internal/container/recover_evaluation_runs.go` | Wiki 中断运行清理恢复 |
| 新建 | `internal/container/recover_evaluation_runs_test.go` | 恢复行为测试 |
| 修改 | `internal/router/routes_infra.go` | Wiki 路由注册 |
| 修改 | `internal/router/router_api_key_capabilities_test.go` | 新路由权限覆盖 |
| 修改 | `frontend/src/api/evaluation/index.ts` | Wiki DTO 与客户端方法 |
| 修改 | `frontend/src/views/settings/EvaluationCenterSettings.vue` | 双页签评测中心外壳 |
| 新建 | `frontend/src/views/settings/evaluation/RagEvaluationPanel.vue` | 原 RAG 评测面板 |
| 新建 | `frontend/src/views/settings/evaluation/WikiEvaluationPanel.vue` | Wiki 配置、运行和结果面板 |
| 新建 | `frontend/src/views/settings/evaluation/EvaluationHistoryTable.vue` | 共享历史记录表 |
| 新建 | `frontend/src/views/settings/evaluation/WikiEvaluationDetail.vue` | 节点和边详情 |
| 新建 | `frontend/src/views/settings/evaluation/wikiEvaluationViewModel.ts` | 阶段和数值展示映射 |
| 新建 | `frontend/src/views/settings/evaluation/wikiEvaluationViewModel.test.ts` | 展示映射测试 |
| 修改 | `frontend/src/i18n/locales/zh-CN.ts` | 中文文案 |
| 修改 | `frontend/src/i18n/locales/en-US.ts` | 英文文案 |
| 修改 | `frontend/src/i18n/locales/ko-KR.ts` | 韩文键集合 |
| 修改 | `frontend/src/i18n/locales/ru-RU.ts` | 俄文键集合 |
| 修改 | `docs/api/evaluation.md` | Wiki API 与报告格式 |
| 修改 | `website-docs/03-features/15-evaluation.md` | 用户操作与指标说明 |
| 修改 | `go.mod` | 将 `golang.org/x/text` 标为直接依赖 |
| 修改 | `go.sum` | 依赖整理产生的校验和变化 |

## 第一阶段：持久化底座与领域类型

### T1：增加 PostgreSQL 运行表迁移

**文件：** `migrations/versioned/000096_wiki_evaluation_runs.up.sql`、`migrations/versioned/000096_wiki_evaluation_runs.down.sql`
**依赖：** 无

**步骤：**
1. 在 up migration 增加 `evaluation_type`、`stage`、`failure_stage`、`stage_progress` 和 `result_detail`。
2. 为 `(tenant_id, evaluation_type, created_at)` 创建索引，并给旧记录提供 `rag` 默认值。
3. 在 down migration 以逆序删除索引和新增列。

**验证：** `rg -n "evaluation_type|failure_stage|stage_progress|result_detail" migrations/versioned/000096_wiki_evaluation_runs.*.sql` 显示 up/down 均覆盖新增字段。

### T2：增加 SQLite 运行表迁移

**文件：** `migrations/sqlite/000018_wiki_evaluation_runs.up.sql`、`migrations/sqlite/000018_wiki_evaluation_runs.down.sql`
**依赖：** T1

**步骤：**
1. 按 SQLite 支持的语法实现与 T1 等价的列和索引。
2. 按仓库现有 SQLite down migration 约定实现回滚。
3. 确认 JSON 字段使用 SQLite 兼容的存储类型和默认值。

**验证：** `go test ./internal/database -run 'Test.*SQLite.*Migration' -count=1` 通过。

### T3：更新 SQLite 最终 schema 断言

**文件：** `internal/database/migration_sqlite_versioned_schema_test.go`
**依赖：** T2

**步骤：**
1. 在 `evaluation_runs` 期望列中加入四类 Wiki 扩展字段。
2. 增加新复合索引的断言。
3. 保留现有表和列断言不变。

**验证：** `go test ./internal/database -run TestSQLiteMigrationsCreateVersionedSchema -count=1` 通过。

### T4：扩展共享评测运行和数据集类型

**文件：** `internal/types/evaluation.go`
**依赖：** T1、T2

**步骤：**
1. 定义 `EvaluationType`、`EvaluationStage` 和 `EvaluationStageProgress` 及 plan 中的常量。
2. 给 `EvaluationRun` 增加类型、阶段、失败阶段、阶段进度和结果详情字段。
3. 定义 `EvaluationDocument`，并给 `EvaluationDataset` 增加 `Documents`。

**验证：** `go test ./internal/types -run 'TestEvaluation|TestRequestGroup' -count=1` 编译并通过现有测试。

### T5：定义 Wiki 评测领域类型

**文件：** `internal/types/wiki_evaluation.go`
**依赖：** T4

**步骤：**
1. 定义请求、Gold、数据集元数据、配置快照、页面快照和报告格式。
2. 定义节点匹配、节点指标、图指标、边明细、内部评分结果和最终详情。
3. 为类型和格式增加合法值校验方法，阈值默认值固定为 0.80。

**验证：** `go test ./internal/types -count=1` 编译通过。

### T6：补齐 Wiki 服务和仓储接口

**文件：** `internal/types/interfaces/evaluation.go`
**依赖：** T5

**步骤：**
1. 增加 `WikiEvaluationService`、Gold loader、语料导入、带标题 passage 创建、监控、冻结、评分、Embedding provider 和报告接口。
2. 给 `EvaluationRunRepository` 增加按类型列表、阶段更新、失败记录、Wiki 结果保存和待清理查询方法。
3. 保留现有 `EvaluationService` 方法签名。

**验证：** `go test ./internal/types/interfaces ./internal/types -count=1` 编译通过。

### T7：实现评测运行的类型过滤与阶段写入

**文件：** `internal/application/repository/evaluation_run.go`
**依赖：** T3、T6

**步骤：**
1. 让现有 `List` 固定查询 `rag`，新增 `ListByType` 并同时限定 tenant、类型、状态和分页。
2. 实现 `UpdateStage` 与 `RecordFailure`，同时刷新 `updated_at/heartbeat_at`。
3. 所有 worker 写操作继续以 run ID 定位，所有用户读操作继续以 tenant + run ID 定位。

**验证：** `go test ./internal/application/repository -run TestEvaluationRun -count=1` 通过。

### T8：实现结果原子保存与清理查询

**文件：** `internal/application/repository/evaluation_run.go`
**依赖：** T7

**步骤：**
1. 实现 `SaveWikiResult`，在同一事务写入 metric、result detail 和最终 config snapshot。
2. 实现 `ListCleanupPending`，只返回带临时 KB ID 的 Wiki 非终态运行。
3. 防止结果保存方法把失败或已结束运行重新写回 running。

**验证：** `go test ./internal/application/repository -run TestEvaluationRun -count=1` 通过。

### T9：覆盖仓储兼容性和租户隔离

**文件：** `internal/application/repository/evaluation_run_test.go`
**依赖：** T8

**步骤：**
1. 增加旧记录默认 `rag`、RAG/Wiki 类型过滤和稳定分页测试。
2. 增加阶段、失败阶段、原子结果和待清理查询测试。
3. 增加跨租户读取、删除和列表不可见测试。

**验证：** `go test ./internal/application/repository -run TestEvaluationRun -count=1` 全部通过。

### T10：保持 RAG 运行行为兼容

**文件：** `internal/application/service/evaluation.go`、`internal/application/service/evaluation_persist_test.go`
**依赖：** T9

**步骤：**
1. 创建 RAG run 时显式写入 `EvaluationTypeRAG`。
2. 现有历史列表继续调用默认 RAG 查询，详情和删除逻辑不解析 Wiki result。
3. 增加既有 RAG 请求、结果、历史和清理行为的回归断言。

**验证：** `go test ./internal/application/service -run 'TestEvaluation.*Persist|TestEvaluation.*Run' -count=1` 通过。

## 第二阶段：数据集与版本化 Gold

### T11：保留 EnterpriseRAG 完整语料

**文件：** `internal/application/service/dataset.go`
**依赖：** T4

**步骤：**
1. 在读取 QA parquet 时按 corpus ID 聚合全部引用文档并去重。
2. 按 ID 稳定排序，生成 `enterprise_rag-corpus-<id>` 标题并填充 `Documents`。
3. 保持现有 `Pairs`、`SampleCount` 和数据集 SHA-256 算法不变。

**验证：** `go test ./internal/application/service -run TestDataset -count=1` 通过。

### T12：验证 85 篇语料和 RAG 哈希兼容

**文件：** `internal/application/service/dataset_test.go`
**依赖：** T11

**步骤：**
1. 断言 `enterprise_rag` 仍有 50 个 QA pair 和原数据集哈希。
2. 断言 `Documents` 恰有 85 篇、ID/标题唯一且顺序稳定。
3. 断言所有 QA 引用的 corpus 都能在 `Documents` 找到。

**验证：** `go test ./internal/application/service -run 'TestDataset.*Enterprise|TestDataset.*Document' -count=1` 通过。

### T13：实现 Gold 严格 JSON 解码

**文件：** `internal/application/service/wiki_evaluation_gold.go`
**依赖：** T5、T12

**步骤：**
1. 用 `DisallowUnknownFields` 解码 `dataset/<id>/wiki_gold.json`，并拒绝尾随 JSON。
2. 校验 schema version、dataset ID、节点字段、合法类型和边端点。
3. 拒绝重复节点 ID、重复规范化别名和重复有向边。

**验证：** `go test ./internal/application/service -run TestWikiGold -count=1` 编译通过。

### T14：实现 Gold 规范化内容哈希

**文件：** `internal/application/service/wiki_evaluation_gold.go`
**依赖：** T13

**步骤：**
1. 复制 Gold 并清空 `content_sha256`，稳定排序节点、别名和边。
2. 对 Go 固定字段顺序的紧凑 JSON 计算 SHA-256。
3. 校验内容哈希和数据集哈希，并生成 `WikiGoldSnapshot`。

**验证：** `go test ./internal/application/service -run TestWikiGold -count=1` 编译通过。

### T15：覆盖 Gold 校验边界

**文件：** `internal/application/service/wiki_evaluation_gold_test.go`
**依赖：** T14

**步骤：**
1. 用最小有效 fixture 验证稳定哈希不受数组顺序和格式空白影响。
2. 分别覆盖未知字段、重复 ID、非法类型、悬空边、错误内容哈希和错误数据集哈希。
3. 覆盖合法自环和非法重复自环。

**验证：** `go test ./internal/application/service -run TestWikiGold -count=1` 全部通过。

### T16：标注 Gold 第 1–10 个问题

**文件：** `dataset/enterprise_rag/wiki_gold.json`
**依赖：** T15

**步骤：**
1. 按 `manifest.json` 当前顺序复核第 1–10 个问题、答案和引用文档。
2. 使用 `entity:<slug>` 与 `concept:<slug>` 稳定 ID 添加有原文依据的核心节点和别名。
3. 只添加由同一资料明确支持的预期页面有向链接，并保持节点和边排序。

**验证：** `python3 -m json.tool dataset/enterprise_rag/wiki_gold.json >/dev/null` 返回成功，且新增边的两个端点均存在。

### T17：标注 Gold 第 11–20 个问题

**文件：** `dataset/enterprise_rag/wiki_gold.json`
**依赖：** T16

**步骤：**
1. 复核第 11–20 个问题、答案和引用文档。
2. 合并已存在实体/概念，补充新节点、别名和有依据的有向边。
3. 避免把日期、金额和完整句子标成独立概念页。

**验证：** `python3 -m json.tool dataset/enterprise_rag/wiki_gold.json >/dev/null` 返回成功，节点 ID 无重复。

### T18：标注 Gold 第 21–30 个问题

**文件：** `dataset/enterprise_rag/wiki_gold.json`
**依赖：** T17

**步骤：**
1. 复核第 21–30 个问题、答案和引用文档。
2. 添加核心组织、产品、组件、流程实体与可复用概念。
3. 合并拼写、缩写和全称别名，保持节点类型一致。

**验证：** `python3 -m json.tool dataset/enterprise_rag/wiki_gold.json >/dev/null` 返回成功，所有别名非空。

### T19：标注 Gold 第 31–40 个问题

**文件：** `dataset/enterprise_rag/wiki_gold.json`
**依赖：** T18

**步骤：**
1. 复核第 31–40 个问题、答案和引用文档。
2. 添加新的 Gold 节点和资料支持的有向边。
3. 复核跨文档同名项，不能因名称相同合并不同对象。

**验证：** `python3 -m json.tool dataset/enterprise_rag/wiki_gold.json >/dev/null` 返回成功，边集合无重复。

### T20：标注 Gold 第 41–50 个问题

**文件：** `dataset/enterprise_rag/wiki_gold.json`
**依赖：** T19

**步骤：**
1. 复核第 41–50 个问题、答案和引用文档。
2. 完成剩余实体、概念、别名和有向边标注。
3. 按 ID 和 `source,target` 对全部数组做最终稳定排序。

**验证：** `python3 -m json.tool dataset/enterprise_rag/wiki_gold.json >/dev/null` 返回成功，节点与边数组均按计划的稳定键排序。

### T21：完成 Gold 全局复核和签名

**文件：** `dataset/enterprise_rag/wiki_gold.json`、`internal/application/service/wiki_evaluation_gold_test.go`
**依赖：** T20

**步骤：**
1. 全局检查跨批次重复节点、类型冲突、循环别名和无依据边。
2. 写入实际 dataset SHA-256 和按 T14 算法计算的 content SHA-256。
3. 增加加载仓库实际 Gold 的测试，断言节点、边、版本和两个哈希有效。

**验证：** `go test ./internal/application/service -run TestWikiGoldFixture -count=1` 通过。

## 第三阶段：异步归属、导入、完成监控与冻结

### T22：在异步 carrier 中加入 request group

**文件：** `internal/types/tracing.go`、`internal/tracing/langfuse/asynq.go`
**依赖：** T5

**步骤：**
1. 给 `TracingContext` 增加 `RequestGroupID` JSON 字段。
2. 让 `InjectTracing` 在 Langfuse 关闭时也从 context 写入 request group。
3. 保持现有 traceparent、用户和会话字段的启用条件与兼容性。

**验证：** `go test ./internal/tracing/langfuse -run TestInjectTracing -count=1` 通过。

### T23：在 worker 中恢复 request group

**文件：** `internal/tracing/langfuse/asynq.go`
**依赖：** T22

**步骤：**
1. 在检查 Langfuse 开关前解析 payload carrier。
2. 使用 `types.WithRequestGroupID` 恢复 worker context，再进入后续 handler。
3. Langfuse 启用时继续在恢复后的 context 上构建 trace/span。

**验证：** `go test ./internal/tracing/langfuse -run TestAsynq -count=1` 通过。

### T24：补充异步归属回归测试

**文件：** `internal/tracing/langfuse/asynq_test.go`
**依赖：** T23

**步骤：**
1. 覆盖 Langfuse 关闭时 request group 仍注入和恢复。
2. 覆盖 Langfuse 开启且有 traceparent 时两类上下文同时恢复。
3. 覆盖旧 payload 没有新字段时保持空值且正常执行。

**验证：** `go test ./internal/tracing/langfuse -count=1` 全部通过。

### T25：传播 Wiki pending-op 的 request group

**文件：** `internal/application/service/wiki_ingest.go`
**依赖：** T23

**步骤：**
1. 让 ingest 和 finalize pending row payload 持久化 `TracingContext`。
2. 从入队 context 写入 `<runID>:generation`，批处理解码后恢复到处理 context。
3. 同一 KB 批次出现不一致非空 group 时返回可定位错误，旧空 group 保持兼容。

**验证：** `go test ./internal/application/service -run TestWikiIngest -count=1` 通过。

### T26：测试 Wiki pending-op 归属链

**文件：** `internal/application/service/wiki_ingest_test.go`
**依赖：** T25

**步骤：**
1. 断言 ingest row 序列化后保留 request group。
2. 断言 ingest 生成的 finalize row 继续携带相同 group。
3. 断言实际模型调用所见 context 的 group 为生成阶段 ID。

**验证：** `go test ./internal/application/service -run 'TestWikiIngest.*RequestGroup|TestWikiFinalize.*RequestGroup' -count=1` 通过。

### T27：增加带标题的 passage 创建入口

**文件：** `internal/application/service/knowledge_create.go`
**依赖：** T6

**步骤：**
1. 实现 `CreateKnowledgeFromPassageWithTitle`，接受标题、passages 和 channel。
2. 复用现有 passage 创建、分块、状态更新和 post-process 调度逻辑。
3. 让现有无标题入口调用新入口并保留原默认标题行为。

**验证：** `go test ./internal/application/service -run TestCreateKnowledgeFromPassage -count=1` 通过。

### T28：测试带标题入口兼容性

**文件：** `internal/application/service/knowledge_create_test.go`
**依赖：** T27

**步骤：**
1. 断言指定标题写入 knowledge 且 passage 内容不变。
2. 断言 post-process payload 保留 request group。
3. 断言旧入口仍使用原默认标题和状态流转。

**验证：** `go test ./internal/application/service -run TestCreateKnowledgeFromPassage -count=1` 全部通过。

### T29：创建临时 Wiki 知识库

**文件：** `internal/application/service/wiki_evaluation_import.go`
**依赖：** T6、T27

**步骤：**
1. 构造当前 tenant 和 run 专属的 document KB 名称与描述。
2. 设置 wiki-only indexing strategy、所选 Summary/Chat 模型、评分 Embedding 模型和确定的 Wiki 配置。
3. 创建成功后立刻把 KB ID 写入 run，后续错误均可按该 ID 清理。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationImport -count=1` 编译通过。

### T30：逐篇导入全部语料

**文件：** `internal/application/service/wiki_evaluation_import.go`
**依赖：** T29、T12

**步骤：**
1. 按 `Documents` 顺序逐篇调用带标题 passage 入口。
2. 给 context 设置 `<runID>:generation` 并收集每条 knowledge ID。
3. 每篇成功后更新导入进度；遇到第一处失败返回已导入 ID 和原始错误。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationImport -count=1` 编译通过。

### T31：测试临时 KB 和部分导入失败

**文件：** `internal/application/service/wiki_evaluation_import_test.go`
**依赖：** T30

**步骤：**
1. 断言 KB 只启用 Wiki，模型 ID 和 tenant 正确。
2. 断言三篇 fixture 按稳定标题和顺序导入并返回 ID。
3. 让第二篇失败，断言停止导入、返回可定位错误且 KB ID 已记录。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationImport -count=1` 全部通过。

### T32：实现 knowledge 完成检查

**文件：** `internal/application/service/wiki_evaluation_wait.go`
**依赖：** T30

**步骤：**
1. 轮询本次 knowledge 状态，计算完成数并调用进度回调。
2. 任一 knowledge 失败、缺失或 KB 被删除时立即返回带 ID 的错误。
3. 响应 context 取消和 4 小时总超时，不创建独立失控计时器。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationWait -count=1` 编译通过。

### T33：实现 Wiki 队列稳定和 dead-letter 检查

**文件：** `internal/application/service/wiki_evaluation_wait.go`
**依赖：** T32

**步骤：**
1. 查询当前 KB 的 `wiki:ingest` 与 `wiki:finalize` pending/claimed 数。
2. 分页检查当前 KB 的 task dead letters，并报告 task 类型和错误。
3. 仅在 knowledge 成功、两条队列为空、无 dead letter 的条件连续两轮成立后完成。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationWait -count=1` 编译通过。

### T34：覆盖完成监控的竞态边界

**文件：** `internal/application/service/wiki_evaluation_wait_test.go`
**依赖：** T33

**步骤：**
1. 覆盖两轮稳定、两轮之间出现 finalize、claimed row 和 context 超时。
2. 覆盖 knowledge 失败及 ingest/finalize dead letter。
3. 断言失败消息包含 KB、knowledge 或 task 标识。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationWait -count=1` 全部通过。

### T35：冻结实体、概念页面和链接

**文件：** `internal/application/service/wiki_evaluation_freeze.go`
**依赖：** T33

**步骤：**
1. 按 tenant 校验 KB 后调用现有 Wiki page service 读取全部页面。
2. 只把 entity/concept 转成 `WikiEvaluationPage`，规范化空别名并去重排序。
3. 保留所有出链 slug，去重排序后返回不可变快照。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationFreeze -count=1` 编译通过。

### T36：测试页面过滤和冻结稳定性

**文件：** `internal/application/service/wiki_evaluation_freeze_test.go`
**依赖：** T35

**步骤：**
1. 构造 entity、concept、summary、index、synthesis 和 comparison 页面。
2. 断言只冻结前两类，未知/非评分出链仍保留。
3. 打乱仓储返回顺序，断言快照 JSON 保持一致。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationFreeze -count=1` 全部通过。

## 第四阶段：节点与图评分

### T37：实现名称规范化和精确候选

**文件：** `internal/application/service/wiki_evaluation_match.go`、`go.mod`
**依赖：** T5

**步骤：**
1. 用 `golang.org/x/text/unicode/norm` 实现 NFKC、Unicode case fold、两端标点清理和空白折叠。
2. 为同类型 Gold 与页面的名称/别名建立精确候选边。
3. 过滤规范化后的空字符串，稳定排序候选。

**验证：** `go test ./internal/application/service -run 'TestWiki.*Normalize|TestWiki.*Exact' -count=1` 编译通过。

### T38：实现精确最大基数一对一匹配

**文件：** `internal/application/service/wiki_evaluation_match.go`
**依赖：** T37

**步骤：**
1. 对 entity 和 concept 候选分别运行确定性二分图最大匹配。
2. 以 Gold ID 和页面 slug 排序固定遍历与平局结果。
3. 输出 exact match，并从语义阶段输入中移除匹配双方。

**验证：** `go test ./internal/application/service -run TestWikiNodeExactMatch -count=1` 编译通过。

### T39：覆盖规范化、类型和精确冲突测试

**文件：** `internal/application/service/wiki_evaluation_match_test.go`
**依赖：** T38

**步骤：**
1. 覆盖全半角、大小写、外部标点、连续空格和空别名。
2. 覆盖跨类型同名不匹配、多个别名候选和最大基数优于贪心的样本。
3. 重排输入多次，断言 exact 结果稳定且一对一。

**验证：** `go test ./internal/application/service -run 'TestWiki.*Normalize|TestWikiNodeExactMatch' -count=1` 通过。

### T40：实现批量 Embedding 和节点相似度

**文件：** `internal/application/service/wiki_evaluation_match.go`
**依赖：** T38

**步骤：**
1. 收集未匹配节点的去重规范化名称/别名并批量调用所选模型。
2. 在单次运行内缓存文本向量，校验向量维度和零向量。
3. 每对同类型节点取名称集合的最大余弦相似度，删除低于阈值的候选。

**验证：** `go test ./internal/application/service -run TestWikiNodeSemanticMatch -count=1` 编译通过。

### T41：实现确定性最大权语义分配

**文件：** `internal/application/service/wiki_evaluation_match.go`
**依赖：** T40

**步骤：**
1. 实现支持矩形矩阵的本地最大权一对一分配，并为缺边使用不可选标记。
2. 以 Gold ID、页面 slug 顺序构建矩阵并固定等权平局。
3. 输出 semantic match 的实际分数，保证阈值外和已精确匹配节点不进入结果。

**验证：** `go test ./internal/application/service -run 'TestWikiMaximumWeightMatch|TestWikiNodeSemanticMatch' -count=1` 编译通过。

### T42：计算覆盖率并测试语义边界

**文件：** `internal/application/service/wiki_evaluation_match.go`、`internal/application/service/wiki_evaluation_match_test.go`
**依赖：** T41

**步骤：**
1. 计算 entity、concept 和按节点汇总的 overall coverage。
2. 为未匹配 Gold 写入原因，为额外生成页面保留快照但不扣 coverage。
3. 测试阈值上下、全局最优优于局部贪心、批量去重、零 Gold 和模型错误。

**验证：** `go test ./internal/application/service -run 'TestWikiNode|TestWikiMaximumWeight' -count=1` 全部通过。

### T43：实现诱导子图有向边评分

**文件：** `internal/application/service/wiki_evaluation_graph.go`
**依赖：** T42、T35

**步骤：**
1. 用 `PageToGoldID` 映射生成边，并构造已匹配 Gold 节点集合。
2. 生成 Gold 诱导边集和生成映射边集，去重后求 correct、missing、extra。
3. 计算 Precision/Recall/F1；把未匹配、未知和非评分端点链接写入 unscored。

**验证：** `go test ./internal/application/service -run TestWikiGraphScore -count=1` 编译通过。

### T44：覆盖图方向、边界和零分母测试

**文件：** `internal/application/service/wiki_evaluation_graph_test.go`
**依赖：** T43

**步骤：**
1. 用手算 fixture 覆盖正确、缺失、多余和反向边。
2. 验证未匹配端点不改变指标，而已匹配节点间多余边降低 Precision。
3. 覆盖重复边、自环和无可评分边的 `scorable=false`。

**验证：** `go test ./internal/application/service -run TestWikiGraphScore -count=1` 全部通过。

### T45：实现 JSON 与 Markdown 报告渲染

**文件：** `internal/application/service/wiki_evaluation_report.go`
**依赖：** T42、T43

**步骤：**
1. JSON 直接序列化持久化 detail，使用稳定缩进和末尾换行。
2. Markdown 输出运行状态、指标、签名、模型、阈值、成本、节点表和四类边表。
3. 对失败且无 metric 的 run 只输出状态、失败阶段、错误和已有快照。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationReport -count=1` 编译通过。

### T46：验证双格式同源和稳定输出

**文件：** `internal/application/service/wiki_evaluation_report_test.go`
**依赖：** T45

**步骤：**
1. 用同一 detail 生成 JSON 与 Markdown，断言指标、配置和明细计数一致。
2. 重排输入后断言输出字节稳定。
3. 覆盖成功、失败和无可评分边三种报告。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationReport -count=1` 全部通过。

## 第五阶段：运行协调、清理和恢复

### T47：实现 Wiki 启动前校验和 pending run

**文件：** `internal/application/service/wiki_evaluation.go`
**依赖：** T9、T12、T15、T31

**步骤：**
1. 校验 tenant、dataset、Gold、Chat 模型、Embedding 模型和 `[0,1]` 阈值。
2. 构建不含密钥的参数与初始配置快照，创建 `type=wiki,status=pending` run。
3. 创建脱离 HTTP 取消、最长 4 小时的后台执行 context，并立即返回 detail。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationStart -count=1` 编译通过。

### T48：串联 Wiki 成功执行链

**文件：** `internal/application/service/wiki_evaluation.go`
**依赖：** T47、T34、T36、T42、T44、T46

**步骤：**
1. 依次更新 creating/importing/generating/scoring nodes/scoring graph/saving 阶段和 heartbeat。
2. 在生成 context 使用 `<runID>:generation`，在评分 context 使用 `<runID>:scoring`。
3. 汇总两个 group 的成本，原子保存 metric/result/config 后进入清理。

**验证：** `go test ./internal/application/service -run TestWikiEvaluationSuccess -count=1` 编译通过。

### T49：实现失败记录和终态清理

**文件：** `internal/application/service/wiki_evaluation.go`
**依赖：** T48

**步骤：**
1. 捕获第一处业务错误并持久化 `FailureStage` 与原始错误，不写伪指标。
2. 用独立 2 分钟 context 删除临时 KB；资源不存在视为已清理。
3. 仅在清理成功后转 success/failed；清理失败保持 `running/cleaning_up` 并保留两类错误。

**验证：** `go test ./internal/application/service -run 'TestWikiEvaluationFailure|TestWikiEvaluationCleanup' -count=1` 编译通过。

### T50：实现详情、列表、报告和删除方法

**文件：** `internal/application/service/wiki_evaluation.go`
**依赖：** T49

**步骤：**
1. 实现 tenant-scoped `Get` 和只列 Wiki 的 `ListRuns`，解码持久化 JSON。
2. 实现数据集元数据列表和只读持久化 detail 的报告下载。
3. 删除只接受终态；复用临时 KB 幂等清理后删除 run。

**验证：** `go test ./internal/application/service -run 'TestWikiEvaluationGet|TestWikiEvaluationList|TestWikiEvaluationDelete' -count=1` 编译通过。

### T51：测试协调器成功、隔离和模型边界

**文件：** `internal/application/service/wiki_evaluation_test.go`
**依赖：** T50

**步骤：**
1. 用 fake 依赖跑通 pending 到 success 的完整阶段顺序和结果提交。
2. 断言生成只使用 Chat group，评分只使用 Embedding group，正式 KB 从未被读写。
3. 断言跨 tenant 获取、列表、报告和删除均不可见。

**验证：** `go test ./internal/application/service -run 'TestWikiEvaluationSuccess|TestWikiEvaluationTenant' -count=1` 通过。

### T52：测试各阶段失败和清理重试

**文件：** `internal/application/service/wiki_evaluation_test.go`
**依赖：** T51

**步骤：**
1. 分别注入数据、导入、生成等待、冻结、Embedding、评分保存和删除失败。
2. 断言原始 `FailureStage`、错误、空 metric 和临时 KB 清理行为。
3. 断言首次清理失败不会进入终态，重试成功后进入正确终态。

**验证：** `go test ./internal/application/service -run 'TestWikiEvaluationFailure|TestWikiEvaluationCleanup' -count=1` 全部通过。

### T53：实现服务启动时的 Wiki 运行恢复

**文件：** `internal/container/recover_evaluation_runs.go`
**依赖：** T50

**步骤：**
1. 查询带临时 KB ID 的陈旧 Wiki 非终态运行。
2. 对非清理阶段记录 interrupted 原因和原阶段，再统一进入清理。
3. 幂等删除 KB 后转 interrupted/failed；删除失败保留运行供下次启动重试。

**验证：** `go test ./internal/container -run TestRecoverEvaluationRuns -count=1` 编译通过。

### T54：测试重启恢复和遗留资源清理

**文件：** `internal/container/recover_evaluation_runs_test.go`
**依赖：** T53

**步骤：**
1. 覆盖 stale generating、stale cleaning、已终态和无临时 KB 四种记录。
2. 断言仅目标 tenant/KB 被删除，原错误和失败阶段保留。
3. 模拟删除失败后再次恢复，断言第二次成功且调用幂等。

**验证：** `go test ./internal/container -run TestRecoverEvaluationRuns -count=1` 全部通过。

## 第六阶段：HTTP、依赖注入和路由

### T55：实现 Wiki HTTP handler

**文件：** `internal/handler/wiki_evaluation.go`
**依赖：** T50

**步骤：**
1. 实现启动、详情、数据集和报告 handler，使用项目统一错误包装与日志清理。
2. 启动成功返回 202；参数错误返回 400；范围外 run 返回 404。
3. 报告设置正确 Content-Type、Content-Disposition 和稳定文件名。

**验证：** `go test ./internal/handler -run TestWikiEvaluationHandler -count=1` 编译通过。

### T56：让共享历史列表按类型分发

**文件：** `internal/handler/evaluation.go`
**依赖：** T50

**步骤：**
1. 给 handler 注入 Wiki service，并解析 `evaluation_type`。
2. 空值或 `rag` 调用现有 RAG 列表，`wiki` 调用 Wiki 列表，其他值返回 400。
3. 保持现有状态、分页和响应 envelope 不变。

**验证：** `go test ./internal/handler -run TestEvaluationHandler -count=1` 编译通过。

### T57：覆盖 Wiki API 和列表兼容测试

**文件：** `internal/handler/wiki_evaluation_test.go`、`internal/handler/evaluation_test.go`
**依赖：** T55、T56

**步骤：**
1. 覆盖 Wiki 请求绑定、202、400、404、报告格式和下载头。
2. 覆盖列表未传类型默认 RAG、显式 Wiki 分发和非法类型。
3. 覆盖用户 tenant context 原样传入服务，错误响应不泄露其他租户数据。

**验证：** `go test ./internal/handler -run 'TestWikiEvaluationHandler|TestEvaluationHandler' -count=1` 全部通过。

### T58：注册 Wiki 服务和恢复器

**文件：** `internal/container/container.go`
**依赖：** T53、T55、T56

**步骤：**
1. 注册 Gold loader、导入器、监控器、冻结器、评分器、报告器、Wiki service 和 handler。
2. 给共享 EvaluationHandler 提供 RAG 与 Wiki 两个服务依赖。
3. 在数据库与业务服务就绪后调用 Wiki 运行恢复器。

**验证：** `go test ./internal/container -run 'TestRecoverEvaluationRuns|TestContainer' -count=1` 通过或至少完成包编译。

### T59：注册路由并覆盖 API Key 权限

**文件：** `internal/router/routes_infra.go`、`internal/router/router_api_key_capabilities_test.go`
**依赖：** T58

**步骤：**
1. 在 `/evaluation` 组注册 Wiki 启动、详情、数据集和报告路由。
2. 启动沿用 Admin + run-evaluations capability，读取/报告沿用 Viewer，删除继续走共享 Admin 路由。
3. 在 capability 测试中加入所有新路径和方法。

**验证：** `go test ./internal/router -run TestAPIKey -count=1` 全部通过。

## 第七阶段：统一评测中心前端

### T60：增加 Wiki API 类型和客户端

**文件：** `frontend/src/api/evaluation/index.ts`
**依赖：** T55、T59

**步骤：**
1. 定义 Wiki options、run stage、dataset meta、metric、node/edge detail 和 config snapshot 类型。
2. 增加启动、详情、Wiki 数据集、按类型历史和报告下载方法。
3. 保持原 RAG 类型与函数签名可用。

**验证：** `cd frontend && npm run type-check` 不报告 evaluation API 类型错误。

### T61：实现 Wiki 展示映射

**文件：** `frontend/src/views/settings/evaluation/wikiEvaluationViewModel.ts`
**依赖：** T60

**步骤：**
1. 映射状态、阶段、失败阶段和阶段进度。
2. 实现四位小数、百分比、空 metric 和 `scorable=false` 展示逻辑。
3. 为节点和边明细提供稳定过滤与排序函数。

**验证：** `cd frontend && npm test -- src/views/settings/evaluation/wikiEvaluationViewModel.test.ts` 可加载待测模块。

### T62：测试 Wiki 展示映射

**文件：** `frontend/src/views/settings/evaluation/wikiEvaluationViewModel.test.ts`
**依赖：** T61

**步骤：**
1. 覆盖全部阶段、失败阶段和 0/满进度。
2. 覆盖四位小数、无可评分边和失败时隐藏 metric。
3. 覆盖节点/边过滤和输入顺序变化后的稳定结果。

**验证：** `cd frontend && npm test -- src/views/settings/evaluation/wikiEvaluationViewModel.test.ts` 全部通过。

### T63：抽取共享历史表

**文件：** `frontend/src/views/settings/evaluation/EvaluationHistoryTable.vue`
**依赖：** T60

**步骤：**
1. 将分页、状态、刷新、查看和删除交互封装为 props/emits。
2. 通过插槽或列配置展示 RAG/Wiki 各自的摘要指标。
3. 运行中禁用删除，空列表和错误状态使用统一展示。

**验证：** `cd frontend && npm run type-check` 通过。

### T64：迁移现有 RAG 面板

**文件：** `frontend/src/views/settings/EvaluationCenterSettings.vue`、`frontend/src/views/settings/evaluation/RagEvaluationPanel.vue`
**依赖：** T63

**步骤：**
1. 将现有 RAG 表单、轮询、指标、配置快照和历史逻辑整体迁入新面板。
2. 使用共享历史表连接原 API、分页和删除回调。
3. 对照迁移前保留默认参数、校验文案、刷新策略和指标含义。

**验证：** `cd frontend && npm run type-check && npm run build` 通过，RAG API 路径未变化。

### T65：实现 Wiki 结果详情

**文件：** `frontend/src/views/settings/evaluation/WikiEvaluationDetail.vue`
**依赖：** T61、T63

**步骤：**
1. 展示实体、概念、总体覆盖率和图 P/R/F1，以及两阶段成本。
2. 实现节点类型/匹配方式过滤和边类别过滤。
3. 展示阈值、语义分数、未覆盖/未评分原因和 JSON/Markdown 下载按钮。

**验证：** `cd frontend && npm run type-check` 通过。

### T66：实现 Wiki 配置和启动流程

**文件：** `frontend/src/views/settings/evaluation/WikiEvaluationPanel.vue`
**依赖：** T60、T63、T65

**步骤：**
1. 加载 EnterpriseRAG/Gold 元数据及可用 Chat、Embedding 模型。
2. 实现必填校验、`[0,1]` 阈值校验、0.80 默认值和启动请求。
3. 启动后保存 run ID 并开始详情轮询，显示后端校验错误。

**验证：** `cd frontend && npm run type-check` 通过。

### T67：实现 Wiki 阶段、历史和终态展示

**文件：** `frontend/src/views/settings/evaluation/WikiEvaluationPanel.vue`
**依赖：** T66

**步骤：**
1. 展示数据准备、导入、生成、节点评分、图评分、保存和清理阶段。
2. 接入 Wiki 历史分页、状态筛选、详情打开和共享删除。
3. 成功显示 `WikiEvaluationDetail`；失败显示 failure stage/error 且不渲染空指标。

**验证：** `cd frontend && npm run type-check` 通过。

### T68：完成双页签评测中心外壳

**文件：** `frontend/src/views/settings/EvaluationCenterSettings.vue`
**依赖：** T64、T67

**步骤：**
1. 添加“RAG 问答评测”和“Wiki 评测”页签。
2. 分别挂载两个面板并保持各自表单、分页和轮询状态。
3. 切换页签时停止不可见面板的轮询，返回时按持久化 run 恢复。

**验证：** `cd frontend && npm run type-check && npm run build` 通过。

### T69：补齐四种语言的评测文案

**文件：** `frontend/src/i18n/locales/zh-CN.ts`、`frontend/src/i18n/locales/en-US.ts`、`frontend/src/i18n/locales/ko-KR.ts`、`frontend/src/i18n/locales/ru-RU.ts`
**依赖：** T68

**步骤：**
1. 添加页签、字段、阶段、状态、指标、明细、报告和错误文案键。
2. 中文和英文写入完整文案，韩文与俄文保持同一键集合和准确短文案。
3. 删除组件中的新增硬编码用户文案。

**验证：** `cd frontend && npm run check-i18n` 通过。

### T70：运行完整前端验证

**文件：** 本阶段全部前端文件
**依赖：** T62、T69

**步骤：**
1. 运行全部 Node 测试。
2. 运行 Vue/TypeScript 类型检查。
3. 构建生产前端并检查没有新增警告升级为错误。

**验证：** `./scripts/verify_frontend_pr.sh` 全部通过。

## 第八阶段：文档、依赖和整体验证

### T71：更新 Wiki 评测 API 文档

**文件：** `docs/api/evaluation.md`
**依赖：** T55、T60

**步骤：**
1. 记录 Wiki 启动、详情、数据集、类型历史和报告接口。
2. 给出请求、阶段、成功、失败、指标和报告响应示例。
3. 说明旧历史接口默认 RAG、租户范围和状态码。

**验证：** 文档中的每个路径都能在 `internal/router/routes_infra.go` 找到对应 method/path。

### T72：更新用户功能文档

**文件：** `website-docs/03-features/15-evaluation.md`
**依赖：** T68、T71

**步骤：**
1. 说明两个评测入口和 Wiki 配置步骤。
2. 解释 coverage、图 P/R/F1、诱导子图、无可评分边和两阶段成本。
3. 明确 Wiki 评测为主动实验功能、不进入 CI、临时 KB 会清理。

**验证：** `rg -n 'Wiki|覆盖率|Precision|Recall|CI|临时' website-docs/03-features/15-evaluation.md` 命中全部主题。

### T73：整理 Go 依赖和格式

**文件：** `go.mod`、`go.sum`、本功能所有 Go 文件
**依赖：** T54、T59

**步骤：**
1. 运行 `go mod tidy`，确认 `golang.org/x/text` 成为直接依赖且未引入无关模块。
2. 对所有修改/新增 Go 文件运行 `gofmt`。
3. 运行 `git diff --check` 清除空白错误。

**验证：** `go mod tidy && git diff --check` 成功，随后 `git status --short` 只显示本功能计划内文件。

### T74：运行后端聚焦测试

**文件：** 本任务清单中的后端文件
**依赖：** T21、T26、T36、T46、T52、T54、T59、T73

**步骤：**
1. 分别运行 types、database、repository、tracing、service、handler、container 和 router 包测试。
2. 对失败用例定位并修复实现或测试，不跳过。
3. 修复后重新运行受影响包，直到全部通过。

**验证：** `go test ./internal/types/... ./internal/database/... ./internal/application/repository/... ./internal/tracing/langfuse/... ./internal/application/service/... ./internal/handler/... ./internal/container/... ./internal/router/... -count=1` 通过。

### T75：运行全仓编译、测试和静态检查

**文件：** 全部改动文件
**依赖：** T70、T74

**步骤：**
1. 运行全仓 Go 测试和构建。
2. 运行仓库配置的 golangci-lint；只修复本次新增或修改代码产生的问题。
3. 再运行前端验证，确认后端修复没有破坏 DTO。

**验证：** `go test ./... -count=1 && go build ./... && golangci-lint run && ./scripts/verify_frontend_pr.sh` 全部通过。

### T76：执行真实端到端 Wiki 评测

**文件：** 不新增文件；结果记录到最终验收报告
**依赖：** T75

**步骤：**
1. 启动 PostgreSQL/Redis、后端和前端，使用具备 Admin 权限的测试租户配置可用 Chat 与 Embedding 模型。
2. 从 Wiki 页签启动 EnterpriseRAG 评测，记录所有阶段、最终覆盖率、图指标和两阶段成本。
3. 下载 JSON/Markdown，刷新页面后重新打开历史详情，并确认临时 KB、pending ops 和 dead letters 均已清理。

**验证：** 保存“页面启动 → 临时 Wiki 生成 → 指标展示 → 双报告下载 → 刷新恢复 → 临时资源清理”的实际证据，所有步骤成功。

### T77：执行范围和兼容性终检

**文件：** 全部改动文件
**依赖：** T76

**步骤：**
1. 对照 `spec.md` 的 F1–F14 和 AC1–AC19 逐项核验实现归属。
2. 确认 `.github/workflows`、CI 门禁和 RAG 指标公式没有变化。
3. 检查 diff 中没有密钥、临时日志、测试数据库、生成报告或临时 KB 数据。

**验证：** `git diff --check && git status --short` 正常，`git diff --name-only -- .github/workflows` 无输出，checklist 的所有条目均有可执行验证方式。

## 执行顺序

```text
T1 → T2 → T3
 └──────→ T4 → T5 → T6 → T7 → T8 → T9 → T10
                    └────→ T11 → T12 → T13 → T14 → T15 → T16 → T17 → T18 → T19 → T20 → T21
                    └────→ T22 → T23 → T24 → T25 → T26
                                      T27 → T28 → T29 → T30 → T31 → T32 → T33 → T34 → T35 → T36
                                      T37 → T38 → T39 → T40 → T41 → T42 → T43 → T44 → T45 → T46
T9 + T21 + T31 + T34 + T36 + T42 + T44 + T46 → T47 → T48 → T49 → T50 → T51 → T52 → T53 → T54
T50 + T53 → T55 → T56 → T57 → T58 → T59
T59 → T60 → T61 → T62 → T63 → T64 → T65 → T66 → T67 → T68 → T69 → T70
T55 + T60 + T68 → T71 → T72
T21 + T26 + T36 + T46 + T52 + T54 + T59 + T70 + T72 → T73 → T74 → T75 → T76 → T77
```

## 提交边界

1. T1–T10：运行表、领域类型与 RAG 兼容。
2. T11–T21：EnterpriseRAG 文档加载、Gold loader 与已审核 Gold。
3. T22–T36：异步归属、临时 KB、导入、等待与页面冻结。
4. T37–T46：节点、图评分与报告。
5. T47–T59：协调器、清理恢复、HTTP 与路由。
6. T60–T70：统一评测中心前端。
7. T71–T77：文档、全量验证与验收证据。

每个提交边界只在所含任务验证通过后提交；提交信息描述该阶段最终行为。

## 自检

- **Plan 覆盖：** 共享底座、数据/Gold、协调、临时资源、监控、冻结、节点匹配、图评分、异步成本、报告、恢复、HTTP 和前端均有实现及验证任务。
- **依赖链：** 所有任务只依赖更小编号任务；执行顺序无循环。
- **验证完整性：** T1–T77 每项均有具体命令或可观察结果。
- **类型一致性：** 类型名、接口名、阶段值和 API 路径与 `plan.md` 一致。
- **范围一致性：** 没有 SPO、LLM-as-Judge、自定义数据集、正式 KB 评分或 CI 门禁任务。
