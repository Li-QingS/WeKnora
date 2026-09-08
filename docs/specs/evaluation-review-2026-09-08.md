# 评测功能合并与代码审查记录（2026-09-08）

## 上游同步结论

- 腾讯源仓库：`upstream = git@github.com:Tencent/WeKnora.git`。
- 本次拉取后的腾讯 `upstream/main`：`647848f3`。
- 当前开发分支：`feat/evaluation-persistence`。
- 合并提交：`0a2f6cf4 Merge remote-tracking branch 'upstream/main' into feat/evaluation-persistence`。
- `git merge-base --is-ancestor upstream/main HEAD` 返回成功，`HEAD...upstream/main` 为 `35 0`：当前分支包含腾讯 main 的全部提交，对腾讯 main 不落后；领先部分是本项目的评测开发及合并提交。
- 合并没有冲突。PostgreSQL 自定义迁移保持在 000092–000096，SQLite 自定义迁移保持在 000014–000018，与本次上游迁移序列没有编号冲突。

## 功能完整性结论

代码层面的 RAG 评测、持久化 Runner、RAG CI 门禁、模型调用台账、Embedding 缓存、统一评测中心和 Wiki 实验评测均已接线。Wiki 评测仍符合既定范围：生成 Wiki 时调用所选 Chat 模型；评分只做名称规范化、Embedding 匹配和确定性有向图集合计算，不使用生成模型裁判，也没有接入 CI 门禁。

Wiki 评测当前覆盖：

- EnterpriseRAG 语料及严格校验的版本化 Gold；
- 实体、概念和总体覆盖率；
- 有向图 Precision、Recall、F1 及正确、缺失、多余、未评分边明细；
- 独立临时知识库、阶段进度、失败阶段、历史、配置快照、JSON/Markdown 报告；
- generation/scoring 两组模型调用和成本归属；
- 成功、失败、超时和服务重启后的临时资源清理；
- 评测中心中的 RAG/Wiki 双页签、任务轮询、刷新恢复、详情和下载。

真实模型、真实 PostgreSQL 服务和浏览器操作仍属于环境验收，详见 `wiki-evaluation/checklist.md` 中未勾选项目。它们需要可用的 Chat/Embedding 凭据和完整服务栈，不能由纯单元测试替代。

## 本次审查发现并修复的问题

| 级别 | 问题 | 修复 |
|---|---|---|
| 高 | Embedding 缓存键只含模型 ID、维度和文本；模型记录原地更换名称、端点或截断配置后可能复用旧向量 | 增加不含凭据的向量配置签名，并纳入文本哈希命名空间 |
| 高 | Batch Embedding 默认相信供应商返回数量，少返回时会数组越界 | 校验返回向量数量、维度、空向量和非有限值，异常返回明确错误 |
| 中 | Batch Embedding 的重复冷文本会重复调用供应商 | 批内按最终缓存键去重，返回结果仍按原输入顺序展开 |
| 高 | Wiki 清理失败只会在下一次服务启动时重试，运行中的进程没有周期恢复 | 增加每分钟 stale 扫描和幂等清理恢复，并纳入 ResourceCleaner 生命周期 |
| 中 | 切走 Wiki 页签后轮询继续；重新进入页面不能恢复当前运行 | Wiki 面板改为按页签挂载，卸载时停止轮询，挂载时从最新持久化运行恢复 |
| 中 | Wiki 启动的数据库或内部错误被错误映射为 HTTP 400 | 增加可识别的参数错误，输入错误返回 400，内部错误返回 500 |
| 中 | 评测运行和模型调用在相同创建时间下分页顺序不稳定 | 增加 `id DESC` 次级排序 |
| 高 | 请求取消后成本台账使用已取消 context 写库，可能丢失已发生的模型调用 | 台账写入保留 context 值、脱离调用方取消，并限制为 5 秒 |
| 高 | Chat 流通过响应通道返回错误时，台账仍记录为 success | 识别 error/incomplete 响应并记录 failed；取消后停止向下游发送并继续排空上游通道 |
| 中 | 价格 API 接受负数、混合计费和非 USD，但页面及字段按美元展示 | 校验非负有限价格、计费方式互斥，仅接受 USD；数据库错误与未找到错误使用不同 HTTP 状态 |
| 低 | 仓库误跟踪 66 MB 的 `wiki-rerun` 二进制和空文件 `=` | 删除两个文件并将 `wiki-rerun` 加入 `.gitignore` |
| 低 | 合并后 spec 中迁移编号和审批状态过期 | 更新到当前迁移编号，并按用户授权改为自主审查状态 |

## 技术栈审查

### Embedding 缓存

当前选择“SQL 持久化缓存、默认关闭”适合本期实验功能，不建议直接替换成 Redis：

1. WeKnora Lite 使用 SQLite 且可以不部署 Redis，SQL 方案让 Lite 和标准模式使用同一语义。
2. SQL 缓存跨进程重启和多副本共享，适合重复评测和可复现实验。
3. 项目 Redis 同时承载队列等关键数据，并推荐 `noeviction`；把无容量上限的大向量放进同一实例会扩大内存和队列可用性风险。
4. 远程 Embedding 调用通常远慢于一次有索引的 SQL 查询，因此即使 SQL 不是最低延迟缓存，也能在实验负载下降低模型调用次数。

当前 SQL 实现不适合直接作为高 QPS、无限期启用的生产缓存：向量以 JSON 存储、批量请求仍按唯一文本逐行读取、每次命中会更新数据库计数，而且目前没有 TTL 或容量清理。保持默认关闭是正确的上线边界。

若后续转为生产能力，建议保留 SQL 作为可选持久化 L2，并按顺序增加：

1. 容量/时间保留策略和按 tenant/model 清理；
2. 批量 Get/Set 与命中计数批量化，降低数据库往返和写竞争；
3. 进程内有界 L1；标准模式有明确高吞吐需求时，再增加带 TTL 的专用 Redis L1，避免与任务队列共用容量预算。

### 模型调用台账

模型调用和价格快照继续使用 SQL 是合理的。它们属于需要租户查询、历史追溯、聚合和报告一致性的业务记录，不应放在会过期或驱逐的 Redis 中。当前同步落库保证 Wiki 在评分结束后立即按 request group 汇总成本；本次增加 5 秒独立上限，避免请求取消导致丢单。生产数据增长后需要增加明细保留或日聚合，但不应通过改用 Redis 解决。

### Wiki 评测编排

运行状态、结果和报告元数据落 SQL，Wiki 生成复用项目已有知识处理队列，评分保持无 Chat 裁判，这一组合与现有架构一致。恢复器既在启动时执行，也周期扫描停止心跳的任务；ResourceCleaner 会在服务退出时停止恢复 goroutine，避免生命周期泄漏。

## 自动化验证

2026-09-08 已通过：

- `go test ./... -count=1`
- `go vet ./...`
- `go build ./...`
- `client`: `go test ./... -count=1`、`go vet ./...`
- `cli`: `go test ./... -count=1`、`go vet ./...`
- `frontend`: `npm test`（638 项测试）、`npm run type-check`、`npm run build`
- `git diff --check`

Vite 仍报告既有的大 chunk 提示，但构建成功；该提示与本次评测功能的正确性无关，适合单独做前端拆包优化。
