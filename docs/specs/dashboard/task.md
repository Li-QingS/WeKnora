# 评测与成本可视化页面（课题三 WP7）Tasks

> 状态：已批准（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）、[plan.md](./plan.md)（已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `frontend/src/api/evaluation/index.ts` | 评测历史/详情 API |
| 修改 | `frontend/src/api/model/usage.ts` | EmbeddingCacheStats / getEmbeddingCacheStats |
| 新建 | `frontend/src/views/settings/EvaluationCenterSettings.vue` | 评测中心页面 |
| 修改 | `frontend/src/views/settings/ModelUsageSettings.vue` | 缓存统计区块 |
| 修改 | `frontend/src/views/settings/Settings.vue` | 注册评测中心 section |
| 修改 | `frontend/src/config/settingsAccess.ts` | evaluation: viewer |
| 新建 | `docs/specs/dashboard/progress-2026-09-02.md` | 实现与验收记录（开发后填写） |

## 执行顺序

```
T1 → T2 → T3 → T4 → T5
```

---

## T1: 前端 API 封装

**文件：** 新建 `frontend/src/api/evaluation/index.ts`，修改 `frontend/src/api/model/usage.ts`
**依赖：** 无
**步骤：**
1. 新建评测 API：
   - `listEvaluationRuns(page, pageSize, status?)` → `GET /api/v1/evaluation/runs`；
   - `getEvaluationResult(taskId)` → `GET /api/v1/evaluation?task_id=...`；
   - 定义 `EvaluationRun / EvaluationMetric / EvaluationDetail` 类型。
2. `usage.ts` 增加 `EmbeddingCacheStats` 类型与 `getEmbeddingCacheStats()`。
3. 沿用现有 `get/put` 请求封装，返回 Promise。

**验证：**
```bash
cd frontend && npm run type-check
```

---

## T2: 评测中心组件

**文件：** 新建 `frontend/src/views/settings/EvaluationCenterSettings.vue`
**依赖：** T1
**步骤：**
1. 页面结构沿用 Settings 现有模式：`.section-header` + `.section-description`。
2. 使用 TDesign `t-table` 展示历史运行：
   - 列：运行 ID、数据集、状态、进度、创建时间；
   - 状态用 `t-tag`（success/failed/interrupted/pending/running）；
   - 使用 `t-loading` 与空状态。
3. 点击行加载详情：
   - 展示检索/生成指标表；
   - 展示 `config_hash`、数据集、样本数、模型/版本摘要；
   - 失败/中断显示 `err_msg`。
4. 分页使用 TDesign `t-pagination`。
5. 样式只使用 TDesign 与现有 CSS 变量，不新增装饰卡片/渐变/阴影。

**验证：**
```bash
cd frontend && npm run type-check
```

---

## T3: 模型用量页缓存统计

**文件：** 修改 `frontend/src/views/settings/ModelUsageSettings.vue`
**依赖：** T1
**步骤：**
1. 在汇总区上方新增“Embedding 缓存”区块，使用 `.settings-group` 风格。
2. 展示 `hits`、`misses`、`provider_calls`；请求失败或无数据时显示 `unknown`。
3. 使用 `getEmbeddingCacheStats()` 加载。

**验证：**
```bash
cd frontend && npm run type-check
```

---

## T4: Settings 导航与权限

**文件：** 修改 `frontend/src/views/settings/Settings.vue`、`frontend/src/config/settingsAccess.ts`
**依赖：** T2、T3
**步骤：**
1. `Settings.vue` 导入 `EvaluationCenterSettings`。
2. 在 navItems 增加 `{ key: 'evaluation', icon: 'chart-bar', label: '评测中心' }`，加入合适分组。
3. 增加 section 渲染分支。
4. `settingsAccess.ts` 增加 `evaluation: 'viewer'`。

**验证：**
```bash
cd frontend && npm run type-check
```

---

## T5: 本地验证与回归

**依赖：** T1-T4
**步骤：**
1. 启动前端并人工打开设置页，确认“评测中心”和“模型用量”展示正常、样式与现有设置一致。
2. 如本地有后端数据，验证列表/详情/缓存统计真实返回。
3. 全量回归：
   - `go build ./...`
   - `go test ./internal/...`
   - `cd client && go test ./...`
   - `cd cli && go test ./... && go vet ./...`
   - `cd frontend && npm run type-check`

**验证：**
```bash
cd frontend && npm run type-check
```

---

## 后续

开发完成后生成 `checklist.md`，逐项验收并填写 `docs/specs/dashboard/progress-2026-09-02.md`。
