package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

type wikiImportCreatorFake struct {
	titles   []string
	channels []string
	failAt   int
}

func (f *wikiImportCreatorFake) CreateKnowledgeFromPassageWithTitle(
	_ context.Context, _ string, title string, _ []string, channel string,
) (*types.Knowledge, error) {
	f.titles = append(f.titles, title)
	f.channels = append(f.channels, channel)
	if f.failAt > 0 && len(f.titles) == f.failAt {
		return nil, errors.New("enqueue failed")
	}
	return &types.Knowledge{ID: "knowledge-" + title, Channel: channel}, nil
}

func wikiImportTestContext() context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
}

func TestWikiCorpusImporterUsesStableTitlesAndProgress(t *testing.T) {
	creator := &wikiImportCreatorFake{}
	importer := NewWikiCorpusImporter(creator)
	var progress []types.EvaluationStageProgress
	ids, err := importer.ImportDocuments(wikiImportTestContext(), 7, "kb", []types.EvaluationDocument{
		{ID: 2, Title: "corpus-2", Content: "two"},
		{ID: 1, Title: "corpus-1", Content: "one"},
	}, func(value types.EvaluationStageProgress) { progress = append(progress, value) })
	require.NoError(t, err)
	require.Equal(t, []string{"corpus-1", "corpus-2"}, creator.titles)
	require.Equal(t, []string{"evaluation", "evaluation"}, creator.channels)
	require.Equal(t, []string{"knowledge-corpus-1", "knowledge-corpus-2"}, ids)
	require.Equal(t, []types.EvaluationStageProgress{
		{Current: 0, Total: 2, Message: "preparing corpus import"},
		{Current: 1, Total: 2, Message: "corpus-1"},
		{Current: 2, Total: 2, Message: "corpus-2"},
	}, progress)
}

func TestWikiCorpusImporterStopsAtFirstFailure(t *testing.T) {
	creator := &wikiImportCreatorFake{failAt: 2}
	importer := NewWikiCorpusImporter(creator)
	ids, err := importer.ImportDocuments(wikiImportTestContext(), 7, "kb", []types.EvaluationDocument{
		{ID: 1, Title: "corpus-1", Content: "one"},
		{ID: 2, Title: "corpus-2", Content: "two"},
		{ID: 3, Title: "corpus-3", Content: "three"},
	}, nil)
	require.ErrorContains(t, err, "import evaluation document 2")
	require.Equal(t, []string{"knowledge-corpus-1"}, ids)
	require.Equal(t, []string{"corpus-1", "corpus-2"}, creator.titles)
}

func TestWikiCorpusImporterRejectsTenantMismatchAndDuplicate(t *testing.T) {
	importer := NewWikiCorpusImporter(&wikiImportCreatorFake{})
	_, err := importer.ImportDocuments(wikiImportTestContext(), 8, "kb", nil, nil)
	require.ErrorContains(t, err, "tenant context")

	_, err = importer.ImportDocuments(wikiImportTestContext(), 7, "kb", []types.EvaluationDocument{
		{ID: 1, Title: "one", Content: "first"},
		{ID: 1, Title: "again", Content: "second"},
	}, nil)
	require.ErrorContains(t, err, "duplicate evaluation document id 1")
}
