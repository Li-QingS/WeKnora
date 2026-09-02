package service

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

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
	records, _, err := e.modelCalls.List(ctx, tenantID, &types.ModelCallFilter{
		RequestGroupID: taskID,
	}, &types.Pagination{Page: 1, PageSize: 100000})
	if err != nil {
		logger.Errorf(ctx, "Failed to load model calls for evaluation %s: %v", taskID, err)
		return nil, 0
	}
	if len(records) == 0 {
		return &types.EvaluationCostMetrics{}, 0
	}

	metrics := &types.EvaluationCostMetrics{}
	var costSum float64
	var durationMS int64
	hasCost := false
	for _, record := range records {
		metrics.ModelCalls++
		metrics.PromptTokens += int64(record.PromptTokens)
		metrics.CompletionTokens += int64(record.CompletionTokens)
		metrics.TotalTokens += int64(record.TotalTokens)
		metrics.CacheReadTokens += int64(record.CacheReadTokens)
		metrics.CacheWriteTokens += int64(record.CacheWriteTokens)
		durationMS += record.DurationMS
		if record.EstimatedCostUSD != nil {
			costSum += *record.EstimatedCostUSD
			hasCost = true
		}
	}
	if hasCost {
		metrics.EstimatedCostUSD = &costSum
	}
	return metrics, durationMS
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
