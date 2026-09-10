import assert from 'node:assert/strict'
import test from 'node:test'
import type { EmbeddingCacheStats } from '@/api/model/usage'
import { documentEmbeddingCacheModels } from './modelUsageEmbeddingStats'

test('document embedding metrics exclude wiki evaluation workload', () => {
  const counters = (hits: number, misses: number) => ({
    hits,
    normalized_hits: 0,
    coalesced_requests: 0,
    misses,
    provider_calls: misses,
  })
  const stats: EmbeddingCacheStats = {
    enabled: true,
    ...counters(101, 202),
    models: [
      {
        model_id: 'mixed-model',
        model_name: 'mixed',
        ...counters(100, 200),
        workloads: [
          { workload: 'document_index', ...counters(7, 3) },
          { workload: 'wiki_evaluation', ...counters(1, 99) },
        ],
      },
      {
        model_id: 'wiki-only-model',
        model_name: 'wiki-only',
        ...counters(1, 2),
        workloads: [{ workload: 'wiki_evaluation', ...counters(1, 2) }],
      },
    ],
  }

  assert.deepEqual(documentEmbeddingCacheModels(stats), [
    {
      model_id: 'mixed-model',
      model_name: 'mixed',
      workload: 'document_index',
      ...counters(7, 3),
    },
  ])
})
