<template>
  <div class="evaluation-center-settings">
    <div class="section-header">
      <div>
        <h2>评测中心</h2>
        <p class="section-description">配置数据集、模型和分块参数，启动评测并查看历史运行</p>
      </div>
    </div>

    <div class="evaluator-switch" role="group" aria-label="评测类型">
      <button
        type="button"
        class="evaluator-option"
        :class="{ 'evaluator-option--active': activeEvaluator === 'rag' }"
        :aria-pressed="activeEvaluator === 'rag'"
        @click="activeEvaluator = 'rag'"
      >
        <span class="evaluator-option__code">RAG</span>
        <span class="evaluator-option__body">
          <strong>问答效果评测</strong>
          <small>检索命中、重排与回答质量</small>
        </span>
        <span class="evaluator-option__metrics">检索召回 · 排序质量 · 回答质量</span>
      </button>
      <button
        type="button"
        class="evaluator-option"
        :class="{ 'evaluator-option--active': activeEvaluator === 'wiki' }"
        :aria-pressed="activeEvaluator === 'wiki'"
        @click="activeEvaluator = 'wiki'"
      >
        <span class="evaluator-option__code evaluator-option__code--wiki">WIKI</span>
        <span class="evaluator-option__body">
          <strong>知识图谱评测</strong>
          <small>实体、概念覆盖与页面连接结构</small>
        </span>
        <span class="evaluator-option__metrics">节点覆盖 · 连接准确 · 连接召回</span>
      </button>
    </div>

    <div v-show="activeEvaluator === 'rag'" class="evaluation-panel">

    <div class="evaluation-run settings-group">
      <div class="evaluation-run__head">
        <div>
          <h3>新建评测</h3>
          <p v-if="!canRun" class="evaluation-run__permission-hint">当前角色只能查看评测记录，发起评测需要管理员权限</p>
        </div>
        <span v-if="canRun && !optionsLoading" class="evaluation-run__dataset-count">
          可用数据集 {{ datasets.length }} 个
        </span>
      </div>

      <t-loading v-if="canRun" :loading="optionsLoading" size="small">
        <div v-if="optionsError" class="evaluation-run__options-error">
          <t-alert theme="warning" :message="optionsError" />
        </div>
        <div class="evaluation-run__form">
          <div class="evaluation-flow" aria-label="RAG 评测流程">
            <span>评测流程</span><strong>数据集</strong><i>→</i><strong>切分与检索</strong><i>→</i><strong>生成回答</strong><i>→</i><strong>计算指标</strong>
          </div>
          <div class="run-field">
            <label>数据集</label>
            <t-select
              v-model="form.datasetId"
              :options="datasetOptions"
              filterable
              placeholder="选择评测数据集"
            />
            <p v-if="selectedDataset" class="run-field__hint">
              {{ selectedDataset.sample_count }} 个样本 · sha256 {{ shortHash(selectedDataset.sha256) }}
            </p>
          </div>

          <div class="run-field">
            <label>Embedding 模型</label>
            <t-select
              v-model="form.embeddingModelId"
              :options="modelOptions('Embedding')"
              filterable
              placeholder="服务端默认"
            />
          </div>

          <div class="run-field">
            <label>问答模型</label>
            <t-select
              v-model="form.chatModelId"
              :options="modelOptions('KnowledgeQA')"
              filterable
              placeholder="服务端默认"
            />
          </div>

          <div class="run-field">
            <label>Rerank 模型</label>
            <t-select
              v-model="form.rerankModelId"
              :options="modelOptions('Rerank')"
              filterable
              placeholder="服务端默认"
            />
          </div>

          <div class="evaluation-run__chunking">
            <div class="chunking-label">分块参数</div>
            <div class="chunking-grid">
              <div class="run-field">
                <label>切分策略</label>
                <t-select v-model="form.chunkStrategy" :options="chunkStrategyOptions" />
              </div>
              <div class="run-field">
                <label>chunk_size（字符）</label>
                <t-input-number
                  v-model="form.chunkSize"
                  :min="0"
                  :max="4000"
                  :step="64"
                  :disabled="!usesChunkSize"
                />
              </div>
              <div class="run-field">
                <label>chunk_overlap（字符）</label>
                <t-input-number
                  v-model="form.chunkOverlap"
                  :min="0"
                  :max="500"
                  :step="20"
                  :disabled="!usesChunkSize"
                />
              </div>
            </div>
          </div>

          <div class="evaluation-run__actions">
            <t-button theme="primary" :loading="starting || polling" @click="submitRun">
              <template #icon v-if="!starting && !polling"><t-icon name="play-circle" /></template>
              {{ polling ? '评测进行中' : '开始评测' }}
            </t-button>
            <div v-if="polling && activeTask" class="evaluation-run__progress">
              <span>正在评测样本</span><strong>{{ activeTask.task.finished || 0 }}/{{ activeTask.task.total || 0 }}</strong>
              <span class="progress-track"><i :style="{ width: `${activeProgressPercent}%` }"></i></span>
            </div>
          </div>

          <div v-if="runError" class="evaluation-run__error">
            <t-alert theme="error" :message="runError" />
          </div>
        </div>
      </t-loading>
    </div>

    <div class="evaluation-history-heading">
      <h3>历史记录</h3>
      <t-button variant="text" size="small" @click="loadRuns">
        <template #icon><t-icon name="refresh" /></template>
        刷新
      </t-button>
    </div>
    <div v-if="deleteError" class="evaluation-delete-error">
      <t-alert theme="error" :message="deleteError" close @close="deleteError = ''" />
    </div>

    <t-loading :loading="loading" size="small">
      <div v-if="error" class="evaluation-branch evaluation-branch--error">
        <t-alert theme="error" :message="error">
          <template #operation>
            <t-button size="small" @click="loadRuns">重试</t-button>
          </template>
        </t-alert>
      </div>

      <div v-else-if="!loading && runs.length === 0" class="evaluation-branch evaluation-branch--empty">
        <t-empty description="暂无评测记录" />
      </div>

      <div v-else class="evaluation-table-shell data-table-shell data-table-shell--with-footer">
        <div class="evaluation-table-shell__scroll">
          <t-table
            row-key="id"
            :data="runs"
            :columns="columns"
            size="medium"
            hover
            @row-click="openDetail"
          >
            <template #id="{ row }">
              <span class="run-id" :title="row.id">{{ shortIdentifier(row.id) }}</span>
            </template>
            <template #status="{ row }">
              <t-tag :theme="statusTheme(row.status)" size="small" variant="light">
                {{ statusLabel(row.status) }}
              </t-tag>
            </template>
            <template #progress="{ row }">
              <span>{{ row.finished }}/{{ row.total }}</span>
            </template>
            <template #created_at="{ row }">
              <span>{{ formatDate(row.created_at) }}</span>
            </template>
            <template #action="{ row }">
              <t-button variant="text" size="small" @click.stop="openDetail({ row })">查看</t-button>
              <t-popconfirm
                v-if="canRun"
                :content="`确定删除评测 ${row.id} 吗？`"
                theme="danger"
                :disabled="isActiveRun(row.status)"
                @confirm="removeRun(row.id)"
              >
                <t-button
                  variant="text"
                  size="small"
                  :disabled="isActiveRun(row.status)"
                  title="删除"
                  @click.stop
                >
                  删除
                </t-button>
              </t-popconfirm>
            </template>
          </t-table>
        </div>
        <div class="evaluation-table-shell__pager">
          <t-pagination
            v-model="page"
            v-model:page-size="pageSize"
            :total="total"
            size="small"
            show-jumper
            show-page-number
            show-page-size
            :page-size-options="[10, 20, 50]"
            @change="loadRuns"
          />
        </div>
      </div>
    </t-loading>

    <div v-if="selectedRun" class="evaluation-detail">
      <div class="evaluation-detail__head">
        <h3>运行详情</h3>
        <t-button
          v-if="detailLoading"
          variant="text"
          size="small"
          disabled
        >
          <template #icon><t-icon name="loading" /></template>
          加载中
        </t-button>
      </div>

      <t-loading :loading="detailLoading" size="small">
        <div class="settings-group evaluation-detail__meta">
          <dl class="evaluation-detail-fields">
            <div class="field-row">
              <dt>运行 ID</dt>
              <dd>{{ selectedRun.id }}</dd>
            </div>
            <div class="field-row">
              <dt>数据集</dt>
              <dd>{{ selectedRun.dataset_id }}</dd>
            </div>
            <div class="field-row">
              <dt>样本数</dt>
              <dd>{{ snapshotSampleCount }}</dd>
            </div>
            <div class="field-row">
              <dt>状态</dt>
              <dd>
                <t-tag :theme="statusTheme(selectedRun.status)" size="small" variant="light">
                  {{ statusLabel(selectedRun.status) }}
                </t-tag>
              </dd>
            </div>
            <div class="field-row">
              <dt>配置指纹</dt>
              <dd class="hash-cell" :title="selectedRun.config_hash">{{ shortIdentifier(selectedRun.config_hash, 12) }}</dd>
            </div>
            <div v-if="detailErrorText" class="field-row field-row--error">
              <dt>错误信息</dt>
              <dd>{{ detailErrorText }}</dd>
            </div>
          </dl>
        </div>

        <div v-if="detailError" class="evaluation-detail-error">
          <t-alert theme="error" :message="detailError">
            <template #operation>
              <t-button size="small" @click="loadDetail(selectedRun.id)">重试</t-button>
            </template>
          </t-alert>
        </div>

        <div v-if="metric" class="evaluation-detail__metrics">
          <section class="settings-group metric-group">
            <div class="metric-group__head"><div><h4>检索表现</h4><p>衡量能否找到相关文档，并把正确结果排在前面</p></div><span>越高越好</span></div>
            <div class="rag-metric-grid">
              <article v-for="item in retrievalMetricCards" :key="item.name" class="rag-metric-card">
                <span>{{ item.label }}</span><strong>{{ formatPercent(item.value) }}</strong><small>{{ item.help }}</small>
              </article>
            </div>
          </section>

          <section class="settings-group metric-group">
            <div class="metric-group__head"><div><h4>回答质量</h4><p>比较生成回答与标准答案的内容和表达</p></div><span>越高越好</span></div>
            <div class="rag-metric-grid">
              <article v-for="item in generationMetricCards" :key="item.name" class="rag-metric-card">
                <span>{{ item.label }}</span><strong>{{ formatPercent(item.value) }}</strong><small>{{ item.help }}</small>
              </article>
            </div>
          </section>

          <section v-if="metric.cost_metrics" class="settings-group metric-group metric-group--compact">
            <h4>模型消耗</h4>
            <table class="metric-table">
              <thead>
                <tr>
                  <th>指标</th>
                  <th>值</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(value, name) in metric.cost_metrics" :key="name">
                  <td>{{ evaluationCostLabel(name) }}</td>
                  <td>{{ formatCostMetric(name, value) }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section v-if="metric.latency_metrics" class="settings-group metric-group metric-group--compact">
            <h4>运行耗时</h4>
            <table class="metric-table">
              <thead>
                <tr>
                  <th>指标</th>
                  <th>值</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(value, name) in metric.latency_metrics" :key="name">
                  <td>{{ evaluationLatencyLabel(name) }}</td>
                  <td>{{ formatLatencyMetric(name, value) }}</td>
                </tr>
              </tbody>
            </table>
          </section>
        </div>

        <div v-if="configSnapshot" class="settings-group evaluation-detail__snapshot">
          <h4>模型快照</h4>
          <table v-if="snapshotModels.length > 0" class="metric-table">
            <thead>
              <tr>
                <th>模型</th>
                <th>类型</th>
                <th>服务商</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="model in snapshotModels" :key="model.id || model.name">
                <td>{{ model.name || model.id || '—' }}</td>
                <td :title="model.type">{{ modelTypeLabel(model.type) }}</td>
                <td>{{ model.provider || '—' }}</td>
              </tr>
            </tbody>
          </table>
          <p v-else class="snapshot-empty">无模型快照</p>
          <div v-if="configSnapshot.chunking" class="field-row">
            <dt>分块参数</dt>
            <dd>{{ chunkingText(configSnapshot.chunking) }}</dd>
          </div>
          <div class="field-row">
            <dt>版本</dt>
            <dd>{{ snapshotVersion }}</dd>
          </div>
        </div>
      </t-loading>
    </div>
	</div>

	<WikiEvaluationPanel v-if="activeEvaluator === 'wiki'" :can-run="canRun" class="evaluation-panel" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { listModels, type ModelConfig } from '@/api/model'
import {
  getEvaluationResult,
  deleteEvaluationRun,
  listEvaluationDatasets,
  listEvaluationRuns,
  startEvaluation,
  type EvaluationConfigSnapshot,
  type EvaluationDetail,
  type EvaluationDatasetOption,
  type EvaluationMetric,
  type EvaluationRun,
  type StartEvaluationRequest,
} from '@/api/evaluation'
import { useAuthStore } from '@/stores/auth'
import WikiEvaluationPanel from './WikiEvaluationPanel.vue'
import {
  evaluationCostLabel,
  evaluationLatencyLabel,
  evaluationMetricPresentation,
  formatCount,
  formatUSD,
  modelTypeLabel,
  shortIdentifier,
} from './presentation'

const authStore = useAuthStore()
const activeEvaluator = ref<'rag' | 'wiki'>('rag')

const runs = ref<EvaluationRun[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const error = ref('')
const selectedRun = ref<EvaluationRun | null>(null)
const detail = ref<EvaluationDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const deleteError = ref('')
const datasets = ref<EvaluationDatasetOption[]>([])
const models = ref<ModelConfig[]>([])
const optionsLoading = ref(false)
const optionsError = ref('')
const starting = ref(false)
const polling = ref(false)
const activeTask = ref<EvaluationDetail | null>(null)
const runError = ref('')

const canRun = computed(() => authStore.hasRole('admin') || authStore.canAccessAllTenants)

interface RunForm {
  datasetId: string
  embeddingModelId: string
  chatModelId: string
  rerankModelId: string
  chunkStrategy: string
  chunkSize: number
  chunkOverlap: number
}

const form = reactive<RunForm>({
  datasetId: '',
  embeddingModelId: '',
  chatModelId: '',
  rerankModelId: '',
  chunkStrategy: 'recursive',
  chunkSize: 1024,
  chunkOverlap: 80,
})

const metric = computed<EvaluationMetric | null>(() => detail.value?.metric || selectedRun.value?.metric || null)
const configSnapshot = computed<EvaluationConfigSnapshot | null>(() => selectedRun.value?.config_snapshot || null)
const snapshotModels = computed(() => configSnapshot.value?.models || [])
const detailErrorText = computed(() => detail.value?.task?.err_msg || selectedRun.value?.err_msg || '')
const snapshotSampleCount = computed(() => {
  const count = configSnapshot.value?.dataset?.sample_count
  return count == null ? '—' : formatCount(count)
})
const snapshotVersion = computed(() => {
  const version = configSnapshot.value?.version
  if (!version?.app_version && !version?.git_commit) return '—'
  const appVersion = version.app_version || ''
  const commit = version.git_commit || ''
  return appVersion && commit ? `${appVersion} (${commit})` : appVersion || commit
})
const datasetOptions = computed(() => datasets.value.map((dataset) => ({
  label: dataset.id === 'default' ? 'default（内置 samples）' : dataset.id,
  value: dataset.id,
})))
const selectedDataset = computed(() => datasets.value.find((dataset) => dataset.id === form.datasetId))
const usesChunkSize = computed(() => form.chunkStrategy !== 'passthrough' && form.chunkStrategy !== '')
const activeProgressPercent = computed(() => {
  const finished = activeTask.value?.task.finished || 0
  const total = activeTask.value?.task.total || 0
  return total > 0 ? Math.min(100, (finished / total) * 100) : 0
})
const retrievalMetricCards = computed(() => Object.entries(metric.value?.retrieval_metrics || {}).map(([name, value]) => ({
  name, value, ...evaluationMetricPresentation(name),
})))
const generationMetricCards = computed(() => Object.entries(metric.value?.generation_metrics || {}).map(([name, value]) => ({
  name, value, ...evaluationMetricPresentation(name),
})))
const chunkStrategyOptions = [
  { label: '递归切分（recursive）', value: 'recursive' },
  { label: '自适应（auto）', value: 'auto' },
  { label: '按标题（heading）', value: 'heading' },
  { label: '启发式（heuristic）', value: 'heuristic' },
  { label: 'legacy', value: 'legacy' },
  { label: '不切分（pass-through）', value: 'passthrough' },
]

const columns = computed(() => [
  { colKey: 'id', title: '运行 ID', width: 140 },
  { colKey: 'dataset_id', title: '数据集', width: 120 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'progress', title: '进度', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 220 },
  { colKey: 'action', title: '操作', width: 130 },
])

function statusLabel(status: number): string {
  const labels: Record<number, string> = {
    0: '等待中',
    1: '运行中',
    2: '成功',
    3: '失败',
    4: '已中断',
  }
  return labels[status] || '未知'
}

function statusTheme(status: number): 'primary' | 'success' | 'warning' | 'danger' | 'default' {
  const themes: Record<number, 'primary' | 'success' | 'warning' | 'danger' | 'default'> = {
    0: 'default',
    1: 'primary',
    2: 'success',
    3: 'danger',
    4: 'warning',
  }
  return themes[status] || 'default'
}

function isActiveRun(status: number): boolean {
  return status === 0 || status === 1
}

async function removeRun(taskId: string) {
  deleteError.value = ''
  try {
    await deleteEvaluationRun(taskId)
    if (selectedRun.value?.id === taskId) {
      selectedRun.value = null
      detail.value = null
    }
    await loadRuns()
  } catch (err: any) {
    deleteError.value = err?.message || '删除评测记录失败'
  }
}

async function loadRuns() {
  loading.value = true
  error.value = ''
  try {
    const result = await listEvaluationRuns(page.value, pageSize.value)
    runs.value = result.data
    total.value = result.total
  } catch (err: any) {
    error.value = err?.message || '加载评测记录失败'
  } finally {
    loading.value = false
  }
}

async function loadDatasets() {
  try {
    datasets.value = await listEvaluationDatasets()
    if (!datasets.value.some((dataset) => dataset.id === form.datasetId)) {
      const preferred = datasets.value.find((dataset) => dataset.id === 'enterprise_rag')
        || datasets.value.find((dataset) => dataset.id === 'demo')
        || datasets.value[0]
      form.datasetId = preferred?.id || ''
    }
  } catch (err: any) {
    optionsError.value = err?.message || '加载数据集失败'
  }
}

async function loadModels() {
  try {
    models.value = await listModels()
  } catch (err: any) {
    optionsError.value = optionsError.value || err?.message || '加载模型失败'
  }
}

async function loadOptions() {
  optionsLoading.value = true
  optionsError.value = ''
  try {
    await Promise.allSettled([loadDatasets(), loadModels()])
  } finally {
    optionsLoading.value = false
  }
}

function modelOptions(type: ModelConfig['type']) {
  const matched = models.value.filter((model) => model.type === type && model.id)
  return [
    { label: '服务端默认', value: '' },
    ...matched.map((model) => ({
      label: modelDisplayName(model),
      value: model.id || '',
    })),
  ]
}

function modelDisplayName(model: ModelConfig): string {
  const name = model.display_name || model.name || model.id || ''
  const provider = model.parameters?.provider
  return provider && name ? `${name} · ${provider}` : name
}

async function submitRun() {
  if (starting.value || polling.value) return
  if (!form.datasetId) {
    runError.value = '请选择评测数据集'
    return
  }
  if (usesChunkSize.value && (!form.chunkSize || form.chunkSize <= 0)) {
    runError.value = 'chunk_size 必须大于 0'
    return
  }
  if (usesChunkSize.value && form.chunkOverlap >= form.chunkSize) {
    runError.value = 'chunk_overlap 必须小于 chunk_size'
    return
  }

  runError.value = ''
  starting.value = true
  activeTask.value = null
  const payload: StartEvaluationRequest = {
    dataset_id: form.datasetId,
  }
  if (form.embeddingModelId) payload.embedding_id = form.embeddingModelId
  if (form.chatModelId) payload.chat_id = form.chatModelId
  if (form.rerankModelId) payload.rerank_id = form.rerankModelId
  if (form.chunkStrategy === 'passthrough') {
    payload.chunking = { strategy: 'passthrough', chunk_size: 0, chunk_overlap: 0 }
  } else {
    payload.chunking = {
      strategy: form.chunkStrategy,
      chunk_size: form.chunkSize,
      chunk_overlap: form.chunkOverlap,
    }
  }

  try {
    const detail = await startEvaluation(payload)
    activeTask.value = detail
    polling.value = true
    await pollTask(detail.task.id)
    await loadRuns()
    const latest = runs.value.find((run) => run.id === detail.task.id)
    if (latest) await openDetail({ row: latest })
  } catch (err: any) {
    runError.value = err?.message || '发起评测失败'
  } finally {
    starting.value = false
    polling.value = false
  }
}

async function pollTask(taskId: string) {
  for (let attempt = 0; attempt < 900; attempt += 1) {
    await new Promise((resolve) => window.setTimeout(resolve, 2000))
    try {
      const detail = await getEvaluationResult(taskId)
      activeTask.value = detail
      if (detail.task.status >= 2) return
    } catch (err: any) {
      runError.value = err?.message || '轮询评测状态失败'
      return
    }
  }
  runError.value = '等待评测完成超时'
}

async function openDetail(context: { row: EvaluationRun }) {
  selectedRun.value = context.row
  detail.value = null
  await loadDetail(context.row.id)
}

async function loadDetail(taskId: string) {
  detailLoading.value = true
  detailError.value = ''
  try {
    detail.value = await getEvaluationResult(taskId)
  } catch (err: any) {
    detailError.value = err?.message || '加载评测详情失败'
  } finally {
    detailLoading.value = false
  }
}

function formatDate(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

function formatCostMetric(name: string, value: number | null): string {
  if (name === 'estimated_cost_usd') return formatUSD(value)
  return formatCount(value)
}

function formatLatencyMetric(name: string, value: number): string {
  if (name === 'model_calls') return `${formatCount(value)} 次`
  if (name.endsWith('_ms') || name.includes('ms_per')) {
    return value >= 1000 ? `${(value / 1000).toFixed(2)} 秒` : `${formatCount(value)} ms`
  }
  return String(value)
}

function chunkingText(chunking: NonNullable<EvaluationConfigSnapshot['chunking']>): string {
  if (chunking.strategy === 'passthrough') return '不切分（pass-through）'
  return `strategy ${chunking.strategy || 'recursive'} · chunk_size ${chunking.chunk_size ?? '-'} · chunk_overlap ${chunking.chunk_overlap ?? '-'}`
}

function shortHash(value?: string): string {
  return value ? value.slice(0, 8) : '-'
}

onMounted(async () => {
  await Promise.all([loadOptions(), loadRuns()])
})
</script>

<style scoped>
.evaluation-center-settings {
  max-width: 960px;
}

.section-header {
  margin-bottom: 20px;
}

.section-header h2 {
  margin: 0 0 6px;
  font-size: 18px;
}

.section-description {
  margin: 0;
  color: var(--td-text-color-secondary, #666);
}

.evaluator-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 24px;
}

.evaluator-option {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 4px 12px;
  align-items: center;
  min-width: 0;
  padding: 15px 16px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 10px;
  color: var(--td-text-color-primary, #333);
  background: var(--td-bg-color-container, #fff);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s, box-shadow 0.2s, background-color 0.2s;
}

.evaluator-option:hover {
  border-color: var(--td-brand-color-5, #5b8ff9);
}

.evaluator-option--active {
  border-color: var(--td-brand-color, #0052d9);
  background: var(--td-brand-color-light, #f2f3ff);
  box-shadow: 0 0 0 1px var(--td-brand-color, #0052d9);
}

.evaluator-option__code {
  grid-row: 1 / 3;
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 9px;
  color: var(--td-brand-color, #0052d9);
  background: var(--td-brand-color-light, #eef4ff);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.evaluator-option__code--wiki {
  color: var(--td-warning-color, #d99000);
  background: var(--td-warning-color-1, #fff2d5);
}

.evaluator-option__body {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.evaluator-option__body strong {
  font-size: 14px;
}

.evaluator-option__body small,
.evaluator-option__metrics {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.evaluator-option__metrics {
  grid-column: 2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.evaluation-run {
  margin-bottom: 24px;
  padding: 18px;
}

.evaluation-run__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.evaluation-run__head h3 {
  margin: 0 0 4px;
  font-size: 15px;
}

.evaluation-run__permission-hint,
.evaluation-run__dataset-count {
  margin: 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.evaluation-run__form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 16px;
}

.evaluation-flow {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  padding: 9px 11px;
  border-radius: 7px;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.evaluation-flow > span {
  margin-right: 4px;
  color: var(--td-text-color-placeholder, #999);
}

.evaluation-flow strong {
  color: var(--td-text-color-primary, #333);
  font-weight: 500;
}

.evaluation-flow i {
  color: var(--td-text-color-placeholder, #999);
  font-style: normal;
}

.run-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.run-field label {
  color: var(--td-text-color-primary, #333);
  font-size: 13px;
  font-weight: 600;
}

.run-field :deep(.t-select),
.run-field :deep(.t-input-number) {
  width: 100%;
}

.run-field__hint {
  margin: 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.evaluation-run__chunking {
  grid-column: 1 / -1;
  padding-top: 4px;
}

.chunking-label {
  margin-bottom: 10px;
  color: var(--td-text-color-primary, #333);
  font-size: 13px;
  font-weight: 600;
}

.chunking-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 16px;
}

.evaluation-run__actions,
.evaluation-run__options-error,
.evaluation-run__error {
  grid-column: 1 / -1;
}

.evaluation-run__actions {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 2px;
}

.evaluation-run__progress {
  display: grid;
  grid-template-columns: auto auto;
  align-items: center;
  gap: 3px 10px;
  min-width: 230px;
  color: var(--td-text-color-secondary, #666);
  font-size: 13px;
}

.evaluation-run__progress strong {
  justify-self: end;
  color: var(--td-text-color-primary, #333);
  font-variant-numeric: tabular-nums;
}

.progress-track {
  grid-column: 1 / -1;
  height: 5px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--td-bg-color-secondarycontainer, #e7e7e7);
}

.progress-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--td-brand-color, #0052d9);
  transition: width 0.2s ease;
}

.evaluation-run__options-error,
.evaluation-run__error {
  min-width: 0;
}

.evaluation-history-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 4px 0 12px;
}

.evaluation-history-heading h3 {
  margin: 0;
  font-size: 16px;
}

.evaluation-delete-error {
  margin-bottom: 12px;
}

.evaluation-branch {
  padding: 24px 0;
}

.evaluation-table-shell.data-table-shell--with-footer {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.evaluation-table-shell__scroll {
  overflow-x: auto;
  min-width: 0;
}

.evaluation-table-shell__pager {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  padding: 10px 14px;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
  background-color: var(--td-bg-color-container);

  :deep(.t-pagination) {
    flex-wrap: wrap;
    justify-content: flex-end;
    row-gap: 8px;
  }
}

.evaluation-table-shell {
  margin-bottom: 24px;
}

.run-id {
  display: inline-block;
  font-family: var(--td-font-family-mono, monospace);
  font-size: 12px;
  cursor: help;
}

.evaluation-detail {
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
  padding-top: 20px;
}

.evaluation-detail__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.evaluation-detail__head h3 {
  margin: 0;
  font-size: 16px;
}

.evaluation-detail__meta {
  margin-bottom: 20px;
}

.evaluation-detail__metrics h4,
.evaluation-detail__snapshot h4 {
  margin: 0 0 8px;
  font-size: 14px;
}

.evaluation-detail__metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.metric-group {
  padding: 16px;
}

.metric-group__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.metric-group__head h4 { margin-bottom: 3px; }
.metric-group__head p {
  margin: 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  line-height: 1.45;
}

.metric-group__head > span {
  flex: 0 0 auto;
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--td-success-color-1, #eaf7ee);
  color: var(--td-success-color, #2ba471);
  font-size: 11px;
}

.rag-metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px;
}

.rag-metric-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 11px;
  border-radius: 7px;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
}

.rag-metric-card > span,
.rag-metric-card small {
  color: var(--td-text-color-secondary, #666);
  font-size: 11px;
  line-height: 1.4;
}

.rag-metric-card strong {
  margin: 5px 0 3px;
  color: var(--td-brand-color, #0052d9);
  font-size: 21px;
  font-variant-numeric: tabular-nums;
}

.metric-group--compact {
  align-self: start;
}

.metric-group--compact .metric-table { margin-bottom: 0; }
.metric-group--compact .metric-table td:last-child {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.evaluation-detail-error {
  margin-bottom: 16px;
}

.snapshot-empty {
  margin: 0 0 12px;
  color: var(--td-text-color-secondary, #666);
  font-size: 13px;
}

.evaluation-detail-fields {
  display: grid;
  gap: 8px;
  margin: 0;
}

.field-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 12px;
}

.field-row dt {
  color: var(--td-text-color-secondary, #666);
}

.field-row dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.field-row--error dd {
  color: var(--td-error-color, #d54941);
}

.hash-cell {
  font-family: var(--td-font-family-mono, monospace);
  font-size: 12px;
}

.metric-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 16px;
}

.metric-table th,
.metric-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  text-align: left;
}

.metric-table th {
  color: var(--td-text-color-secondary, #666);
  font-weight: 600;
}

@media (max-width: 720px) {
  .evaluator-switch {
    grid-template-columns: 1fr;
  }

  .evaluation-run__form,
  .chunking-grid,
  .evaluation-detail__metrics,
  .rag-metric-grid {
    grid-template-columns: 1fr;
  }

  .evaluation-run__head {
    flex-direction: column;
  }

  .evaluation-run__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .evaluation-run__progress { min-width: 0; }
}
</style>
