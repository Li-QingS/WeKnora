<template>
  <div class="model-usage-settings">
    <div class="section-header">
      <div>
        <h2>模型用量</h2>
        <p class="section-description">按模型查看调用次数、输入/输出 Token、成功失败、缓存命中与费用</p>
      </div>
    </div>

    <div class="usage-filters">
      <div class="usage-filter-field">
        <label>模型</label>
        <t-select
          v-model="filterModelId"
          :options="modelFilterOptions"
          clearable
          filterable
          placeholder="全部模型"
        />
      </div>
      <div class="usage-filter-field usage-filter-field--range">
        <label>时间区间</label>
        <t-date-range-picker
          v-model="filterRange"
          value-type="YYYY-MM-DD"
          placeholder="开始日期 / 结束日期"
          clearable
          allow-input
        />
      </div>
      <t-button theme="primary" variant="outline" @click="applyFilters">
        <template #icon><t-icon name="search" /></template>
        查询
      </t-button>
      <t-button variant="text" @click="resetFilters">重置</t-button>
    </div>
    <div v-if="usageError" class="usage-error">
      <t-alert theme="error" :message="usageError" close @close="usageError = ''" />
    </div>

    <t-loading :loading="loading" size="small">
      <div class="usage-overview" aria-label="查询区间用量概览">
        <article class="overview-card">
          <span>调用次数</span>
          <strong>{{ formatCount(overview.calls) }}</strong>
          <small>{{ overview.modelCount }} 个模型</small>
        </article>
        <article class="overview-card">
          <span>总 Token</span>
          <strong>{{ formatCount(overview.totalTokens) }}</strong>
          <small>输入 {{ formatCount(overview.promptTokens) }} · 输出 {{ formatCount(overview.completionTokens) }}</small>
        </article>
        <article class="overview-card" :class="{ 'overview-card--danger': overview.failed > 0 }">
          <span>失败调用</span>
          <strong>{{ formatCount(overview.failed) }}</strong>
          <small>{{ overview.calls ? `${overview.successRate.toFixed(1)}% 成功率` : '暂无调用' }}</small>
        </article>
        <article class="overview-card">
          <span>估算费用</span>
          <strong>{{ formatUSD(overview.cost) }}</strong>
          <small>{{ overview.missingCost ? `${overview.missingCost} 个模型未配置单价` : '所有模型均已计价' }}</small>
        </article>
      </div>

      <section class="usage-section">
        <div class="usage-section__head">
          <div><h3>各模型用量</h3><p>比较不同模型在当前查询区间内的调用与消耗</p></div>
        </div>
        <div class="usage-table-scroll"><table class="usage-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>类型</th>
              <th>调用次数</th>
              <th>成功</th>
              <th>失败</th>
              <th>输入 Token</th>
              <th>输出 Token</th>
              <th>总 Token</th>
              <th>Chat 缓存命中率</th>
              <th>估算费用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in pagedSummary" :key="item.model_id">
              <td class="model-cell" :title="item.model_id">{{ item.model_name || item.model_id }}</td>
              <td><span class="soft-tag" :title="item.model_type">{{ modelTypeLabel(item.model_type) }}</span></td>
              <td class="numeric-cell">{{ formatCount(item.calls) }}</td>
              <td class="numeric-cell">{{ formatCount(item.success_count) }}</td>
              <td class="numeric-cell" :class="{ 'danger-text': item.failed_count > 0 }">{{ formatCount(item.failed_count) }}</td>
              <td class="numeric-cell">{{ formatCount(item.prompt_tokens) }}</td>
              <td class="numeric-cell">{{ formatCount(item.completion_tokens) }}</td>
              <td class="numeric-cell"><strong>{{ formatCount(item.total_tokens) }}</strong></td>
              <td>{{ chatCacheRate(item) }}</td>
              <td class="numeric-cell">{{ formatUSD(item.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && summary.length === 0">
              <td colspan="10" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table></div>
        <div class="usage-pagination"><t-pagination
          v-if="summaryTotal > 0"
          v-model="summaryPage"
          v-model:page-size="summaryPageSize"
          :total="summaryTotal"
          size="small"
          show-jumper
          show-page-number
          :page-size-options="[5, 10, 20]"
        /></div>
      </section>

      <section class="usage-section">
        <div class="usage-section__head">
          <h3>文档向量化复用（进程内累计）</h3>
          <t-tag v-if="cacheStats" :theme="cacheEnabled ? 'success' : 'default'" size="small" variant="light">
            {{ cacheEnabled ? '已开启' : '未开启' }}
          </t-tag>
        </div>
        <div class="cache-metric-hint">
          <strong>统计口径</strong>
          <span>仅包含文档上传、重解析和重建索引时的 Chunk/FAQ 向量化；不含 Wiki 评测、RAG 查询和模型连接测试。服务重启后重新计数。</span>
        </div>
        <div class="usage-table-scroll"><table class="usage-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>有效复用率</th>
              <th>复用次数</th>
              <th>格式归一命中</th>
              <th>请求合并</th>
              <th>未命中次数</th>
              <th>Provider 调用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="cacheStats && !cacheStats.enabled">
              <td colspan="7" class="empty-cell">Embedding 缓存未开启</td>
            </tr>
            <tr v-else-if="cacheStats && documentEmbeddingModels.length === 0">
              <td colspan="7" class="empty-cell">本次启动后暂无文档向量化数据</td>
            </tr>
            <tr v-for="model in documentEmbeddingModels" :key="model.model_id">
              <td class="model-cell" :title="model.model_id">{{ model.model_name || model.model_id }}</td>
              <td><strong class="rate-value">{{ effectiveEmbeddingReuseRate(model) }}</strong></td>
              <td class="numeric-cell">{{ formatCount(model.hits) }}</td>
              <td class="numeric-cell">{{ formatCount(model.normalized_hits) }}</td>
              <td class="numeric-cell">{{ formatCount(model.coalesced_requests) }}</td>
              <td class="numeric-cell">{{ formatCount(model.misses) }}</td>
              <td class="numeric-cell">{{ formatCount(model.provider_calls) }}</td>
            </tr>
          </tbody>
        </table></div>
      </section>

      <section class="usage-section">
        <div class="usage-section__head"><div><h3>调用明细</h3><p>逐条查看模型用途、状态、耗时和费用</p></div></div>
        <div class="usage-table-scroll"><table class="usage-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>模型</th>
              <th>类型</th>
              <th>用途</th>
              <th>状态</th>
              <th>输入 Token</th>
              <th>输出 Token</th>
              <th>缓存读 Token</th>
              <th>耗时(ms)</th>
              <th>费用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="record in records" :key="record.id">
              <td>{{ formatDateTime(record.started_at) }}</td>
              <td class="model-cell" :title="record.model_id">{{ record.model_name || record.model_id }}</td>
              <td><span class="soft-tag" :title="record.model_type">{{ modelTypeLabel(record.model_type) }}</span></td>
              <td :title="record.purpose">{{ modelPurposeLabel(record.purpose) }}</td>
              <td><span class="status-dot" :class="`status-dot--${statusTone(record.status)}`"></span>{{ modelCallStatusLabel(record.status) }}</td>
              <td class="numeric-cell">{{ formatCount(record.prompt_tokens) }}</td>
              <td class="numeric-cell">{{ formatCount(record.completion_tokens) }}</td>
              <td class="numeric-cell">{{ formatCount(record.cache_read_tokens) }}</td>
              <td class="numeric-cell">{{ formatDuration(record.duration_ms) }}</td>
              <td class="numeric-cell">{{ formatUSD(record.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && records.length === 0">
              <td colspan="10" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table></div>
        <div class="usage-pagination"><t-pagination
          v-if="recordTotal > 0"
          v-model="recordPage"
          v-model:page-size="recordPageSize"
          :total="recordTotal"
          size="small"
          show-jumper
          show-page-number
          :page-size-options="[10, 20, 50]"
          @change="onRecordPageChange"
        /></div>
      </section>

      <details v-if="priceModels.length" class="price-settings usage-section">
        <summary>
          <span><strong>模型单价配置</strong><small>用于计算后续调用的估算费用</small></span>
          <span class="price-settings__unit">USD / 百万 Token</span>
        </summary>
        <div class="usage-table-scroll"><table class="usage-table price-table">
          <thead><tr><th>模型</th><th>类型</th><th>输入</th><th>输出</th><th>缓存读</th><th>缓存写</th><th></th></tr></thead>
          <tbody><tr v-for="model in priceModels" :key="model.id">
            <td class="model-cell">{{ model.display_name || model.name }}</td>
            <td>{{ modelTypeLabel(model.type) }}</td>
            <td><t-input v-model="priceDrafts[model.id!].input" type="number" min="0" step="0.001" placeholder="未配置" /></td>
            <td><t-input v-model="priceDrafts[model.id!].output" type="number" min="0" step="0.001" placeholder="未配置" /></td>
            <td><t-input v-model="priceDrafts[model.id!].cacheRead" type="number" min="0" step="0.001" placeholder="未配置" /></td>
            <td><t-input v-model="priceDrafts[model.id!].cacheWrite" type="number" min="0" step="0.001" placeholder="未配置" /></td>
            <td><t-button variant="outline" theme="primary" size="small" :loading="savingPriceId === model.id" @click="saveModelPrice(model)">保存</t-button></td>
          </tr></tbody>
        </table></div>
      </details>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { listModels, type ModelConfig } from '@/api/model'
import {
  getEmbeddingCacheStats,
  getModelCallSummary,
  listModelCalls,
  listModelPrices,
  upsertModelPrice,
} from '@/api/model/usage'
import type {
  EmbeddingCacheCounterStats,
  EmbeddingCacheStats,
  ModelCallRecord,
  ModelCallSummaryItem,
  ModelPrice,
} from '@/api/model/usage'
import { modelUsageDateBounds } from './modelUsageDateRange'
import { documentEmbeddingCacheModels } from './modelUsageEmbeddingStats'
import {
  formatCount,
  formatDateTime,
  formatUSD,
  modelCallStatusLabel,
  modelPurposeLabel,
  modelTypeLabel,
} from './presentation'

const summary = ref<ModelCallSummaryItem[]>([])
const records = ref<ModelCallRecord[]>([])
const loading = ref(false)
const usageError = ref('')
const cacheStats = ref<EmbeddingCacheStats | null>(null)
const allModels = ref<ModelConfig[]>([])
const priceMap = ref<Record<string, ModelPrice>>({})
const priceDrafts = ref<Record<string, PriceDraft>>({})
const savingPriceId = ref('')
const filterModelId = ref('')
const filterRange = ref<string[]>([])
const summaryPage = ref(1)
const summaryPageSize = ref(10)
const recordPage = ref(1)
const recordPageSize = ref(20)
const recordTotal = ref(0)
const cacheEnabled = computed(() => cacheStats.value?.enabled === true)
const summaryTotal = computed(() => summary.value.length)
const pagedSummary = computed(() => {
  const start = (summaryPage.value - 1) * summaryPageSize.value
  return summary.value.slice(start, start + summaryPageSize.value)
})
const documentEmbeddingModels = computed(() => documentEmbeddingCacheModels(cacheStats.value))
const overview = computed(() => {
  const totals = summary.value.reduce((result, item) => {
    result.calls += item.calls
    result.failed += item.failed_count
    result.promptTokens += item.prompt_tokens
    result.completionTokens += item.completion_tokens
    result.totalTokens += item.total_tokens
    if (item.estimated_cost_usd == null) result.missingCost += 1
    else result.cost += item.estimated_cost_usd
    return result
  }, { calls: 0, failed: 0, promptTokens: 0, completionTokens: 0, totalTokens: 0, cost: 0, missingCost: 0 })
  return {
    ...totals,
    modelCount: summary.value.length,
    successRate: totals.calls > 0 ? ((totals.calls - totals.failed) / totals.calls) * 100 : 0,
    cost: totals.missingCost === summary.value.length && summary.value.length > 0 ? null : totals.cost,
  }
})
const priceModels = computed(() =>
  allModels.value.filter((model) => {
    if (!model.id || model.source !== 'remote') return false
    return model.type === 'KnowledgeQA' || model.type === 'Embedding' || model.type === 'Rerank'
  }),
)
const modelFilterOptions = computed(() => {
  const byId = new Map<string, string>()
  for (const model of allModels.value) {
    if (model.id) byId.set(model.id, model.display_name || model.name || model.id)
  }
  for (const item of summary.value) {
    if (!byId.has(item.model_id)) byId.set(item.model_id, item.model_name || item.model_id)
  }
  return [
    { label: '全部模型', value: '' },
    ...Array.from(byId.entries()).map(([value, label]) => ({ label, value })),
  ]
})

function chatCacheRate(item: ModelCallSummaryItem): string {
  if (item.model_type !== 'KnowledgeQA') return '-'
  const denominator = item.prompt_tokens
  if (denominator <= 0) return '-'
  return `${((item.cache_read_tokens / denominator) * 100).toFixed(1)}%`
}

function effectiveEmbeddingReuseRate(model: EmbeddingCacheCounterStats): string {
  const coalesced = model.coalesced_requests || 0
  const total = model.hits + model.misses + coalesced
  if (total <= 0) return '-'
  return `${(((model.hits + coalesced) / total) * 100).toFixed(1)}%`
}

function formatDuration(value: number): string {
  if (value < 1000) return `${formatCount(value)} ms`
  return `${(value / 1000).toFixed(2)} 秒`
}

function statusTone(value: string): string {
  const normalized = value?.toLowerCase()
  if (normalized === 'success' || normalized === 'succeeded' || normalized === 'completed') return 'success'
  if (normalized === 'failed' || normalized === 'error') return 'danger'
  if (normalized === 'running') return 'primary'
  return 'default'
}

interface PriceDraft {
  input: string
  output: string
  cacheRead: string
  cacheWrite: string
}

function toText(value: number | null | undefined): string {
  return value == null ? '' : String(value)
}

function toNullableNumber(raw: string): number | null | undefined {
  const value = raw.trim()
  if (!value) return null
  const number = Number(value)
  return Number.isFinite(number) ? number : undefined
}

function syncPriceDrafts() {
  const next: Record<string, PriceDraft> = {}
  for (const model of priceModels.value) {
    const id = model.id!
    const price = priceMap.value[id] || {}
    next[id] = {
      input: toText(price.input_price_per_million),
      output: toText(price.output_price_per_million),
      cacheRead: toText(price.cache_read_price_per_million),
      cacheWrite: toText(price.cache_write_price_per_million),
    }
  }
  priceDrafts.value = next
}

async function saveModelPrice(model: ModelConfig) {
  const id = model.id
  if (!id || savingPriceId.value) return
  const draft = priceDrafts.value[id]
  const values = {
    input: toNullableNumber(draft.input),
    output: toNullableNumber(draft.output),
    cacheRead: toNullableNumber(draft.cacheRead),
    cacheWrite: toNullableNumber(draft.cacheWrite),
  }
  if (Object.values(values).some((value) => value === undefined)) {
    MessagePlugin.error('价格必须是数字或留空')
    return
  }

  const existing = priceMap.value[id] || {}
  savingPriceId.value = id
  try {
    const payload: Partial<ModelPrice> = {
      input_price_per_million: values.input,
      output_price_per_million: values.output,
      cache_read_price_per_million: values.cacheRead,
      cache_write_price_per_million: values.cacheWrite,
      unit_type: existing.unit_type || '',
      unit_price: existing.unit_price ?? null,
      currency: existing.currency || 'USD',
    }
    const saved = await upsertModelPrice(id, payload)
    priceMap.value = {
      ...priceMap.value,
      [id]: { ...existing, ...saved },
    }
    MessagePlugin.success(`${model.display_name || model.name} 价格已保存`)
  } catch (err: any) {
    MessagePlugin.error(err?.message || '保存模型价格失败')
  } finally {
    savingPriceId.value = ''
  }
}

function buildFilterParams(): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  if (filterModelId.value) params.model_id = filterModelId.value
  Object.assign(params, modelUsageDateBounds(filterRange.value))
  return params
}

async function applyFilters() {
  summaryPage.value = 1
  recordPage.value = 1
  await loadData()
}

async function loadData() {
  loading.value = true
  usageError.value = ''
  try {
    const params = buildFilterParams()
    const [summaries, cache] = await Promise.all([
      getModelCallSummary(params),
      getEmbeddingCacheStats().catch(() => null),
    ])
    summary.value = summaries.sort((a, b) => {
      const typeOrder = (a.model_type || '').localeCompare(b.model_type || '')
      if (typeOrder !== 0) return typeOrder
      const nameOrder = (a.model_name || '').localeCompare(b.model_name || '')
      if (nameOrder !== 0) return nameOrder
      return (a.model_id || '').localeCompare(b.model_id || '')
    })
    cacheStats.value = cache
    await loadRecords(recordPage.value)
  } catch (err: any) {
    usageError.value = err?.message || '加载模型用量失败'
  } finally {
    loading.value = false
  }
}

async function loadRecords(page: number) {
  const params: Record<string, unknown> = {
    ...buildFilterParams(),
    page,
    page_size: recordPageSize.value,
  }
  const result = await listModelCalls(params)
  records.value = result.data
  recordTotal.value = result.total
}

async function onRecordPageChange() {
  usageError.value = ''
  try {
    await loadRecords(recordPage.value)
  } catch (err: any) {
    usageError.value = err?.message || '加载最近调用失败'
  }
}

async function resetFilters() {
  filterModelId.value = ''
  filterRange.value = []
  await applyFilters()
}

async function loadModelOptions() {
  try {
    allModels.value = await listModels()
  } catch {
    allModels.value = []
  }
  try {
    const prices = await listModelPrices()
    priceMap.value = Object.fromEntries(prices.map((price) => [price.model_id, price]))
  } catch {
    priceMap.value = {}
  }
  syncPriceDrafts()
}

onMounted(async () => {
  await loadModelOptions()
  await loadData()
})
</script>

<style scoped>
.model-usage-settings {
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

.usage-section {
  margin-bottom: 24px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 10px;
  background: var(--td-bg-color-container, #fff);
  overflow: hidden;
}

.usage-filters {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 10px 14px;
  margin-bottom: 18px;
}

.usage-filter-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 220px;
}

.usage-filter-field label {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.usage-filter-field--range {
  min-width: 320px;
}

.usage-filter-field :deep(.t-select),
.usage-filter-field :deep(.t-date-range-picker) {
  width: 100%;
}

.usage-error {
  margin-bottom: 14px;
}

.usage-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px 12px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
}

.usage-section__head p {
  margin: 4px 0 0;
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.usage-section__head h3 {
  margin: 0;
  font-size: 15px;
}

.cache-metric-hint {
  display: flex;
  gap: 9px;
  margin: 14px 18px 4px;
  padding: 10px 12px;
  border-radius: 7px;
  background: var(--td-brand-color-light, #eef4ff);
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  line-height: 1.55;
}

.cache-metric-hint strong {
  flex: 0 0 auto;
  color: var(--td-text-color-primary, #333);
}

.usage-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}

.overview-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 10px;
  background: var(--td-bg-color-container, #fff);
}

.overview-card > span,
.overview-card small {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.overview-card strong {
  margin: 7px 0 5px;
  color: var(--td-text-color-primary, #333);
  font-size: 24px;
  line-height: 1.1;
  overflow-wrap: anywhere;
}

.overview-card--danger strong,
.danger-text {
  color: var(--td-error-color, #d54941);
}

.usage-table-scroll {
  width: 100%;
  overflow-x: auto;
}

.usage-table {
  width: 100%;
  min-width: 900px;
  border-collapse: collapse;
  font-size: 13px;
}

.usage-table th,
.usage-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  text-align: left;
  white-space: nowrap;
}

.usage-table th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--td-bg-color-secondarycontainer, #f7f8fa);
  color: var(--td-text-color-secondary, #666);
  font-weight: 600;
}

.usage-table tbody tr:hover {
  background: var(--td-bg-color-container-hover, #f3f3f3);
}

.numeric-cell {
  text-align: right !important;
  font-variant-numeric: tabular-nums;
}

.model-cell {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.soft-tag {
  display: inline-flex;
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--td-bg-color-secondarycontainer, #f3f3f3);
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.rate-value {
  color: var(--td-success-color, #2ba471);
  font-size: 15px;
}

.status-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 6px;
  border-radius: 50%;
  background: var(--td-text-color-placeholder, #aaa);
}

.status-dot--success { background: var(--td-success-color, #2ba471); }
.status-dot--danger { background: var(--td-error-color, #d54941); }
.status-dot--primary { background: var(--td-brand-color, #0052d9); }

.usage-pagination {
  display: flex;
  justify-content: flex-end;
  padding: 12px 16px;
  border-top: 1px solid var(--td-component-stroke, #e7e7e7);
}

.price-table {
  min-width: 1080px;
}

.price-table :deep(.t-input) {
  width: 130px;
}

.price-table :deep(.t-button) {
  white-space: nowrap;
}

.price-settings summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  cursor: pointer;
  list-style-position: inside;
}

.price-settings summary > span:first-child {
  display: inline-flex;
  align-items: baseline;
  gap: 10px;
}

.price-settings summary small,
.price-settings__unit {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
  font-weight: 400;
}

.price-settings[open] summary {
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
}

.empty-cell {
  text-align: center;
  color: var(--td-text-color-secondary, #999);
  padding: 24px 0;
}

@media (max-width: 880px) {
  .usage-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .usage-filters {
    align-items: stretch;
  }

  .usage-filter-field,
  .usage-filter-field--range {
    width: 100%;
    min-width: 0;
  }

  .usage-overview {
    grid-template-columns: 1fr;
  }

  .cache-metric-hint,
  .price-settings summary,
  .price-settings summary > span:first-child {
    flex-direction: column;
    align-items: flex-start;
  }

  .usage-pagination {
    overflow-x: auto;
    justify-content: flex-start;
  }
}
</style>
