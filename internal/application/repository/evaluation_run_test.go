package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupEvaluationRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(evaluationRunTestDDL).Error)
	return db
}

const evaluationRunTestDDL = `
CREATE TABLE evaluation_runs (
    id              TEXT PRIMARY KEY,
    tenant_id       INTEGER NOT NULL,
    dataset_id      TEXT NOT NULL,
    status          INTEGER NOT NULL,
    start_time      DATETIME NOT NULL,
    err_msg         TEXT NOT NULL DEFAULT '',
    total           INTEGER NOT NULL DEFAULT 0,
    finished        INTEGER NOT NULL DEFAULT 0,
    params          TEXT NOT NULL DEFAULT '{}',
    metric          TEXT,
    heartbeat_at    DATETIME,
    finished_at     DATETIME,
    config_hash     TEXT NOT NULL DEFAULT '',
    config_snapshot TEXT NOT NULL DEFAULT '{}',
    temporary_kb_id TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func evaluationRunCtx(tenantID uint64) context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
}

func newTestEvaluationRun(
	id string,
	tenantID uint64,
	status types.EvaluationStatue,
	createdAt time.Time,
) *types.EvaluationRun {
	return &types.EvaluationRun{
		ID:             id,
		TenantID:       tenantID,
		DatasetID:      "dataset-a",
		Status:         status,
		StartTime:      createdAt,
		Total:          10,
		Params:         json.RawMessage(`{"chat_model_id":"chat-a"}`),
		ConfigSnapshot: json.RawMessage(`{"dataset":{"id":"dataset-a"}}`),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
}

func createEvaluationRuns(
	t *testing.T,
	repo interfaces.EvaluationRunRepository,
	runs ...*types.EvaluationRun,
) {
	t.Helper()
	for _, run := range runs {
		require.NoError(t, repo.Create(evaluationRunCtx(run.TenantID), run))
	}
}

func TestEvaluationRun_CreateGetTenantIsolation(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	createdAt := time.Now().Add(-time.Hour)
	run := newTestEvaluationRun("run-1", 1, types.EvaluationStatuePending, createdAt)

	createEvaluationRuns(t, repo, run)

	got, err := repo.GetByID(evaluationRunCtx(1), 1, "run-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "run-1", got.ID)
	assert.Equal(t, uint64(1), got.TenantID)
	assert.Equal(t, types.EvaluationStatuePending, got.Status)
	assert.Equal(t, `{"chat_model_id":"chat-a"}`, string(got.Params))

	otherTenant, err := repo.GetByID(evaluationRunCtx(2), 2, "run-1")
	require.ErrorIs(t, err, ErrEvaluationRunNotFound)
	assert.Nil(t, otherTenant)
}

func TestEvaluationRun_ListPagedAndStatusFiltered(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	now := time.Now()

	createEvaluationRuns(t, repo,
		newTestEvaluationRun("run-1", 1, types.EvaluationStatueSuccess, now.Add(-1*time.Hour)),
		newTestEvaluationRun("run-2", 1, types.EvaluationStatueRunning, now.Add(-2*time.Hour)),
		newTestEvaluationRun("run-3", 1, types.EvaluationStatueSuccess, now.Add(-3*time.Hour)),
		newTestEvaluationRun("run-4", 1, types.EvaluationStatueFailed, now.Add(-4*time.Hour)),
		newTestEvaluationRun("run-5", 1, types.EvaluationStatuePending, now.Add(-5*time.Hour)),
	)

	runs, total, err := repo.List(
		evaluationRunCtx(1), 1, nil, &types.Pagination{Page: 1, PageSize: 2},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	require.Len(t, runs, 2)
	assert.Equal(t, "run-1", runs[0].ID)
	assert.Equal(t, "run-2", runs[1].ID)

	runs, total, err = repo.List(
		evaluationRunCtx(1), 1, nil, &types.Pagination{Page: 3, PageSize: 2},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	require.Len(t, runs, 1)
	assert.Equal(t, "run-5", runs[0].ID)

	status := types.EvaluationStatueSuccess
	runs, total, err = repo.List(
		evaluationRunCtx(1), 1, &status, &types.Pagination{Page: 1, PageSize: 20},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, runs, 2)
	assert.Equal(t, "run-1", runs[0].ID)
	assert.Equal(t, "run-3", runs[1].ID)

	runs, total, err = repo.List(
		evaluationRunCtx(2), 2, nil, &types.Pagination{Page: 1, PageSize: 20},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, runs)
}

func TestEvaluationRun_TransitionStatusCASProtectsTerminal(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	ctx := evaluationRunCtx(1)
	run := newTestEvaluationRun("run-cas", 1, types.EvaluationStatueRunning, time.Now())
	createEvaluationRuns(t, repo, run)

	ok, err := repo.TransitionStatus(
		ctx, "run-cas", []types.EvaluationStatue{types.EvaluationStatuePending},
		types.EvaluationStatueSuccess, "",
	)
	require.NoError(t, err)
	assert.False(t, ok)

	ok, err = repo.TransitionStatus(
		ctx, "run-cas", []types.EvaluationStatue{types.EvaluationStatueRunning},
		types.EvaluationStatueSuccess, "",
	)
	require.NoError(t, err)
	assert.True(t, ok)

	got, err := repo.GetByID(ctx, 1, "run-cas")
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatueSuccess, got.Status)
	require.NotNil(t, got.FinishedAt)

	ok, err = repo.TransitionStatus(
		ctx, "run-cas", []types.EvaluationStatue{types.EvaluationStatueSuccess},
		types.EvaluationStatueFailed, "boom",
	)
	require.NoError(t, err)
	assert.False(t, ok)

	ok, err = repo.TransitionStatus(
		ctx, "run-cas", []types.EvaluationStatue{types.EvaluationStatueRunning},
		types.EvaluationStatueFailed, "boom",
	)
	require.NoError(t, err)
	assert.False(t, ok)

	got, err = repo.GetByID(ctx, 1, "run-cas")
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatueSuccess, got.Status)
	assert.Empty(t, got.ErrMsg)
}

func TestEvaluationRun_ProgressAndHeartbeatStopAtTerminal(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	ctx := evaluationRunCtx(1)
	run := newTestEvaluationRun("run-progress", 1, types.EvaluationStatueRunning, time.Now())
	createEvaluationRuns(t, repo, run)

	require.NoError(t, repo.UpdateProgress(
		ctx, "run-progress", 3, 10, json.RawMessage(`{"precision":0.5}`),
	))
	heartbeat := time.Now().Add(-10 * time.Second)
	require.NoError(t, repo.UpdateHeartbeat(ctx, "run-progress", heartbeat))

	got, err := repo.GetByID(ctx, 1, "run-progress")
	require.NoError(t, err)
	assert.Equal(t, 3, got.Finished)
	require.NotNil(t, got.HeartbeatAt)
	assert.WithinDuration(t, heartbeat, *got.HeartbeatAt, time.Second)

	ok, err := repo.TransitionStatus(
		ctx, "run-progress", []types.EvaluationStatue{types.EvaluationStatueRunning},
		types.EvaluationStatueSuccess, "",
	)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, repo.UpdateProgress(
		ctx, "run-progress", 5, 10, json.RawMessage(`{"precision":0.9}`),
	))
	require.NoError(t, repo.UpdateHeartbeat(ctx, "run-progress", time.Now()))

	got, err = repo.GetByID(ctx, 1, "run-progress")
	require.NoError(t, err)
	assert.Equal(t, types.EvaluationStatueSuccess, got.Status)
	assert.Equal(t, 3, got.Finished)
	assert.WithinDuration(t, heartbeat, *got.HeartbeatAt, time.Second)
	assert.Equal(t, `{"precision":0.5}`, string(got.Metric))
}

func TestEvaluationRun_SetDatasetHashOnlyWhileRunning(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	ctx := evaluationRunCtx(1)
	run := newTestEvaluationRun("run-hash", 1, types.EvaluationStatueRunning, time.Now())
	createEvaluationRuns(t, repo, run)

	require.NoError(t, repo.SetDatasetHash(ctx, "run-hash", "sha-abc", 5))

	got, err := repo.GetByID(ctx, 1, "run-hash")
	require.NoError(t, err)
	var snapshot types.EvaluationConfigSnapshot
	require.NoError(t, json.Unmarshal(got.ConfigSnapshot, &snapshot))
	assert.Equal(t, "dataset-a", snapshot.Dataset.ID)
	assert.Equal(t, "sha-abc", snapshot.Dataset.SHA256)
	assert.Equal(t, 5, snapshot.Dataset.SampleCount)

	ok, err := repo.TransitionStatus(
		ctx, "run-hash", []types.EvaluationStatue{types.EvaluationStatueRunning},
		types.EvaluationStatueSuccess, "",
	)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, repo.SetDatasetHash(ctx, "run-hash", "sha-xyz", 6))
	got, err = repo.GetByID(ctx, 1, "run-hash")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(got.ConfigSnapshot, &snapshot))
	assert.Equal(t, "sha-abc", snapshot.Dataset.SHA256)
	assert.Equal(t, 5, snapshot.Dataset.SampleCount)
}

func TestEvaluationRun_MarkStaleInterrupted(t *testing.T) {
	db := setupEvaluationRunTestDB(t)
	repo := NewEvaluationRunRepository(db)
	now := time.Now()
	old := now.Add(-time.Hour)

	createEvaluationRuns(t, repo,
		newTestEvaluationRun("pending-stale", 1, types.EvaluationStatuePending, now.Add(-2*time.Hour)),
		newTestEvaluationRun("running-stale", 1, types.EvaluationStatueRunning, now.Add(-3*time.Hour)),
		newTestEvaluationRun("running-fresh", 1, types.EvaluationStatueRunning, now.Add(-4*time.Hour)),
		newTestEvaluationRun("success-old", 1, types.EvaluationStatueSuccess, now.Add(-5*time.Hour)),
		newTestEvaluationRun("failed-old", 1, types.EvaluationStatueFailed, now.Add(-6*time.Hour)),
	)
	require.NoError(t, db.Model(&types.EvaluationRun{}).
		Where("id IN ?", []string{"running-stale", "running-fresh", "success-old", "failed-old"}).
		Update("heartbeat_at", old).Error)
	require.NoError(t, db.Model(&types.EvaluationRun{}).
		Where("id = ?", "running-fresh").
		Update("heartbeat_at", now).Error)

	affected, err := repo.MarkStaleInterrupted(evaluationRunCtx(1), now.Add(-45*time.Second))
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	cases := []struct {
		id           string
		wantStatus   types.EvaluationStatue
		wantFinished bool
	}{
		{"pending-stale", types.EvaluationStatueInterrupted, true},
		{"running-stale", types.EvaluationStatueInterrupted, true},
		{"running-fresh", types.EvaluationStatueRunning, false},
		{"success-old", types.EvaluationStatueSuccess, false},
		{"failed-old", types.EvaluationStatueFailed, false},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			got, err := repo.GetByID(evaluationRunCtx(1), 1, c.id)
			require.NoError(t, err)
			assert.Equal(t, c.wantStatus, got.Status)
			if c.wantFinished {
				require.NotNil(t, got.FinishedAt)
				assert.Equal(t, "interrupted by service restart", got.ErrMsg)
			} else {
				assert.Nil(t, got.FinishedAt)
			}
		})
	}
}
