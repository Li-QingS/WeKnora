import { del, get, post } from '../../utils/request'

export interface EvaluationMetric {
  retrieval_metrics: Record<string, number>
  generation_metrics: Record<string, number>
  cost_metrics?: {
    model_calls: number
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    cache_read_tokens: number
    cache_write_tokens: number
    estimated_cost_usd: number | null
  }
  latency_metrics?: {
    duration_ms: number
    avg_ms_per_sample: number
    model_calls: number
    avg_ms_per_model_call: number
  }
}

export interface EvaluationConfigSnapshot {
  dataset?: {
    id?: string
    sha256?: string
    sample_count?: number
  }
  models?: Array<{
    id?: string
    name?: string
    provider?: string
    type?: string
  }>
  chunking?: {
    strategy?: string
    chunk_size?: number
    chunk_overlap?: number
    token_limit?: number
    languages?: string[]
  }
  version?: {
    app_version?: string
    git_commit?: string
    git_dirty?: boolean
    go_version?: string
  }
}

export interface EvaluationDatasetOption {
  id: string
  sha256: string
  sample_count: number
}

export interface StartEvaluationRequest {
  dataset_id: string
  chat_id?: string
  embedding_id?: string
  rerank_id?: string
  chunking?: {
    strategy: string
    chunk_size: number
    chunk_overlap: number
    token_limit?: number
    languages?: string[]
  }
  params?: Record<string, unknown>
}

export interface EvaluationRun {
  id: string
  tenant_id: number
  dataset_id: string
  status: number
  err_msg?: string
  start_time: string
  total: number
  finished: number
  metric?: EvaluationMetric
  config_hash: string
  config_snapshot?: EvaluationConfigSnapshot
  created_at: string
  updated_at: string
}

export interface EvaluationDetail {
  task: {
    id: string
    tenant_id: number
    dataset_id: string
    start_time: string
    status: number
    err_msg?: string
    total?: number
    finished?: number
  }
  params: Record<string, unknown>
  metric?: EvaluationMetric
}

export function listEvaluationRuns(
  page: number,
  pageSize: number,
  status?: number,
): Promise<{ data: EvaluationRun[]; total: number }> {
  return new Promise((resolve, reject) => {
    const params: Record<string, unknown> = { page, page_size: pageSize }
    if (status !== undefined) {
      params.status = status
    }
    get('/api/v1/evaluation/runs', { params })
      .then((response: any) => resolve({ data: response.data || [], total: response.total || 0 }))
      .catch(reject)
  })
}

export function listEvaluationDatasets(): Promise<EvaluationDatasetOption[]> {
  return new Promise((resolve, reject) => {
    get('/api/v1/evaluation/datasets')
      .then((response: any) => resolve(response.data || []))
      .catch(reject)
  })
}

export function startEvaluation(payload: StartEvaluationRequest): Promise<EvaluationDetail> {
  return new Promise((resolve, reject) => {
    post('/api/v1/evaluation', payload)
      .then((response: any) => resolve(response.data))
      .catch(reject)
  })
}

export function getEvaluationResult(taskId: string): Promise<EvaluationDetail> {
  return new Promise((resolve, reject) => {
    get('/api/v1/evaluation', { params: { task_id: taskId } })
      .then((response: any) => resolve(response.data))
      .catch(reject)
  })
}

export function deleteEvaluationRun(taskId: string): Promise<void> {
  return new Promise((resolve, reject) => {
    del(`/api/v1/evaluation/runs/${encodeURIComponent(taskId)}`)
      .then(() => resolve())
      .catch(reject)
  })
}
