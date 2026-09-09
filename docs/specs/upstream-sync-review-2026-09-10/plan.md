# 上游同步、评测功能复核与架构 Review Plan

## 执行结构

1. Git 同步层：fetch、提交差异检查、merge、祖先关系与迁移冲突检查。
2. 功能追踪层：从前端入口沿 API、路由、服务、仓储和迁移反向核对 RAG、Wiki、成本账本和缓存。
3. 架构 review 层：检查数据模型、并发与失败路径，并通过调用频率、数据规模和部署方式判断缓存存储。
4. 修复验证层：对确定问题做最小修改，运行聚焦测试后执行全仓回归。
5. 证据层：更新 review 文档和 checklist，提交并推送开发分支。

## 技术决策准则

| 场景 | 首选 | 判断依据 |
|---|---|---|
| 可跨重启复用、需 Lite 兼容的 Embedding 结果 | SQL | 已有双数据库、无需新增依赖、向量按键精确读取 |
| 高吞吐短生命周期热点 | Redis/L1 内存 | TTL 与淘汰便宜，但必须接受重启丢失和独立运维 |
| 当前实验及中等规模部署 | SQL 持久层，可选进程内热点层 | 优先保证复用、可观测和部署一致性，避免双写复杂度 |

## 审计重点

- 缓存键必须包含租户、模型、维度、文本哈希和可影响向量结果的模型身份。
- SQL 写入必须支持并发幂等；缓存失败必须回退 Provider。
- 缓存需要容量或保留期治理，避免无界增长。
- Wiki 临时资源必须在成功、失败和服务恢复路径清理。
- 页面指标必须与持久化结果和报告同源。
- 上游新增迁移、路由或前端组件不得覆盖定制接线。

## 文件范围

- `internal/models/embedding`、`internal/application/repository/embedding_cache.go`
- `internal/application/service/evaluation*.go`、`wiki_evaluation*.go`
- `internal/handler`、`internal/router`、`internal/container`
- `frontend/src/views/settings/*Evaluation*`、`ModelUsageSettings.vue`
- `migrations/versioned`、`migrations/sqlite`
- `docs/specs/evaluation-review-2026-09-08.md` 及本目录验收文档
