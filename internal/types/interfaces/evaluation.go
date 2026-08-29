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
	Evaluation(ctx context.Context, datasetID string, knowledgeBaseID string,
		chatModelID string, rerankModelID string,
	) (*types.EvaluationDetail, error)
	// EvaluationResult retrieves evaluation result by task ID
	EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error)
	// ListEvaluationRuns lists tenant-scoped evaluation runs with pagination.
	ListEvaluationRuns(
		ctx context.Context,
		status *types.EvaluationStatue,
		p *types.Pagination,
	) (*types.PageResult, error)
}

// EvaluationRunRepository defines persistent storage for evaluation runs.
type EvaluationRunRepository interface {
	// Create persists a newly created evaluation run in pending state.
	Create(ctx context.Context, run *types.EvaluationRun) error
	// GetByID retrieves an evaluation run scoped to tenantID.
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.EvaluationRun, error)
	// List returns tenant-scoped runs ordered by creation time descending.
	List(
		ctx context.Context,
		tenantID uint64,
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
	// GetDatasetByID retrieves QA pairs from dataset by ID
	GetDatasetByID(ctx context.Context, datasetID string) ([]*types.QAPair, error)
}
