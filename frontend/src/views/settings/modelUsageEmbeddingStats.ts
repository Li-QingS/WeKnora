import type { EmbeddingCacheCounterStats, EmbeddingCacheStats } from '@/api/model/usage'

export interface DocumentEmbeddingCacheModel extends EmbeddingCacheCounterStats {
  model_id: string
  model_name: string
}

export function documentEmbeddingCacheModels(
  stats: EmbeddingCacheStats | null,
): DocumentEmbeddingCacheModel[] {
  return (stats?.models ?? []).flatMap((model) => {
    const documentStats = model.workloads?.find((item) => item.workload === 'document_index')
    if (!documentStats) return []
    return [{ model_id: model.model_id, model_name: model.model_name, ...documentStats }]
  })
}
