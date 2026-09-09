package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupModelCallTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(modelCallTestDDL).Error)
	return db
}

const modelCallTestDDL = `
CREATE TABLE model_call_records (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    model_id TEXT NOT NULL,
    model_name TEXT NOT NULL DEFAULT '',
    model_type TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    started_at DATETIME NOT NULL,
    finished_at DATETIME NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    cache_miss_tokens INTEGER NOT NULL DEFAULT 0,
    unit_type TEXT NOT NULL DEFAULT '',
    unit_count INTEGER NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    principal_type TEXT NOT NULL DEFAULT '',
    principal_id TEXT NOT NULL DEFAULT '',
    request_group_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    estimated_cost_usd REAL,
    price_snapshot TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE model_prices (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL DEFAULT 0,
    model_id TEXT NOT NULL,
    input_price_per_million REAL,
    output_price_per_million REAL,
    cache_read_price_per_million REAL,
    cache_write_price_per_million REAL,
    unit_type TEXT NOT NULL DEFAULT '',
    unit_price REAL,
    currency TEXT NOT NULL DEFAULT 'USD',
    updated_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, model_id)
);
`

func modelCallCtx(tenantID uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

func newTestModelCall(id string, tenantID uint64, modelID string, status string, cost *float64) *types.ModelCallRecord {
	return &types.ModelCallRecord{
		ID:               id,
		TenantID:         tenantID,
		ModelID:          modelID,
		ModelName:        "model-" + modelID,
		ModelType:        string(types.ModelTypeKnowledgeQA),
		Status:           status,
		StartedAt:        time.Now().Add(-time.Second),
		FinishedAt:       time.Now(),
		DurationMS:       100,
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		PriceSnapshot:    json.RawMessage(`{}`),
		EstimatedCostUSD: cost,
		CreatedAt:        time.Now(),
	}
}

func TestModelCallRepositoryListAndTenantIsolation(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	cost := 0.5
	require.NoError(t, repo.Create(modelCallCtx(1), newTestModelCall("a", 1, "m1", string(types.ModelCallStatusSuccess), &cost)))
	require.NoError(t, repo.Create(modelCallCtx(2), newTestModelCall("b", 2, "m2", string(types.ModelCallStatusSuccess), &cost)))

	records, total, err := repo.List(modelCallCtx(1), 1, nil, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	assert.Equal(t, "a", records[0].ID)

	_, total, err = repo.List(modelCallCtx(2), 2, nil, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestModelCallRepositoryListUsesStableIDTieBreaker(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	createdAt := time.Now()
	first := newTestModelCall("call-a", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	second := newTestModelCall("call-b", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	first.CreatedAt = createdAt
	second.CreatedAt = createdAt
	first.StartedAt = createdAt
	second.StartedAt = createdAt
	require.NoError(t, repo.Create(modelCallCtx(1), first))
	require.NoError(t, repo.Create(modelCallCtx(1), second))

	records, total, err := repo.List(modelCallCtx(1), 1, nil, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, records, 2)
	assert.Equal(t, "call-b", records[0].ID)
	assert.Equal(t, "call-a", records[1].ID)
}

func TestModelCallRepositoryListByRequestGroup(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	runRecord := newTestModelCall("run-1", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	runRecord.RequestGroupID = "evaluation_run_1"
	otherRecord := newTestModelCall("run-2", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	otherRecord.RequestGroupID = "evaluation_run_2"
	require.NoError(t, repo.Create(modelCallCtx(1), runRecord))
	require.NoError(t, repo.Create(modelCallCtx(1), otherRecord))

	records, total, err := repo.List(modelCallCtx(1), 1, &types.ModelCallFilter{
		RequestGroupID: "evaluation_run_1",
	}, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	assert.Equal(t, "run-1", records[0].ID)
}

func TestModelCallRepositoryDateRangeUsesExclusiveNextMidnight(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	dayStart := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	nextDay := dayStart.AddDate(0, 0, 1)

	before := newTestModelCall("before", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	before.StartedAt = dayStart.Add(-time.Second)
	during := newTestModelCall("during", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	during.StartedAt = nextDay.Add(-time.Millisecond)
	after := newTestModelCall("after", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	after.StartedAt = nextDay
	for _, record := range []*types.ModelCallRecord{before, during, after} {
		require.NoError(t, repo.Create(modelCallCtx(1), record))
	}

	records, total, err := repo.List(modelCallCtx(1), 1, &types.ModelCallFilter{
		From:        &dayStart,
		To:          &nextDay,
		ToExclusive: true,
	}, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, records, 1)
	assert.Equal(t, "during", records[0].ID)
}

func TestModelCallRepositoryRangeUsesDisplayedStartTime(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	dayStart := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	nextDay := dayStart.AddDate(0, 0, 1)
	record := newTestModelCall("cross-midnight", 1, "m1", string(types.ModelCallStatusSuccess), nil)
	record.StartedAt = dayStart.Add(-time.Second)
	record.CreatedAt = dayStart.Add(time.Second)
	require.NoError(t, repo.Create(modelCallCtx(1), record))

	_, total, err := repo.List(modelCallCtx(1), 1, &types.ModelCallFilter{
		From:        &dayStart,
		To:          &nextDay,
		ToExclusive: true,
	}, &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestModelCallRepositorySummary(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	okCost := 0.3
	failCost := 0.1
	require.NoError(t, repo.Create(modelCallCtx(1), newTestModelCall("a", 1, "m1", string(types.ModelCallStatusSuccess), &okCost)))
	require.NoError(t, repo.Create(modelCallCtx(1), newTestModelCall("b", 1, "m1", string(types.ModelCallStatusFailed), &failCost)))
	require.NoError(t, repo.Create(modelCallCtx(1), newTestModelCall("c", 1, "m2", string(types.ModelCallStatusSuccess), &okCost)))

	items, err := repo.Summary(modelCallCtx(1), 1, nil)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "m1", items[0].ModelID)
	assert.Equal(t, "m2", items[1].ModelID)
	byID := map[string]*types.ModelCallSummaryItem{}
	for _, item := range items {
		byID[item.ModelID] = item
	}
	assert.Equal(t, int64(2), byID["m1"].Calls)
	assert.Equal(t, int64(1), byID["m1"].SuccessCount)
	assert.Equal(t, int64(1), byID["m1"].FailedCount)
	require.NotNil(t, byID["m1"].EstimatedCostUSD)
	assert.InDelta(t, 0.4, *byID["m1"].EstimatedCostUSD, 0.0001)
}

func TestModelCallRepositorySummaryNoCostStaysNil(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	require.NoError(t, repo.Create(modelCallCtx(1), newTestModelCall("a", 1, "m1", string(types.ModelCallStatusSuccess), nil)))

	items, err := repo.Summary(modelCallCtx(1), 1, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Nil(t, items[0].EstimatedCostUSD)
}

func TestModelCallRepositoryRollupRequestGroup(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelCallRepository(db)
	okCost := 0.5
	failCost := 0.25
	first := newTestModelCall("a", 1, "m1", string(types.ModelCallStatusSuccess), &okCost)
	first.RequestGroupID = "evaluation_run_1"
	first.DurationMS = 100
	second := newTestModelCall("b", 1, "m1", string(types.ModelCallStatusFailed), &failCost)
	second.RequestGroupID = "evaluation_run_1"
	second.DurationMS = 50
	other := newTestModelCall("c", 1, "m2", string(types.ModelCallStatusSuccess), &okCost)
	other.RequestGroupID = "evaluation_run_2"
	require.NoError(t, repo.Create(modelCallCtx(1), first))
	require.NoError(t, repo.Create(modelCallCtx(1), second))
	require.NoError(t, repo.Create(modelCallCtx(1), other))

	concrete := repo.(*modelCallRepository)
	rollup, err := concrete.RollupRequestGroup(modelCallCtx(1), 1, "evaluation_run_1")
	require.NoError(t, err)
	require.NotNil(t, rollup)
	assert.Equal(t, int64(2), rollup.Calls)
	assert.Equal(t, int64(150), rollup.DurationMS)
	assert.Equal(t, int64(20), rollup.PromptTokens)
	assert.Equal(t, int64(10), rollup.CompletionTokens)
	assert.Equal(t, int64(30), rollup.TotalTokens)
	require.NotNil(t, rollup.EstimatedCostUSD)
	assert.InDelta(t, 0.75, *rollup.EstimatedCostUSD, 0.0001)

	missing, err := concrete.RollupRequestGroup(modelCallCtx(1), 1, "missing")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestModelPriceRepositoryUpsert(t *testing.T) {
	db := setupModelCallTestDB(t)
	repo := NewModelPriceRepository(db)
	input := 1.0
	output := 2.0
	price := &types.ModelPrice{TenantID: 1, ModelID: "m1", InputPricePerMillion: &input, OutputPricePerMillion: &output, Currency: "USD"}
	require.NoError(t, repo.Upsert(modelCallCtx(1), price))

	got, err := repo.Get(modelCallCtx(1), 1, "m1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1.0, *got.InputPricePerMillion)

	input = 3.0
	price = &types.ModelPrice{TenantID: 1, ModelID: "m1", InputPricePerMillion: &input, Currency: "USD"}
	require.NoError(t, repo.Upsert(modelCallCtx(1), price))
	got, err = repo.Get(modelCallCtx(1), 1, "m1")
	require.NoError(t, err)
	assert.Equal(t, 3.0, *got.InputPricePerMillion)
}
