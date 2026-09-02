import { get, put } from '../../utils/request'

export interface ModelCallRecord {
  id: string
  tenant_id: number
  model_id: string
  model_name: string
  model_type: string
  purpose: string
  status: string
  started_at: string
  finished_at: string
  duration_ms: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cache_miss_tokens: number
  unit_type: string
  unit_count: number
  error_type: string
  error_message: string
  session_id: string
  user_id: string
  trace_id: string
  estimated_cost_usd: number | null
  price_snapshot: Record<string, unknown>
}

export interface ModelCallSummaryItem {
  model_id: string
  model_name: string
  model_type: string
  calls: number
  success_count: number
  failed_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cache_miss_tokens: number
  estimated_cost_usd: number | null
}

export interface ModelPrice {
  model_id: string
  input_price_per_million?: number | null
  output_price_per_million?: number | null
  cache_read_price_per_million?: number | null
  cache_write_price_per_million?: number | null
  unit_type?: string
  unit_price?: number | null
  currency?: string
}

export interface EmbeddingCacheStats {
  enabled: boolean
  hits: number
  misses: number
  provider_calls: number
  models?: Array<{
    model_id: string
    model_name: string
    hits: number
    misses: number
    provider_calls: number
  }>
}

export function listModelCalls(params: Record<string, unknown>): Promise<{ data: ModelCallRecord[]; total: number }> {
  return new Promise((resolve, reject) => {
    get('/api/v1/model-calls', { params })
      .then((response: any) => {
        resolve({ data: response.data || [], total: response.total || 0 })
      })
      .catch(reject)
  })
}

export function getModelCallSummary(params?: Record<string, unknown>): Promise<ModelCallSummaryItem[]> {
  return new Promise((resolve, reject) => {
    const request = params
      ? get('/api/v1/model-calls/summary', { params })
      : get('/api/v1/model-calls/summary')
    request
      .then((response: any) => resolve(response.data || []))
      .catch(reject)
  })
}

export function listModelPrices(): Promise<ModelPrice[]> {
  return new Promise((resolve, reject) => {
    get('/api/v1/model-prices')
      .then((response: any) => resolve(response.data || []))
      .catch(reject)
  })
}

export function upsertModelPrice(modelId: string, price: Partial<ModelPrice>): Promise<ModelPrice> {
  return new Promise((resolve, reject) => {
    put(`/api/v1/model-prices/${modelId}`, price)
      .then((response: any) => resolve(response.data))
      .catch(reject)
  })
}

export function getEmbeddingCacheStats(): Promise<EmbeddingCacheStats> {
  return new Promise((resolve, reject) => {
    get('/api/v1/embedding-cache/stats')
      .then((response: any) => resolve(response.data))
      .catch(reject)
  })
}
