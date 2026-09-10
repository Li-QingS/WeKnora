package embedding

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/Tencent/WeKnora/internal/types"
)

// EmbeddingCache is the narrow interface cachedEmbedder uses.
type EmbeddingCache interface {
	Get(ctx context.Context, key *types.EmbeddingCacheKey) ([]float32, bool, error)
	Set(ctx context.Context, key *types.EmbeddingCacheKey, vector []float32) error
	IncrementHit(ctx context.Context, key *types.EmbeddingCacheKey) error
}

var (
	cacheMu             sync.RWMutex
	globalCache         EmbeddingCache
	statsHits           atomic.Int64
	statsNormalizedHits atomic.Int64
	statsCoalesced      atomic.Int64
	statsMisses         atomic.Int64
	statsProviderCalls  atomic.Int64
	modelStatsMu        sync.Mutex
	modelStats          = map[string]*modelCacheStats{}
)

type modelCacheStats struct {
	modelID        string
	modelName      string
	hits           int64
	normalizedHits int64
	coalesced      int64
	misses         int64
	providerCalls  int64
	workloads      map[CacheWorkload]*workloadCacheStats
}

type workloadCacheStats struct {
	hits           int64
	normalizedHits int64
	coalesced      int64
	misses         int64
	providerCalls  int64
}

// SetEmbeddingCache installs the process-wide embedding cache. Tests may
// replace or clear it.
func SetEmbeddingCache(c EmbeddingCache) {
	cacheMu.Lock()
	globalCache = c
	cacheMu.Unlock()
}

// GetEmbeddingCache returns the installed cache, or nil when disabled.
func GetEmbeddingCache() EmbeddingCache {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return globalCache
}

// CacheStats returns process-level hit/miss counters.
func CacheStats() types.EmbeddingCacheStats {
	stats := types.EmbeddingCacheStats{
		Enabled:           GetEmbeddingCache() != nil,
		Hits:              statsHits.Load(),
		NormalizedHits:    statsNormalizedHits.Load(),
		CoalescedRequests: statsCoalesced.Load(),
		Misses:            statsMisses.Load(),
		ProviderCalls:     statsProviderCalls.Load(),
	}
	modelStatsMu.Lock()
	defer modelStatsMu.Unlock()
	for _, model := range modelStats {
		workloads := make([]types.EmbeddingCacheWorkloadStats, 0, len(model.workloads))
		for workload, counters := range model.workloads {
			workloads = append(workloads, types.EmbeddingCacheWorkloadStats{
				Workload:          string(workload),
				Hits:              counters.hits,
				NormalizedHits:    counters.normalizedHits,
				CoalescedRequests: counters.coalesced,
				Misses:            counters.misses,
				ProviderCalls:     counters.providerCalls,
			})
		}
		sort.Slice(workloads, func(i, j int) bool { return workloads[i].Workload < workloads[j].Workload })
		stats.Models = append(stats.Models, types.EmbeddingCacheModelStats{
			ModelID:           model.modelID,
			ModelName:         model.modelName,
			Hits:              model.hits,
			NormalizedHits:    model.normalizedHits,
			CoalescedRequests: model.coalesced,
			Misses:            model.misses,
			ProviderCalls:     model.providerCalls,
			Workloads:         workloads,
		})
	}
	sort.Slice(stats.Models, func(i, j int) bool {
		if stats.Models[i].ModelID != stats.Models[j].ModelID {
			return stats.Models[i].ModelID < stats.Models[j].ModelID
		}
		return stats.Models[i].ModelName < stats.Models[j].ModelName
	})
	return stats
}

// ResetCacheStats clears hit/miss counters (used by tests and demos).
func ResetCacheStats() {
	statsHits.Store(0)
	statsNormalizedHits.Store(0)
	statsCoalesced.Store(0)
	statsMisses.Store(0)
	statsProviderCalls.Store(0)
	modelStatsMu.Lock()
	modelStats = map[string]*modelCacheStats{}
	modelStatsMu.Unlock()
}

func recordCacheHit(ctx context.Context, modelID, modelName string) {
	statsHits.Add(1)
	recordModelStat(ctx, modelID, modelName, func(s *workloadCacheStats) { s.hits++ }, func(s *modelCacheStats) { s.hits++ })
}

func recordNormalizedCacheHit(ctx context.Context, modelID, modelName string) {
	statsNormalizedHits.Add(1)
	recordModelStat(ctx, modelID, modelName, func(s *workloadCacheStats) { s.normalizedHits++ }, func(s *modelCacheStats) { s.normalizedHits++ })
}

func recordCoalescedRequest(ctx context.Context, modelID, modelName string) {
	statsCoalesced.Add(1)
	recordModelStat(ctx, modelID, modelName, func(s *workloadCacheStats) { s.coalesced++ }, func(s *modelCacheStats) { s.coalesced++ })
}

func recordCacheMiss(ctx context.Context, modelID, modelName string) {
	statsMisses.Add(1)
	recordModelStat(ctx, modelID, modelName, func(s *workloadCacheStats) { s.misses++ }, func(s *modelCacheStats) { s.misses++ })
}

func recordProviderCall(ctx context.Context, modelID, modelName string) {
	statsProviderCalls.Add(1)
	recordModelStat(ctx, modelID, modelName, func(s *workloadCacheStats) { s.providerCalls++ }, func(s *modelCacheStats) { s.providerCalls++ })
}

func recordModelStat(
	ctx context.Context,
	modelID, modelName string,
	mutateWorkload func(*workloadCacheStats),
	mutateModel func(*modelCacheStats),
) {
	modelStatsMu.Lock()
	defer modelStatsMu.Unlock()
	model := modelStats[modelID]
	if model == nil {
		model = &modelCacheStats{
			modelID: modelID, modelName: modelName,
			workloads: make(map[CacheWorkload]*workloadCacheStats),
		}
		modelStats[modelID] = model
	}
	if model.workloads == nil {
		model.workloads = make(map[CacheWorkload]*workloadCacheStats)
	}
	workload := CacheWorkloadFromContext(ctx)
	counters := model.workloads[workload]
	if counters == nil {
		counters = &workloadCacheStats{}
		model.workloads[workload] = counters
	}
	mutateModel(model)
	mutateWorkload(counters)
}
