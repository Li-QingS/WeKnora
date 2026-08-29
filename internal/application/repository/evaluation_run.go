package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// ErrEvaluationRunNotFound is returned when an evaluation run does not exist
// for the requested tenant.
var ErrEvaluationRunNotFound = errors.New("evaluation run not found")

// evaluationRunRepository implements interfaces.EvaluationRunRepository.
type evaluationRunRepository struct {
	db *gorm.DB
}

// NewEvaluationRunRepository constructs a GORM-backed implementation.
func NewEvaluationRunRepository(db *gorm.DB) interfaces.EvaluationRunRepository {
	return &evaluationRunRepository{db: db}
}

func (r *evaluationRunRepository) Create(ctx context.Context, run *types.EvaluationRun) error {
	if run == nil {
		return errors.New("evaluation run: nil run")
	}
	if len(run.Params) == 0 {
		run.Params = json.RawMessage("{}")
	}
	if len(run.ConfigSnapshot) == 0 {
		run.ConfigSnapshot = json.RawMessage("{}")
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now()
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *evaluationRunRepository) GetByID(
	ctx context.Context,
	tenantID uint64,
	id string,
) (*types.EvaluationRun, error) {
	var run types.EvaluationRun
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEvaluationRunNotFound
		}
		return nil, err
	}
	return &run, nil
}

func (r *evaluationRunRepository) List(
	ctx context.Context,
	tenantID uint64,
	status *types.EvaluationStatue,
	p *types.Pagination,
) ([]*types.EvaluationRun, int64, error) {
	if p == nil {
		p = &types.Pagination{}
	}

	var runs []*types.EvaluationRun
	var total int64
	query := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where("tenant_id = ?", tenantID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Order("created_at DESC").
		Offset(p.Offset()).
		Limit(p.Limit()).
		Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

func (r *evaluationRunRepository) UpdateProgress(
	ctx context.Context,
	id string,
	finished int,
	total int,
	metric json.RawMessage,
) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, types.EvaluationStatueRunning).
		Updates(map[string]interface{}{
			"finished":   finished,
			"total":      total,
			"metric":     metric,
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *evaluationRunRepository) UpdateHeartbeat(ctx context.Context, id string, at time.Time) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, types.EvaluationStatueRunning).
		Updates(map[string]interface{}{
			"heartbeat_at": at,
			"updated_at":   time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (r *evaluationRunRepository) SetDatasetHash(
	ctx context.Context,
	id string,
	sha256 string,
	samples int,
) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}

	var run types.EvaluationRun
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, types.EvaluationStatueRunning).
		First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	snapshot := types.EvaluationConfigSnapshot{}
	if len(run.ConfigSnapshot) > 0 {
		if err := json.Unmarshal(run.ConfigSnapshot, &snapshot); err != nil {
			return fmt.Errorf("evaluation run: decode config snapshot: %w", err)
		}
	}
	snapshot.Dataset.ID = run.DatasetID
	snapshot.Dataset.SHA256 = sha256
	snapshot.Dataset.SampleCount = samples
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("evaluation run: encode config snapshot: %w", err)
	}

	res := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, types.EvaluationStatueRunning).
		Updates(map[string]interface{}{
			"config_snapshot": encoded,
			"updated_at":      time.Now(),
		})
	return res.Error
}

func (r *evaluationRunRepository) TransitionStatus(
	ctx context.Context,
	id string,
	from []types.EvaluationStatue,
	to types.EvaluationStatue,
	errMsg string,
) (bool, error) {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return false, err
	}

	updates := map[string]interface{}{
		"status":     to,
		"err_msg":    errMsg,
		"updated_at": time.Now(),
	}
	if isTerminalStatus(to) {
		now := time.Now()
		updates["finished_at"] = &now
	}

	res := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where(
			"id = ? AND tenant_id = ? AND status IN ? AND status NOT IN ?",
			id,
			tenantID,
			from,
			[]types.EvaluationStatue{
				types.EvaluationStatueSuccess,
				types.EvaluationStatueFailed,
				types.EvaluationStatueInterrupted,
			},
		).
		Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *evaluationRunRepository) MarkStaleInterrupted(
	ctx context.Context,
	cutoff time.Time,
) (int64, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&types.EvaluationRun{}).
		Where(
			"status IN ? AND (heartbeat_at IS NULL OR heartbeat_at < ?)",
			[]types.EvaluationStatue{types.EvaluationStatuePending, types.EvaluationStatueRunning},
			cutoff,
		).
		Updates(map[string]interface{}{
			"status":      types.EvaluationStatueInterrupted,
			"finished_at": &now,
			"err_msg":     "interrupted by service restart",
			"updated_at":  now,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func tenantIDFromContext(ctx context.Context) (uint64, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return 0, errors.New("evaluation run: tenant id not found in context")
	}
	return tenantID, nil
}

func isTerminalStatus(status types.EvaluationStatue) bool {
	return status == types.EvaluationStatueSuccess ||
		status == types.EvaluationStatueFailed ||
		status == types.EvaluationStatueInterrupted
}
