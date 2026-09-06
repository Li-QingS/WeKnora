package interfaces

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// EvaluationService defines operations for evaluation tasks
type EvaluationService interface {
	// Evaluation starts a new evaluation task
	Evaluation(ctx context.Context, opts *types.EvaluationOptions) (*types.EvaluationDetail, error)
	// ListAvailableDatasets returns datasets that can be selected for a run.
	ListAvailableDatasets(ctx context.Context) ([]*types.EvaluationDatasetMeta, error)
	// EvaluationResult retrieves evaluation result by task ID
	EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error)
	// ListEvaluationRuns lists tenant-scoped evaluation runs with pagination.
	ListEvaluationRuns(
		ctx context.Context,
		status *types.EvaluationStatue,
		p *types.Pagination,
	) (*types.PageResult, error)
	// DeleteEvaluationRun deletes a terminal evaluation run scoped to tenant.
	DeleteEvaluationRun(ctx context.Context, taskID string) error
}

// EvaluationRunRepository defines persistent storage for evaluation runs.
type EvaluationRunRepository interface {
	// Create persists a newly created evaluation run in pending state.
	Create(ctx context.Context, run *types.EvaluationRun) error
	// GetByID retrieves an evaluation run scoped to tenantID.
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.EvaluationRun, error)
	// DeleteByID deletes an evaluation run scoped to tenantID.
	DeleteByID(ctx context.Context, tenantID uint64, id string) error
	// List returns tenant-scoped runs ordered by creation time descending.
	List(
		ctx context.Context,
		tenantID uint64,
		status *types.EvaluationStatue,
		p *types.Pagination,
	) ([]*types.EvaluationRun, int64, error)
	// ListByType returns tenant-scoped runs for one evaluator.
	ListByType(
		ctx context.Context,
		tenantID uint64,
		evaluationType types.EvaluationType,
		status *types.EvaluationStatue,
		p *types.Pagination,
	) ([]*types.EvaluationRun, int64, error)
	// UpdateProgress updates progress and live metrics; only applies to running runs.
	UpdateProgress(
		ctx context.Context,
		id string,
		finished int,
		total int,
		metric json.RawMessage,
	) error
	// UpdateHeartbeat refreshes the liveness timestamp of a running run.
	UpdateHeartbeat(ctx context.Context, id string, at time.Time) error
	// UpdateStage persists the current Wiki evaluation stage and its progress.
	UpdateStage(
		ctx context.Context,
		id string,
		stage types.EvaluationStage,
		progress types.EvaluationStageProgress,
	) error
	// RecordFailure preserves the first failed stage and diagnostic message.
	RecordFailure(ctx context.Context, id string, failureStage types.EvaluationStage, errMsg string) error
	// SaveWikiResult atomically stores the complete Wiki result and snapshot.
	SaveWikiResult(
		ctx context.Context,
		id string,
		metric json.RawMessage,
		resultDetail json.RawMessage,
		configSnapshot json.RawMessage,
	) error
	// ListCleanupPending returns non-terminal Wiki runs that still own a temporary KB.
	ListCleanupPending(ctx context.Context) ([]*types.EvaluationRun, error)
	// SetDatasetHash records the dataset content hash and sample count in the
	// config snapshot of a running run.
	SetDatasetHash(ctx context.Context, id string, sha256 string, samples int) error
	// TransitionStatus performs a compare-and-swap state transition.
	TransitionStatus(
		ctx context.Context,
		id string,
		from []types.EvaluationStatue,
		to types.EvaluationStatue,
		errMsg string,
	) (bool, error)
	// MarkStaleInterrupted marks pending/running runs whose heartbeat is older
	// than cutoff as interrupted and returns the number of affected rows.
	MarkStaleInterrupted(ctx context.Context, cutoff time.Time) (int64, error)
}

// WikiEvaluationService defines tenant-scoped Wiki evaluation operations.
type WikiEvaluationService interface {
	Start(ctx context.Context, opts *types.WikiEvaluationOptions) (*types.WikiEvaluationDetail, error)
	Get(ctx context.Context, runID string) (*types.WikiEvaluationDetail, error)
	ListDatasets(ctx context.Context) ([]*types.WikiEvaluationDatasetMeta, error)
	ListRuns(
		ctx context.Context,
		status *types.EvaluationStatue,
		p *types.Pagination,
	) (*types.PageResult, error)
	RenderReport(
		ctx context.Context,
		runID string,
		format types.EvaluationReportFormat,
	) ([]byte, string, error)
	DeleteRun(ctx context.Context, runID string) error
}

type WikiGoldLoader interface {
	Load(ctx context.Context, dataset *types.EvaluationDataset) (*types.WikiGold, error)
}

type WikiCorpusImporter interface {
	ImportDocuments(
		ctx context.Context,
		tenantID uint64,
		kbID string,
		docs []types.EvaluationDocument,
		onProgress func(types.EvaluationStageProgress),
	) ([]string, error)
}

type TitledPassageKnowledgeCreator interface {
	CreateKnowledgeFromPassageWithTitle(
		ctx context.Context,
		kbID string,
		title string,
		passages []string,
		channel string,
	) (*types.Knowledge, error)
}

type WikiGenerationMonitor interface {
	WaitUntilStable(
		ctx context.Context,
		tenantID uint64,
		kbID string,
		knowledgeIDs []string,
		onProgress func(types.EvaluationStageProgress),
	) error
}

type WikiPageFreezer interface {
	Freeze(ctx context.Context, tenantID uint64, kbID string) ([]types.WikiEvaluationPage, error)
}

type WikiEvaluationScorer interface {
	ScoreNodes(
		ctx context.Context,
		gold *types.WikiGold,
		pages []types.WikiEvaluationPage,
		embeddingModelID string,
		threshold float64,
	) (*types.WikiNodeScore, error)
	ScoreGraph(
		gold *types.WikiGold,
		pages []types.WikiEvaluationPage,
		nodes *types.WikiNodeScore,
	) *types.WikiGraphScore
}

type WikiEmbeddingProvider interface {
	Embed(ctx context.Context, modelID string, texts []string) ([][]float32, error)
}

type WikiEvaluationReportRenderer interface {
	JSON(detail *types.WikiEvaluationDetail) ([]byte, error)
	Markdown(detail *types.WikiEvaluationDetail) ([]byte, error)
}

// Metrics defines interface for computing evaluation metrics
type Metrics interface {
	// Compute calculates metric score based on input data
	Compute(metricInput *types.MetricInput) float64
}

// EvalHook defines interface for evaluation process hooks
type EvalHook interface {
	// Handle processes evaluation state change
	Handle(ctx context.Context, state types.EvalState, index int, data interface{}) error
}

// DatasetService defines operations for dataset management
type DatasetService interface {
	// GetDatasetByID loads, validates and hashes the named dataset.
	GetDatasetByID(ctx context.Context, datasetID string) (*types.EvaluationDataset, error)
	// ListAvailableDatasets returns all valid dataset directories as metadata.
	ListAvailableDatasets(ctx context.Context) ([]*types.EvaluationDatasetMeta, error)
}
