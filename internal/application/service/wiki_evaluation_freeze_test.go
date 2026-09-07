package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wikiFreezePageServiceFake struct {
	interfaces.WikiPageService
	pages []*types.WikiPage
	err   error
}

func (f *wikiFreezePageServiceFake) ListAllPages(context.Context, string) ([]*types.WikiPage, error) {
	return f.pages, f.err
}

func TestWikiPageFreezerFiltersAndStabilizesPages(t *testing.T) {
	service := &wikiFreezePageServiceFake{pages: []*types.WikiPage{
		{ID: "summary", TenantID: 7, KnowledgeBaseID: "kb", Slug: "summary/doc", PageType: types.WikiPageTypeSummary},
		{ID: "entity", TenantID: 7, KnowledgeBaseID: "kb", Slug: "entity/z", Title: " Z ",
			PageType: types.WikiPageTypeEntity, Aliases: types.StringArray{"Zulu", " Z ", "Zulu"},
			OutLinks: types.StringArray{"concept/b", "unknown/x", "concept/b"}},
		{ID: "concept", TenantID: 7, KnowledgeBaseID: "kb", Slug: "concept/b", Title: "B",
			PageType: types.WikiPageTypeConcept},
	}}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))

	pages, err := NewWikiPageFreezer(service).Freeze(ctx, 7, "kb")
	require.NoError(t, err)
	require.Equal(t, []types.WikiEvaluationPage{
		{ID: "concept", Slug: "concept/b", Title: "B", Type: types.WikiPageTypeConcept, Aliases: []string{}, OutLinks: []string{}},
		{ID: "entity", Slug: "entity/z", Title: "Z", Type: types.WikiPageTypeEntity,
			Aliases: []string{"Z", "Zulu"}, OutLinks: []string{"concept/b", "unknown/x"}},
	}, pages)
}

func TestWikiPageFreezerRejectsCrossTenantAndDuplicateSlug(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	freezer := NewWikiPageFreezer(&wikiFreezePageServiceFake{pages: []*types.WikiPage{{
		ID: "foreign", TenantID: 8, KnowledgeBaseID: "kb", Slug: "entity/x", PageType: types.WikiPageTypeEntity,
	}}})
	_, err := freezer.Freeze(ctx, 7, "kb")
	require.ErrorContains(t, err, "outside the evaluation scope")

	freezer = NewWikiPageFreezer(&wikiFreezePageServiceFake{pages: []*types.WikiPage{
		{ID: "one", TenantID: 7, KnowledgeBaseID: "kb", Slug: "entity/x", PageType: types.WikiPageTypeEntity},
		{ID: "two", TenantID: 7, KnowledgeBaseID: "kb", Slug: "entity/x", PageType: types.WikiPageTypeEntity},
	}})
	_, err = freezer.Freeze(ctx, 7, "kb")
	require.ErrorContains(t, err, "duplicate wiki page slug")
}
