package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const wikiEvaluationImportChannel = "evaluation"

type wikiCorpusImporter struct {
	creator interfaces.TitledPassageKnowledgeCreator
}

func NewWikiCorpusImporter(creator interfaces.TitledPassageKnowledgeCreator) *wikiCorpusImporter {
	return &wikiCorpusImporter{creator: creator}
}

func (i *wikiCorpusImporter) ImportDocuments(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	docs []types.EvaluationDocument,
	onProgress func(types.EvaluationStageProgress),
) ([]string, error) {
	if i == nil || i.creator == nil {
		return nil, fmt.Errorf("wiki corpus importer is not configured")
	}
	if tenantID == 0 {
		return nil, fmt.Errorf("tenant id is required")
	}
	contextTenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || contextTenantID != tenantID {
		return nil, fmt.Errorf("tenant context does not match wiki evaluation tenant")
	}
	if strings.TrimSpace(kbID) == "" {
		return nil, fmt.Errorf("temporary knowledge base id is required")
	}

	ordered := append([]types.EvaluationDocument(nil), docs...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].ID != ordered[right].ID {
			return ordered[left].ID < ordered[right].ID
		}
		return ordered[left].Title < ordered[right].Title
	})
	seen := make(map[int64]struct{}, len(ordered))
	knowledgeIDs := make([]string, 0, len(ordered))
	reportWikiImportProgress(onProgress, 0, len(ordered), "preparing corpus import")
	for index, doc := range ordered {
		if err := ctx.Err(); err != nil {
			return knowledgeIDs, err
		}
		if doc.ID <= 0 || strings.TrimSpace(doc.Title) == "" || strings.TrimSpace(doc.Content) == "" {
			return knowledgeIDs, fmt.Errorf("invalid evaluation document at position %d", index)
		}
		if _, exists := seen[doc.ID]; exists {
			return knowledgeIDs, fmt.Errorf("duplicate evaluation document id %d", doc.ID)
		}
		seen[doc.ID] = struct{}{}

		knowledge, err := i.creator.CreateKnowledgeFromPassageWithTitle(
			ctx, kbID, doc.Title, []string{doc.Content}, wikiEvaluationImportChannel,
		)
		if err != nil {
			return knowledgeIDs, fmt.Errorf("import evaluation document %d: %w", doc.ID, err)
		}
		if knowledge == nil || knowledge.ID == "" {
			return knowledgeIDs, fmt.Errorf("import evaluation document %d returned no knowledge id", doc.ID)
		}
		knowledgeIDs = append(knowledgeIDs, knowledge.ID)
		reportWikiImportProgress(onProgress, index+1, len(ordered), doc.Title)
	}
	return knowledgeIDs, nil
}

func reportWikiImportProgress(
	onProgress func(types.EvaluationStageProgress), current int, total int, message string,
) {
	if onProgress != nil {
		onProgress(types.EvaluationStageProgress{Current: current, Total: total, Message: message})
	}
}
