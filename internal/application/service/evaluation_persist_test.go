package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEvaluationRunRepository struct {
	mu             sync.Mutex
	runs           map[string]*types.EvaluationRun
	transitionGate chan struct{}
}

func newFakeEvaluationRunRepository() *fakeEvaluationRunRepository {
	return &fakeEvaluationRunRepository{
		runs: make(map[string]*types.EvaluationRun),
	}
}

func (f *fakeEvaluationRunRepository) Create(_ context.Context, run *types.EvaluationRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[run.ID] = cloneEvaluationRun(run)
	return nil
}

func (f *fakeEvaluationRunRepository) GetByID(
	_ context.Context,
	tenantID uint64,
	id string,
) (*types.EvaluationRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok || run.TenantID != tenantID {
		return nil, repository.ErrEvaluationRunNotFound
	}
	return cloneEvaluationRun(run), nil
}

func (f *fakeEvaluationRunRepository) List(
	_ context.Context,
	tenantID uint64,
	status *types.EvaluationStatue,
	p *types.Pagination,
) ([]*types.EvaluationRun, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p == nil {
		p = &types.Pagination{}
	}

	runs := make([]*types.EvaluationRun, 0)
	for _, run := range f.runs {
		if run.TenantID != tenantID {
			continue
		}
		if status != nil && run.Status != *status {
			continue
		}
		runs = append(runs, cloneEvaluationRun(run))
	}
	slices.SortFunc(runs, func(a, b *types.EvaluationRun) int {
		switch {
		case b.CreatedAt.Before(a.CreatedAt):
			return -1
		case a.CreatedAt.Before(b.CreatedAt):
			return 1
		default:
			return 0
		}
	})

	total := int64(len(runs))
	offset := p.Offset()
	limit := p.Limit()
	if offset >= len(runs) {
		runs = nil
	} else {
		end := min(offset+limit, len(runs))
		runs = runs[offset:end]
	}
	return runs, total, nil
}

func (f *fakeEvaluationRunRepository) UpdateProgress(
	_ context.Context,
	id string,
	finished int,
	total int,
	metric json.RawMessage,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok || run.Status != types.EvaluationStatueRunning {
		return nil
	}
	run.Finished = finished
	run.Total = total
	run.Metric = append(json.RawMessage(nil), metric...)
	run.UpdatedAt = time.Now()
	return nil
}

func (f *fakeEvaluationRunRepository) UpdateHeartbeat(_ context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok || run.Status != types.EvaluationStatueRunning {
		return nil
	}
	run.HeartbeatAt = &at
	run.UpdatedAt = time.Now()
	return nil
}

func (f *fakeEvaluationRunRepository) SetDatasetHash(
	_ context.Context,
	id string,
	sha256 string,
	samples int,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok || run.Status != types.EvaluationStatueRunning {
		return nil
	}
	snapshot := types.EvaluationConfigSnapshot{}
	if len(run.ConfigSnapshot) > 0 {
		if err := json.Unmarshal(run.ConfigSnapshot, &snapshot); err != nil {
			return err
		}
	}
	snapshot.Dataset.ID = run.DatasetID
	snapshot.Dataset.SHA256 = sha256
	snapshot.Dataset.SampleCount = samples
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	run.ConfigSnapshot = encoded
	run.UpdatedAt = time.Now()
	return nil
}

func (f *fakeEvaluationRunRepository) TransitionStatus(
	_ context.Context,
	id string,
	from []types.EvaluationStatue,
	to types.EvaluationStatue,
	errMsg string,
) (bool, error) {
	if f.transitionGate != nil &&
		slices.Contains(from, types.EvaluationStatuePending) &&
		to == types.EvaluationStatueRunning {
		<-f.transitionGate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[id]
	if !ok || isFakeTerminal(run.Status) || !slices.Contains(from, run.Status) {
		return false, nil
	}
	run.Status = to
	run.ErrMsg = errMsg
	if isFakeTerminal(to) {
		now := time.Now()
		run.FinishedAt = &now
	}
	run.UpdatedAt = time.Now()
	return true, nil
}

func (f *fakeEvaluationRunRepository) MarkStaleInterrupted(
	_ context.Context,
	cutoff time.Time,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	var affected int64
	for _, run := range f.runs {
		if run.Status != types.EvaluationStatuePending &&
			run.Status != types.EvaluationStatueRunning {
			continue
		}
		if run.HeartbeatAt != nil && !run.HeartbeatAt.Before(cutoff) {
			continue
		}
		run.Status = types.EvaluationStatueInterrupted
		run.FinishedAt = &now
		run.ErrMsg = "interrupted by service restart"
		run.UpdatedAt = now
		affected++
	}
	return affected, nil
}

func cloneEvaluationRun(run *types.EvaluationRun) *types.EvaluationRun {
	if run == nil {
		return nil
	}
	cloned := *run
	cloned.Params = append(json.RawMessage(nil), run.Params...)
	cloned.Metric = append(json.RawMessage(nil), run.Metric...)
	cloned.ConfigSnapshot = append(json.RawMessage(nil), run.ConfigSnapshot...)
	return &cloned
}

func isFakeTerminal(status types.EvaluationStatue) bool {
	return status == types.EvaluationStatueSuccess ||
		status == types.EvaluationStatueFailed ||
		status == types.EvaluationStatueInterrupted
}

type evalModelService struct {
	interfaces.ModelService
	modelsByID map[string]*types.Model
	list       []*types.Model
}

func (s *evalModelService) GetModelByID(_ context.Context, id string) (*types.Model, error) {
	return s.modelsByID[id], nil
}

func (s *evalModelService) ListModels(context.Context) ([]*types.Model, error) {
	return s.list, nil
}

type evalKnowledgeBaseService struct {
	interfaces.KnowledgeBaseService
}

func (s *evalKnowledgeBaseService) CreateKnowledgeBase(
	_ context.Context,
	_ *types.KnowledgeBase,
) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: "kb-1", EmbeddingModelID: "embed-1"}, nil
}

func (s *evalKnowledgeBaseService) GetKnowledgeBaseByID(
	_ context.Context,
	_ string,
) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: "kb-1", EmbeddingModelID: "embed-1", SummaryModelID: "chat-1"}, nil
}

func (s *evalKnowledgeBaseService) DeleteKnowledgeBase(context.Context, string) error {
	return nil
}

type evalKnowledgeService struct {
	interfaces.KnowledgeService
}

func (s *evalKnowledgeService) CreateKnowledgeFromPassageSync(
	context.Context,
	string,
	[]string,
	string,
) (*types.Knowledge, error) {
	return &types.Knowledge{ID: "knowledge-1"}, nil
}

func (s *evalKnowledgeService) DeleteKnowledge(context.Context, string) error {
	return nil
}

type evalSessionService struct {
	interfaces.SessionService
}

func (s *evalSessionService) KnowledgeQAByEvent(
	context.Context,
	*types.ChatManage,
	[]types.EventType,
) error {
	return nil
}

type evalDatasetService struct {
	pairs []*types.QAPair
	err   error
}

func (d *evalDatasetService) GetDatasetByID(context.Context, string) ([]*types.QAPair, error) {
	return d.pairs, d.err
}

func newEvaluationPersistTestService(
	repo interfaces.EvaluationRunRepository,
	models *evalModelService,
	dataset *evalDatasetService,
) *EvaluationService {
	cfg := &config.Config{
		Conversation: &config.ConversationConfig{
			MaxRounds:           3,
			KeywordThreshold:    0.2,
			EmbeddingTopK:       10,
			VectorThreshold:     0.3,
			RerankTopK:          5,
			RerankThreshold:     0.4,
			FallbackResponse:    "fallback",
			RewritePromptSystem: "rewrite-system",
			RewritePromptUser:   "rewrite-user",
			Summary:             &config.SummaryConfig{MaxTokens: 512, Temperature: 0.7},
		},
	}
	return &EvaluationService{
		config:                  cfg,
		dataset:                 dataset,
		knowledgeBaseService:    &evalKnowledgeBaseService{},
		knowledgeService:        &evalKnowledgeService{},
		sessionService:          &evalSessionService{},
		modelService:            models,
		evaluationRunRepository: repo,
	}
}

func evaluationPersistCtx(tenantID uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

func evalPersistModels() *evalModelService {
	return &evalModelService{
		modelsByID: map[string]*types.Model{
			"embed-1": {
				ID:   "embed-1",
				Name: "Embed Model",
				Type: types.ModelTypeEmbedding,
				Parameters: types.ModelParameters{
					BaseURL:  "http://embed.example",
					APIKey:   "embed-secret",
					Provider: "openai",
				},
			},
			"chat-1": {
				ID:   "chat-1",
				Name: "Chat Model",
				Type: types.ModelTypeKnowledgeQA,
				Parameters: types.ModelParameters{
					BaseURL:  "http://chat.example",
					APIKey:   "chat-secret",
					Provider: "zhipu",
				},
			},
			"rerank-1": {
				ID:   "rerank-1",
				Name: "Rerank Model",
				Type: types.ModelTypeRerank,
				Parameters: types.ModelParameters{
					BaseURL:  "http://rerank.example",
					APIKey:   "rerank-secret",
					Provider: "bge",
				},
			},
		},
	}
}

func failingEvalDataset() *evalDatasetService {
	return &evalDatasetService{err: errors.New("dataset boom")}
}

func waitForEvaluationRunStatus(
	t *testing.T,
	repo *fakeEvaluationRunRepository,
	tenantID uint64,
	id string,
	want types.EvaluationStatue,
) *types.EvaluationRun {
	t.Helper()
	ctx := evaluationPersistCtx(tenantID)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		run, err := repo.GetByID(ctx, tenantID, id)
		if err == nil && run != nil && run.Status == want {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for evaluation run %s status %d", id, want)
	return nil
}

func TestEvaluationPersist_CreatesPendingAndSanitizedSnapshot(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	repo.transitionGate = make(chan struct{})
	svc := newEvaluationPersistTestService(repo, evalPersistModels(), failingEvalDataset())
	ctx := evaluationPersistCtx(1)

	detail, err := svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")
	require.NoError(t, err)

	run, err := repo.GetByID(ctx, 1, detail.Task.ID)
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatuePending, run.Status)
	assert.Equal(t, "dataset-1", run.DatasetID)
	assert.Equal(t, "kb-1", run.TemporaryKBID)

	var snapshot types.EvaluationConfigSnapshot
	require.NoError(t, json.Unmarshal(run.ConfigSnapshot, &snapshot))
	assert.Equal(t, "dataset-1", snapshot.Dataset.ID)
	require.Len(t, snapshot.Models, 3)
	assert.Equal(t, "embed-1", snapshot.Models[0].ID)
	assert.Equal(t, "chat-1", snapshot.Models[1].ID)
	assert.Equal(t, "rerank-1", snapshot.Models[2].ID)
	assert.Equal(t, "openai", snapshot.Models[0].Provider)
	assert.Equal(t, "zhipu", snapshot.Models[1].Provider)
	assert.Equal(t, "bge", snapshot.Models[2].Provider)
	assert.NotEmpty(t, snapshot.Version.AppVersion)
	assert.NotEmpty(t, snapshot.Version.GitCommit)

	snapshotJSON := string(run.ConfigSnapshot)
	assert.NotContains(t, snapshotJSON, "embed-secret")
	assert.NotContains(t, snapshotJSON, "chat-secret")
	assert.NotContains(t, snapshotJSON, "rerank-secret")
	assert.NotContains(t, snapshotJSON, "http://embed.example")
	assert.NotContains(t, snapshotJSON, "http://chat.example")
	assert.NotContains(t, snapshotJSON, "http://rerank.example")
	assert.NotContains(t, string(run.Params), "rewrite-system")
	assert.NotContains(t, string(run.Params), "rewrite-user")

	close(repo.transitionGate)
	waitForEvaluationRunStatus(t, repo, 1, detail.Task.ID, types.EvaluationStatueFailed)
}

func TestEvaluationPersist_ConfigHashStableForSameConfig(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	svc := newEvaluationPersistTestService(repo, evalPersistModels(), failingEvalDataset())
	ctx := evaluationPersistCtx(1)

	first, err := svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")
	require.NoError(t, err)
	second, err := svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")
	require.NoError(t, err)

	firstRun, err := repo.GetByID(ctx, 1, first.Task.ID)
	require.NoError(t, err)
	secondRun, err := repo.GetByID(ctx, 1, second.Task.ID)
	require.NoError(t, err)
	assert.Equal(t, firstRun.ConfigHash, secondRun.ConfigHash)
	assert.NotEmpty(t, firstRun.ConfigHash)

	waitForEvaluationRunStatus(t, repo, 1, first.Task.ID, types.EvaluationStatueFailed)
	waitForEvaluationRunStatus(t, repo, 1, second.Task.ID, types.EvaluationStatueFailed)
}

func TestEvaluationPersist_FailureMarksFailedWithErrMsg(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	svc := newEvaluationPersistTestService(repo, evalPersistModels(), failingEvalDataset())
	ctx := evaluationPersistCtx(1)

	detail, err := svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")
	require.NoError(t, err)

	run := waitForEvaluationRunStatus(t, repo, 1, detail.Task.ID, types.EvaluationStatueFailed)
	assert.Contains(t, run.ErrMsg, "dataset boom")
	require.NotNil(t, run.FinishedAt)
}

func TestEvaluationPersist_SuccessCompletesWithMetrics(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	dataset := &evalDatasetService{
		pairs: []*types.QAPair{
			{
				QID:      1,
				Question: "question",
				PIDs:     []int{1},
				Passages: []string{"passage"},
				AID:      1,
				Answer:   "answer",
			},
		},
	}
	svc := newEvaluationPersistTestService(repo, evalPersistModels(), dataset)
	ctx := evaluationPersistCtx(1)

	detail, err := svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")
	require.NoError(t, err)

	run := waitForEvaluationRunStatus(t, repo, 1, detail.Task.ID, types.EvaluationStatueSuccess)
	assert.Equal(t, 1, run.Total)
	assert.Equal(t, 1, run.Finished)
	require.NotNil(t, run.FinishedAt)
	require.NotEmpty(t, run.Metric)

	var snapshot types.EvaluationConfigSnapshot
	require.NoError(t, json.Unmarshal(run.ConfigSnapshot, &snapshot))
	assert.Equal(t, "dataset-1", snapshot.Dataset.ID)
	assert.Equal(t, 1, snapshot.Dataset.SampleCount)
	assert.NotEmpty(t, snapshot.Dataset.SHA256)
}

func TestEvaluationPersist_StartupScanMarksStaleRuns(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	ctx := evaluationPersistCtx(1)
	now := time.Now()
	old := now.Add(-time.Hour)

	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "pending-stale", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatuePending,
		StartTime: now.Add(-2 * time.Hour), CreatedAt: now.Add(-2 * time.Hour),
	}))
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "running-stale", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueRunning,
		StartTime: now.Add(-3 * time.Hour), CreatedAt: now.Add(-3 * time.Hour), HeartbeatAt: &old,
	}))
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "running-fresh", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueRunning,
		StartTime: now.Add(-4 * time.Hour), CreatedAt: now.Add(-4 * time.Hour), HeartbeatAt: &now,
	}))

	affected, err := repo.MarkStaleInterrupted(ctx, now.Add(-45*time.Second))
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	pending, err := repo.GetByID(ctx, 1, "pending-stale")
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatueInterrupted, pending.Status)
	assert.NotNil(t, pending.FinishedAt)

	fresh, err := repo.GetByID(ctx, 1, "running-fresh")
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatueRunning, fresh.Status)
}

func TestEvaluationPersist_EvaluationResultIsTenantScoped(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	svc := &EvaluationService{evaluationRunRepository: repo}
	ctx := evaluationPersistCtx(1)
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "run-1", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueSuccess,
		StartTime: time.Now(), CreatedAt: time.Now(),
	}))

	_, err := svc.EvaluationResult(evaluationPersistCtx(2), "run-1")
	assert.ErrorIs(t, err, ErrEvaluationTaskNotFound)

	detail, err := svc.EvaluationResult(evaluationPersistCtx(1), "run-1")
	require.NoError(t, err)
	require.NotNil(t, detail.Task)
	assert.Equal(t, "run-1", detail.Task.ID)
	assert.Equal(t, types.EvaluationStatueSuccess, detail.Task.Status)
}

func TestEvaluationPersist_ListEvaluationRuns(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	svc := &EvaluationService{evaluationRunRepository: repo}
	ctx := evaluationPersistCtx(1)
	now := time.Now()
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "run-1", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueSuccess,
		StartTime: now, CreatedAt: now.Add(-time.Hour),
	}))
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "run-2", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueFailed,
		StartTime: now, CreatedAt: now.Add(-2 * time.Hour),
	}))

	result, err := svc.ListEvaluationRuns(ctx, nil, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	runs := result.Data.([]*types.EvaluationRun)
	require.Len(t, runs, 2)
	assert.Equal(t, "run-1", runs[0].ID)
	assert.Equal(t, "run-2", runs[1].ID)
}

func TestEvaluationPersist_ListEvaluationRunsFiltersByStatus(t *testing.T) {
	repo := newFakeEvaluationRunRepository()
	svc := &EvaluationService{evaluationRunRepository: repo}
	ctx := evaluationPersistCtx(1)
	now := time.Now()
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "run-success", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueSuccess,
		StartTime: now, CreatedAt: now.Add(-time.Hour),
	}))
	require.NoError(t, repo.Create(ctx, &types.EvaluationRun{
		ID: "run-failed", TenantID: 1, DatasetID: "ds", Status: types.EvaluationStatueFailed,
		StartTime: now, CreatedAt: now.Add(-2 * time.Hour),
	}))

	status := types.EvaluationStatueSuccess
	result, err := svc.ListEvaluationRuns(ctx, &status, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	runs := result.Data.([]*types.EvaluationRun)
	require.Len(t, runs, 1)
	assert.Equal(t, "run-success", runs[0].ID)
}
