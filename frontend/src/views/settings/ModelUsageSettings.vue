<template>
  <div class="model-usage-settings">
    <div class="section-header">
      <div>
        <h2>模型用量</h2>
        <p class="section-description">按模型查看调用次数、Token 与估算费用</p>
      </div>
    </div>

    <div class="usage-section settings-group">
      <h3>Embedding 缓存</h3>
      <dl class="cache-stats">
        <div class="cache-stat">
          <dt>命中（hits）</dt>
          <dd>{{ cacheNumber(cacheStats?.hits) }}</dd>
        </div>
        <div class="cache-stat">
          <dt>未命中（misses）</dt>
          <dd>{{ cacheNumber(cacheStats?.misses) }}</dd>
        </div>
        <div class="cache-stat">
          <dt>模型请求（provider_calls）</dt>
          <dd>{{ cacheNumber(cacheStats?.provider_calls) }}</dd>
        </div>
      </dl>
    </div>

    <t-loading :loading="loading" size="small">
      <div class="usage-section">
        <h3>汇总</h3>
        <table class="usage-table">
          <thead>
            <tr>
              <th>模型</th>
              <th>类型</th>
              <th>调用次数</th>
              <th>成功</th>
              <th>失败</th>
              <th>Token</th>
              <th>估算费用</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in summary" :key="item.model_id">
              <td>{{ item.model_name || item.model_id }}</td>
              <td>{{ item.model_type }}</td>
              <td>{{ item.calls }}</td>
              <td>{{ item.success_count }}</td>
              <td>{{ item.failed_count }}</td>
              <td>{{ item.total_tokens }}</td>
              <td>{{ formatCost(item.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && summary.length === 0">
              <td colspan="7" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="usage-section">
        <h3>最近调用</h3>
        <table class="usage-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>模型</th>
              <th>类型</th>
              <th>用途</th>
              <th>状态</th>
              <th>Token</th>
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
              <td>{{ record.total_tokens }}</td>
              <td>{{ record.duration_ms }}</td>
              <td>{{ formatCost(record.estimated_cost_usd) }}</td>
            </tr>
            <tr v-if="!loading && records.length === 0">
              <td colspan="8" class="empty-cell">暂无调用记录</td>
            </tr>
          </tbody>
        </table>
      </div>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  getEmbeddingCacheStats,
  getModelCallSummary,
  listModelCalls,
} from '@/api/model/usage'
import type {
  EmbeddingCacheStats,
  ModelCallRecord,
  ModelCallSummaryItem,
} from '@/api/model/usage'

const summary = ref<ModelCallSummaryItem[]>([])
const records = ref<ModelCallRecord[]>([])
const loading = ref(false)
const cacheStats = ref<EmbeddingCacheStats | null>(null)

function formatCost(value: number | null | undefined): string {
  return value == null ? 'unknown' : `$${value.toFixed(4)}`
}

function cacheNumber(value: number | undefined): number | string {
  return value == null ? 'unknown' : value
}

onMounted(async () => {
  loading.value = true
  try {
    summary.value = await getModelCallSummary()
    const result = await listModelCalls({ page: 1, page_size: 20 })
    records.value = result.data
  } finally {
    loading.value = false
  }
  try {
    cacheStats.value = await getEmbeddingCacheStats()
  } catch {
    cacheStats.value = null
  }
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
}

.usage-section h3 {
  margin: 0 0 10px;
  font-size: 14px;
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.cache-stats {
  display: flex;
  flex-direction: column;
  margin: 0;
}

.cache-stat {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
}

.cache-stat:last-child {
  border-bottom: none;
}

.cache-stat dt {
  color: var(--td-text-color-secondary, #666);
  font-size: 13px;
}

.cache-stat dd {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
}

.usage-table {
  width: 100%;
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

.empty-cell {
  text-align: center;
  color: var(--td-text-color-secondary, #999);
  padding: 24px 0;
}
</style>
