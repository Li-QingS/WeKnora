# 验收清单

- [x] 同日起止返回当天调用。
- [x] 次日零点不归入前一天。
- [x] RFC3339 精确时间查询保持兼容。
- [x] RAG 与 Wiki 入口清楚且可切换。
- [x] Wiki 节点覆盖和连接结构的含义、参数、结果清楚。
- [x] Wiki 生成调用 Chat，评分只用确定性匹配与 Embedding 的说明准确。
- [x] 自动化测试、类型检查和构建通过。

## 验收记录

- 日期实数核对：旧 UTC 平移区间查询 2026-09-08 得到 0 条；新自然日区间得到 56 条。
- 后端：`go test ./... -count=1` 通过。
- 前端：`npm test` 通过 666 项测试；`npm run type-check` 和 `npm run build` 通过。
- 数据库：SQLite 从 0 和旧版本升级到 migration 19 的测试通过；PostgreSQL started_at 索引迁移已做事务内验证。
