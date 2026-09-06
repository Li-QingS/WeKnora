package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestWikiEvaluationReportUsesOneStableDetail(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	score := 0.91
	detail := &types.WikiEvaluationDetail{
		Run: &types.EvaluationRun{
			ID: "wiki-run", DatasetID: "enterprise_rag", EvaluationType: types.EvaluationTypeWiki,
			Status: types.EvaluationStatueSuccess, Stage: types.EvaluationStageCompleted,
			StartTime: now, CreatedAt: now, UpdatedAt: now,
		},
		Params: &types.WikiEvaluationOptions{DatasetID: "enterprise_rag", SemanticThreshold: 0.8},
		ConfigSnapshot: &types.WikiEvaluationConfigSnapshot{
			Dataset:        types.DatasetSnapshot{ID: "enterprise_rag", SHA256: "dataset-sha", SampleCount: 85},
			Gold:           types.WikiGoldSnapshot{SchemaVersion: "1", ContentSHA256: "gold-sha", NodeCount: 2, EdgeCount: 1},
			ChatModel:      types.ModelSnapshot{ID: "chat-1", Name: "Chat"},
			EmbeddingModel: types.ModelSnapshot{ID: "embed-1", Name: "Embed"},
			Threshold:      0.8,
		},
		Metric: &types.WikiEvaluationMetric{
			Entity:  types.WikiNodeMetric{GoldTotal: 1, ExactMatched: 1, Coverage: 1},
			Concept: types.WikiNodeMetric{GoldTotal: 1, SemanticMatched: 1, Coverage: 1},
			Overall: types.WikiNodeMetric{GoldTotal: 2, ExactMatched: 1, SemanticMatched: 1, Coverage: 1},
			Graph:   types.WikiGraphMetric{Correct: 1, Precision: 1, Recall: 1, F1: 1, Scorable: true},
		},
		Result: &types.WikiEvaluationResult{
			NodeMatches: []types.WikiNodeMatch{
				{GoldNodeID: "z", GoldType: types.WikiPageTypeEntity, GoldName: "Z", Method: "semantic", Score: &score},
				{GoldNodeID: "a", GoldType: types.WikiPageTypeConcept, GoldName: "A", Method: "exact"},
			},
			CorrectEdges: []types.WikiEdgeRef{{Source: "z", Target: "a"}, {Source: "a", Target: "z"}},
		},
	}
	detail.Result.Metric = *detail.Metric

	renderer := NewWikiEvaluationReportRenderer()
	jsonReport, err := renderer.JSON(detail)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(jsonReport), "\n"))
	var decoded types.WikiEvaluationDetail
	require.NoError(t, json.Unmarshal(jsonReport, &decoded))
	require.Equal(t, detail.Metric, decoded.Metric)
	require.Len(t, decoded.Result.NodeMatches, 2)
	require.Equal(t, "a", decoded.Result.NodeMatches[0].GoldNodeID)
	require.Equal(t, "a", decoded.Result.CorrectEdges[0].Source)

	markdown, err := renderer.Markdown(detail)
	require.NoError(t, err)
	text := string(markdown)
	require.Contains(t, text, "| Overall | 2 | 1 | 1 | 0 | 1.000000 |")
	require.Contains(t, text, "## Correct edges (2)")
	require.Contains(t, text, "gold-sha")

	reordered := *detail
	resultCopy := *detail.Result
	resultCopy.NodeMatches = []types.WikiNodeMatch{detail.Result.NodeMatches[1], detail.Result.NodeMatches[0]}
	resultCopy.CorrectEdges = []types.WikiEdgeRef{detail.Result.CorrectEdges[1], detail.Result.CorrectEdges[0]}
	reordered.Result = &resultCopy
	secondJSON, err := renderer.JSON(&reordered)
	require.NoError(t, err)
	require.Equal(t, jsonReport, secondJSON)
	secondMarkdown, err := renderer.Markdown(&reordered)
	require.NoError(t, err)
	require.Equal(t, markdown, secondMarkdown)
}

func TestWikiEvaluationReportFailureHasNoFakeMetrics(t *testing.T) {
	detail := &types.WikiEvaluationDetail{Run: &types.EvaluationRun{
		ID: "failed", DatasetID: "enterprise_rag", EvaluationType: types.EvaluationTypeWiki,
		Status: types.EvaluationStatueFailed, Stage: types.EvaluationStageCleaningUp,
		FailureStage: types.EvaluationStageGenerating, ErrMsg: "generation failed",
	}}

	markdown, err := NewWikiEvaluationReportRenderer().Markdown(detail)
	require.NoError(t, err)
	require.Contains(t, string(markdown), "generation failed")
	require.Contains(t, string(markdown), "No quality metric was produced")
	require.NotContains(t, string(markdown), "## Metrics")
}

func TestWikiEvaluationReportNoScorableEdges(t *testing.T) {
	detail := &types.WikiEvaluationDetail{
		Run:    &types.EvaluationRun{ID: "empty", Status: types.EvaluationStatueSuccess},
		Metric: &types.WikiEvaluationMetric{Graph: types.WikiGraphMetric{Scorable: false, Note: "no induced Gold edges"}},
		Result: &types.WikiEvaluationResult{},
	}
	markdown, err := NewWikiEvaluationReportRenderer().Markdown(detail)
	require.NoError(t, err)
	require.Contains(t, string(markdown), "no induced Gold edges")
	require.Contains(t, string(markdown), "| Directed edges | 0 | 0 | 0 | 0.000000 |")
}
