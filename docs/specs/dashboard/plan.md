# 评测与成本可视化页面（课题三 WP7）Plan

> 状态：已批准（2026-09-02）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

WP7 全部在前端完成，后端只复用已有 API：

```text
Settings.vue
  ├─ 评测中心（EvaluationCenterSettings.vue）
  │    └─ listEvaluationRuns / getEvaluationResult
  └─ 模型用量（ModelUsageSettings.vue，增强）
       ├─ getModelCallSummary / listModelCalls
       └─ getEmbeddingCacheStats
```

新增前端 API 封装，不新增后端路由、迁移或服务。

## 核心数据结构（前端 TS）

### api/evaluation/index.ts

```ts
export interface EvaluationRun {
  id: string
  dataset_id: string
  status: number
  err_msg?: string
  total: number
  finished: number
  metric?: EvaluationMetric
  config_hash: string
  config_snapshot?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface EvaluationMetric {
  retrieval_metrics: Record<string, number>
  generation_metrics: Record<string, number>
}

export interface EvaluationDetail {
  task: {
    id: string
    dataset_id: string
    status: number
    err_msg?: string
    total?: number
    finished?: number
  }
  params: Record<string, unknown>
  metric?: EvaluationMetric
}
```

函数：

```ts
listEvaluationRuns(page, pageSize, status?) => { data: EvaluationRun[]; total: number }
getEvaluationResult(taskId) => EvaluationDetail
```

### api/model/usage.ts（扩展）

```ts
export interface EmbeddingCacheStats {
  hits: number
  misses: number
  provider_calls: number
}

getEmbeddingCacheStats() => EmbeddingCacheStats
```

## 模块设计

### EvaluationCenterSettings.vue

**文件：** `frontend/src/views/settings/EvaluationCenterSettings.vue`

- 页面标题：评测中心；
- UI 严格沿用 Settings 页现有模式：`.section-header` + `.section-description`、TDesign 组件、`--td-*` CSS 变量；
- 加载 `listEvaluationRuns({ page: 1, page_size: 20 })`；
- 表格使用 TDesign `t-table`，列：运行 ID、数据集、状态、进度、创建时间；
- 状态列使用 `t-tag`：success / failed / interrupted 等不同主题，与现有列表风格一致；
- 点击行打开详情区域：调用 `getEvaluationResult(id)` 或直接使用列表内 metric；
  - 检索指标表 + 生成指标表；
  - `config_hash`、数据集、模型/版本摘要（从 `config_snapshot` 解析）；
  - 失败/中断显示 `err_msg`；
- 分页使用 TDesign `t-pagination`，与 TenantMembers 等现有列表一致。

### ModelUsageSettings.vue（增强）

- 保留现有汇总 + 明细；
- 顶部新增“Embedding 缓存”区块（复用 `.settings-group`，不做独立装饰卡片）：
  - `hits`、`misses`、`provider_calls`；
  - 请求失败或未开启时显示 `unknown`；
- 数据来源 `getEmbeddingCacheStats()`。

### UI 一致性约束

- 不新增自定义圆角卡片、渐变、阴影或装饰背景；
- 颜色只使用 TDesign 组件主题与现有 CSS 变量；
- 表格、分页、加载、空状态全部复用现有组件；
- 页面宽度、间距与当前 Settings 内容区一致；
- 不改变其他页面行为与布局。

### Settings.vue 集成

```text
navItems:
  { key: 'evaluation', icon: 'chart-bar', label: '评测中心' }

navGroups:
  workspace 或 models_runtime 下加入 evaluation

render:
  <div v-if="currentSection === 'evaluation'" class="section">
    <EvaluationCenterSettings />
  </div>
```

### settingsAccess.ts

```ts
evaluation: 'viewer'
```

### 无后端改动

本工作包不改 `internal/**`、迁移与 API。

## 模块交互

```text
评测中心页面
  onMounted → listEvaluationRuns(page=1,size=20)
  click row → getEvaluationResult(taskId) → 详情区渲染

模型用量页面
  onMounted → getModelCallSummary() + listModelCalls()
  onMounted → getEmbeddingCacheStats()
```

## 文件组织

```text
frontend/src/api/evaluation/index.ts
frontend/src/api/model/usage.ts          — 增加 EmbeddingCacheStats / getEmbeddingCacheStats
frontend/src/views/settings/EvaluationCenterSettings.vue
frontend/src/views/settings/ModelUsageSettings.vue
frontend/src/views/settings/Settings.vue
frontend/src/config/settingsAccess.ts
docs/specs/dashboard/...
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 页面位置 | 设置页 section | 最小改动，复用现有 Settings 导航与权限机制 |
| 详情获取 | 点击行再调 `getEvaluationResult` | 列表接口已含聚合指标，详情补全 params/版本 |
| 状态样式 | 文本标签 + 简单颜色 | 不加图表库 |
| UI 一致性 | 复用 TDesign + Settings 现有 CSS | 用户要求与现有 UI 保持一致，不引入新设计语言 |
| 缓存统计 | 请求失败显示 unknown | 默认关闭/后端无缓存时页面不报错 |
| i18n | 中文写死 | 快速交付，后补 |
| 后端 | 不改 | 避免回归，API 已够用 |

## Spec 覆盖自检

| Spec 需求 | Plan 归属 |
|-----------|-----------|
| F1 评测中心入口 | Settings.vue + EvaluationCenterSettings |
| F2 评测详情 | click row + getEvaluationResult |
| F3 缓存统计 | ModelUsageSettings 缓存卡片 |
| F4 API 封装 | evaluation/index.ts + usage.ts |
| F5 权限 | settingsAccess.ts |
| F6 交互约束 | 表格/卡片、无图表、unknown |
| N1-N5 | 不改后端、分页、按需请求、type-check、只读 |
