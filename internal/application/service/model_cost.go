package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// modelCallRecorder implements costledger.Recorder using the ledger repos.
type modelCallRecorder struct {
	calls  interfaces.ModelCallRepository
	prices interfaces.ModelPriceRepository
}

// NewModelCallRecorder constructs the recorder installed by the container.
func NewModelCallRecorder(
	calls interfaces.ModelCallRepository,
	prices interfaces.ModelPriceRepository,
) costledger.Recorder {
	return &modelCallRecorder{calls: calls, prices: prices}
}

func (r *modelCallRecorder) Record(ctx context.Context, info *types.ModelCallInfo) error {
	if info == nil {
		return errors.New("model call: nil info")
	}
	price, err := r.prices.Get(ctx, info.TenantID, info.ModelID)
	if err != nil {
		return fmt.Errorf("model call: load price: %w", err)
	}
	if price == nil {
		price, _ = r.prices.Get(ctx, 0, info.ModelID)
	}

	snapshot, cost := buildPriceSnapshot(price, info)
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("model call: encode price snapshot: %w", err)
	}

	record := &types.ModelCallRecord{
		TenantID:         info.TenantID,
		ModelID:          info.ModelID,
		ModelName:        info.ModelName,
		ModelType:        info.ModelType,
		Purpose:          info.Purpose,
		Status:           string(info.Status),
		StartedAt:        info.StartedAt,
		FinishedAt:       info.FinishedAt,
		DurationMS:       info.DurationMS,
		PromptTokens:     info.PromptTokens,
		CompletionTokens: info.CompletionTokens,
		TotalTokens:      info.TotalTokens,
		CacheReadTokens:  info.CacheReadTokens,
		CacheWriteTokens: info.CacheWriteTokens,
		CacheMissTokens:  info.CacheMissTokens,
		UnitType:         info.UnitType,
		UnitCount:        info.UnitCount,
		ErrorType:        info.ErrorType,
		ErrorMessage:     info.ErrorMessage,
		SessionID:        info.SessionID,
		UserID:           info.UserID,
		PrincipalType:    info.PrincipalType,
		PrincipalID:      info.PrincipalID,
		RequestGroupID:   info.RequestGroupID,
		TraceID:          info.TraceID,
		EstimatedCostUSD: cost,
		PriceSnapshot:    snapshotJSON,
	}
	return r.calls.Create(ctx, record)
}

func buildPriceSnapshot(price *types.ModelPrice, info *types.ModelCallInfo) (types.PriceSnapshot, *float64) {
	if price == nil {
		return types.PriceSnapshot{}, nil
	}
	snapshot := types.PriceSnapshot{
		Currency:                  price.Currency,
		InputPricePerMillion:      price.InputPricePerMillion,
		OutputPricePerMillion:     price.OutputPricePerMillion,
		CacheReadPricePerMillion:  price.CacheReadPricePerMillion,
		CacheWritePricePerMillion: price.CacheWritePricePerMillion,
		UnitType:                  price.UnitType,
		UnitPrice:                 price.UnitPrice,
	}
	return snapshot, estimateCost(price, info)
}

func estimateCost(price *types.ModelPrice, info *types.ModelCallInfo) *float64 {
	if price == nil {
		return nil
	}
	if price.UnitType != "" && price.UnitPrice != nil {
		value := float64(info.UnitCount) * *price.UnitPrice
		return &value
	}
	if price.InputPricePerMillion == nil &&
		price.OutputPricePerMillion == nil &&
		price.CacheReadPricePerMillion == nil &&
		price.CacheWritePricePerMillion == nil {
		return nil
	}
	value := 0.0
	if price.InputPricePerMillion != nil {
		uncachedInputTokens := info.PromptTokens - info.CacheReadTokens - info.CacheWriteTokens
		if uncachedInputTokens < 0 {
			uncachedInputTokens = 0
		}
		value += float64(uncachedInputTokens) / 1_000_000 * *price.InputPricePerMillion
	}
	if price.OutputPricePerMillion != nil {
		value += float64(info.CompletionTokens) / 1_000_000 * *price.OutputPricePerMillion
	}
	if price.CacheReadPricePerMillion != nil {
		value += float64(info.CacheReadTokens) / 1_000_000 * *price.CacheReadPricePerMillion
	}
	if price.CacheWritePricePerMillion != nil {
		value += float64(info.CacheWriteTokens) / 1_000_000 * *price.CacheWritePricePerMillion
	}
	return &value
}

// modelCallService implements interfaces.ModelCallService.
type modelCallService struct {
	calls  interfaces.ModelCallRepository
	prices interfaces.ModelPriceRepository
}

// NewModelCallService constructs the tenant-scoped ledger query service.
func NewModelCallService(
	calls interfaces.ModelCallRepository,
	prices interfaces.ModelPriceRepository,
) interfaces.ModelCallService {
	return &modelCallService{calls: calls, prices: prices}
}

func (s *modelCallService) List(
	ctx context.Context,
	filter *types.ModelCallFilter,
	p *types.Pagination,
) (*types.PageResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	records, total, err := s.calls.List(ctx, tenantID, filter, p)
	if err != nil {
		return nil, err
	}
	return types.NewPageResult(total, p, records), nil
}

func (s *modelCallService) Summary(
	ctx context.Context,
	filter *types.ModelCallFilter,
) ([]*types.ModelCallSummaryItem, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	return s.calls.Summary(ctx, tenantID, filter)
}

func (s *modelCallService) UpsertPrice(ctx context.Context, price *types.ModelPrice) error {
	if price == nil {
		return errors.New("model price: nil price")
	}
	if price.ModelID == "" {
		return errors.New("model price: model_id is required")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	price.TenantID = tenantID
	if price.Currency == "" {
		price.Currency = "USD"
	}
	return s.prices.Upsert(ctx, price)
}

func (s *modelCallService) GetPrice(ctx context.Context, modelID string) (*types.ModelPrice, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	price, err := s.prices.Get(ctx, tenantID, modelID)
	if err != nil {
		return nil, err
	}
	if price == nil {
		return nil, errors.New("model price not found")
	}
	return price, nil
}

func (s *modelCallService) ListPrices(ctx context.Context) ([]*types.ModelPrice, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	return s.prices.List(ctx, tenantID)
}
