package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const (
	wikiEvaluationMonitorInterval = time.Second
	wikiEvaluationDeadLetterPage  = 200
)

type wikiGenerationMonitor struct {
	knowledge  interfaces.KnowledgeService
	pending    interfaces.TaskPendingOpsRepository
	deadLetter interfaces.TaskDeadLetterRepository
	interval   time.Duration
}

func NewWikiGenerationMonitor(
	knowledge interfaces.KnowledgeService,
	pending interfaces.TaskPendingOpsRepository,
	deadLetter interfaces.TaskDeadLetterRepository,
) *wikiGenerationMonitor {
	return &wikiGenerationMonitor{
		knowledge: knowledge, pending: pending, deadLetter: deadLetter,
		interval: wikiEvaluationMonitorInterval,
	}
}

func (m *wikiGenerationMonitor) WaitUntilStable(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	knowledgeIDs []string,
	onProgress func(types.EvaluationStageProgress),
) error {
	if m == nil || m.knowledge == nil || m.pending == nil || m.deadLetter == nil {
		return fmt.Errorf("wiki generation monitor is not configured")
	}
	if tenantID == 0 || strings.TrimSpace(kbID) == "" || len(knowledgeIDs) == 0 {
		return fmt.Errorf("tenant, knowledge base, and imported knowledge ids are required")
	}
	contextTenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || contextTenantID != tenantID {
		return fmt.Errorf("tenant context does not match wiki evaluation tenant")
	}

	interval := m.interval
	if interval <= 0 {
		interval = wikiEvaluationMonitorInterval
	}
	stableChecks := 0
	for {
		stable, err := m.inspect(ctx, tenantID, kbID, knowledgeIDs, onProgress)
		if err != nil {
			return err
		}
		if stable {
			stableChecks++
			if stableChecks >= 2 {
				return nil
			}
		} else {
			stableChecks = 0
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (m *wikiGenerationMonitor) inspect(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	knowledgeIDs []string,
	onProgress func(types.EvaluationStageProgress),
) (bool, error) {
	deadLetter, err := m.firstDeadLetter(ctx, tenantID, kbID)
	if err != nil {
		return false, err
	}
	if deadLetter != nil {
		return false, fmt.Errorf("wiki task %s failed for %s: %s", deadLetter.TaskType, deadLetter.RelatedID, deadLetter.LastError)
	}

	knowledge, err := m.knowledge.GetKnowledgeBatch(ctx, tenantID, knowledgeIDs)
	if err != nil {
		return false, fmt.Errorf("read imported knowledge status: %w", err)
	}
	byID := make(map[string]*types.Knowledge, len(knowledge))
	for _, item := range knowledge {
		if item != nil {
			byID[item.ID] = item
		}
	}
	completed := 0
	for _, id := range knowledgeIDs {
		item, exists := byID[id]
		if !exists {
			return false, fmt.Errorf("imported knowledge %s no longer exists", id)
		}
		switch item.ParseStatus {
		case types.ParseStatusCompleted:
			completed++
		case types.ParseStatusFailed, types.ParseStatusCancelled, types.ParseStatusDeleting:
			return false, fmt.Errorf("knowledge %s entered %s: %s", id, item.ParseStatus, item.ErrorMessage)
		}
	}

	ingestPending, err := m.pending.PendingCount(ctx, wikiTaskType, wikiTaskScope, kbID)
	if err != nil {
		return false, fmt.Errorf("count pending wiki ingest operations: %w", err)
	}
	finalizePending, err := m.pending.PendingCount(ctx, wikiFinalizeTaskType, wikiTaskScope, kbID)
	if err != nil {
		return false, fmt.Errorf("count pending wiki finalize operations: %w", err)
	}
	message := fmt.Sprintf("documents %d/%d; wiki pending %d; finalize pending %d",
		completed, len(knowledgeIDs), ingestPending, finalizePending)
	if onProgress != nil {
		onProgress(types.EvaluationStageProgress{Current: completed, Total: len(knowledgeIDs), Message: message})
	}
	return completed == len(knowledgeIDs) && ingestPending == 0 && finalizePending == 0, nil
}

func (m *wikiGenerationMonitor) firstDeadLetter(
	ctx context.Context, tenantID uint64, kbID string,
) (*types.TaskDeadLetter, error) {
	cursor := ""
	for {
		rows, next, err := m.deadLetter.ListByScope(
			ctx, types.TaskScopeKnowledgeBase, kbID, cursor, wikiEvaluationDeadLetterPage,
		)
		if err != nil {
			return nil, fmt.Errorf("list wiki task dead letters: %w", err)
		}
		for _, row := range rows {
			if row != nil && row.TenantID == tenantID {
				return row, nil
			}
		}
		if next == "" {
			return nil, nil
		}
		cursor = next
	}
}
