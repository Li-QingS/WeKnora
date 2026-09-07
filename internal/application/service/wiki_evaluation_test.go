package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wikiGoldLoaderFake struct{ gold *types.WikiGold }

func (f *wikiGoldLoaderFake) Load(context.Context, *types.EvaluationDataset) (*types.WikiGold, error) {
	return f.gold, nil
}

type wikiImporterFake struct{ err error }

func (f *wikiImporterFake) ImportDocuments(
	ctx context.Context, _ uint64, _ string, docs []types.EvaluationDocument,
	onProgress func(types.EvaluationStageProgress),
) ([]string, error) {
	if types.RequestGroupIDFromContext(ctx) == "" {
		return nil, errors.New("missing generation request group")
	}
	if f.err != nil {
		return nil, f.err
	}
	ids := make([]string, len(docs))
	for index := range docs {
		ids[index] = "knowledge-" + docs[index].Title
	}
	if onProgress != nil {
		onProgress(types.EvaluationStageProgress{Current: len(docs), Total: len(docs)})
	}
	return ids, nil
}

type wikiMonitorFake struct{}

func (f *wikiMonitorFake) WaitUntilStable(
	ctx context.Context, _ uint64, _ string, ids []string,
	onProgress func(types.EvaluationStageProgress),
) error {
	if types.RequestGroupIDFromContext(ctx) == "" {
		return errors.New("missing generation request group")
	}
	if onProgress != nil {
		onProgress(types.EvaluationStageProgress{Current: len(ids), Total: len(ids)})
	}
	return nil
}

type wikiFreezerFake struct{ pages []types.WikiEvaluationPage }

func (f *wikiFreezerFake) Freeze(context.Context, uint64, string) ([]types.WikiEvaluationPage, error) {
	return f.pages, nil
}

type wikiScorerFake struct{}

func (f *wikiScorerFake) ScoreNodes(
	ctx context.Context, _ *types.WikiGold, _ []types.WikiEvaluationPage, _ string, _ float64,
) (*types.WikiNodeScore, error) {
	if types.RequestGroupIDFromContext(ctx) == "" {
		return nil, errors.New("missing scoring request group")
	}
	metric := types.WikiNodeMetric{GoldTotal: 1, ExactMatched: 1, Coverage: 1}
	return &types.WikiNodeScore{
		Entity: metric, Overall: metric,
		Matches:      []types.WikiNodeMatch{{GoldNodeID: "entity:a", GoldType: types.WikiPageTypeEntity, GoldName: "A", Method: "exact", PageSlug: "entity/a"}},
		PageToGoldID: map[string]string{"entity/a": "entity:a"},
	}, nil
}

func (f *wikiScorerFake) ScoreGraph(
	*types.WikiGold, []types.WikiEvaluationPage, *types.WikiNodeScore,
) *types.WikiGraphScore {
	return &types.WikiGraphScore{Metric: types.WikiGraphMetric{Scorable: false, Note: "no induced Gold edges"}}
}

type wikiKnowledgeBaseFake struct {
	interfaces.KnowledgeBaseService
	mu      sync.Mutex
	created *types.KnowledgeBase
	deleted []string
}

func (f *wikiKnowledgeBaseFake) CreateKnowledgeBase(
	_ context.Context, kb *types.KnowledgeBase,
) (*types.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := *kb
	f.created = &copy
	return kb, nil
}

func (f *wikiKnowledgeBaseFake) DeleteKnowledgeBase(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.created == nil {
		return apprepo.ErrKnowledgeBaseNotFound
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func newWikiCoordinatorTestService(
	runs interfaces.EvaluationRunRepository, importer interfaces.WikiCorpusImporter, kb *wikiKnowledgeBaseFake,
) *WikiEvaluationService {
	dataset := &types.EvaluationDataset{
		ID: "enterprise_rag", SHA256: "dataset-sha", SampleCount: 1,
		Documents: []types.EvaluationDocument{{ID: 1, Title: "enterprise_rag-corpus-1", Content: "content"}},
	}
	models := &evalModelService{modelsByID: map[string]*types.Model{
		"chat-1":  {ID: "chat-1", Name: "Chat", Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusActive},
		"embed-1": {ID: "embed-1", Name: "Embed", Type: types.ModelTypeEmbedding, Status: types.ModelStatusActive},
	}}
	gold := &types.WikiGold{
		SchemaVersion: "1", DatasetID: "enterprise_rag", DatasetSHA256: "dataset-sha", ContentSHA256: "gold-sha",
		Nodes: []types.WikiGoldNode{{ID: "entity:a", Type: types.WikiPageTypeEntity, Name: "A"}},
	}
	return NewWikiEvaluationService(
		&evalDatasetService{dataset: dataset}, &wikiGoldLoaderFake{gold}, models, kb,
		importer, &wikiMonitorFake{}, &wikiFreezerFake{pages: []types.WikiEvaluationPage{{
			ID: "page-a", Slug: "entity/a", Title: "A", Type: types.WikiPageTypeEntity,
		}}}, &wikiScorerFake{}, NewWikiEvaluationReportRenderer(), runs, nil,
	)
}

func waitWikiRunTerminal(t *testing.T, repo *fakeEvaluationRunRepository, tenantID uint64, id string) *types.EvaluationRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, err := repo.GetByID(context.Background(), tenantID, id)
		require.NoError(t, err)
		if run.Status >= types.EvaluationStatueSuccess {
			return run
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Wiki evaluation did not reach a terminal state")
	return nil
}

func TestWikiEvaluationSuccessAndPersistedReport(t *testing.T) {
	runs := newFakeEvaluationRunRepository()
	kb := &wikiKnowledgeBaseFake{}
	svc := newWikiCoordinatorTestService(runs, &wikiImporterFake{}, kb)
	ctx := evaluationPersistCtx(7)
	detail, err := svc.Start(ctx, &types.WikiEvaluationOptions{
		DatasetID: "enterprise_rag", ChatModelID: "chat-1", EmbeddingModelID: "embed-1", SemanticThreshold: 0.8,
	})
	require.NoError(t, err)
	require.Equal(t, types.EvaluationStatuePending, detail.Run.Status)
	run := waitWikiRunTerminal(t, runs, 7, detail.Run.ID)
	require.Equal(t, types.EvaluationStatueSuccess, run.Status)
	require.Equal(t, types.EvaluationStageCompleted, run.Stage)
	require.NotEmpty(t, run.Metric)
	require.NotEmpty(t, run.ResultDetail)
	require.Equal(t, run.TemporaryKBID, kb.created.ID)
	require.Equal(t, []string{run.TemporaryKBID}, kb.deleted)

	persisted, err := svc.Get(ctx, run.ID)
	require.NoError(t, err)
	require.InDelta(t, 1, persisted.Metric.Overall.Coverage, 1e-9)
	require.Len(t, persisted.Result.NodeMatches, 1)
	jsonReport, mime, err := svc.RenderReport(ctx, run.ID, types.EvaluationReportJSON)
	require.NoError(t, err)
	require.Equal(t, "application/json", mime)
	require.Contains(t, string(jsonReport), `"coverage": 1`)

	_, err = svc.Get(evaluationPersistCtx(8), run.ID)
	require.ErrorIs(t, err, ErrEvaluationTaskNotFound)
}

func TestWikiEvaluationFailureKeepsNoQualityMetricAndCleansUp(t *testing.T) {
	runs := newFakeEvaluationRunRepository()
	kb := &wikiKnowledgeBaseFake{}
	svc := newWikiCoordinatorTestService(runs, &wikiImporterFake{err: errors.New("import exploded")}, kb)
	ctx := evaluationPersistCtx(7)
	detail, err := svc.Start(ctx, &types.WikiEvaluationOptions{
		DatasetID: "enterprise_rag", ChatModelID: "chat-1", EmbeddingModelID: "embed-1",
	})
	require.NoError(t, err)
	run := waitWikiRunTerminal(t, runs, 7, detail.Run.ID)
	require.Equal(t, types.EvaluationStatueFailed, run.Status)
	require.Equal(t, types.EvaluationStageImporting, run.FailureStage)
	require.Contains(t, run.ErrMsg, "import exploded")
	require.Empty(t, run.Metric)
	require.Empty(t, run.ResultDetail)
	require.Equal(t, []string{run.TemporaryKBID}, kb.deleted)
}

func TestWikiEvaluationValidatesBeforeCreatingRun(t *testing.T) {
	runs := newFakeEvaluationRunRepository()
	svc := newWikiCoordinatorTestService(runs, &wikiImporterFake{}, &wikiKnowledgeBaseFake{})
	_, err := svc.Start(evaluationPersistCtx(7), &types.WikiEvaluationOptions{
		DatasetID: "enterprise_rag", ChatModelID: "embed-1", EmbeddingModelID: "embed-1",
	})
	require.ErrorContains(t, err, "not a KnowledgeQA model")
	require.Empty(t, runs.runs)
}

func TestNormalizeWikiEvaluationOptionsDistinguishesOmittedAndZeroThreshold(t *testing.T) {
	var omitted types.WikiEvaluationOptions
	require.NoError(t, json.Unmarshal([]byte(`{
		"dataset_id":"enterprise_rag","chat_id":"chat-1","embedding_id":"embed-1"
	}`), &omitted))
	normalized, err := normalizeWikiEvaluationOptions(&omitted)
	require.NoError(t, err)
	require.Equal(t, types.DefaultWikiSemanticThreshold, normalized.SemanticThreshold)

	var explicitZero types.WikiEvaluationOptions
	require.NoError(t, json.Unmarshal([]byte(`{
		"dataset_id":"enterprise_rag","chat_id":"chat-1","embedding_id":"embed-1","semantic_threshold":0
	}`), &explicitZero))
	normalized, err = normalizeWikiEvaluationOptions(&explicitZero)
	require.NoError(t, err)
	require.Zero(t, normalized.SemanticThreshold)
}
