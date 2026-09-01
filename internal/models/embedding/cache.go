package embedding

import (
	"context"
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
	cacheMu            sync.RWMutex
	globalCache        EmbeddingCache
	statsHits          atomic.Int64
	statsMisses        atomic.Int64
	statsProviderCalls atomic.Int64
)

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
	return types.EmbeddingCacheStats{
		Hits:          statsHits.Load(),
		Misses:        statsMisses.Load(),
		ProviderCalls: statsProviderCalls.Load(),
	}
}

// ResetCacheStats clears hit/miss counters (used by tests and demos).
func ResetCacheStats() {
	statsHits.Store(0)
	statsMisses.Store(0)
	statsProviderCalls.Store(0)
}
