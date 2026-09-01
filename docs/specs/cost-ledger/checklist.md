# 模型调用台账与费用估算（课题三 WP4）Checklist

> 状态：已验收（2026-09-02）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行命令或观察行为验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC11）

- [x] AC1 双库迁移：PG 000089/000090 与 SQLite 000014/000015 创建两张表；SQLite 守卫测试更新到版本 15，从零建库与升级路径通过。（验证：`go test ./internal/database/ -v`）
- [x] AC2 Chat 落库：真实 mock Chat 调用后 `model_call_records` 出现 17 条成功记录，含租户、模型、状态、Token、耗时与价格快照。（验证：SQLite 查库 + API 明细）
- [x] AC3 Embedding 落库：mock Embedding 调用后出现 30 条成功记录，Token 为近似值。（验证：SQLite 查库 + API 明细）
- [x] AC4 失败落库：cost wrapper 失败路径单测覆盖，记录 status=failed、错误类型与摘要；实现不存 Prompt/Key。（验证：`cost_wrapper_test.go`）
- [x] AC5 未知价格为空：未配置价格时 `estimated_cost_usd=nil`，页面/API 显示 unknown。（验证：`TestModelCallRecorderUnknownPriceNil` + 前端 formatCost）
- [x] AC6 明细查询：`GET /api/v1/model-calls` 分页 + 筛选生效，只返回当前租户数据。（验证：handler/repository 单测 + 真实请求）
- [x] AC7 汇总查询：`GET /api/v1/model-calls/summary` 返回按模型的次数、成功/失败、Token、费用。（验证：真实请求 + repository 单测）
- [x] AC8 价格配置：设置价格后新调用使用新价格并保存快照；改价后旧记录不变（快照随记录保存）。（验证：价格 upsert 单测 + e2e 价格快照）
- [x] AC9 页面可见：`ModelUsageSettings.vue` 已接入设置页并展示汇总与明细；`npm run type-check` 通过。（验证：前端 type-check + 页面路由/菜单代码）
- [x] AC10 隔离与脱敏：Repository 全部带 tenant 条件；DB 与 API 响应无 Prompt 正文、API Key、endpoint 凭据。（验证：repository 单测 + 记录字段检查）
- [x] AC11 回归：cost wrapper 未安装 Recorder 时 no-op；`go build` / `go test` / vet / 前端 type-check 全过。（验证：costledger 单测 + 最终回归）

## 集成

- [x] 五个模型类型都有 cost wrapper 并安装：Chat / Embedding / Rerank / VLM / ASR。（验证：代码 + 各包测试）
- [x] 流式 Chat 在流结束时用最终 Usage 落库。（验证：`costChat.ChatStream` 实现 + 单测路径）
- [x] Recorder 未设置时 no-op。（验证：`TestRecordNoopWithoutRecorder`）
- [x] 租户来源：context 优先，模型 TenantID 兜底。（验证：`costledger.NewCallInfo` + wrapper 单测）
- [x] 价格快照随记录保存，费用计算覆盖 token、unit。（验证：`model_cost_test.go`）
- [x] 容器启动时安装 Recorder。（验证：`go test ./internal/container/` + 启动日志）
- [x] 路由使用 Admin 权限并注册成功。（验证：router/container 测试 + 真实 API）

## 编译与测试

- [x] 服务端：`go build ./...` 通过
- [x] 服务端：`go test ./internal/...` 通过
- [x] SDK：`cd client && go test ./...` 通过
- [x] CLI：`cd cli && go test ./...` 通过
- [x] CLI：`cd cli && go vet ./...` 通过
- [x] 数据库：`go test ./internal/database/ -v` 通过
- [x] 前端：`npm run type-check` 通过

## 端到端场景

- [x] 场景 1（正常记账）：Lite + mock 模型端到端 47 条记录验证；另用真实模型（qwen3.7-text-embedding / qwen3.8-27b）完整跑通，Token 与费用正确，API 明细/汇总可查。
- [x] 场景 2（失败与未知价格）：cost wrapper 失败路径单测覆盖；未知价格单测确认费用 null。
- [x] 场景 3（隔离与脱敏）：repository 单测覆盖租户过滤；记录字段无 Prompt/Key。

## 验收记录

实际命令、数据库核对与 API 响应见 [progress-2026-09-02.md](./progress-2026-09-02.md)。
