package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWikiEmbeddingProvider struct {
	vectors map[string][]float32
	texts   []string
	modelID string
	err     error
}

func (f *fakeWikiEmbeddingProvider) Embed(_ context.Context, modelID string, texts []string) ([][]float32, error) {
	f.modelID = modelID
	f.texts = append([]string(nil), texts...)
	if f.err != nil {
		return nil, f.err
	}
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = append([]float32(nil), f.vectors[text]...)
	}
	return result, nil
}

func TestWikiEvaluationNormalizeName(t *testing.T) {
	tests := map[string]string{
		"  ＡＣＭＥ， ":        "acme",
		"Hello\t  World!": "hello world",
		"（RAG）":           "rag",
		"!!!":             "",
	}
	for input, want := range tests {
		assert.Equal(t, want, normalizeWikiEvaluationName(input), input)
	}
}

func TestWikiNodeExactMatchMaximumCardinalityAndType(t *testing.T) {
	gold := &types.WikiGold{Nodes: []types.WikiGoldNode{
		{ID: "entity:1", Type: types.WikiPageTypeEntity, Name: "Alpha", Aliases: []string{"Beta"}},
		{ID: "entity:2", Type: types.WikiPageTypeEntity, Name: "Alpha"},
		{ID: "concept:1", Type: types.WikiPageTypeConcept, Name: "Alpha"},
	}}
	pages := []types.WikiEvaluationPage{
		{ID: "p1", Slug: "entity/alpha", Type: types.WikiPageTypeEntity, Title: "Alpha"},
		{ID: "p2", Slug: "entity/beta", Type: types.WikiPageTypeEntity, Title: "Beta"},
		{ID: "p3", Slug: "concept/other", Type: types.WikiPageTypeConcept, Title: "Other"},
	}
	embedder := &fakeWikiEmbeddingProvider{vectors: map[string][]float32{
		"alpha": {1, 0},
		"other": {0, 1},
	}}
	score, err := NewWikiEvaluationScorer(embedder).ScoreNodes(context.Background(), gold, pages, "embed", 0.9)
	require.NoError(t, err)
	assert.Equal(t, 2, score.Entity.ExactMatched)
	assert.Equal(t, 0, score.Concept.ExactMatched)
	assert.Equal(t, 1, score.Concept.Unmatched)
	assert.Equal(t, "entity:2", score.PageToGoldID["entity/alpha"])
	assert.Equal(t, "entity:1", score.PageToGoldID["entity/beta"])
}

func TestWikiNodeSemanticMatchAndCoverage(t *testing.T) {
	gold := &types.WikiGold{Nodes: []types.WikiGoldNode{
		{ID: "entity:acme", Type: types.WikiPageTypeEntity, Name: "Acme Corp"},
		{ID: "concept:retrieval", Type: types.WikiPageTypeConcept, Name: "semantic retrieval"},
		{ID: "concept:missing", Type: types.WikiPageTypeConcept, Name: "missing concept"},
	}}
	pages := []types.WikiEvaluationPage{
		{ID: "p1", Slug: "entity/acme", Type: types.WikiPageTypeEntity, Title: "ACME CORP."},
		{ID: "p2", Slug: "concept/vector-search", Type: types.WikiPageTypeConcept, Title: "vector search"},
		{ID: "p3", Slug: "concept/extra", Type: types.WikiPageTypeConcept, Title: "extra page"},
	}
	embedder := &fakeWikiEmbeddingProvider{vectors: map[string][]float32{
		"semantic retrieval": {1, 0},
		"missing concept":    {0, 1},
		"vector search":      {0.9, 0.1},
		"extra page":         {0.7, 0.7},
	}}
	score, err := NewWikiEvaluationScorer(embedder).ScoreNodes(context.Background(), gold, pages, "embed-1", 0.95)
	require.NoError(t, err)
	assert.Equal(t, "embed-1", embedder.modelID)
	assert.Equal(t, 1, score.Entity.ExactMatched)
	assert.Equal(t, 1, score.Concept.SemanticMatched)
	assert.Equal(t, 1, score.Concept.Unmatched)
	assert.InDelta(t, 0.5, score.Concept.Coverage, 1e-9)
	assert.InDelta(t, 2.0/3.0, score.Overall.Coverage, 1e-9)
	var semanticScore *float64
	for _, match := range score.Matches {
		if match.GoldNodeID == "concept:retrieval" {
			semanticScore = match.Score
		}
	}
	require.NotNil(t, semanticScore)
	assert.GreaterOrEqual(t, *semanticScore, 0.95)
}

func TestWikiMaximumWeightAssignmentUsesGlobalOptimum(t *testing.T) {
	weights := [][]float64{{0.9, 0.8}, {0.85, 0.1}}
	valid := [][]bool{{true, true}, {true, true}}
	assert.Equal(t, []int{1, 0}, maximumWeightAssignment(weights, valid))

	valid = [][]bool{{false, true}, {false, false}}
	assert.Equal(t, []int{1, -1}, maximumWeightAssignment(weights, valid))
}

func TestWikiNodeSemanticTextsAreDeduplicated(t *testing.T) {
	gold := &types.WikiGold{Nodes: []types.WikiGoldNode{{
		ID: "concept:a", Type: types.WikiPageTypeConcept, Name: "Same", Aliases: []string{"same"},
	}}}
	pages := []types.WikiEvaluationPage{{
		ID: "p", Slug: "concept/b", Type: types.WikiPageTypeConcept, Title: "Different", Aliases: []string{"different"},
	}}
	embedder := &fakeWikiEmbeddingProvider{vectors: map[string][]float32{
		"same": {1, 0}, "different": {0, 1},
	}}
	_, err := NewWikiEvaluationScorer(embedder).ScoreNodes(context.Background(), gold, pages, "embed", 0.8)
	require.NoError(t, err)
	assert.Equal(t, []string{"different", "same"}, embedder.texts)
}
