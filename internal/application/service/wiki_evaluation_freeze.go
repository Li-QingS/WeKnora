package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wikiPageFreezer struct {
	pages interfaces.WikiPageService
}

func NewWikiPageFreezer(pages interfaces.WikiPageService) *wikiPageFreezer {
	return &wikiPageFreezer{pages: pages}
}

func (f *wikiPageFreezer) Freeze(
	ctx context.Context, tenantID uint64, kbID string,
) ([]types.WikiEvaluationPage, error) {
	if f == nil || f.pages == nil {
		return nil, fmt.Errorf("wiki page freezer is not configured")
	}
	if tenantID == 0 || strings.TrimSpace(kbID) == "" {
		return nil, fmt.Errorf("tenant id and knowledge base id are required")
	}
	contextTenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || contextTenantID != tenantID {
		return nil, fmt.Errorf("tenant context does not match wiki evaluation tenant")
	}

	storedPages, err := f.pages.ListAllPages(ctx, kbID)
	if err != nil {
		return nil, fmt.Errorf("list generated wiki pages: %w", err)
	}
	frozen := make([]types.WikiEvaluationPage, 0, len(storedPages))
	seenSlugs := make(map[string]struct{}, len(storedPages))
	for _, page := range storedPages {
		if page == nil || (page.PageType != types.WikiPageTypeEntity && page.PageType != types.WikiPageTypeConcept) {
			continue
		}
		if page.TenantID != tenantID || page.KnowledgeBaseID != kbID {
			return nil, fmt.Errorf("wiki page %q is outside the evaluation scope", page.Slug)
		}
		slug := strings.TrimSpace(page.Slug)
		if slug == "" {
			return nil, fmt.Errorf("wiki page %q has no slug", page.ID)
		}
		if _, exists := seenSlugs[slug]; exists {
			return nil, fmt.Errorf("duplicate wiki page slug %q", slug)
		}
		seenSlugs[slug] = struct{}{}
		frozen = append(frozen, types.WikiEvaluationPage{
			ID:       page.ID,
			Slug:     slug,
			Title:    strings.TrimSpace(page.Title),
			Type:     page.PageType,
			Aliases:  stableUniqueStrings(page.Aliases),
			OutLinks: stableUniqueStrings(page.OutLinks),
		})
	}
	sort.Slice(frozen, func(left, right int) bool {
		if frozen[left].Type != frozen[right].Type {
			return frozen[left].Type < frozen[right].Type
		}
		return frozen[left].Slug < frozen[right].Slug
	})
	return frozen, nil
}

func stableUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
