package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// modelCallRepository implements interfaces.ModelCallRepository.
type modelCallRepository struct {
	db *gorm.DB
}

// NewModelCallRepository constructs a GORM-backed implementation.
func NewModelCallRepository(db *gorm.DB) interfaces.ModelCallRepository {
	return &modelCallRepository{db: db}
}

func (r *modelCallRepository) Create(ctx context.Context, record *types.ModelCallRecord) error {
	if record == nil {
		return errors.New("model call: nil record")
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *modelCallRepository) List(
	ctx context.Context,
	tenantID uint64,
	filter *types.ModelCallFilter,
	p *types.Pagination,
) ([]*types.ModelCallRecord, int64, error) {
	if p == nil {
		p = &types.Pagination{}
	}
	query := r.db.WithContext(ctx).Model(&types.ModelCallRecord{}).
		Where("tenant_id = ?", tenantID)
	query = applyModelCallFilters(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []*types.ModelCallRecord
	if err := query.
		Order("created_at DESC").
		Offset(p.Offset()).
		Limit(p.Limit()).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *modelCallRepository) Summary(
	ctx context.Context,
	tenantID uint64,
	filter *types.ModelCallFilter,
) ([]*types.ModelCallSummaryItem, error) {
	query := r.db.WithContext(ctx).Model(&types.ModelCallRecord{}).
		Where("tenant_id = ?", tenantID)
	query = applyModelCallFilters(query, filter)

	var items []*types.ModelCallSummaryItem
	if err := query.
		Select(`model_id,
			MAX(model_name) AS model_name,
			MAX(model_type) AS model_type,
			COUNT(*) AS calls,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed_count,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS completion_tokens,
			SUM(total_tokens) AS total_tokens,
			SUM(cache_read_tokens) AS cache_read_tokens,
			SUM(cache_write_tokens) AS cache_write_tokens,
			SUM(cache_miss_tokens) AS cache_miss_tokens,
			SUM(estimated_cost_usd) AS estimated_cost_usd`).
		Group("model_id").
		Scan(&items).Error; err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ModelType != items[j].ModelType {
			return items[i].ModelType < items[j].ModelType
		}
		if items[i].ModelName != items[j].ModelName {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].ModelID < items[j].ModelID
	})
	return items, nil
}

// RollupRequestGroup returns one aggregate row for every model call attributed
// to a request group. Used by evaluation cost/latency reporting.
func (r *modelCallRepository) RollupRequestGroup(
	ctx context.Context,
	tenantID uint64,
	requestGroupID string,
) (*types.ModelCallRollup, error) {
	var rollup types.ModelCallRollup
	err := r.db.WithContext(ctx).Model(&types.ModelCallRecord{}).
		Select(`COUNT(*) AS calls,
			COALESCE(SUM(duration_ms), 0) AS duration_ms,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens,
			COALESCE(SUM(cache_miss_tokens), 0) AS cache_miss_tokens,
			SUM(estimated_cost_usd) AS estimated_cost_usd`).
		Where("tenant_id = ? AND request_group_id = ?", tenantID, requestGroupID).
		Scan(&rollup).Error
	if err != nil {
		return nil, err
	}
	if rollup.Calls == 0 {
		return nil, nil
	}
	return &rollup, nil
}

func applyModelCallFilters(query *gorm.DB, filter *types.ModelCallFilter) *gorm.DB {
	if filter == nil {
		return query
	}
	if filter.ModelID != "" {
		query = query.Where("model_id = ?", filter.ModelID)
	}
	if filter.ModelType != "" {
		query = query.Where("model_type = ?", filter.ModelType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.RequestGroupID != "" {
		query = query.Where("request_group_id = ?", filter.RequestGroupID)
	}
	if filter.From != nil {
		query = query.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("created_at <= ?", *filter.To)
	}
	return query
}

// modelPriceRepository implements interfaces.ModelPriceRepository.
type modelPriceRepository struct {
	db *gorm.DB
}

// NewModelPriceRepository constructs a GORM-backed implementation.
func NewModelPriceRepository(db *gorm.DB) interfaces.ModelPriceRepository {
	return &modelPriceRepository{db: db}
}

func (r *modelPriceRepository) Get(
	ctx context.Context,
	tenantID uint64,
	modelID string,
) (*types.ModelPrice, error) {
	var price types.ModelPrice
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND model_id = ?", tenantID, modelID).
		First(&price).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &price, nil
}

func (r *modelPriceRepository) Upsert(ctx context.Context, price *types.ModelPrice) error {
	if price == nil {
		return errors.New("model price: nil price")
	}
	if price.ID == "" {
		price.ID = uuid.NewString()
	}
	now := time.Now()
	if price.CreatedAt.IsZero() {
		price.CreatedAt = now
	}
	price.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "model_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"input_price_per_million",
			"output_price_per_million",
			"cache_read_price_per_million",
			"cache_write_price_per_million",
			"unit_type",
			"unit_price",
			"currency",
			"updated_by",
			"updated_at",
		}),
	}).Create(price).Error
}

func (r *modelPriceRepository) List(
	ctx context.Context,
	tenantID uint64,
) ([]*types.ModelPrice, error) {
	var prices []*types.ModelPrice
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("model_id ASC").
		Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}
