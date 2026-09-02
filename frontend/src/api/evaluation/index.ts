import { get } from '../../utils/request'

export interface EvaluationMetric {
  retrieval_metrics: Record<string, number>
  generation_metrics: Record<string, number>
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
  version?: {
    app_version?: string
    git_commit?: string
    git_dirty?: boolean
    go_version?: string
  }
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

export function getEvaluationResult(taskId: string): Promise<EvaluationDetail> {
  return new Promise((resolve, reject) => {
    get('/api/v1/evaluation', { params: { task_id: taskId } })
      .then((response: any) => resolve(response.data))
      .catch(reject)
  })
}
