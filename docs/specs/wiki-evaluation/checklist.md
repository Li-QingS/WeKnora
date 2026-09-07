# Wiki 评测与统一评测中心 Checklist

> 状态：已批准（2026-09-07，用户授权开始开发）
> 每一项都通过运行命令、调用接口或观察用户可见行为验证；验收时填写实际结果和证据。

## 需求验收

- [ ] **C1 / AC1：统一入口。** 打开设置中的评测中心，能看到“RAG 问答评测”和“Wiki 评测”两个页签；切换页签后各自的表单、运行状态和历史分页互不覆盖。（验证：在浏览器中分别填写两个表单并来回切换，观察已填内容和列表状态）

- [ ] **C2 / AC2：RAG 启动兼容。** 使用升级前相同的参数启动一次 RAG 评测，任务正常运行并展示原有检索、生成、成本和延迟指标。（验证：通过 RAG 页签实际启动，记录请求、阶段和最终指标）

- [ ] **C3 / AC2：RAG 历史兼容。** 升级前已有的 RAG 记录仍出现在默认历史列表，详情、配置快照和删除操作正常，且不会混入 Wiki 指标结构。（验证：升级带旧记录的数据库后刷新页面并调用不带 `evaluation_type` 的历史接口）

- [ ] **C4 / AC3：Wiki 参数完整。** Wiki 页签可选择 EnterpriseRAG、Chat 模型、Embedding 模型和 `[0,1]` 语义阈值，默认阈值显示为 0.80。（验证：观察四项控件并分别选择有效值）

- [ ] **C5 / AC3：Wiki 参数拦截。** 任一必填项缺失、阈值越界、模型不存在或模型类型错误时不能启动，并显示对应字段的明确错误；系统中不产生 run 或临时 KB。（验证：逐类提交无效参数，比较提交前后的运行和知识库数量）

- [ ] **C6 / AC4：真实且隔离的 Wiki 生成。** 有效任务创建独立临时知识库，导入 85 篇语料，并使用所选 Chat 模型完成现有 Wiki 流水线。（验证：运行期间查询该 run 的临时 KB、knowledge 数量和生成阶段模型调用记录）

- [ ] **C7 / AC4：正式知识库不变。** Wiki 评测前后，当前租户已有正式知识库、knowledge 和 Wiki 页面数量及内容签名一致。（验证：运行前后导出同一正式 KB 的资源清单和哈希并比较）

- [ ] **C8 / AC5：阶段可见。** 成功运行依次可观察数据准备、文档导入、Wiki 生成、节点评分、图评分、保存报告和资源清理，进度与当前阶段同步变化。（验证：轮询详情接口并保存阶段序列，同时观察前端阶段条）

- [ ] **C9 / AC5：失败阶段可见。** 在导入、Wiki 生成或评分任一位置注入失败后，详情和页面显示最初 `failure_stage` 与可定位错误，清理阶段不会覆盖原失败位置。（验证：运行故障注入测试并读取终态详情）

- [ ] **C10 / AC6：页面类型过滤。** 输入同时含 entity、concept、summary、index、synthesis 和 comparison 页面时，节点总数、匹配候选和 coverage 只使用 entity/concept。（验证：运行固定页面 fixture 的冻结与评分测试，核对候选 ID）

- [ ] **C11 / AC7：名称规范化。** 全半角、大小写、两端标点和连续空白差异能精确匹配，空别名不能产生匹配。（验证：运行名称规范化固定样本并查看逐节点 method）

- [ ] **C12 / AC7：类型和一对一约束。** 跨类型同名项不匹配；多个 Gold/页面共享别名时，每个 Gold 和页面最多使用一次，结果不随输入顺序改变。（验证：多次打乱固定冲突样本运行评分并比较结果 JSON）

- [ ] **C13 / AC7：精确优先与语义阈值。** 精确候选先于 Embedding 分配；剩余节点仅在同类型且分数达到阈值时语义匹配，低于阈值的候选保持未匹配。（验证：运行含阈值上下样本的匹配测试，核对 method 和 score）

- [ ] **C14 / AC8：实体覆盖率正确。** 固定样本的实体 Gold 总数、精确数、语义数、未匹配数和 coverage 与手工计算一致。（验证：对测试输出按 `(精确+语义)/Gold` 复算）

- [ ] **C15 / AC8：概念与总体覆盖率正确。** 概念 coverage 与手工结果一致；overall 使用全部节点汇总计算，不是实体和概念 coverage 的简单平均。（验证：使用两类 Gold 数量不同的固定样本复算）

- [ ] **C16 / AC8：额外页面不惩罚覆盖率。** 在完全相同匹配结果中加入额外 entity/concept 页面后三类 coverage 不变，额外页面仍能在冻结快照中查看。（验证：比较加入页面前后的指标和快照）

- [ ] **C17 / AC9：有向边指标正确。** 含正确、缺失、多余和反向边的固定图输出 correct/missing/extra、Precision、Recall、F1 与手工集合计算一致。（验证：对输出边集求交集和差集后复算）

- [ ] **C18 / AC10：诱导子图边界正确。** 加入端点未匹配的 Wiki/Gold 链接后图指标不变；在两个已匹配节点间加入 Gold 不存在的边后 extra 增加且 Precision 下降。（验证：比较两组固定图评分结果）

- [ ] **C19 / AC9/AC10：重复边、自环和零分母明确。** 重复边只计一次，自环按普通有向边比较；没有可评分 Gold 边和生成边时数值为 0 且 `scorable=false`、页面显示“无可评分边”。（验证：运行三类边界 fixture 并观察 API 与页面）

- [ ] **C20 / AC11：节点详情可解释。** 每个 Gold 节点均显示类型、匹配状态、对应页面、exact/semantic/unmatched 方法；semantic 显示分数，unmatched 显示原因。（验证：在结果详情筛选三种方法并与 API JSON 逐项核对）

- [ ] **C21 / AC11：边详情可解释。** 正确、缺失、多余和未参与评分的链接可以分别筛选；未评分链接显示未匹配端点、未知 slug 或非评分页面等原因。（验证：在详情切换四种边类型并与报告边数组核对）

- [ ] **C22 / AC12：生成与评分模型隔离。** `<runID>:generation` 只有 Wiki 生成所需 Chat 调用，`<runID>:scoring` 没有 Chat 调用且只包含未精确匹配节点所需 Embedding 调用。（验证：按两个 request group 查询 `model_call_records` 并比较 model type）

- [ ] **C23 / AC12/N4：Embedding 去重和复用。** 同一运行中重复出现的规范化名称或别名只请求一次向量，详情和报告不触发新的 Embedding。（验证：使用重复文本 fixture 比较去重文本数、模型调用输入数，以及打开详情/下载前后的调用记录数）

- [ ] **C24 / AC13：结果可追溯。** 运行详情包含 dataset ID/版本/哈希、Gold schema/哈希、Chat/Embedding 模型身份、实际阈值、Wiki 配置、应用版本和 Git 签名。（验证：检查成功详情与 JSON 报告的 config snapshot）

- [ ] **C25 / AC13：成本分开统计。** 页面和报告分别显示 generation cost 与 scoring cost，两者的调用数和 token/cost 汇总等于对应 request group 的 ledger 汇总。（验证：查询两个 group 的模型调用并手工求和比较）

- [ ] **C26 / AC14：历史持久化。** Wiki 成功/失败运行均出现在 `evaluation_type=wiki` 历史中，刷新浏览器后状态、指标、详情和错误仍可查看。（验证：完成运行后刷新并重新打开详情）

- [ ] **C27 / AC14/N7：服务重启持久化与恢复。** 完成记录在后端重启后仍可查询；运行中的 Wiki 任务重启后保留原阶段，转 interrupted 并完成遗留临时 KB 清理。（验证：分别在终态和 generating 阶段重启服务，观察历史和资源）

- [ ] **C28 / AC14/N2：租户隔离。** 租户 B 无法通过详情、历史、报告或删除接口访问租户 A 的 Wiki run，也看不到 A 的临时 KB。（验证：用两个租户 token 对同一 run ID 调用所有读取/删除接口，期望 B 得到 404）

- [ ] **C29 / AC15：JSON 报告可用。** 下载的 JSON 可解析，包含页面所见运行信息、配置、指标、节点/边明细、成本、时间和错误字段。（验证：`python3 -m json.tool <downloaded-report>.json` 成功并与详情 API 比较）

- [ ] **C30 / AC15：Markdown 报告可用。** Markdown 报告包含与 JSON 相同的指标、配置、节点/边数量和失败摘要，临时 KB 删除后仍可下载。（验证：比较两份报告并在终态清理后再次下载）

- [ ] **C31 / AC16：导入和生成异常可信。** 模拟文档导入失败及 Wiki worker dead letter，run 进入 failed，保留准确失败阶段/任务标识，metric/result 为空。（验证：运行协调器失败用例并读取持久化 run）

- [ ] **C32 / AC16：模型和评分异常可信。** 模拟 Chat 调用、Embedding 调用、向量维度和结果保存失败，均显示实际阶段和原因，不渲染全零质量指标。（验证：运行各故障注入用例并检查 API/页面空指标处理）

- [ ] **C33 / AC17：成功资源清理。** success run 的临时知识库、knowledge、Wiki 页面、pending ops 和 dead letters 均不存在，run、结果与报告仍存在。（验证：按 `temporary_kb_id` 查询所有资源表/接口，再查询 run 和报告）

- [ ] **C34 / AC17：失败资源清理及重试。** 业务失败后清理成功才进入 failed；首次删除失败时保持 cleaning_up，恢复器重试成功后资源消失并进入正确终态。（验证：注入一次性删除失败并观察两次恢复结果）

- [ ] **C35 / AC18：Gold 完整性拦截。** Gold 缺失、未知字段、非法类型、重复 ID、悬空边、内容哈希错误和数据集哈希不匹配均在创建 run/KB 前被拒绝，并返回具体原因。（验证：逐个运行固定无效 Gold fixture）

- [x] **C36 / AC18：仓库 Gold 有效。** 提交的 EnterpriseRAG Gold 能通过严格加载，节点/边有稳定顺序，所有端点存在，schema、dataset 和 content 签名匹配。（验证：`go test ./internal/application/service -run TestWikiGoldFixture -count=1` 已包含于全量测试；`python3 -m json.tool dataset/enterprise_rag/wiki_gold.json` 通过）

- [x] **C37 / AC19：功能只能主动启动。** 未进行用户操作时不会自动创建 Wiki evaluation run；仓库 CI、合并门禁和定时任务没有新增 Wiki 评测步骤。（验证：仅注册手动 POST 入口；`git diff --name-only -- .github/workflows evaluation/configs` 无输出）

## 集成与部署检查

- [ ] **C38：PostgreSQL 升级和回滚。** 在包含旧 RAG 记录的数据库执行 up migration 后新增列/索引存在且旧记录为 `rag`；执行 down 后恢复原 schema，数据迁移过程无错误。（验证：在临时 PostgreSQL 数据库依次 migrate up/down/up 并查询 schema 与旧记录）

- [ ] **C39：SQLite/Lite 升级兼容。** SQLite 从上一版本升级后包含相同字段与索引，旧 RAG 数据可读，Wiki 结果 JSON 可写入并在重开连接后读取。（验证：运行 SQLite versioned migration 与数据保留测试）

- [ ] **C40：异步 request group 跨配置传播。** Langfuse 开启和关闭时，HTTP → knowledge post-process → wiki ingest → wiki finalize 都保留相同 generation group；旧 payload 没有新字段时仍能处理。（验证：运行异步 carrier 与 Wiki pending-op 集成测试）

- [ ] **C41：完成判定没有提前冻结。** 所有 knowledge 成功但 finalize 尚未结束时任务仍处于 generating；两条队列为空且无 dead letter 连续两次后才进入 scoring。（验证：在两次稳定检查之间插入 finalize row 并观察阶段）

- [ ] **C42：运行上限和取消生效。** 超过 4 小时的生成进入可定位失败/中断清理流程；轮询 context 取消后不残留监控 goroutine。（验证：用可控时钟缩短超时并检查终态与 goroutine/调用退出）

- [ ] **C43：并发运行相互隔离。** 同一租户同时启动两个 Wiki run 时使用不同临时 KB、request group、结果和清理目标，任一失败不影响另一任务。（验证：并发运行两个短 fixture 协调器并比较所有 ID 和结果）

- [x] **C44：API 权限一致。** Wiki 启动要求 Admin 和 run-evaluations capability；详情、数据集和报告允许 Viewer；共享删除仍要求 Admin。（验证：`go test ./internal/router -count=1` 通过，路由能力矩阵覆盖新增端点）

- [x] **C45：前后端 DTO 一致。** 页面能解析 pending/running/success/failed/interrupted、全部 Wiki stages、空 metric 和完整结果，浏览器控制台无字段或类型错误。（验证：`./scripts/verify_frontend_pr.sh` 的类型检查和生产构建通过）

- [ ] **C46：报告与页面同源。** 对同一 run，详情 API、页面、JSON 和 Markdown 的聚合指标及节点/边计数完全一致；打开或下载不会重新读取临时 Wiki。（验证：终态清理后重复四处比较，并确认无模型调用增加）

- [x] **C47：公开接口有真实调用方。** 新增服务、仓储和评分接口均被协调器、handler 或恢复器使用，没有只为测试存在的公开入口。（验证：协调器、handler、容器注入和启动恢复均已接线；`go build ./...` 通过）

## 编译与自动化测试

- [x] **C48：Gold 与数据集测试通过。** （验证：已通过，并再次包含于 `go test ./... -count=1`）

- [x] **C49：节点和图评分测试通过。** （验证：已通过，并再次包含于 `go test ./... -count=1`）

- [x] **C50：协调、失败、清理和报告测试通过。** （验证：已通过，并再次包含于 `go test ./... -count=1`）

- [x] **C51：仓储、迁移、异步和租户测试通过。** （验证：相关包与 `go test ./... -count=1` 均通过）

- [x] **C52：HTTP 和权限测试通过。** （验证：相关包与 `go test ./... -count=1` 均通过）

- [x] **C53：前端测试、类型检查和生产构建通过。** （验证：`./scripts/verify_frontend_pr.sh` 通过：599 项断言、类型检查、Vite 生产构建）

- [x] **C54：全仓 Go 测试和构建通过。** （验证：`go test ./... -count=1` 与 `go build ./...` 均通过）

- [ ] **C55：Go lint 与格式检查通过。** （验证：`gofmt -l` 对本次 Go 文件无输出，`golangci-lint run` 无新增问题，`git diff --check` 无输出）

- [x] **C56：国际化键检查通过。** （验证：`cd frontend && npm run check-i18n` 通过，11 项断言）

## 端到端场景

- [ ] **E1：完整成功流程。** 使用测试租户在评测中心打开 Wiki 页签，选择 EnterpriseRAG、有效 Chat/Embedding 模型和 0.80 阈值并启动；观察 85 篇文档导入和全部阶段，最终看到实体/概念/总体 coverage、图 Precision/Recall/F1、两阶段成本和可下钻明细；下载 JSON/Markdown，刷新后从历史重新打开；确认临时资源已删除而结果仍可用。（验证：保存页面截图、run ID、阶段轮询记录、报告文件和资源查询结果）

- [ ] **E2：RAG 回归流程。** 在同一评测中心切换到 RAG 页签，以现有配置完成一次评测，并打开一条升级前历史记录；两者的参数、指标、进度和详情与改动前一致。（验证：保存新旧 RAG run 的页面和 API 结果）

- [ ] **E3：启动前失败流程。** 依次使用缺失模型、错误模型类型、越界阈值和哈希不匹配 Gold 发起 Wiki 评测；页面显示具体错误，没有创建 run 或知识库。（验证：保存错误响应及前后资源计数）

- [ ] **E4：运行中失败与恢复流程。** 让一次 Wiki 运行在生成阶段失败，再让临时 KB 首次删除失败；页面先显示 cleaning_up 和原 failure stage，恢复器成功后显示 failed，质量指标为空，临时资源消失，错误报告仍可下载。（验证：保存故障注入、两次状态和最终资源查询证据）

- [ ] **E5：租户隔离流程。** 租户 A 完成 Wiki 评测后，用租户 B 查询列表、详情、报告和删除该 run，均无法访问；租户 A 仍可正常查看和删除历史记录。（验证：保存两个 token 对同一 run ID 的 HTTP 状态与响应）

## 范围检查

- [ ] **S1：没有三元组或谓词指标。** 页面、API、报告和代码公开结构中没有 SPO、关系谓词准确率或三元组总分。（验证：检查用户界面、API 文档和结果 JSON）

- [ ] **S2：评分阶段没有生成模型裁判。** 节点与图得分只来自规范化、Embedding 相似度和确定性图集合运算。（验证：评分单元测试使用 fake Embedding 即可完整计算，scoring request group 无 Chat 调用）

- [ ] **S3：没有跨类型合成总分。** RAG 和 Wiki 分别运行、分别展示，结果中不存在 RAG+Wiki 综合分。（验证：检查两个详情 API 与页面指标卡）

- [ ] **S4：没有扩展到任意用户数据集。** Wiki 数据集选择器首期只列出带有效 Gold 的 EnterpriseRAG，不提供上传 Gold 或选择正式 KB 的入口。（验证：观察选择器和启动 API 校验）

- [ ] **S5：没有修改 CI 门禁。** `.github/workflows` 和现有评测门禁配置没有本功能改动，Wiki 评测只由用户从 API/页面主动启动。（验证：`git diff --name-only -- .github/workflows evaluation/configs` 无输出）

## AC 覆盖索引

| 验收标准 | Checklist |
|---|---|
| AC1 | C1 |
| AC2 | C2–C3 |
| AC3 | C4–C5 |
| AC4 | C6–C7 |
| AC5 | C8–C9 |
| AC6 | C10 |
| AC7 | C11–C13 |
| AC8 | C14–C16 |
| AC9 | C17、C19 |
| AC10 | C18–C19 |
| AC11 | C20–C21 |
| AC12 | C22–C23 |
| AC13 | C24–C25 |
| AC14 | C26–C28 |
| AC15 | C29–C30 |
| AC16 | C31–C32 |
| AC17 | C33–C34 |
| AC18 | C35–C36 |
| AC19 | C37、C48–C56、E1 |

## 验收记录格式

执行时在每项后追加：

```text
结果：通过 / 未通过
证据：实际命令输出、HTTP 状态、run ID、报告路径或页面观察
问题：未通过时记录预期、实际和修复动作
```
