<template>
  <div class="evaluation-center-settings">
    <div class="section-header">
      <div>
        <h2>评测中心</h2>
        <p class="section-description">查看历史评测运行、指标与配置哈希</p>
      </div>
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
              <span class="run-id">{{ row.id }}</span>
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
              <dt>config_hash</dt>
              <dd class="hash-cell">{{ selectedRun.config_hash || 'unknown' }}</dd>
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

        <div v-if="metric" class="settings-group evaluation-detail__metrics">
          <h4>检索指标</h4>
          <table class="metric-table">
            <thead>
              <tr>
                <th>指标</th>
                <th>值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(value, name) in metric.retrieval_metrics" :key="name">
                <td>{{ name }}</td>
                <td>{{ formatMetric(value) }}</td>
              </tr>
            </tbody>
          </table>

          <h4>生成指标</h4>
          <table class="metric-table">
            <thead>
              <tr>
                <th>指标</th>
                <th>值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(value, name) in metric.generation_metrics" :key="name">
                <td>{{ name }}</td>
                <td>{{ formatMetric(value) }}</td>
              </tr>
            </tbody>
          </table>
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
                <td>{{ model.name || model.id || 'unknown' }}</td>
                <td>{{ model.type || 'unknown' }}</td>
                <td>{{ model.provider || 'unknown' }}</td>
              </tr>
            </tbody>
          </table>
          <p v-else class="snapshot-empty">无模型快照</p>
          <div class="field-row">
            <dt>版本</dt>
            <dd>{{ snapshotVersion }}</dd>
          </div>
        </div>
      </t-loading>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  getEvaluationResult,
  listEvaluationRuns,
  type EvaluationConfigSnapshot,
  type EvaluationDetail,
  type EvaluationMetric,
  type EvaluationRun,
} from '@/api/evaluation'

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

const metric = computed<EvaluationMetric | null>(() => detail.value?.metric || selectedRun.value?.metric || null)
const configSnapshot = computed<EvaluationConfigSnapshot | null>(() => selectedRun.value?.config_snapshot || null)
const snapshotModels = computed(() => configSnapshot.value?.models || [])
const detailErrorText = computed(() => detail.value?.task?.err_msg || selectedRun.value?.err_msg || '')
const snapshotSampleCount = computed(() => {
  const count = configSnapshot.value?.dataset?.sample_count
  return count == null ? 'unknown' : String(count)
})
const snapshotVersion = computed(() => {
  const version = configSnapshot.value?.version
  if (!version?.app_version && !version?.git_commit) return 'unknown'
  const appVersion = version.app_version || ''
  const commit = version.git_commit || ''
  return appVersion && commit ? `${appVersion} (${commit})` : appVersion || commit
})

const columns = computed(() => [
  { colKey: 'id', title: '运行 ID', width: 260 },
  { colKey: 'dataset_id', title: '数据集', width: 120 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'progress', title: '进度', width: 100 },
  { colKey: 'created_at', title: '创建时间', width: 220 },
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

function formatMetric(value: number): string {
  return value == null ? 'unknown' : value.toFixed(4)
}

onMounted(loadRuns)
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
  font-family: var(--td-font-family-mono, monospace);
  font-size: 12px;
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
  margin-bottom: 20px;
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
</style>
