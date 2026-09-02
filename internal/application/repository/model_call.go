package repository

import (
	"context"
	"errors"
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

	var records []*types.ModelCallRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	byModel := map[string]*types.ModelCallSummaryItem{}
	for _, record := range records {
		key := record.ModelID
		item, ok := byModel[key]
		if !ok {
			item = &types.ModelCallSummaryItem{
				ModelID:   record.ModelID,
				ModelName: record.ModelName,
				ModelType: record.ModelType,
			}
			byModel[key] = item
		}
		item.Calls++
		if record.Status == string(types.ModelCallStatusSuccess) {
			item.SuccessCount++
		} else {
			item.FailedCount++
		}
		item.PromptTokens += int64(record.PromptTokens)
		item.CompletionTokens += int64(record.CompletionTokens)
		item.TotalTokens += int64(record.TotalTokens)
		item.CacheReadTokens += int64(record.CacheReadTokens)
		item.CacheWriteTokens += int64(record.CacheWriteTokens)
		item.CacheMissTokens += int64(record.CacheMissTokens)
		if record.EstimatedCostUSD != nil {
			if item.EstimatedCostUSD == nil {
				value := 0.0
				item.EstimatedCostUSD = &value
			}
			*item.EstimatedCostUSD += *record.EstimatedCostUSD
		}
	}
	items := make([]*types.ModelCallSummaryItem, 0, len(byModel))
	for _, item := range byModel {
		items = append(items, item)
	}
	return items, nil
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
