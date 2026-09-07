import { del, get, getDown, post } from '../../utils/request'

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

export interface WikiNodeMetric {
  gold_total: number
  exact_matched: number
  semantic_matched: number
  unmatched: number
  coverage: number
}

export interface WikiGraphMetric {
  correct: number
  missing: number
  extra: number
  precision: number
  recall: number
  f1: number
  scorable: boolean
  note?: string
}

export interface WikiEvaluationMetric {
  entity: WikiNodeMetric
  concept: WikiNodeMetric
  overall: WikiNodeMetric
  graph: WikiGraphMetric
  generation_cost?: EvaluationMetric['cost_metrics']
  scoring_cost?: EvaluationMetric['cost_metrics']
}

export interface WikiNodeMatch {
  gold_node_id: string
  gold_type: string
  gold_name: string
  page_slug?: string
  page_title?: string
  method: 'exact' | 'semantic' | 'unmatched'
  score?: number
  reason?: string
}

export interface WikiEdgeRef {
  source: string
  target: string
  reason?: string
}

export interface WikiEvaluationRun extends EvaluationRun {
  evaluation_type: 'wiki'
  stage?: string
  failure_stage?: string
  stage_progress?: { current?: number; total?: number; message?: string }
}

export interface WikiEvaluationDetail {
  run: WikiEvaluationRun
  params?: {
    dataset_id: string
    chat_id: string
    embedding_id: string
    semantic_threshold: number
  }
  metric?: WikiEvaluationMetric
  result?: {
    metric: WikiEvaluationMetric
    node_matches: WikiNodeMatch[]
    correct_edges: WikiEdgeRef[]
    missing_edges: WikiEdgeRef[]
    extra_edges: WikiEdgeRef[]
    unscored_edges: WikiEdgeRef[]
  }
}

export interface WikiEvaluationDatasetOption {
  id: string
  sha256: string
  document_count: number
  gold: {
    schema_version: string
    dataset_sha256: string
    content_sha256: string
    node_count: number
    edge_count: number
  }
}

export interface StartWikiEvaluationRequest {
  dataset_id: string
  chat_id: string
  embedding_id: string
  semantic_threshold: number
}

export async function listWikiEvaluationRuns(
  page: number,
  pageSize: number,
  status?: number,
): Promise<{ data: WikiEvaluationRun[]; total: number }> {
  const params: Record<string, unknown> = { page, page_size: pageSize }
  if (status !== undefined) params.status = status
  const response: any = await get('/api/v1/evaluation/wiki/runs', { params })
  return { data: response.data || [], total: response.total || 0 }
}

export async function listWikiEvaluationDatasets(): Promise<WikiEvaluationDatasetOption[]> {
  const response: any = await get('/api/v1/evaluation/wiki/datasets')
  return response.data || []
}

export async function startWikiEvaluation(payload: StartWikiEvaluationRequest): Promise<WikiEvaluationDetail> {
  const response: any = await post('/api/v1/evaluation/wiki/runs', payload)
  return response.data
}

export async function getWikiEvaluation(runId: string): Promise<WikiEvaluationDetail> {
  const response: any = await get(`/api/v1/evaluation/wiki/runs/${encodeURIComponent(runId)}`)
  return response.data
}

export async function deleteWikiEvaluation(runId: string): Promise<void> {
  await del(`/api/v1/evaluation/wiki/runs/${encodeURIComponent(runId)}`)
}

export function downloadWikiEvaluationReport(runId: string, format: 'json' | 'markdown'): Promise<Blob> {
  return getDown(`/api/v1/evaluation/wiki/runs/${encodeURIComponent(runId)}/report?format=${format}`)
}
