# 模型调用台账与费用估算（课题三 WP4）Tasks

> 状态：待审批（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）、[plan.md](./plan.md)（已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/types/model_call.go` | ModelCallInfo / Record / Price / Filter / Summary |
| 新建 | `internal/types/interfaces/model_call.go` | Repository / Service 接口 |
| 新建 | `internal/costledger/recorder.go` | 全局 Recorder hook |
| 新建 | `internal/costledger/recorder_test.go` | Recorder 单测 |
| 修改 | `internal/models/chat/chat.go` | ChatConfig.TenantID + ConfigFromModel |
| 新建 | `internal/models/chat/cost_wrapper.go` | Chat cost wrapper |
| 修改 | `internal/models/chat/chat.go` | NewChat 安装 wrapper |
| 修改 | `internal/models/embedding/embedder.go` | Config.TenantID + ConfigFromModel + NewEmbedder |
| 新建 | `internal/models/embedding/cost_wrapper.go` | Embedding cost wrapper |
| 修改 | `internal/models/rerank/reranker.go` | Config.TenantID + ConfigFromModel + NewReranker |
| 新建 | `internal/models/rerank/cost_wrapper.go` | Rerank cost wrapper |
| 修改 | `internal/models/vlm/vlm.go` | Config.TenantID + ConfigFromModel + NewVLM |
| 新建 | `internal/models/vlm/cost_wrapper.go` | VLM cost wrapper |
| 修改 | `internal/models/asr/asr.go` | Config.TenantID + ConfigFromModel + NewASR |
| 新建 | `internal/models/asr/cost_wrapper.go` | ASR cost wrapper |
| 新建 | `internal/application/repository/model_call.go` | Repository GORM 实现 |
| 新建 | `internal/application/repository/model_call_test.go` | Repository 单测 |
| 新建 | `internal/application/service/model_cost.go` | ModelCallRecorder + 费用计算 + Service |
| 新建 | `internal/application/service/model_cost_test.go` | 费用/recorder 单测 |
| 新建 | `internal/handler/model_call.go` | 明细/汇总/价格 API |
| 新建 | `internal/handler/model_call_test.go` | Handler 单测 |
| 修改 | `internal/router/routes_infra.go` | 注册路由 |
| 修改 | `internal/container/container.go` | Repository/Service/Recorder 接线 |
| 新建 | `migrations/versioned/000089_model_call_records.up.sql` / `.down.sql` | PG 台账表 |
| 新建 | `migrations/versioned/000090_model_prices.up.sql` / `.down.sql` | PG 价格表 |
| 新建 | `migrations/sqlite/000014_model_call_records.up.sql` / `.down.sql` | SQLite 台账表 |
| 新建 | `migrations/sqlite/000015_model_prices.up.sql` / `.down.sql` | SQLite 价格表 |
| 修改 | `internal/database/migration_sqlite_versioned_schema_test.go` | 守卫测试 |
| 新建 | `frontend/src/api/model/usage.ts` | 前端 API |
| 新建 | `frontend/src/views/system/ModelUsage.vue` | 模型用量页面 |
| 修改 | `frontend/src/router/...` | 路由与菜单 |
| 新建 | `docs/specs/cost-ledger/progress-2026-09-02.md` | 实现与验收记录 |

## 执行顺序

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10
```

T1/T2 是类型与 Recorder，T3 是五个模型包装层，T4/T5 是数据与费用，T6 是迁移，T7/T8 是 API 与接线，T9 是页面，T10 是端到端验收。

---

## T1: 台账类型与接口

**文件：** 新建 `internal/types/model_call.go`、`internal/types/interfaces/model_call.go`
**依赖：** 无
**步骤：**
1. 定义 `ModelCallStatus` 常量（success/failed）。
2. 定义 `ModelCallInfo / ModelCallRecord / ModelPrice / ModelCallFilter / ModelCallSummaryItem / PriceSnapshot`，字段与 [plan.md](./plan.md) 一致。
3. `ModelCallRecord` 与 `ModelPrice` 加 GORM tag；`PriceSnapshot` 加 JSON tag。
4. 定义 `ModelCallRepository`、`ModelPriceRepository`、`ModelCallService` 接口。

**验证：**
```bash
go test ./internal/types/...
```

---

## T2: 全局 Recorder

**文件：** 新建 `internal/costledger/recorder.go`、`internal/costledger/recorder_test.go`
**依赖：** T1
**步骤：**
1. 定义 `Recorder` 接口：`Record(ctx, *types.ModelCallInfo) error`。
2. 包级单例 + 互斥锁，提供 `SetRecorder` / `GetRecorder` / `Record`；recorder 为 nil 时 `Record` no-op。
3. 单测：未设置时 no-op；设置后调用；可替换。

**验证：**
```bash
go test ./internal/costledger/
```

---

## T3: 五个模型 cost wrapper

**文件：** 修改 chat/embedding/rerank/vlm/asr 的 Config/构造，新增各自 `cost_wrapper.go`
**依赖：** T2
**步骤：**
1. 为 `ChatConfig`、`embedding.Config`、`RerankerConfig`、`vlm.Config`、`asr.Config` 增加 `TenantID uint64`，并在各自 `ConfigFromModel` 中填充 `m.TenantID`。
2. 每个包新增 `cost_wrapper.go`：
   - 包装接口方法，调用开始记录 `StartedAt`，结束组装 `types.ModelCallInfo`；
   - Chat 流式在消费 channel 时捕获最终 `Usage`；
   - Embedding 无供应商 usage 时用近似估算（沿用 Langfuse 口径）；
   - Rerank 记录 `unit_type=documents, unit_count=len(documents)`；
   - VLM / ASR 记录 `unit_type=requests, unit_count=1`；
   - 从 context 读取 tenant/session/principal/purpose；缺失租户回退 `TenantID`；
   - 调用 `costledger.Record`，错误只记日志。
3. 在 `NewChat`、`NewEmbedder`、`NewReranker`、`NewVLM`、`NewASR` 最外层安装 cost wrapper。
4. 为每个 wrapper 增加最小单测（成功路径 + 失败路径 + recorder no-op）。

**验证：**
```bash
go test ./internal/models/chat/ ./internal/models/embedding/ ./internal/models/rerank/ ./internal/models/vlm/ ./internal/models/asr/
```

---

## T4: Repository

**文件：** 新建 `internal/application/repository/model_call.go`、`internal/application/repository/model_call_test.go`
**依赖：** T1
**步骤：**
1. `modelCallRepository.Create`：写入记录。
2. `modelCallRepository.List`：tenant 必带，支持 model_id/model_type/status/from/to，分页，按 created_at DESC。
3. `modelCallRepository.Summary`：按 model_id/model_name/model_type 分组聚合。
4. `modelPriceRepository.Get / Upsert / List`：`(tenant_id, model_id)` 唯一约束，冲突时更新。
5. SQLite 内存库单测：写入/筛选/分页/汇总/租户隔离/价格 upsert。

**验证：**
```bash
go test ./internal/application/repository/ -run 'ModelCall|ModelPrice'
```

---

## T5: ModelCallRecorder 与费用计算

**文件：** 新建 `internal/application/service/model_cost.go`、`internal/application/service/model_cost_test.go`
**依赖：** T1、T4
**步骤：**
1. `modelCallRecorder` 实现 `costledger.Recorder`：
   - 查租户价格，查不到查全局价格；
   - 组装 `PriceSnapshot`；
   - 计算费用；价格缺失时 `EstimatedCostUSD=nil`；
   - 落库。
2. `ModelCallService` 实现 `List / Summary / UpsertPrice / ListPrices`，租户从 context 取。
3. 单测：token 费用、cache 费用、unit 费用、未知价格 null、租户隔离。

**验证：**
```bash
go test ./internal/application/service/ -run 'ModelCall|ModelCost|ModelPrice'
```

---

## T6: 双库迁移

**文件：** 新建 PG 000089/000090、SQLite 000014/000015，修改守卫测试
**依赖：** T1
**步骤：**
1. 编写 `model_call_records` 建表/回滚 SQL，PG 用 JSONB，SQLite 用 TEXT。
2. 编写 `model_prices` 建表/回滚 SQL，唯一约束 `(tenant_id, model_id)`。
3. 更新 `versionedSQLiteTables` 与 `expectedSQLiteMigrationVersion = 15`。
4. 运行数据库测试验证从零建库与升级路径。

**验证：**
```bash
go test ./internal/database/ -v
```

---

## T7: API Handler 与路由

**文件：** 新建 `internal/handler/model_call.go`、`internal/handler/model_call_test.go`，修改 `internal/router/routes_infra.go`
**依赖：** T5
**步骤：**
1. 实现：
   - `GET /api/v1/model-calls`（分页明细 + 筛选）；
   - `GET /api/v1/model-calls/summary`；
   - `GET /api/v1/model-prices`；
   - `GET /api/v1/model-prices/:modelId`；
   - `PUT /api/v1/model-prices/:modelId`。
2. 校验筛选参数与分页；错误映射遵循现有 handler 风格。
3. 路由使用 Admin 权限组。
4. 单测：明细/汇总/价格写入/参数错误/租户隔离。

**验证：**
```bash
go test ./internal/handler/ -run 'ModelCall|ModelPrice'
```

---

## T8: 容器接线

**文件：** `internal/container/container.go`
**依赖：** T5、T6
**步骤：**
1. Provide `NewModelCallRepository`、`NewModelPriceRepository`、`NewModelCallService`。
2. 启动后 Invoke 构造 `ModelCallRecorder` 并 `costledger.SetRecorder`；失败只记日志。
3. 确认容器构建与全量测试通过。

**验证：**
```bash
go build ./...
go test ./internal/container/
```

---

## T9: 前端最小页面

**文件：** 新建 `frontend/src/api/model/usage.ts`、`frontend/src/views/system/ModelUsage.vue`，修改路由/菜单
**依赖：** T7
**步骤：**
1. `usage.ts`：封装 `listModelCalls`、`getModelCallSummary`、`listModelPrices`、`upsertModelPrice`。
2. `ModelUsage.vue`：
   - 汇总表：模型、类型、调用次数、成功/失败、Token、估算费用；
   - 明细表：最近调用，分页；
   - 费用为空显示 `unknown`。
3. 注册路由与设置菜单入口（以现有 settings 结构为准）。
4. 如依赖已安装，运行 type-check / build；否则记录未验证。

**验证：**
```bash
cd frontend && npm run type-check
```

---

## T10: 端到端验收与进展记录

**文件：** 新建 `docs/specs/cost-ledger/progress-2026-09-02.md`
**依赖：** T1-T9
**步骤：**
1. 启动 Lite 服务，配置一个模型价格。
2. 发起一次 Chat/Embedding 调用，查询数据库确认 `model_call_records` 落库。
3. 调用明细/汇总 API 验证数据与费用。
4. 制造一次失败调用，确认 failed 记录。
5. 全量回归：`go build ./...`、`go test ./internal/...`、client/CLI 测试、vet；前端如有依赖则 type-check。
6. 记录命令、输出、数据库核对到 progress 文档。

**验证：**
```bash
python3 - <<'PY'
import sqlite3
con = sqlite3.connect('data/e2e.db')
print(con.execute('select count(*) from model_call_records').fetchone())
PY
```

---

## 最终回归

```bash
go build ./...
go test ./internal/...
cd client && go test ./...
cd cli && go test ./... && go vet ./...
```

并在 `progress-2026-09-02.md` 记录结果。
