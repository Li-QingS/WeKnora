package service

import (
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestBuildEvaluationLatencyMetrics(t *testing.T) {
	started := time.Now()
	metrics := buildEvaluationLatencyMetrics(
		started,
		started.Add(10*time.Second),
		5,
		&types.EvaluationCostMetrics{ModelCalls: 10},
		4000,
	)
	require.NotNil(t, metrics)
	require.Equal(t, int64(10_000), metrics.DurationMS)
	require.Equal(t, float64(2_000), metrics.AvgMsPerSample)
	require.Equal(t, float64(400), metrics.AvgMsPerModelCall)
}
