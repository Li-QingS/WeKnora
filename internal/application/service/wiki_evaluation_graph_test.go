package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWikiGraphScoreDirectedEdgesAndInducedBoundary(t *testing.T) {
	gold := &types.WikiGold{
		Nodes: []types.WikiGoldNode{
			{ID: "a", Type: types.WikiPageTypeEntity, Name: "A"},
			{ID: "b", Type: types.WikiPageTypeConcept, Name: "B"},
			{ID: "c", Type: types.WikiPageTypeConcept, Name: "C"},
		},
		Edges: []types.WikiGoldEdge{
			{Source: "a", Target: "b"},
			{Source: "b", Target: "a"},
			{Source: "a", Target: "c"},
		},
	}
	pages := []types.WikiEvaluationPage{
		{Slug: "page/a", Type: types.WikiPageTypeEntity, OutLinks: []string{"page/b", "page/b", "page/missing"}},
		{Slug: "page/b", Type: types.WikiPageTypeConcept, OutLinks: []string{"page/b"}},
	}
	nodes := &types.WikiNodeScore{PageToGoldID: map[string]string{"page/a": "a", "page/b": "b"}}
	result := (&wikiEvaluationScorer{}).ScoreGraph(gold, pages, nodes)

	assert.Equal(t, 1, result.Metric.Correct)
	assert.Equal(t, 1, result.Metric.Missing)
	assert.Equal(t, 1, result.Metric.Extra)
	assert.InDelta(t, 0.5, result.Metric.Precision, 1e-9)
	assert.InDelta(t, 0.5, result.Metric.Recall, 1e-9)
	assert.InDelta(t, 0.5, result.Metric.F1, 1e-9)
	assert.True(t, result.Metric.Scorable)
	require.Len(t, result.UnscoredEdges, 2)
}

func TestWikiGraphScoreNoScorableEdges(t *testing.T) {
	gold := &types.WikiGold{Edges: []types.WikiGoldEdge{{Source: "a", Target: "b"}}}
	result := (&wikiEvaluationScorer{}).ScoreGraph(gold, nil, &types.WikiNodeScore{PageToGoldID: map[string]string{}})
	assert.False(t, result.Metric.Scorable)
	assert.Zero(t, result.Metric.Precision)
	assert.Equal(t, "no scorable edges", result.Metric.Note)
	require.Len(t, result.UnscoredEdges, 1)
}

func TestWikiGraphScoreSelfLoop(t *testing.T) {
	gold := &types.WikiGold{Edges: []types.WikiGoldEdge{{Source: "a", Target: "a"}}}
	pages := []types.WikiEvaluationPage{{Slug: "page/a", Type: types.WikiPageTypeEntity, OutLinks: []string{"page/a"}}}
	nodes := &types.WikiNodeScore{PageToGoldID: map[string]string{"page/a": "a"}}
	result := (&wikiEvaluationScorer{}).ScoreGraph(gold, pages, nodes)
	assert.Equal(t, 1, result.Metric.Correct)
	assert.Equal(t, 1.0, result.Metric.F1)
}
