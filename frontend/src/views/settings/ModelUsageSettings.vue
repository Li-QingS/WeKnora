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
      <div v-if="priceModels.length" class="usage-section">
        <div class="usage-section__head">
          <h3>模型单价配置</h3>
          <span class="usage-section__hint">USD / 百万 Token，保存后影响后续调用</span>
        </div>
        <table class="usage-table price-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>类型</th>
              <th>输入</th>
              <th>输出</th>
              <th>缓存读</th>
              <th>缓存写</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="model in priceModels" :key="model.id">
              <td>{{ model.display_name || model.name }}</td>
              <td>{{ model.type }}</td>
              <td>
                <t-input v-model="priceDrafts[model.id!].input" type="number" min="0" step="0.001"
                  placeholder="0" />
              </td>
              <td>
                <t-input v-model="priceDrafts[model.id!].output" type="number" min="0" step="0.001"
                  placeholder="0" />
              </td>
              <td>
                <t-input v-model="priceDrafts[model.id!].cacheRead" type="number" min="0" step="0.001"
                  placeholder="0" />
              </td>
              <td>
                <t-input v-model="priceDrafts[model.id!].cacheWrite" type="number" min="0" step="0.001"
                  placeholder="0" />
              </td>
              <td>
                <t-button variant="outline" theme="primary" size="small"
                  :loading="savingPriceId === model.id" @click="saveModelPrice(model)">
                  保存
                </t-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="usage-section">
        <div class="usage-section__head">
          <h3>各模型用量</h3>
        </div>
        <table class="usage-table">
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
              <td>{{ item.model_name || item.model_id }}</td>
              <td>{{ item.model_type }}</td>
              <td>{{ item.calls }}</td>
              <td>{{ item.success_count }}</td>
              <td>{{ item.failed_count }}</td>
              <td>{{ item.prompt_tokens }}</td>
              <td>{{ item.completion_tokens }}</td>
              <td>{{ item.total_tokens }}</td>
              <td>{{ chatCacheRate(item) }}</td>
              <td>{{ formatCost(item.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && summary.length === 0">
              <td colspan="10" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table>
        <t-pagination
          v-if="summaryTotal > 0"
          v-model="summaryPage"
          v-model:page-size="summaryPageSize"
          :total="summaryTotal"
          size="small"
          show-jumper
          show-page-number
          :page-size-options="[5, 10, 20]"
        />
      </div>

      <div class="usage-section">
        <div class="usage-section__head">
          <h3>Embedding 向量复用（进程内累计）</h3>
          <t-tag v-if="cacheStats" :theme="cacheEnabled ? 'success' : 'default'" size="small" variant="light">
            {{ cacheEnabled ? '已开启' : '未开启' }}
          </t-tag>
        </div>
        <table class="usage-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>复用次数</th>
              <th>未命中次数</th>
              <th>Provider 调用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="cacheStats && !cacheStats.enabled">
              <td colspan="4" class="empty-cell">Embedding 缓存未开启</td>
            </tr>
            <tr v-else-if="cacheStats && embeddingModels.length === 0">
              <td colspan="4" class="empty-cell">暂无数据</td>
            </tr>
            <tr v-for="model in embeddingModels" :key="model.model_id">
              <td>{{ model.model_name || model.model_id }}</td>
              <td>{{ model.hits }}</td>
              <td>{{ model.misses }}</td>
              <td>{{ model.provider_calls }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="usage-section">
        <h3>最近模型调用</h3>
        <table class="usage-table">
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
              <td>{{ record.started_at }}</td>
              <td>{{ record.model_name || record.model_id }}</td>
              <td>{{ record.model_type }}</td>
              <td>{{ record.purpose }}</td>
              <td>{{ record.status }}</td>
              <td>{{ record.prompt_tokens }}</td>
              <td>{{ record.completion_tokens }}</td>
              <td>{{ record.cache_read_tokens }}</td>
              <td>{{ record.duration_ms }}</td>
              <td>{{ formatCost(record.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && records.length === 0">
              <td colspan="10" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table>
        <t-pagination
          v-if="recordTotal > 0"
          v-model="recordPage"
          v-model:page-size="recordPageSize"
          :total="recordTotal"
          size="small"
          show-jumper
          show-page-number
          :page-size-options="[10, 20, 50]"
          @change="onRecordPageChange"
        />
      </div>
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
  EmbeddingCacheStats,
  ModelCallRecord,
  ModelCallSummaryItem,
  ModelPrice,
} from '@/api/model/usage'

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
const embeddingModels = computed(() => cacheStats.value?.models ?? [])
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
  const denominator = item.cache_read_tokens + item.cache_miss_tokens
  if (denominator <= 0) return '-'
  return `${((item.cache_read_tokens / denominator) * 100).toFixed(1)}%`
}

function formatCost(value: number | null | undefined): string {
  return value == null ? 'unknown' : `$${value.toFixed(4)}`
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
  const [fromDate, toDate] = filterRange.value
  if (fromDate) params.from = dateToApiTime(fromDate, false)
  if (toDate) params.to = dateToApiTime(toDate, true)
  return params
}

function dateToApiTime(value: string, endOfDay: boolean): string {
  const parsed = new Date(`${value}T${endOfDay ? '23:59:59.999' : '00:00:00'}`)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toISOString()
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
  overflow-x: auto;
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

.usage-section h3 {
  margin: 0 0 10px;
  font-size: 14px;
}

.usage-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.usage-section__hint {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.usage-section__head h3 {
  margin: 0;
  font-size: 14px;
}

.usage-table {
  width: 100%;
  min-width: 900px;
  border-collapse: collapse;
  font-size: 13px;
}

.usage-table th,
.usage-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  text-align: left;
  white-space: nowrap;
}

.usage-table th {
  font-weight: 600;
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

.empty-cell {
  text-align: center;
  color: var(--td-text-color-secondary, #999);
  padding: 24px 0;
}
</style>
