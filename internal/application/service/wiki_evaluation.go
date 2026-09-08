package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/buildinfo"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

const (
	wikiEvaluationMaxDuration    = 4 * time.Hour
	wikiEvaluationCleanupTimeout = 2 * time.Minute
)

// ErrInvalidWikiEvaluationParams identifies caller-correctable Wiki
// evaluation configuration errors for the HTTP layer.
var ErrInvalidWikiEvaluationParams = errors.New("invalid wiki evaluation parameters")

type WikiEvaluationService struct {
	datasets   interfaces.DatasetService
	gold       interfaces.WikiGoldLoader
	models     interfaces.ModelService
	knowledge  interfaces.KnowledgeBaseService
	importer   interfaces.WikiCorpusImporter
	monitor    interfaces.WikiGenerationMonitor
	freezer    interfaces.WikiPageFreezer
	scorer     interfaces.WikiEvaluationScorer
	reports    interfaces.WikiEvaluationReportRenderer
	runs       interfaces.EvaluationRunRepository
	modelCalls interfaces.ModelCallRepository
}

func NewWikiEvaluationService(
	datasets interfaces.DatasetService,
	gold interfaces.WikiGoldLoader,
	models interfaces.ModelService,
	knowledge interfaces.KnowledgeBaseService,
	importer interfaces.WikiCorpusImporter,
	monitor interfaces.WikiGenerationMonitor,
	freezer interfaces.WikiPageFreezer,
	scorer interfaces.WikiEvaluationScorer,
	reports interfaces.WikiEvaluationReportRenderer,
	runs interfaces.EvaluationRunRepository,
	modelCalls interfaces.ModelCallRepository,
) *WikiEvaluationService {
	return &WikiEvaluationService{
		datasets: datasets, gold: gold, models: models, knowledge: knowledge,
		importer: importer, monitor: monitor, freezer: freezer, scorer: scorer,
		reports: reports, runs: runs, modelCalls: modelCalls,
	}
}

func (s *WikiEvaluationService) Start(
	ctx context.Context, opts *types.WikiEvaluationOptions,
) (*types.WikiEvaluationDetail, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, fmt.Errorf("wiki evaluation requires a tenant context")
	}
	params, err := normalizeWikiEvaluationOptions(opts)
	if err != nil {
		return nil, err
	}
	dataset, err := s.datasets.GetDatasetByID(ctx, params.DatasetID)
	if err != nil {
		return nil, fmt.Errorf("load wiki evaluation dataset %q: %w", params.DatasetID, err)
	}
	if len(dataset.Documents) == 0 {
		return nil, fmt.Errorf("wiki evaluation dataset %q has no documents", params.DatasetID)
	}
	gold, err := s.gold.Load(ctx, dataset)
	if err != nil {
		return nil, fmt.Errorf("load wiki evaluation Gold: %w", err)
	}
	chat, err := s.resolveModelSnapshot(ctx, params.ChatModelID, types.ModelTypeKnowledgeQA)
	if err != nil {
		return nil, err
	}
	embedding, err := s.resolveModelSnapshot(ctx, params.EmbeddingModelID, types.ModelTypeEmbedding)
	if err != nil {
		return nil, err
	}

	wikiConfig := types.WikiConfig{
		SynthesisModelID:      params.ChatModelID,
		ExtractionGranularity: types.WikiExtractionStandard,
	}
	indexing := types.IndexingStrategy{WikiEnabled: true}
	snapshot := &types.WikiEvaluationConfigSnapshot{
		Dataset: types.DatasetSnapshot{
			ID: params.DatasetID, SHA256: dataset.SHA256, SampleCount: len(dataset.Documents),
		},
		Gold: types.WikiGoldSnapshot{
			SchemaVersion: gold.SchemaVersion, DatasetSHA256: gold.DatasetSHA256,
			ContentSHA256: gold.ContentSHA256, NodeCount: len(gold.Nodes), EdgeCount: len(gold.Edges),
		},
		ChatModel: chat, EmbeddingModel: embedding, Threshold: params.SemanticThreshold,
		Wiki: types.WikiGenerationSnapshot{IndexingStrategy: indexing, WikiConfig: wikiConfig},
		Version: types.VersionSignature{
			AppVersion: buildinfo.Version, GitCommit: buildinfo.CommitID,
			GitDirty: buildinfo.IsGitDirty(), GoVersion: buildinfo.GoVersion,
		},
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode wiki evaluation parameters: %w", err)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode wiki evaluation snapshot: %w", err)
	}
	configHash, err := wikiEvaluationConfigHash(params, snapshot)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	run := &types.EvaluationRun{
		ID: uuid.NewString(), TenantID: tenantID, DatasetID: params.DatasetID,
		Status: types.EvaluationStatuePending, StartTime: now, Total: len(dataset.Documents),
		Params: paramsJSON, ConfigHash: configHash, ConfigSnapshot: snapshotJSON,
		TemporaryKBID: uuid.NewString(), EvaluationType: types.EvaluationTypeWiki,
		Stage: types.EvaluationStageValidating,
	}
	progress, _ := json.Marshal(types.EvaluationStageProgress{
		Current: len(dataset.Documents), Total: len(dataset.Documents), Message: "dataset and models validated",
	})
	run.StageProgress = progress
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create wiki evaluation run: %w", err)
	}
	detail := &types.WikiEvaluationDetail{Run: run, Params: params, ConfigSnapshot: snapshot}

	background := logger.CloneContext(context.WithoutCancel(ctx))
	background, cancel := context.WithTimeout(background, wikiEvaluationMaxDuration)
	go func() {
		defer cancel()
		s.execute(background, run, dataset, gold, snapshot)
	}()
	return detail, nil
}

func (s *WikiEvaluationService) execute(
	ctx context.Context,
	run *types.EvaluationRun,
	dataset *types.EvaluationDataset,
	gold *types.WikiGold,
	snapshot *types.WikiEvaluationConfigSnapshot,
) {
	started, err := s.runs.TransitionStatus(ctx, run.ID,
		[]types.EvaluationStatue{types.EvaluationStatuePending}, types.EvaluationStatueRunning, "")
	if err != nil || !started {
		if err != nil {
			logger.Errorf(ctx, "Failed to start Wiki evaluation %s: %v", run.ID, err)
		}
		return
	}
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go s.runHeartbeat(heartbeatCtx, run.ID)

	stage := types.EvaluationStageCreatingKB
	if err := s.setStage(ctx, run.ID, stage, 0, 1, "creating temporary Wiki knowledge base"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	kb := &types.KnowledgeBase{
		ID: run.TemporaryKBID, Name: "wiki-evaluation-" + run.ID[:8],
		Description: "Temporary knowledge base for Wiki evaluation", IsTemporary: true,
		Type:             types.KnowledgeBaseTypeDocument,
		EmbeddingModelID: snapshot.EmbeddingModel.ID, SummaryModelID: snapshot.ChatModel.ID,
		WikiConfig: &snapshot.Wiki.WikiConfig, IndexingStrategy: snapshot.Wiki.IndexingStrategy,
	}
	if _, err := s.knowledge.CreateKnowledgeBase(ctx, kb); err != nil {
		s.failAndCleanup(ctx, run, stage, fmt.Errorf("create temporary knowledge base: %w", err))
		return
	}

	generationCtx := types.WithRequestGroupID(ctx, run.ID+":generation")
	stage = types.EvaluationStageImporting
	if err := s.setStage(ctx, run.ID, stage, 0, len(dataset.Documents), "importing evaluation corpus"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	knowledgeIDs, err := s.importer.ImportDocuments(
		generationCtx, run.TenantID, run.TemporaryKBID, dataset.Documents,
		func(progress types.EvaluationStageProgress) {
			if updateErr := s.runs.UpdateStage(generationCtx, run.ID, stage, progress); updateErr != nil {
				logger.Warnf(generationCtx, "Update Wiki evaluation import progress: %v", updateErr)
			}
		},
	)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}

	stage = types.EvaluationStageGenerating
	if err := s.setStage(ctx, run.ID, stage, 0, len(knowledgeIDs), "waiting for Wiki generation"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	if err := s.monitor.WaitUntilStable(
		generationCtx, run.TenantID, run.TemporaryKBID, knowledgeIDs,
		func(progress types.EvaluationStageProgress) {
			if updateErr := s.runs.UpdateStage(generationCtx, run.ID, stage, progress); updateErr != nil {
				logger.Warnf(generationCtx, "Update Wiki evaluation generation progress: %v", updateErr)
			}
		},
	); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	pages, err := s.freezer.Freeze(ctx, run.TenantID, run.TemporaryKBID)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}

	scoringCtx := types.WithRequestGroupID(ctx, run.ID+":scoring")
	stage = types.EvaluationStageScoringNodes
	if err := s.setStage(ctx, run.ID, stage, 0, len(gold.Nodes), "matching entities and concepts"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	nodes, err := s.scorer.ScoreNodes(scoringCtx, gold, pages, snapshot.EmbeddingModel.ID, snapshot.Threshold)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}

	stage = types.EvaluationStageScoringGraph
	if err := s.setStage(ctx, run.ID, stage, 0, len(gold.Edges), "scoring directed graph structure"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	graph := s.scorer.ScoreGraph(gold, pages, nodes)
	generationCost := s.costRollup(ctx, run.TenantID, run.ID+":generation")
	scoringCost := s.costRollup(ctx, run.TenantID, run.ID+":scoring")
	metric := types.WikiEvaluationMetric{
		Entity: nodes.Entity, Concept: nodes.Concept, Overall: nodes.Overall, Graph: graph.Metric,
		GenerationCost: generationCost, ScoringCost: scoringCost,
	}
	result := &types.WikiEvaluationResult{
		Metric: metric, NodeMatches: nodes.Matches, CorrectEdges: graph.CorrectEdges,
		MissingEdges: graph.MissingEdges, ExtraEdges: graph.ExtraEdges,
		UnscoredEdges: graph.UnscoredEdges, FrozenPages: pages,
	}

	stage = types.EvaluationStageSavingReport
	if err := s.setStage(ctx, run.ID, stage, 0, 1, "saving evaluation result"); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	metricJSON, err := json.Marshal(metric)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}
	if err := s.runs.SaveWikiResult(ctx, run.ID, metricJSON, resultJSON, snapshotJSON); err != nil {
		s.failAndCleanup(ctx, run, stage, err)
		return
	}

	if err := s.cleanup(ctx, run); err != nil {
		logger.Errorf(ctx, "Wiki evaluation %s result saved but cleanup failed: %v", run.ID, err)
		return
	}
	_ = s.setStage(ctx, run.ID, types.EvaluationStageCompleted, 1, 1, "completed")
	if _, err := s.runs.TransitionStatus(ctx, run.ID,
		[]types.EvaluationStatue{types.EvaluationStatueRunning}, types.EvaluationStatueSuccess, ""); err != nil {
		logger.Errorf(ctx, "Complete Wiki evaluation %s: %v", run.ID, err)
	}
}

func (s *WikiEvaluationService) failAndCleanup(
	ctx context.Context, run *types.EvaluationRun, stage types.EvaluationStage, cause error,
) {
	// The run context is also the four-hour execution deadline. Once that
	// deadline fires, status persistence and cleanup still need a live context
	// so the run cannot remain stuck in running forever.
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wikiEvaluationCleanupTimeout)
	defer cancel()
	message := cause.Error()
	if err := s.runs.RecordFailure(finalizeCtx, run.ID, stage, message); err != nil {
		logger.Errorf(finalizeCtx, "Record Wiki evaluation failure %s: %v", run.ID, err)
	}
	if cleanupErr := s.cleanup(finalizeCtx, run); cleanupErr != nil {
		message += "; cleanup: " + cleanupErr.Error()
		_ = s.runs.RecordFailure(finalizeCtx, run.ID, stage, message)
		return
	}
	if _, err := s.runs.TransitionStatus(finalizeCtx, run.ID,
		[]types.EvaluationStatue{types.EvaluationStatueRunning}, types.EvaluationStatueFailed, message); err != nil {
		logger.Errorf(finalizeCtx, "Finalize Wiki evaluation failure %s: %v", run.ID, err)
	}
}

func (s *WikiEvaluationService) cleanup(ctx context.Context, run *types.EvaluationRun) error {
	_ = s.setStage(ctx, run.ID, types.EvaluationStageCleaningUp, 0, 1, "deleting temporary knowledge base")
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wikiEvaluationCleanupTimeout)
	defer cancel()
	if err := s.knowledge.DeleteKnowledgeBase(cleanupCtx, run.TemporaryKBID); err != nil &&
		!errors.Is(err, apprepo.ErrKnowledgeBaseNotFound) {
		return fmt.Errorf("delete temporary knowledge base %s: %w", run.TemporaryKBID, err)
	}
	return nil
}

func (s *WikiEvaluationService) setStage(
	ctx context.Context, runID string, stage types.EvaluationStage, current int, total int, message string,
) error {
	return s.runs.UpdateStage(ctx, runID, stage, types.EvaluationStageProgress{
		Current: current, Total: total, Message: message,
	})
}

func (s *WikiEvaluationService) runHeartbeat(ctx context.Context, runID string) {
	ticker := time.NewTicker(evaluationHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.runs.UpdateHeartbeat(ctx, runID, now); err != nil {
				logger.Warnf(ctx, "Update Wiki evaluation heartbeat: %v", err)
			}
		}
	}
}

func (s *WikiEvaluationService) Get(ctx context.Context, runID string) (*types.WikiEvaluationDetail, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("wiki evaluation requires a tenant context")
	}
	run, err := s.runs.GetByID(ctx, tenantID, runID)
	if err != nil || run.EvaluationType != types.EvaluationTypeWiki {
		if errors.Is(err, apprepo.ErrEvaluationRunNotFound) || (err == nil && run.EvaluationType != types.EvaluationTypeWiki) {
			return nil, ErrEvaluationTaskNotFound
		}
		return nil, err
	}
	detail := &types.WikiEvaluationDetail{Run: run}
	if err := decodeWikiEvaluationJSON(run.Params, &detail.Params); err != nil {
		return nil, fmt.Errorf("decode Wiki evaluation params: %w", err)
	}
	if err := decodeWikiEvaluationJSON(run.ConfigSnapshot, &detail.ConfigSnapshot); err != nil {
		return nil, fmt.Errorf("decode Wiki evaluation config: %w", err)
	}
	if err := decodeWikiEvaluationJSON(run.Metric, &detail.Metric); err != nil {
		return nil, fmt.Errorf("decode Wiki evaluation metric: %w", err)
	}
	if err := decodeWikiEvaluationJSON(run.ResultDetail, &detail.Result); err != nil {
		return nil, fmt.Errorf("decode Wiki evaluation result: %w", err)
	}
	return detail, nil
}

func (s *WikiEvaluationService) ListRuns(
	ctx context.Context, status *types.EvaluationStatue, pagination *types.Pagination,
) (*types.PageResult, error) {
	if pagination == nil {
		pagination = &types.Pagination{}
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	runs, total, err := s.runs.ListByType(ctx, tenantID, types.EvaluationTypeWiki, status, pagination)
	if err != nil {
		return nil, err
	}
	return types.NewPageResult(total, pagination, runs), nil
}

func (s *WikiEvaluationService) ListDatasets(ctx context.Context) ([]*types.WikiEvaluationDatasetMeta, error) {
	metas, err := s.datasets.ListAvailableDatasets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*types.WikiEvaluationDatasetMeta, 0, len(metas))
	for _, meta := range metas {
		if meta == nil {
			continue
		}
		dataset, loadErr := s.datasets.GetDatasetByID(ctx, meta.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		gold, loadErr := s.gold.Load(ctx, dataset)
		if errors.Is(loadErr, ErrWikiGoldNotFound) {
			continue
		}
		if loadErr != nil {
			return nil, loadErr
		}
		result = append(result, &types.WikiEvaluationDatasetMeta{
			ID: dataset.ID, SHA256: dataset.SHA256, DocumentCount: len(dataset.Documents),
			Gold: types.WikiGoldSnapshot{
				SchemaVersion: gold.SchemaVersion, DatasetSHA256: gold.DatasetSHA256,
				ContentSHA256: gold.ContentSHA256, NodeCount: len(gold.Nodes), EdgeCount: len(gold.Edges),
			},
		})
	}
	return result, nil
}

func (s *WikiEvaluationService) RenderReport(
	ctx context.Context, runID string, format types.EvaluationReportFormat,
) ([]byte, string, error) {
	if !format.IsValid() {
		return nil, "", fmt.Errorf("unsupported Wiki evaluation report format %q", format)
	}
	detail, err := s.Get(ctx, runID)
	if err != nil {
		return nil, "", err
	}
	if format == types.EvaluationReportJSON {
		data, err := s.reports.JSON(detail)
		return data, "application/json", err
	}
	data, err := s.reports.Markdown(detail)
	return data, "text/markdown; charset=utf-8", err
}

func (s *WikiEvaluationService) DeleteRun(ctx context.Context, runID string) error {
	detail, err := s.Get(ctx, runID)
	if err != nil {
		return err
	}
	if detail.Run.Status == types.EvaluationStatuePending || detail.Run.Status == types.EvaluationStatueRunning {
		return ErrEvaluationRunActive
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wikiEvaluationCleanupTimeout)
	defer cancel()
	if err := s.knowledge.DeleteKnowledgeBase(cleanupCtx, detail.Run.TemporaryKBID); err != nil &&
		!errors.Is(err, apprepo.ErrKnowledgeBaseNotFound) {
		return err
	}
	return s.runs.DeleteByID(ctx, detail.Run.TenantID, runID)
}

func (s *WikiEvaluationService) validateDependencies() error {
	if s == nil || s.datasets == nil || s.gold == nil || s.models == nil || s.knowledge == nil ||
		s.importer == nil || s.monitor == nil || s.freezer == nil || s.scorer == nil ||
		s.reports == nil || s.runs == nil {
		return fmt.Errorf("wiki evaluation service is not configured")
	}
	return nil
}

func normalizeWikiEvaluationOptions(opts *types.WikiEvaluationOptions) (*types.WikiEvaluationOptions, error) {
	if opts == nil {
		return nil, fmt.Errorf("%w: options are required", ErrInvalidWikiEvaluationParams)
	}
	copy := *opts
	copy.DatasetID = strings.TrimSpace(copy.DatasetID)
	copy.ChatModelID = strings.TrimSpace(copy.ChatModelID)
	copy.EmbeddingModelID = strings.TrimSpace(copy.EmbeddingModelID)
	if copy.DatasetID == "" || copy.ChatModelID == "" || copy.EmbeddingModelID == "" {
		return nil, fmt.Errorf("%w: dataset_id, chat_id, and embedding_id are required", ErrInvalidWikiEvaluationParams)
	}
	if copy.SemanticThreshold == 0 && !copy.SemanticThresholdProvided {
		copy.SemanticThreshold = types.DefaultWikiSemanticThreshold
	}
	if copy.SemanticThreshold < 0 || copy.SemanticThreshold > 1 {
		return nil, fmt.Errorf("%w: semantic_threshold must be in [0,1]", ErrInvalidWikiEvaluationParams)
	}
	return &copy, nil
}

func (s *WikiEvaluationService) resolveModelSnapshot(
	ctx context.Context, id string, expected types.ModelType,
) (types.ModelSnapshot, error) {
	model, err := s.models.GetModelByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrModelNotFound) {
			return types.ModelSnapshot{}, fmt.Errorf("%w: model %q was not found", ErrInvalidWikiEvaluationParams, id)
		}
		return types.ModelSnapshot{}, fmt.Errorf("resolve model %q: %w", id, err)
	}
	if model == nil || model.Type != expected {
		return types.ModelSnapshot{}, fmt.Errorf("%w: model %q is not a %s model", ErrInvalidWikiEvaluationParams, id, expected)
	}
	if model.Status != "" && model.Status != types.ModelStatusActive {
		return types.ModelSnapshot{}, fmt.Errorf("%w: model %q is not active", ErrInvalidWikiEvaluationParams, id)
	}
	return types.ModelSnapshot{
		ID: model.ID, Name: model.Name, Provider: model.Parameters.Provider, Type: string(model.Type),
	}, nil
}

func wikiEvaluationConfigHash(
	params *types.WikiEvaluationOptions, snapshot *types.WikiEvaluationConfigSnapshot,
) (string, error) {
	data, err := json.Marshal(struct {
		Params   *types.WikiEvaluationOptions        `json:"params"`
		Snapshot *types.WikiEvaluationConfigSnapshot `json:"snapshot"`
	}{params, snapshot})
	if err != nil {
		return "", fmt.Errorf("encode Wiki evaluation config hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func decodeWikiEvaluationJSON[T any](data json.RawMessage, target **T) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	value := new(T)
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	*target = value
	return nil
}

func (s *WikiEvaluationService) costRollup(
	ctx context.Context, tenantID uint64, requestGroupID string,
) *types.EvaluationCostMetrics {
	if s.modelCalls == nil {
		return nil
	}
	var rollup *types.ModelCallRollup
	var err error
	if repository, ok := s.modelCalls.(requestGroupRollupRepository); ok {
		rollup, err = repository.RollupRequestGroup(ctx, tenantID, requestGroupID)
	} else {
		rollup = &types.ModelCallRollup{}
		const pageSize = 10000
		for page := 1; ; page++ {
			rows, total, listErr := s.modelCalls.List(ctx, tenantID,
				&types.ModelCallFilter{RequestGroupID: requestGroupID},
				&types.Pagination{Page: page, PageSize: pageSize})
			if listErr != nil {
				err = listErr
				break
			}
			for _, row := range rows {
				rollup.Calls++
				rollup.PromptTokens += int64(row.PromptTokens)
				rollup.CompletionTokens += int64(row.CompletionTokens)
				rollup.TotalTokens += int64(row.TotalTokens)
				rollup.CacheReadTokens += int64(row.CacheReadTokens)
				rollup.CacheWriteTokens += int64(row.CacheWriteTokens)
				if row.EstimatedCostUSD != nil {
					if rollup.EstimatedCostUSD == nil {
						zero := 0.0
						rollup.EstimatedCostUSD = &zero
					}
					*rollup.EstimatedCostUSD += *row.EstimatedCostUSD
				}
			}
			if len(rows) < pageSize || int64(page*pageSize) >= total {
				break
			}
		}
	}
	if err != nil {
		logger.Warnf(ctx, "Roll up Wiki evaluation model calls for %s: %v", requestGroupID, err)
		return nil
	}
	if rollup == nil {
		return &types.EvaluationCostMetrics{}
	}
	return rollupToCostMetrics(rollup)
}
