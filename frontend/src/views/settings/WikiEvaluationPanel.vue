<template>
  <div class="wiki-evaluation">
    <div class="settings-group wiki-start">
      <div class="wiki-start__head">
        <div>
          <h3>新建 Wiki 评测</h3>
          <p>用生成模型从 EnterpriseRAG 语料构建临时 Wiki；评分只使用确定性匹配和 Embedding。</p>
        </div>
      </div>
      <t-loading :loading="optionsLoading" size="small">
        <div class="wiki-form">
          <div class="run-field">
            <label>数据集</label>
            <t-select v-model="form.datasetId" :options="datasetOptions" />
            <p v-if="selectedDataset" class="field-hint">
              {{ selectedDataset.document_count }} 篇文档 · Gold {{ selectedDataset.gold.node_count }} 个节点 / {{ selectedDataset.gold.edge_count }} 条边
            </p>
          </div>
          <div class="run-field">
            <label>Wiki 生成模型</label>
            <t-select v-model="form.chatModelId" :options="modelOptions('KnowledgeQA')" filterable />
          </div>
          <div class="run-field">
            <label>语义匹配 Embedding</label>
            <t-select v-model="form.embeddingModelId" :options="modelOptions('Embedding')" filterable />
          </div>
          <div class="run-field">
            <label>语义阈值</label>
            <t-input-number v-model="form.threshold" :min="0" :max="1" :step="0.01" :decimal-places="2" />
          </div>
          <div class="wiki-actions">
            <t-button theme="primary" :disabled="!canStart" :loading="starting || polling" @click="start">
              {{ polling ? 'Wiki 评测进行中' : '开始 Wiki 评测' }}
            </t-button>
            <span v-if="active" class="field-hint">{{ stageLabel(active.run.stage) }} · {{ progressText(active.run) }}</span>
          </div>
        </div>
      </t-loading>
      <t-alert v-if="formError || optionsError" theme="error" :message="formError || optionsError" />
    </div>

    <div class="history-head">
      <h3>Wiki 评测历史</h3>
      <t-button size="small" variant="text" @click="loadRuns">刷新</t-button>
    </div>
    <t-loading :loading="loading" size="small">
      <t-empty v-if="!loading && runs.length === 0" description="暂无 Wiki 评测记录" />
      <div v-else class="table-shell">
        <t-table row-key="id" :data="runs" :columns="columns" hover @row-click="openDetail">
          <template #id="{ row }"><span class="mono">{{ row.id }}</span></template>
          <template #status="{ row }"><t-tag :theme="statusTheme(row.status)" variant="light">{{ statusLabel(row.status) }}</t-tag></template>
          <template #stage="{ row }">{{ stageLabel(row.stage) }}</template>
          <template #created_at="{ row }">{{ formatDate(row.created_at) }}</template>
          <template #action="{ row }">
            <t-button variant="text" size="small" :disabled="row.status < 2" @click.stop="remove(row.id)">删除</t-button>
          </template>
        </t-table>
        <t-pagination v-model="page" v-model:page-size="pageSize" :total="total" @change="loadRuns" />
      </div>
    </t-loading>
    <t-alert v-if="listError" theme="error" :message="listError" />

    <div v-if="detail" class="wiki-detail">
      <div class="detail-head">
        <h3>Wiki 评测详情</h3>
        <div>
          <t-button size="small" variant="outline" @click="download('json')">下载 JSON</t-button>
          <t-button size="small" variant="outline" @click="download('markdown')">下载 Markdown</t-button>
        </div>
      </div>
      <div class="settings-group meta-grid">
        <div><strong>运行 ID</strong><span class="mono">{{ detail.run.id }}</span></div>
        <div><strong>状态</strong><span>{{ statusLabel(detail.run.status) }}</span></div>
        <div><strong>阶段</strong><span>{{ stageLabel(detail.run.stage) }}</span></div>
        <div><strong>阈值</strong><span>{{ detail.params?.semantic_threshold?.toFixed(2) || '-' }}</span></div>
        <div v-if="detail.run.failure_stage"><strong>失败阶段</strong><span>{{ stageLabel(detail.run.failure_stage) }}</span></div>
        <div v-if="detail.run.err_msg" class="error-row"><strong>错误</strong><span>{{ detail.run.err_msg }}</span></div>
      </div>

      <div v-if="detail.metric" class="metric-cards">
        <div v-for="item in nodeCards" :key="item.name" class="metric-card">
          <span>{{ item.name }}覆盖率</span><strong>{{ percent(item.metric.coverage) }}</strong>
          <small>精确 {{ item.metric.exact_matched }} · 语义 {{ item.metric.semantic_matched }} · 缺失 {{ item.metric.unmatched }}</small>
        </div>
        <div class="metric-card">
          <span>有向图 F1</span><strong>{{ detail.metric.graph.scorable ? percent(detail.metric.graph.f1) : '不可评分' }}</strong>
          <small>P {{ decimal(detail.metric.graph.precision) }} · R {{ decimal(detail.metric.graph.recall) }} · 正确 {{ detail.metric.graph.correct }}</small>
        </div>
        <div v-if="detail.metric.generation_cost" class="metric-card">
          <span>Wiki 生成开销</span><strong>{{ detail.metric.generation_cost.model_calls }} 次</strong>
          <small>{{ detail.metric.generation_cost.total_tokens }} tokens · {{ formatCost(detail.metric.generation_cost.estimated_cost_usd) }}</small>
        </div>
        <div v-if="detail.metric.scoring_cost" class="metric-card">
          <span>Embedding 评分开销</span><strong>{{ detail.metric.scoring_cost.model_calls }} 次</strong>
          <small>{{ detail.metric.scoring_cost.total_tokens }} tokens · {{ formatCost(detail.metric.scoring_cost.estimated_cost_usd) }}</small>
        </div>
      </div>

      <div v-if="detail.result" class="settings-group result-tables">
        <h4>节点明细（{{ detail.result.node_matches.length }}）</h4>
        <div class="scroll-table">
          <table>
            <thead><tr><th>类型</th><th>Gold</th><th>方式</th><th>Wiki 页面</th><th>分数/原因</th></tr></thead>
            <tbody><tr v-for="node in detail.result.node_matches" :key="node.gold_node_id">
              <td>{{ node.gold_type }}</td><td>{{ node.gold_name }}</td><td>{{ node.method }}</td>
              <td>{{ node.page_title || node.page_slug || '-' }}</td><td>{{ node.score == null ? (node.reason || '-') : decimal(node.score) }}</td>
            </tr></tbody>
          </table>
        </div>
        <h4>边明细</h4>
        <div class="edge-filter">
          <t-radio-group v-model="edgeKind" variant="default-filled" size="small">
            <t-radio-button value="correct">正确 {{ detail.result.correct_edges.length }}</t-radio-button>
            <t-radio-button value="missing">缺失 {{ detail.result.missing_edges.length }}</t-radio-button>
            <t-radio-button value="extra">多余 {{ detail.result.extra_edges.length }}</t-radio-button>
            <t-radio-button value="unscored">未评分 {{ detail.result.unscored_edges.length }}</t-radio-button>
          </t-radio-group>
        </div>
        <div class="scroll-table edge-table">
          <table>
            <thead><tr><th>源节点</th><th>目标节点</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-for="edge in selectedEdges" :key="`${edge.source}\u0000${edge.target}\u0000${edge.reason || ''}`">
                <td>{{ edge.source }}</td><td>{{ edge.target }}</td><td>{{ edge.reason || '-' }}</td>
              </tr>
              <tr v-if="selectedEdges.length === 0"><td colspan="3" class="empty-cell">当前分类没有边</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { listModels, type ModelConfig } from '@/api/model'
import {
  deleteWikiEvaluation,
  downloadWikiEvaluationReport,
  getWikiEvaluation,
  listWikiEvaluationDatasets,
  listWikiEvaluationRuns,
  startWikiEvaluation,
  type WikiEvaluationDatasetOption,
  type WikiEvaluationDetail,
  type WikiEdgeRef,
  type WikiEvaluationRun,
} from '@/api/evaluation'

const props = defineProps<{ canRun: boolean }>()
const datasets = ref<WikiEvaluationDatasetOption[]>([])
const models = ref<ModelConfig[]>([])
const runs = ref<WikiEvaluationRun[]>([])
const detail = ref<WikiEvaluationDetail | null>(null)
const active = ref<WikiEvaluationDetail | null>(null)
const optionsLoading = ref(false)
const loading = ref(false)
const starting = ref(false)
const polling = ref(false)
const optionsError = ref('')
const formError = ref('')
const listError = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const edgeKind = ref<'correct' | 'missing' | 'extra' | 'unscored'>('correct')
const form = reactive({ datasetId: '', chatModelId: '', embeddingModelId: '', threshold: 0.8 })
let disposed = false
let pollGeneration = 0

const selectedDataset = computed(() => datasets.value.find((item) => item.id === form.datasetId))
const canStart = computed(() => props.canRun && Boolean(form.datasetId && form.chatModelId && form.embeddingModelId))
const datasetOptions = computed(() => datasets.value.map((item) => ({ label: item.id, value: item.id })))
const columns = [
  { colKey: 'id', title: '运行 ID', width: 250 }, { colKey: 'dataset_id', title: '数据集', width: 140 },
  { colKey: 'status', title: '状态', width: 100 }, { colKey: 'stage', title: '阶段', width: 150 },
  { colKey: 'created_at', title: '创建时间', width: 180 }, { colKey: 'action', title: '操作', width: 80 },
]
const nodeCards = computed(() => detail.value?.metric ? [
  { name: '实体', metric: detail.value.metric.entity },
  { name: '概念', metric: detail.value.metric.concept },
  { name: '总体', metric: detail.value.metric.overall },
] : [])
const selectedEdges = computed<WikiEdgeRef[]>(() => {
  if (!detail.value?.result) return []
  const edges = {
    correct: detail.value.result.correct_edges,
    missing: detail.value.result.missing_edges,
    extra: detail.value.result.extra_edges,
    unscored: detail.value.result.unscored_edges,
  }
  return edges[edgeKind.value]
})

function modelOptions(type: ModelConfig['type']) {
  return models.value.filter((model) => model.type === type && model.id).map((model) => ({
    label: model.display_name || model.name || model.id || '', value: model.id || '',
  }))
}
function statusLabel(status: number) { return ['等待中', '运行中', '成功', '失败', '已中断'][status] || '未知' }
function statusTheme(status: number): 'default' | 'primary' | 'success' | 'danger' | 'warning' {
  return (['default', 'primary', 'success', 'danger', 'warning'][status] || 'default') as any
}
function stageLabel(stage?: string) {
  const labels: Record<string, string> = {
    validating: '校验', creating_kb: '创建临时库', importing_documents: '导入文档', generating_wiki: '生成 Wiki',
    scoring_nodes: '节点评分', scoring_graph: '图评分', saving_report: '保存报告', cleaning_up: '清理资源', completed: '完成',
  }
  return stage ? labels[stage] || stage : '-'
}
function progressText(run: WikiEvaluationRun) {
  const progress = run.stage_progress
  return progress ? `${progress.current || 0}/${progress.total || 0} ${progress.message || ''}` : '-'
}
function decimal(value: number) { return value.toFixed(4) }
function percent(value: number) { return `${(value * 100).toFixed(2)}%` }
function formatCost(value: number | null) { return value == null ? '成本未配置' : `$${value.toFixed(6)}` }
function formatDate(value: string) { return value ? new Date(value).toLocaleString() : '-' }

async function loadOptions() {
  optionsLoading.value = true
  try {
    const [datasetList, modelList] = await Promise.all([listWikiEvaluationDatasets(), listModels()])
    datasets.value = datasetList
    models.value = modelList
    form.datasetId = datasetList[0]?.id || ''
    form.chatModelId = modelOptions('KnowledgeQA')[0]?.value || ''
    form.embeddingModelId = modelOptions('Embedding')[0]?.value || ''
  } catch (error: any) { optionsError.value = error?.message || '加载 Wiki 评测选项失败' }
  finally { optionsLoading.value = false }
}
async function loadRuns() {
  loading.value = true
  try { const result = await listWikiEvaluationRuns(page.value, pageSize.value); runs.value = result.data; total.value = result.total }
  catch (error: any) { listError.value = error?.message || '加载 Wiki 评测历史失败' }
  finally { loading.value = false }
}
async function start() {
  if (!props.canRun || starting.value || polling.value) return
  if (!form.datasetId || !form.chatModelId || !form.embeddingModelId) { formError.value = '请选择数据集、生成模型和 Embedding 模型'; return }
  starting.value = true; formError.value = ''
  try {
    active.value = await startWikiEvaluation({ dataset_id: form.datasetId, chat_id: form.chatModelId, embedding_id: form.embeddingModelId, semantic_threshold: form.threshold })
    await pollRun(active.value.run.id)
  } catch (error: any) { formError.value = error?.message || '启动 Wiki 评测失败' }
  finally { starting.value = false }
}

async function pollRun(runId: string) {
  const generation = ++pollGeneration
  polling.value = true
  try {
    while (!disposed && generation === pollGeneration) {
      await new Promise((resolve) => window.setTimeout(resolve, 2000))
      if (disposed || generation !== pollGeneration) return
      active.value = await getWikiEvaluation(runId)
      if (active.value.run.status >= 2) {
        detail.value = active.value
        await loadRuns()
        return
      }
    }
  } catch (error: any) {
    if (!disposed && generation === pollGeneration) {
      formError.value = error?.message || '轮询 Wiki 评测状态失败'
    }
  } finally {
    if (generation === pollGeneration) polling.value = false
  }
}
async function openDetail(context: { row: WikiEvaluationRun }) {
  try { detail.value = await getWikiEvaluation(context.row.id) }
  catch (error: any) { listError.value = error?.message || '加载详情失败' }
}
async function remove(id: string) {
  try { await deleteWikiEvaluation(id); if (detail.value?.run.id === id) detail.value = null; await loadRuns() }
  catch (error: any) { listError.value = error?.message || '删除 Wiki 评测失败' }
}
async function download(format: 'json' | 'markdown') {
  if (!detail.value) return
  try {
    const blob = await downloadWikiEvaluationReport(detail.value.run.id, format)
    const url = URL.createObjectURL(blob); const link = document.createElement('a')
    link.href = url; link.download = `wiki-evaluation-${detail.value.run.id}.${format === 'markdown' ? 'md' : 'json'}`; link.click(); URL.revokeObjectURL(url)
  } catch (error: any) { listError.value = error?.message || '下载报告失败' }
}

onMounted(async () => {
  await Promise.all([loadOptions(), loadRuns()])
  if (disposed) return
  const running = runs.value.find((run) => run.status === 0 || run.status === 1)
  if (!running) return
  try {
    active.value = await getWikiEvaluation(running.id)
    if (active.value.run.status < 2) void pollRun(running.id)
  } catch (error: any) {
    formError.value = error?.message || '恢复 Wiki 评测状态失败'
  }
})

onUnmounted(() => {
  disposed = true
  pollGeneration += 1
  polling.value = false
})
</script>

<style scoped>
.wiki-start { padding: 18px; margin-bottom: 24px; }
.wiki-start__head h3, .history-head h3, .detail-head h3 { margin: 0 0 5px; }
.wiki-start__head p, .field-hint { color: var(--td-text-color-secondary); font-size: 12px; margin: 0; }
.wiki-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; margin-top: 16px; }
.run-field { display: flex; flex-direction: column; gap: 6px; }
.run-field label { font-size: 13px; font-weight: 600; }
.wiki-actions { grid-column: 1/-1; display: flex; gap: 14px; align-items: center; }
.history-head, .detail-head { display: flex; align-items: center; justify-content: space-between; margin: 12px 0; }
.detail-head > div { display: flex; gap: 8px; }
.table-shell { overflow-x: auto; margin-bottom: 24px; }
.table-shell :deep(.t-pagination) { margin-top: 12px; justify-content: flex-end; }
.mono { font-family: var(--td-font-family-mono, monospace); font-size: 12px; }
.wiki-detail { border-top: 1px solid var(--td-component-stroke); padding-top: 16px; }
.meta-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 16px; }
.meta-grid > div { display: flex; flex-direction: column; gap: 4px; }
.error-row { color: var(--td-error-color); grid-column: 1/-1; }
.metric-cards { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin: 16px 0; }
.metric-card { border: 1px solid var(--td-component-stroke); border-radius: 6px; padding: 14px; display: flex; flex-direction: column; gap: 6px; }
.metric-card strong { font-size: 22px; }.metric-card small { color: var(--td-text-color-secondary); }
.result-tables { padding: 16px; }.scroll-table { max-height: 420px; overflow: auto; }
table { width: 100%; border-collapse: collapse; font-size: 13px; } th, td { border-bottom: 1px solid var(--td-component-stroke); padding: 8px; text-align: left; }
.edge-filter { margin-bottom: 10px; }.edge-table { max-height: 300px; }.empty-cell { color: var(--td-text-color-placeholder); text-align: center; }
@media (max-width: 720px) { .wiki-form, .meta-grid, .metric-cards { grid-template-columns: 1fr; } }
</style>
