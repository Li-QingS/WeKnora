package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wikiMonitorKnowledgeFake struct {
	interfaces.KnowledgeService
	calls int
}

func (f *wikiMonitorKnowledgeFake) GetKnowledgeBatch(
	_ context.Context, _ uint64, ids []string,
) ([]*types.Knowledge, error) {
	f.calls++
	status := types.ParseStatusProcessing
	if f.calls >= 2 {
		status = types.ParseStatusCompleted
	}
	result := make([]*types.Knowledge, 0, len(ids))
	for _, id := range ids {
		result = append(result, &types.Knowledge{ID: id, ParseStatus: status})
	}
	return result, nil
}

type wikiMonitorPendingFake struct {
	interfaces.TaskPendingOpsRepository
	counts map[string][]int64
	calls  map[string]int
}

func (f *wikiMonitorPendingFake) PendingCount(_ context.Context, taskType, _, _ string) (int64, error) {
	index := f.calls[taskType]
	f.calls[taskType]++
	values := f.counts[taskType]
	if index >= len(values) {
		return values[len(values)-1], nil
	}
	return values[index], nil
}

type wikiMonitorDeadLetterFake struct {
	interfaces.TaskDeadLetterRepository
	rows []*types.TaskDeadLetter
}

func (f *wikiMonitorDeadLetterFake) ListByScope(
	context.Context, string, string, string, int,
) ([]*types.TaskDeadLetter, string, error) {
	return f.rows, "", nil
}

func TestWikiGenerationMonitorWaitsForDocumentsAndBothQueues(t *testing.T) {
	knowledge := &wikiMonitorKnowledgeFake{}
	pending := &wikiMonitorPendingFake{
		counts: map[string][]int64{wikiTaskType: {1, 0, 0}, wikiFinalizeTaskType: {1, 0, 0}},
		calls:  map[string]int{},
	}
	monitor := NewWikiGenerationMonitor(knowledge, pending, &wikiMonitorDeadLetterFake{})
	monitor.interval = time.Millisecond
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	var progress []types.EvaluationStageProgress

	err := monitor.WaitUntilStable(ctx, 7, "kb", []string{"one", "two"}, func(value types.EvaluationStageProgress) {
		progress = append(progress, value)
	})
	require.NoError(t, err)
	require.Equal(t, 3, knowledge.calls)
	require.Len(t, progress, 3)
	require.Equal(t, 2, progress[2].Current)
	require.Contains(t, progress[2].Message, "wiki pending 0")
}

func TestWikiGenerationMonitorReportsKnowledgeFailure(t *testing.T) {
	knowledge := &wikiMonitorKnowledgeFake{}
	knowledge.calls = 1
	monitor := NewWikiGenerationMonitor(knowledge, &wikiMonitorPendingFake{}, &wikiMonitorDeadLetterFake{})
	// Override the completing fake with a failed row through a small wrapper.
	monitor.knowledge = &wikiMonitorKnowledgeOverride{fn: func(context.Context, uint64, []string) ([]*types.Knowledge, error) {
		return []*types.Knowledge{{ID: "one", ParseStatus: types.ParseStatusFailed, ErrorMessage: "parse error"}}, nil
	}}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	err := monitor.WaitUntilStable(ctx, 7, "kb", []string{"one"}, nil)
	require.ErrorContains(t, err, "parse error")
}

type wikiMonitorKnowledgeOverride struct {
	interfaces.KnowledgeService
	fn func(context.Context, uint64, []string) ([]*types.Knowledge, error)
}

func (f *wikiMonitorKnowledgeOverride) GetKnowledgeBatch(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
	return f.fn(ctx, tenantID, ids)
}

func TestWikiGenerationMonitorReportsScopedDeadLetter(t *testing.T) {
	monitor := NewWikiGenerationMonitor(
		&wikiMonitorKnowledgeFake{},
		&wikiMonitorPendingFake{},
		&wikiMonitorDeadLetterFake{rows: []*types.TaskDeadLetter{{
			TenantID: 7, TaskType: wikiTaskType, RelatedID: "knowledge-1", LastError: "model exhausted",
		}}},
	)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	err := monitor.WaitUntilStable(ctx, 7, "kb", []string{"one"}, nil)
	require.ErrorContains(t, err, "model exhausted")
}

func TestWikiGenerationMonitorHonorsCancellation(t *testing.T) {
	monitor := NewWikiGenerationMonitor(
		&wikiMonitorKnowledgeFake{},
		&wikiMonitorPendingFake{counts: map[string][]int64{wikiTaskType: {1}, wikiFinalizeTaskType: {1}}, calls: map[string]int{}},
		&wikiMonitorDeadLetterFake{},
	)
	monitor.interval = time.Millisecond
	ctx, cancel := context.WithTimeout(
		context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7)), 5*time.Millisecond,
	)
	defer cancel()
	err := monitor.WaitUntilStable(ctx, 7, "kb", []string{"one"}, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
