<template>
  <div class="wiki-evaluation">
    <div class="settings-group wiki-start">
      <div class="wiki-start__head">
        <div>
          <span class="section-kicker">WIKI KNOWLEDGE GRAPH</span>
          <h3>新建 Wiki 图谱评测</h3>
          <p>从固定语料生成隔离的临时 Wiki，再从节点覆盖和页面连接两个层面衡量结果。</p>
        </div>
        <span class="scoring-boundary">评分阶段不调用生成模型</span>
      </div>
      <div class="evaluation-scope">
        <article class="scope-card scope-card--nodes">
          <span class="scope-card__number">01</span>
          <div>
            <strong>节点覆盖</strong>
            <p>实体与概念分别对齐 Gold，先名称精确匹配，再使用 Embedding 语义匹配。</p>
            <div class="scope-card__metrics"><span>实体覆盖率</span><span>概念覆盖率</span><span>总体覆盖率</span></div>
          </div>
        </article>
        <article class="scope-card scope-card--graph">
          <span class="scope-card__number">02</span>
          <div>
            <strong>连接结构</strong>
            <p>在已匹配节点之间比较 Wiki 页面有向链接，区分正确、缺失和多余连接。</p>
            <div class="scope-card__metrics"><span>Precision</span><span>Recall</span><span>F1</span></div>
          </div>
        </article>
      </div>
      <t-loading :loading="optionsLoading" size="small">
        <div class="form-heading">
          <strong>运行配置</strong>
          <span>生成使用 Chat 模型；名称语义对齐使用 Embedding 模型</span>
        </div>
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
            <p class="field-hint">仅影响未被名称精确匹配的实体和概念</p>
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
            <t-popconfirm
              :content="`确定删除 Wiki 评测 ${row.id} 吗？`"
              theme="danger"
              :disabled="row.status < 2"
              @confirm="remove(row.id)"
            >
              <t-button variant="text" size="small" :disabled="row.status < 2" @click.stop>删除</t-button>
            </t-popconfirm>
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

      <div v-if="detail.metric" class="result-overview">
        <section class="metric-section">
          <div class="metric-section__head">
            <div><span>01</span><div><h4>节点覆盖</h4><p>Gold 实体与概念在生成 Wiki 中的覆盖情况</p></div></div>
            <strong class="overall-score">总体 {{ percent(detail.metric.overall.coverage) }}</strong>
          </div>
          <div class="metric-cards metric-cards--nodes">
            <div v-for="item in nodeCards" :key="item.name" class="metric-card">
              <span>{{ item.name }}覆盖率</span><strong>{{ percent(item.metric.coverage) }}</strong>
              <small>Gold {{ item.metric.gold_total }} · 精确 {{ item.metric.exact_matched }} · 语义 {{ item.metric.semantic_matched }} · 缺失 {{ item.metric.unmatched }}</small>
            </div>
          </div>
        </section>

        <section class="metric-section graph-section">
          <div class="metric-section__head">
            <div><span>02</span><div><h4>连接结构</h4><p>已匹配节点构成的有向链接图</p></div></div>
            <strong class="overall-score">F1 {{ detail.metric.graph.scorable ? percent(detail.metric.graph.f1) : '不可评分' }}</strong>
          </div>
          <div class="graph-metrics">
            <div><span>Precision</span><strong>{{ percent(detail.metric.graph.precision) }}</strong></div>
            <div><span>Recall</span><strong>{{ percent(detail.metric.graph.recall) }}</strong></div>
            <div><span>正确连接</span><strong>{{ detail.metric.graph.correct }}</strong></div>
            <div><span>缺失连接</span><strong>{{ detail.metric.graph.missing }}</strong></div>
            <div><span>多余连接</span><strong>{{ detail.metric.graph.extra }}</strong></div>
          </div>
          <p v-if="detail.metric.graph.note" class="graph-note">{{ detail.metric.graph.note }}</p>
        </section>

        <section v-if="detail.metric.generation_cost || detail.metric.scoring_cost" class="metric-section cost-section">
          <div class="cost-section__head"><h4>运行开销</h4><p>生成与评分分开统计</p></div>
          <div class="cost-cards">
            <div v-if="detail.metric.generation_cost" class="cost-card">
              <span>Wiki 生成 · Chat</span><strong>{{ detail.metric.generation_cost.model_calls }} 次调用</strong>
              <small>{{ detail.metric.generation_cost.total_tokens }} tokens · {{ formatCost(detail.metric.generation_cost.estimated_cost_usd) }}</small>
            </div>
            <div v-if="detail.metric.scoring_cost" class="cost-card">
              <span>语义评分 · Embedding</span><strong>{{ detail.metric.scoring_cost.model_calls }} 次调用</strong>
              <small>{{ detail.metric.scoring_cost.total_tokens }} tokens · {{ formatCost(detail.metric.scoring_cost.estimated_cost_usd) }}</small>
            </div>
          </div>
        </section>
      </div>

      <div v-if="wikiResult" class="settings-group result-tables">
        <div class="result-detail-head">
          <div><h4>评测明细</h4><p>从聚合分数下钻到具体节点或连接</p></div>
          <t-radio-group v-model="detailKind" variant="default-filled" size="small">
            <t-radio-button value="nodes">节点 {{ wikiResult.node_matches.length }}</t-radio-button>
            <t-radio-button value="edges">连接</t-radio-button>
          </t-radio-group>
        </div>
        <div v-if="detailKind === 'nodes'" class="scroll-table">
          <table>
            <thead><tr><th>类型</th><th>Gold</th><th>方式</th><th>Wiki 页面</th><th>分数/原因</th></tr></thead>
            <tbody><tr v-for="node in wikiResult.node_matches" :key="node.gold_node_id">
              <td>{{ node.gold_type }}</td><td>{{ node.gold_name }}</td><td>{{ node.method }}</td>
              <td>{{ node.page_title || node.page_slug || '-' }}</td><td>{{ node.score == null ? (node.reason || '-') : decimal(node.score) }}</td>
            </tr></tbody>
          </table>
        </div>
        <div v-else>
          <div class="edge-filter">
            <t-radio-group v-model="edgeKind" variant="default-filled" size="small">
              <t-radio-button value="correct">正确 {{ wikiResult.correct_edges.length }}</t-radio-button>
              <t-radio-button value="missing">缺失 {{ wikiResult.missing_edges.length }}</t-radio-button>
              <t-radio-button value="extra">多余 {{ wikiResult.extra_edges.length }}</t-radio-button>
              <t-radio-button value="unscored">未评分 {{ wikiResult.unscored_edges.length }}</t-radio-button>
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
const detailKind = ref<'nodes' | 'edges'>('nodes')
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
] : [])
const wikiResult = computed(() => detail.value?.result ?? null)
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
  optionsError.value = ''
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
  listError.value = ''
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
  listError.value = ''
  try { detail.value = await getWikiEvaluation(context.row.id) }
  catch (error: any) { listError.value = error?.message || '加载详情失败' }
}
async function remove(id: string) {
  listError.value = ''
  try { await deleteWikiEvaluation(id); if (detail.value?.run.id === id) detail.value = null; await loadRuns() }
  catch (error: any) { listError.value = error?.message || '删除 Wiki 评测失败' }
}
async function download(format: 'json' | 'markdown') {
  if (!detail.value) return
  listError.value = ''
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
.wiki-start {
  padding: 20px;
  margin-bottom: 26px;
}

.wiki-start__head,
.history-head,
.detail-head,
.result-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.wiki-start__head h3,
.history-head h3,
.detail-head h3 {
  margin: 3px 0 5px;
}

.section-kicker {
  color: var(--td-brand-color, #0052d9);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.wiki-start__head p,
.field-hint,
.form-heading span,
.cost-section__head p,
.result-detail-head p {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  margin: 0;
}

.scoring-boundary {
  flex-shrink: 0;
  padding: 5px 9px;
  border-radius: 999px;
  color: var(--td-success-color, #2ba471);
  background: var(--td-success-color-1, #eaf7ee);
  font-size: 11px;
  font-weight: 600;
}

.evaluation-scope {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 18px 0 20px;
}

.scope-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  padding: 15px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 9px;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
}

.scope-card--nodes {
  border-top: 3px solid var(--td-brand-color, #0052d9);
}

.scope-card--graph {
  border-top: 3px solid var(--td-warning-color, #d99000);
}

.scope-card__number {
  color: var(--td-text-color-placeholder, #999);
  font-size: 11px;
  font-weight: 700;
}

.scope-card strong {
  display: block;
  margin-bottom: 5px;
  font-size: 14px;
}

.scope-card p {
  min-height: 34px;
  margin: 0 0 10px;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  line-height: 1.45;
}

.scope-card__metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.scope-card__metrics span {
  padding: 3px 7px;
  border-radius: 4px;
  background: var(--td-bg-color-container, #fff);
  color: var(--td-text-color-secondary, #666);
  font-size: 11px;
}

.form-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-top: 17px;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
}

.form-heading strong {
  font-size: 13px;
}

.wiki-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-top: 13px;
}

.run-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.run-field label {
  font-size: 13px;
  font-weight: 600;
}

.run-field :deep(.t-select),
.run-field :deep(.t-input-number) {
  width: 100%;
}

.wiki-actions {
  grid-column: 1 / -1;
  display: flex;
  gap: 14px;
  align-items: center;
}

.history-head,
.detail-head {
  align-items: center;
  margin: 12px 0;
}

.detail-head > div {
  display: flex;
  gap: 8px;
}

.table-shell {
  overflow-x: auto;
  margin-bottom: 24px;
}

.table-shell :deep(.t-pagination) {
  margin-top: 12px;
  justify-content: flex-end;
}

.mono {
  font-family: var(--td-font-family-mono, monospace);
  font-size: 12px;
}

.wiki-detail {
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
  padding-top: 16px;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding: 16px;
}

.meta-grid > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.error-row {
  color: var(--td-error-color, #d54941);
  grid-column: 1 / -1;
}

.result-overview {
  display: grid;
  gap: 14px;
  margin: 16px 0 20px;
}

.metric-section {
  padding: 17px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 9px;
  background: var(--td-bg-color-container, #fff);
}

.metric-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
}

.metric-section__head > div {
  display: flex;
  align-items: center;
  gap: 10px;
}

.metric-section__head > div > span {
  display: grid;
  place-items: center;
  width: 27px;
  height: 27px;
  border-radius: 7px;
  color: var(--td-brand-color, #0052d9);
  background: var(--td-brand-color-light, #eef4ff);
  font-size: 10px;
  font-weight: 700;
}

.metric-section__head h4,
.cost-section__head h4,
.result-detail-head h4 {
  margin: 0 0 2px;
  font-size: 14px;
}

.metric-section__head p {
  margin: 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.overall-score {
  color: var(--td-brand-color, #0052d9);
  font-size: 18px;
}

.metric-cards,
.graph-metrics,
.cost-cards {
  display: grid;
  gap: 10px;
}

.metric-cards--nodes,
.cost-cards {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.metric-card,
.cost-card {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 13px 14px;
  border-radius: 7px;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
}

.metric-card > span,
.cost-card > span,
.graph-metrics span {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.metric-card strong {
  font-size: 23px;
}

.metric-card small,
.cost-card small {
  color: var(--td-text-color-secondary, #666);
}

.graph-section {
  border-left: 3px solid var(--td-warning-color, #d99000);
}

.graph-metrics {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.graph-metrics > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 11px;
  border-radius: 7px;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
}

.graph-metrics strong {
  font-size: 18px;
}

.graph-note {
  margin: 10px 0 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.cost-section {
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
}

.cost-section__head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
}

.cost-card {
  background: var(--td-bg-color-container, #fff);
}

.result-tables {
  padding: 16px;
}

.result-detail-head {
  align-items: center;
  margin-bottom: 14px;
}

.scroll-table {
  max-height: 420px;
  overflow: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

th,
td {
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 8px;
  text-align: left;
}

.edge-filter {
  margin-bottom: 10px;
}

.edge-table {
  max-height: 300px;
}

.empty-cell {
  color: var(--td-text-color-placeholder, #999);
  text-align: center;
}

@media (max-width: 720px) {
  .wiki-start__head,
  .form-heading,
  .result-detail-head {
    flex-direction: column;
  }

  .evaluation-scope,
  .wiki-form,
  .meta-grid,
  .metric-cards--nodes,
  .cost-cards {
    grid-template-columns: 1fr;
  }

  .graph-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .scoring-boundary {
    align-self: flex-start;
  }
}
</style>
