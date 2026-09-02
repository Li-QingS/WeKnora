# 评测与成本可视化页面（课题三 WP7）Checklist

> 状态：已验收（2026-09-02）
> 上游文档：[spec.md](./spec.md)、[plan.md](./plan.md)、[task.md](./task.md)（均已批准）
> 说明：每一项通过运行命令或观察行为验证；标记 `[x]` 前必须有命令输出或可复现行为作为证据。

## 实现完整性（对应 AC1-AC7）

- [x] AC1 评测中心可见：`Settings.vue` 注册 `evaluation` section 与导航项；真实 `GET /evaluation/runs` 返回 1 条历史运行，页面数据源可渲染。（验证：页面代码 + API 响应 + `npm run build`）
- [x] AC2 详情可用：点击行后展示 12 项指标（检索 6 + 生成 6）、`config_hash`、数据集、样本数、模型快照与版本；真实详情接口返回同结构数据。（验证：`EvaluationCenterSettings.vue` + API `metric_count=12`）
- [x] AC3 缓存统计可见：`ModelUsageSettings.vue` 展示 hits/misses/provider_calls，请求失败或字段为空走 `unknown`；真实统计接口返回 `{hits:0, misses:0, provider_calls:0}`。（验证：组件代码 + API）
- [x] AC4 API 封装可用：`listEvaluationRuns` / `getEvaluationResult` / `getEmbeddingCacheStats` 与真实后端响应字段一致，路由为 `/evaluation/runs`、`/evaluation`、`/embedding-cache/stats`。（验证：真实 curl + type-check）
- [x] AC5 权限正确：`settingsAccess.ts` 中 `evaluation: 'viewer'`、`modelusage: 'admin'`；后端 `RegisterEvaluationRoutes` 读接口为 `g.Viewer()`，模型用量/缓存读接口为 `g.Admin()`。（验证：配置 + `internal/router/routes_infra.go`）
- [x] AC6 只读：评测中心与模型用量页面均无删除/修改评测、价格编辑表单；新增 API 封装只有 GET。（验证：代码 review）
- [x] AC7 回归：前端 type-check/build 通过；后端 build/test、SDK test、CLI test/vet 通过。（验证：最终回归命令）

## UI 一致性

- [x] 评测中心使用 Settings 现有 `.section-header / .section-description` 结构与间距。（验证：组件模板 + 页面样式对照）
- [x] 列表使用 TDesign `t-table`，状态使用 `t-tag`，分页使用 `t-pagination`；空态与加载使用 `t-empty` / `t-loading`。（验证：组件模板 + DOM 类名）
- [x] 缓存统计区块复用 `.settings-group` 风格，采用同色系分隔线行，不引入独立装饰卡片。（验证：`ModelUsageSettings.vue` 样式）
- [x] 未新增渐变、阴影、异形圆角、装饰背景；颜色只用 TDesign 主题与现有 CSS 变量。（验证：CSS review + git diff）
- [x] 其他 Settings 页面布局与行为未被改动，改动仅限新页面注册、API 封装与模型用量页顶部区块。（验证：`git diff` 范围）

## 编译与测试

- [x] 前端：`cd frontend && npm run type-check` 通过
- [x] 前端：`cd frontend && npm run build` 通过
- [x] 服务端：`go build ./...` 通过
- [x] 服务端：`go test ./internal/...` 通过
- [x] SDK：`cd client && go test ./...` 通过
- [x] CLI：`cd cli && go test ./...` 与 `go vet ./...` 通过

## 端到端场景

- [x] 场景 1（评测中心）：真实后端建一条成功运行后，列表接口返回运行记录与 `config_snapshot`，详情接口返回 12 项指标与参数；页面组件按列表行加载详情区。（验证：真实 API + 组件逻辑）
- [x] 场景 2（模型用量 + 缓存）：模型用量页保留原汇总/明细，并在顶部渲染缓存统计；缓存关闭/无数据时数值为 0 或 `unknown`，页面不报错。（验证：组件 + `/embedding-cache/stats`）
- [x] 场景 3（无回归）：`settingsAccess.ts` 将评测中心开放给 Viewer、模型用量保留 Admin，后端路由矩阵与之一致；普通用户的入口由 Settings 导航统一过滤。（验证：配置 + 路由代码）

## 验收记录

- 真实 API 验证使用本地 PG 临时插入一条 `evaluation_runs`，列表/详情/缓存统计均返回成功；测试行与临时账号已清理。
- 未做浏览器截图式目视复核；视觉一致性通过组件代码、CSS 变量与 `vite build` 产物检查完成。
