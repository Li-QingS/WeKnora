package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// ModelCallRepository stores and queries model call ledger records.
type ModelCallRepository interface {
	Create(ctx context.Context, record *types.ModelCallRecord) error
	List(
		ctx context.Context,
		tenantID uint64,
		filter *types.ModelCallFilter,
		p *types.Pagination,
	) ([]*types.ModelCallRecord, int64, error)
	Summary(
		ctx context.Context,
		tenantID uint64,
		filter *types.ModelCallFilter,
	) ([]*types.ModelCallSummaryItem, error)
}

// ModelPriceRepository stores per-tenant model prices.
type ModelPriceRepository interface {
	Get(ctx context.Context, tenantID uint64, modelID string) (*types.ModelPrice, error)
	Upsert(ctx context.Context, price *types.ModelPrice) error
	List(ctx context.Context, tenantID uint64) ([]*types.ModelPrice, error)
}

// ModelCallService exposes tenant-scoped ledger queries and price management.
type ModelCallService interface {
	List(
		ctx context.Context,
		filter *types.ModelCallFilter,
		p *types.Pagination,
	) (*types.PageResult, error)
	Summary(
		ctx context.Context,
		filter *types.ModelCallFilter,
	) ([]*types.ModelCallSummaryItem, error)
	UpsertPrice(ctx context.Context, price *types.ModelPrice) error
	GetPrice(ctx context.Context, modelID string) (*types.ModelPrice, error)
	ListPrices(ctx context.Context) ([]*types.ModelPrice, error)
}
