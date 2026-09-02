package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDemoDataset(
	t *testing.T,
	dir string,
	queries []TextInfo,
	corpus []TextInfo,
	answers []TextInfo,
	qrels []RelsInfo,
	qas []QaInfo,
) {
	t.Helper()
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "queries.parquet"), queries))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "corpus.parquet"), corpus))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "answers.parquet"), answers))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "qrels.parquet"), qrels))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "qas.parquet"), qas))
}

func validDemoFiles() ([]TextInfo, []TextInfo, []TextInfo, []RelsInfo, []QaInfo) {
	return []TextInfo{
			{ID: 1, Text: "What is vector search?"},
			{ID: 2, Text: "What is reranking?"},
		},
		[]TextInfo{
			{ID: 10, Text: "Vector search maps text into a high-dimensional space."},
			{ID: 11, Text: "Reranking reorders retrieval results for precision."},
		},
		[]TextInfo{
			{ID: 100, Text: "Vector search maps text into vectors."},
			{ID: 101, Text: "Reranking improves precision."},
		},
		[]RelsInfo{
			{QID: 1, PID: 10},
			{QID: 2, PID: 11},
		},
		[]QaInfo{
			{QID: 1, AID: 100},
			{QID: 2, AID: 101},
		}
}

func TestDataset_LoadDefault(t *testing.T) {
	oldRoot := datasetRoot
	datasetRoot = filepath.Join("..", "..", "..", "dataset")
	t.Cleanup(func() { datasetRoot = oldRoot })

	svc := NewDatasetService()
	loaded, err := svc.GetDatasetByID(context.Background(), "default")
	require.NoError(t, err)
	assert.NotEmpty(t, loaded.SHA256)
	assert.Greater(t, loaded.SampleCount, 0)
	assert.Len(t, loaded.Pairs, loaded.SampleCount)
}

func TestDataset_LoadEnterpriseRAGSample(t *testing.T) {
	oldRoot := datasetRoot
	datasetRoot = filepath.Join("..", "..", "..", "dataset")
	t.Cleanup(func() { datasetRoot = oldRoot })

	svc := NewDatasetService()
	loaded, err := svc.GetDatasetByID(context.Background(), "enterprise_rag")
	require.NoError(t, err)
	assert.Equal(t, 50, loaded.SampleCount)
	assert.NotEmpty(t, loaded.SHA256)
	assert.Equal(t, "df62c155c94c07f72a85cf1176a3a603d3f61011a3d315a0ee8c750a8544f3f7", loaded.SHA256)
	assert.Len(t, loaded.Pairs, 50)
	assert.Equal(t, 1, loaded.Pairs[0].QID)
}

func TestDataset_ListAvailableDatasets(t *testing.T) {
	oldRoot := datasetRoot
	datasetRoot = filepath.Join("..", "..", "..", "dataset")
	t.Cleanup(func() { datasetRoot = oldRoot })

	datasets, err := NewDatasetService().ListAvailableDatasets(context.Background())
	require.NoError(t, err)
	ids := make([]string, 0, len(datasets))
	for _, dataset := range datasets {
		ids = append(ids, dataset.ID)
	}
	assert.Contains(t, ids, "default")
	assert.Contains(t, ids, "demo")
	assert.Contains(t, ids, "enterprise_rag")
}

func TestDataset_InvalidIDRejected(t *testing.T) {
	svc := NewDatasetService()
	for _, id := range []string{"../escape", "/abs/path", "a b", "demo;rm"} {
		_, err := svc.GetDatasetByID(context.Background(), id)
		require.ErrorIs(t, err, ErrInvalidDataset, "id: %s", id)
	}
}

func TestDataset_NotFound(t *testing.T) {
	svc := NewDatasetService()
	_, err := svc.GetDatasetByID(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, ErrDatasetNotFound)
}

func TestDataset_CustomLoadAndHashStable(t *testing.T) {
	dir := t.TempDir()
	queries, corpus, answers, qrels, qas := validDemoFiles()
	writeDemoDataset(t, dir, queries, corpus, answers, qrels, qas)

	first, err := loadDatasetDir(dir)
	require.NoError(t, err)
	second, err := loadDatasetDir(dir)
	require.NoError(t, err)

	assert.Equal(t, first.SHA256, second.SHA256)
	assert.Equal(t, 2, first.SampleCount)
	require.NotEmpty(t, first.Pairs)
	assert.Equal(t, 1, first.Pairs[0].QID)
}

func TestDataset_HashChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	queries, corpus, answers, qrels, qas := validDemoFiles()
	writeDemoDataset(t, dir, queries, corpus, answers, qrels, qas)
	before, err := loadDatasetDir(dir)
	require.NoError(t, err)

	queries[0].Text = "What is dense vector search?"
	writeDemoDataset(t, dir, queries, corpus, answers, qrels, qas)
	after, err := loadDatasetDir(dir)
	require.NoError(t, err)
	assert.NotEqual(t, before.SHA256, after.SHA256)
}

func TestDataset_MissingFileRejected(t *testing.T) {
	dir := t.TempDir()
	queries, corpus, answers, _, qas := validDemoFiles()
	writeDemoDataset(t, dir, queries, corpus, answers, nil, qas)

	_, err := loadDatasetDir(dir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidDataset))
}

func TestDataset_BadReferencesRejected(t *testing.T) {
	dir := t.TempDir()
	queries, corpus, answers, qrels, qas := validDemoFiles()
	qrels = append(qrels, RelsInfo{QID: 1, PID: 999})
	writeDemoDataset(t, dir, queries, corpus, answers, qrels, qas)

	_, err := loadDatasetDir(dir)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidDataset))
}
