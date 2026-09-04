package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeModelCallRepo struct {
	records []*types.ModelCallRecord
}

func (f *fakeModelCallRepo) Create(_ context.Context, record *types.ModelCallRecord) error {
	f.records = append(f.records, record)
	return nil
}

func (f *fakeModelCallRepo) List(
	_ context.Context,
	tenantID uint64,
	_ *types.ModelCallFilter,
	p *types.Pagination,
) ([]*types.ModelCallRecord, int64, error) {
	var out []*types.ModelCallRecord
	for _, r := range f.records {
		if r.TenantID == tenantID {
			out = append(out, r)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeModelCallRepo) Summary(
	_ context.Context,
	tenantID uint64,
	_ *types.ModelCallFilter,
) ([]*types.ModelCallSummaryItem, error) {
	byModel := map[string]*types.ModelCallSummaryItem{}
	for _, r := range f.records {
		if r.TenantID != tenantID {
			continue
		}
		item := byModel[r.ModelID]
		if item == nil {
			item = &types.ModelCallSummaryItem{ModelID: r.ModelID, ModelName: r.ModelName, ModelType: r.ModelType}
			byModel[r.ModelID] = item
		}
		item.Calls++
	}
	items := make([]*types.ModelCallSummaryItem, 0, len(byModel))
	for _, item := range byModel {
		items = append(items, item)
	}
	return items, nil
}

type fakeModelPriceRepo struct {
	prices map[string]*types.ModelPrice
}

func (f *fakeModelPriceRepo) Get(_ context.Context, tenantID uint64, modelID string) (*types.ModelPrice, error) {
	if f.prices == nil {
		return nil, nil
	}
	return f.prices[fmt.Sprintf("%d:%s", tenantID, modelID)], nil
}

func (f *fakeModelPriceRepo) Upsert(_ context.Context, price *types.ModelPrice) error {
	if f.prices == nil {
		f.prices = map[string]*types.ModelPrice{}
	}
	f.prices[fmt.Sprintf("%d:%s", price.TenantID, price.ModelID)] = price
	return nil
}

func (f *fakeModelPriceRepo) List(_ context.Context, tenantID uint64) ([]*types.ModelPrice, error) {
	var out []*types.ModelPrice
	prefix := fmt.Sprintf("%d:", tenantID)
	for key, price := range f.prices {
		if strings.HasPrefix(key, prefix) {
			out = append(out, price)
		}
	}
	return out, nil
}

func modelCostCtx(tenantID uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

func modelCallInfoForTest() *types.ModelCallInfo {
	now := time.Now()
	return &types.ModelCallInfo{
		TenantID:         1,
		ModelID:          "m1",
		ModelName:        "chat-1",
		ModelType:        string(types.ModelTypeKnowledgeQA),
		Status:           types.ModelCallStatusSuccess,
		StartedAt:        now.Add(-time.Second),
		FinishedAt:       now,
		DurationMS:       1000,
		PromptTokens:     1_000_000,
		CompletionTokens: 500_000,
		TotalTokens:      1_500_000,
	}
}

func TestModelCallRecorderTokenCost(t *testing.T) {
	calls := &fakeModelCallRepo{}
	input := 1.0
	output := 2.0
	prices := &fakeModelPriceRepo{prices: map[string]*types.ModelPrice{
		"1:m1": {TenantID: 1, ModelID: "m1", InputPricePerMillion: &input, OutputPricePerMillion: &output, Currency: "USD"},
	}}
	recorder := NewModelCallRecorder(calls, prices)
	require.NoError(t, recorder.Record(modelCostCtx(1), modelCallInfoForTest()))
	require.Len(t, calls.records, 1)
	require.NotNil(t, calls.records[0].EstimatedCostUSD)
	assert.InDelta(t, 2.0, *calls.records[0].EstimatedCostUSD, 0.0001)
}

func TestModelCallRecorderUnknownPriceNil(t *testing.T) {
	calls := &fakeModelCallRepo{}
	recorder := NewModelCallRecorder(calls, &fakeModelPriceRepo{})
	require.NoError(t, recorder.Record(modelCostCtx(1), modelCallInfoForTest()))
	require.Len(t, calls.records, 1)
	assert.Nil(t, calls.records[0].EstimatedCostUSD)
}

func TestModelCallRecorderUnitPrice(t *testing.T) {
	calls := &fakeModelCallRepo{}
	unit := 0.1
	prices := &fakeModelPriceRepo{prices: map[string]*types.ModelPrice{
		"1:m1": {TenantID: 1, ModelID: "m1", UnitType: "documents", UnitPrice: &unit, Currency: "USD"},
	}}
	info := modelCallInfoForTest()
	info.UnitType = "documents"
	info.UnitCount = 10
	recorder := NewModelCallRecorder(calls, prices)
	require.NoError(t, recorder.Record(modelCostCtx(1), info))
	require.NotNil(t, calls.records[0].EstimatedCostUSD)
	assert.InDelta(t, 1.0, *calls.records[0].EstimatedCostUSD, 0.0001)
}

func TestModelCallRecorderCacheTokensUseDiscountedPrices(t *testing.T) {
	calls := &fakeModelCallRepo{}
	input := 1.0
	output := 2.0
	read := 0.1
	write := 3.0
	prices := &fakeModelPriceRepo{prices: map[string]*types.ModelPrice{
		"1:m1": {
			TenantID:                  1,
			ModelID:                   "m1",
			InputPricePerMillion:      &input,
			OutputPricePerMillion:     &output,
			CacheReadPricePerMillion:  &read,
			CacheWritePricePerMillion: &write,
			Currency:                  "USD",
		},
	}}
	recorder := NewModelCallRecorder(calls, prices)
	info := modelCallInfoForTest()
	info.PromptTokens = 1_000_000
	info.CompletionTokens = 500_000
	info.TotalTokens = 1_800_000
	info.CacheReadTokens = 200_000
	info.CacheWriteTokens = 100_000
	info.CacheMissTokens = 700_000
	require.NoError(t, recorder.Record(modelCostCtx(1), info))
	require.Len(t, calls.records, 1)
	require.NotNil(t, calls.records[0].EstimatedCostUSD)
	assert.InDelta(t, 2.02, *calls.records[0].EstimatedCostUSD, 0.0001)
}

func TestModelCallServiceUpsertPriceTenantScoped(t *testing.T) {
	prices := &fakeModelPriceRepo{}
	svc := NewModelCallService(&fakeModelCallRepo{}, prices)
	input := 1.0
	require.NoError(t, svc.UpsertPrice(modelCostCtx(9), &types.ModelPrice{ModelID: "m1", InputPricePerMillion: &input}))
	got, err := prices.Get(context.Background(), 9, "m1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uint64(9), got.TenantID)
	assert.Equal(t, "USD", got.Currency)
}
