package rerank

import (
	"context"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// costReranker records rerank calls into the model cost ledger.
type costReranker struct {
	inner    Reranker
	tenantID uint64
}

func (r *costReranker) Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error) {
	info := costledger.NewCallInfo(ctx, r.tenantID, string(types.ModelTypeRerank), r.inner.GetModelID(), r.inner.GetModelName())
	info.UnitType = "documents"
	info.UnitCount = int64(len(documents))
	texts := append([]string{query}, documents...)
	info.PromptTokens = costledger.ApproxTokens(texts)
	info.TotalTokens = info.PromptTokens

	results, err := r.inner.Rerank(ctx, query, documents)
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record rerank call: %v", recordErr)
	}
	return results, err
}

func (r *costReranker) GetModelName() string { return r.inner.GetModelName() }
func (r *costReranker) GetModelID() string   { return r.inner.GetModelID() }

func wrapRerankerCost(r Reranker, tenantID uint64) Reranker {
	return &costReranker{inner: r, tenantID: tenantID}
}
