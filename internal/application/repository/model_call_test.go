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
