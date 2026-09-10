package embedding

import "context"

// CacheWorkload identifies the business stage responsible for an embedding
// cache lookup. It affects observability only; it is deliberately excluded
// from cache keys so vectors remain reusable across stages.
type CacheWorkload string

const (
	CacheWorkloadDocumentIndex  CacheWorkload = "document_index"
	CacheWorkloadWikiEvaluation CacheWorkload = "wiki_evaluation"
	CacheWorkloadOther          CacheWorkload = "other"
)

type cacheWorkloadContextKey struct{}

// WithCacheWorkload attributes embedding cache counters to a business stage.
func WithCacheWorkload(ctx context.Context, workload CacheWorkload) context.Context {
	if workload == "" {
		workload = CacheWorkloadOther
	}
	return context.WithValue(ctx, cacheWorkloadContextKey{}, workload)
}

// CacheWorkloadFromContext returns the attributed stage, defaulting to other
// for legacy callers that have not supplied one.
func CacheWorkloadFromContext(ctx context.Context) CacheWorkload {
	if ctx != nil {
		if workload, ok := ctx.Value(cacheWorkloadContextKey{}).(CacheWorkload); ok && workload != "" {
			return workload
		}
	}
	return CacheWorkloadOther
}
