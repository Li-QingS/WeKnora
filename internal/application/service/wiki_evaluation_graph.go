package service

import (
	"sort"

	"github.com/Tencent/WeKnora/internal/types"
)

func (s *wikiEvaluationScorer) ScoreGraph(
	gold *types.WikiGold,
	pages []types.WikiEvaluationPage,
	nodes *types.WikiNodeScore,
) *types.WikiGraphScore {
	result := &types.WikiGraphScore{}
	if gold == nil || nodes == nil {
		result.Metric.Note = "no scorable edges"
		return result
	}

	matchedGold := make(map[string]struct{}, len(nodes.PageToGoldID))
	for _, goldID := range nodes.PageToGoldID {
		matchedGold[goldID] = struct{}{}
	}
	goldEdges := make(map[string]types.WikiEdgeRef)
	for _, edge := range gold.Edges {
		_, sourceMatched := matchedGold[edge.Source]
		_, targetMatched := matchedGold[edge.Target]
		if !sourceMatched || !targetMatched {
			result.UnscoredEdges = append(result.UnscoredEdges, types.WikiEdgeRef{
				Source: edge.Source,
				Target: edge.Target,
				Reason: "gold edge endpoint was not matched",
			})
			continue
		}
		ref := types.WikiEdgeRef{Source: edge.Source, Target: edge.Target}
		goldEdges[wikiEdgeKey(ref.Source, ref.Target)] = ref
	}

	pageBySlug := make(map[string]types.WikiEvaluationPage, len(pages))
	for _, page := range pages {
		pageBySlug[page.Slug] = page
	}
	generatedEdges := make(map[string]types.WikiEdgeRef)
	seenUnscored := make(map[string]struct{})
	for _, page := range pages {
		sourceGold, sourceMatched := nodes.PageToGoldID[page.Slug]
		for _, targetSlug := range page.OutLinks {
			targetPage, targetExists := pageBySlug[targetSlug]
			targetGold, targetMatched := nodes.PageToGoldID[targetSlug]
			if !sourceMatched || !targetExists || !targetMatched {
				reason := "generated edge endpoint was not matched"
				if !targetExists {
					reason = "generated edge target slug does not reference a scored page"
				} else if targetPage.Type != types.WikiPageTypeEntity && targetPage.Type != types.WikiPageTypeConcept {
					reason = "generated edge target is not an entity or concept page"
				}
				key := wikiEdgeKey(page.Slug, targetSlug) + "\x00" + reason
				if _, exists := seenUnscored[key]; !exists {
					seenUnscored[key] = struct{}{}
					result.UnscoredEdges = append(result.UnscoredEdges, types.WikiEdgeRef{
						Source: page.Slug,
						Target: targetSlug,
						Reason: reason,
					})
				}
				continue
			}
			ref := types.WikiEdgeRef{Source: sourceGold, Target: targetGold}
			generatedEdges[wikiEdgeKey(ref.Source, ref.Target)] = ref
		}
	}

	for key, edge := range goldEdges {
		if _, ok := generatedEdges[key]; ok {
			result.CorrectEdges = append(result.CorrectEdges, edge)
		} else {
			result.MissingEdges = append(result.MissingEdges, edge)
		}
	}
	for key, edge := range generatedEdges {
		if _, ok := goldEdges[key]; !ok {
			result.ExtraEdges = append(result.ExtraEdges, edge)
		}
	}
	sortWikiEdges(result.CorrectEdges)
	sortWikiEdges(result.MissingEdges)
	sortWikiEdges(result.ExtraEdges)
	sortWikiEdges(result.UnscoredEdges)
	result.Metric.Correct = len(result.CorrectEdges)
	result.Metric.Missing = len(result.MissingEdges)
	result.Metric.Extra = len(result.ExtraEdges)
	precisionDenominator := result.Metric.Correct + result.Metric.Extra
	recallDenominator := result.Metric.Correct + result.Metric.Missing
	if precisionDenominator > 0 {
		result.Metric.Precision = float64(result.Metric.Correct) / float64(precisionDenominator)
	}
	if recallDenominator > 0 {
		result.Metric.Recall = float64(result.Metric.Correct) / float64(recallDenominator)
	}
	if result.Metric.Precision+result.Metric.Recall > 0 {
		result.Metric.F1 = 2 * result.Metric.Precision * result.Metric.Recall /
			(result.Metric.Precision + result.Metric.Recall)
	}
	result.Metric.Scorable = precisionDenominator > 0 || recallDenominator > 0
	if !result.Metric.Scorable {
		result.Metric.Note = "no scorable edges"
	}
	return result
}

func wikiEdgeKey(source, target string) string {
	return source + "\x00" + target
}

func sortWikiEdges(edges []types.WikiEdgeRef) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Reason < edges[j].Reason
	})
}
