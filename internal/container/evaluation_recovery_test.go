package container

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type recoveryTenantServiceFake struct {
	interfaces.TenantService
}

func (f *recoveryTenantServiceFake) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id, Name: "test"}, nil
}

type recoveryKnowledgeBaseServiceFake struct {
	interfaces.KnowledgeBaseService
	db       *gorm.DB
	attempts int
}

func (f *recoveryKnowledgeBaseServiceFake) DeleteKnowledgeBase(ctx context.Context, id string) error {
	f.attempts++
	if f.attempts == 1 {
		return errors.New("temporary delete failure")
	}
	if tenantID, ok := types.TenantIDFromContext(ctx); !ok || tenantID != 7 {
		return errors.New("tenant context was not restored")
	}
	return f.db.Exec("DELETE FROM knowledge_bases WHERE id = ?", id).Error
}

func TestRecoveredWikiTerminalStatus(t *testing.T) {
	tests := []struct {
		name        string
		typeValue   types.EvaluationType
		failure     types.EvaluationStage
		result      json.RawMessage
		want        types.EvaluationStatue
		wantPromote bool
	}{
		{"failed cleanup", types.EvaluationTypeWiki, types.EvaluationStageGenerating, nil, types.EvaluationStatueFailed, true},
		{"saved result", types.EvaluationTypeWiki, "", json.RawMessage(`{"metric":{}}`), types.EvaluationStatueSuccess, true},
		{"crashed mid-run", types.EvaluationTypeWiki, "", nil, types.EvaluationStatueInterrupted, false},
		{"empty result", types.EvaluationTypeWiki, "", json.RawMessage(`{}`), types.EvaluationStatueInterrupted, false},
		{"rag stays interrupted", types.EvaluationTypeRAG, types.EvaluationStageGenerating, nil, types.EvaluationStatueInterrupted, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, promote := recoveredWikiTerminalStatus(test.typeValue, test.failure, test.result)
			assert.Equal(t, test.want, got)
			assert.Equal(t, test.wantPromote, promote)
		})
	}
}

func TestInterruptedWikiCleanupCanRetryAndFinalize(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.EvaluationRun{}))
	require.NoError(t, db.Exec(`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, deleted_at DATETIME)`).Error)
	now := time.Now().Add(-time.Minute)
	run := &types.EvaluationRun{
		ID: "wiki-recovery", TenantID: 7, DatasetID: "enterprise_rag",
		Status: types.EvaluationStatueInterrupted, StartTime: now,
		Params: json.RawMessage(`{}`), ConfigSnapshot: json.RawMessage(`{}`), StageProgress: json.RawMessage(`{}`),
		TemporaryKBID: "temp-kb", EvaluationType: types.EvaluationTypeWiki,
		Stage: types.EvaluationStageCleaningUp, FailureStage: types.EvaluationStageGenerating,
		ErrMsg: "generation failed", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(run).Error)
	require.NoError(t, db.Exec("INSERT INTO knowledge_bases (id) VALUES (?)", run.TemporaryKBID).Error)
	kb := &recoveryKnowledgeBaseServiceFake{db: db}
	tenant := &recoveryTenantServiceFake{}

	cleanupInterruptedEvaluationKnowledgeBases(context.Background(), db, tenant, kb)
	var afterFirst types.EvaluationRun
	require.NoError(t, db.First(&afterFirst, "id = ?", run.ID).Error)
	assert.Equal(t, types.EvaluationStatueInterrupted, afterFirst.Status)
	assert.Equal(t, 1, kb.attempts)

	cleanupInterruptedEvaluationKnowledgeBases(context.Background(), db, tenant, kb)
	var finalized types.EvaluationRun
	require.NoError(t, db.First(&finalized, "id = ?", run.ID).Error)
	assert.Equal(t, types.EvaluationStatueFailed, finalized.Status)
	assert.Equal(t, types.EvaluationStageCompleted, finalized.Stage)
	assert.Equal(t, 2, kb.attempts)
	var active int64
	require.NoError(t, db.Table("knowledge_bases").Where("id = ?", run.TemporaryKBID).Count(&active).Error)
	assert.Zero(t, active)
}
