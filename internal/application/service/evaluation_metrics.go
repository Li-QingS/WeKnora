package service

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

type requestGroupRollupRepository interface {
	RollupRequestGroup(ctx context.Context, tenantID uint64, requestGroupID string) (*types.ModelCallRollup, error)
}

// evaluationLedgerRollup rolls up every model call attributed to this run's
// request group. Returns nil when the ledger is not available or no call was
// attributed to the run, along with the summed model-call duration.
func (e *EvaluationService) evaluationLedgerRollup(
	ctx context.Context,
	taskID string,
) (*types.EvaluationCostMetrics, int64) {
	if e.modelCalls == nil {
		return nil, 0
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	rollup, err := e.modelCallRollup(ctx, tenantID, taskID)
	if err != nil {
		logger.Errorf(ctx, "Failed to load model calls for evaluation %s: %v", taskID, err)
		return nil, 0
	}
	if rollup == nil {
		return &types.EvaluationCostMetrics{}, 0
	}
	return rollupToCostMetrics(rollup), rollup.DurationMS
}

func (e *EvaluationService) modelCallRollup(
	ctx context.Context,
	tenantID uint64,
	requestGroupID string,
) (*types.ModelCallRollup, error) {
	if repo, ok := e.modelCalls.(requestGroupRollupRepository); ok {
		return repo.RollupRequestGroup(ctx, tenantID, requestGroupID)
	}

	// Fallback for tests and alternative repositories: paginate rather than
	// assuming an arbitrarily large single page is enough.
	rollup := &types.ModelCallRollup{}
	const pageSize = 10000
	for page := 1; ; page++ {
		records, total, err := e.modelCalls.List(ctx, tenantID, &types.ModelCallFilter{
			RequestGroupID: requestGroupID,
		}, &types.Pagination{Page: page, PageSize: pageSize})
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			rollup.Calls++
			rollup.DurationMS += record.DurationMS
			rollup.PromptTokens += int64(record.PromptTokens)
			rollup.CompletionTokens += int64(record.CompletionTokens)
			rollup.TotalTokens += int64(record.TotalTokens)
			rollup.CacheReadTokens += int64(record.CacheReadTokens)
			rollup.CacheWriteTokens += int64(record.CacheWriteTokens)
			rollup.CacheMissTokens += int64(record.CacheMissTokens)
			if record.EstimatedCostUSD != nil {
				if rollup.EstimatedCostUSD == nil {
					value := 0.0
					rollup.EstimatedCostUSD = &value
				}
				*rollup.EstimatedCostUSD += *record.EstimatedCostUSD
			}
		}
		if len(records) == 0 || len(records) < pageSize || int64(page*pageSize) >= total {
			break
		}
	}
	if rollup.Calls == 0 {
		return nil, nil
	}
	return rollup, nil
}

func rollupToCostMetrics(rollup *types.ModelCallRollup) *types.EvaluationCostMetrics {
	metrics := &types.EvaluationCostMetrics{}
	metrics.ModelCalls = rollup.Calls
	metrics.PromptTokens = rollup.PromptTokens
	metrics.CompletionTokens = rollup.CompletionTokens
	metrics.TotalTokens = rollup.TotalTokens
	metrics.CacheReadTokens = rollup.CacheReadTokens
	metrics.CacheWriteTokens = rollup.CacheWriteTokens
	metrics.EstimatedCostUSD = rollup.EstimatedCostUSD
	return metrics
}

func buildEvaluationLatencyMetrics(
	startedAt time.Time,
	finishedAt time.Time,
	samples int,
	cost *types.EvaluationCostMetrics,
	modelCallDurationMS int64,
) *types.EvaluationLatencyMetrics {
	metrics := &types.EvaluationLatencyMetrics{
		DurationMS: finishedAt.Sub(startedAt).Milliseconds(),
	}
	if cost != nil {
		metrics.ModelCalls = cost.ModelCalls
	}
	if samples > 0 {
		metrics.AvgMsPerSample = float64(metrics.DurationMS) / float64(samples)
	}
	if metrics.ModelCalls > 0 {
		metrics.AvgMsPerModelCall = float64(modelCallDurationMS) / float64(metrics.ModelCalls)
	}
	return metrics
}
