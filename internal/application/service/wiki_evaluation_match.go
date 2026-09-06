package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const semanticAssignmentEpsilon = 1e-12

type wikiEvaluationScorer struct {
	embeddings interfaces.WikiEmbeddingProvider
}

func NewWikiEvaluationScorer(embeddings interfaces.WikiEmbeddingProvider) interfaces.WikiEvaluationScorer {
	return &wikiEvaluationScorer{embeddings: embeddings}
}

func (s *wikiEvaluationScorer) ScoreNodes(
	ctx context.Context,
	gold *types.WikiGold,
	pages []types.WikiEvaluationPage,
	embeddingModelID string,
	threshold float64,
) (*types.WikiNodeScore, error) {
	if gold == nil {
		return nil, errors.New("wiki evaluation: gold is required")
	}
	if threshold < 0 || threshold > 1 {
		return nil, fmt.Errorf("wiki evaluation: semantic threshold %.4f is outside [0,1]", threshold)
	}

	goldNodes := append([]types.WikiGoldNode(nil), gold.Nodes...)
	pageNodes := make([]types.WikiEvaluationPage, 0, len(pages))
	for _, page := range pages {
		if page.Type == types.WikiPageTypeEntity || page.Type == types.WikiPageTypeConcept {
			pageNodes = append(pageNodes, page)
		}
	}
	sort.Slice(goldNodes, func(i, j int) bool {
		if goldNodes[i].Type != goldNodes[j].Type {
			return goldNodes[i].Type < goldNodes[j].Type
		}
		return goldNodes[i].ID < goldNodes[j].ID
	})
	sort.Slice(pageNodes, func(i, j int) bool {
		if pageNodes[i].Type != pageNodes[j].Type {
			return pageNodes[i].Type < pageNodes[j].Type
		}
		return pageNodes[i].Slug < pageNodes[j].Slug
	})

	matchedGold := make(map[int]int)
	matchedPages := make(map[int]int)
	methods := make(map[int]string)
	scores := make(map[int]float64)
	for _, nodeType := range []string{types.WikiPageTypeEntity, types.WikiPageTypeConcept} {
		goldIndexes := indexesForGoldType(goldNodes, nodeType, nil)
		pageIndexes := indexesForPageType(pageNodes, nodeType, nil)
		for goldIndex, pageIndex := range exactNodeMatches(goldNodes, pageNodes, goldIndexes, pageIndexes) {
			matchedGold[goldIndex] = pageIndex
			matchedPages[pageIndex] = goldIndex
			methods[goldIndex] = "exact"
		}
	}

	unmatchedGold := func(index int) bool {
		_, ok := matchedGold[index]
		return !ok
	}
	unmatchedPage := func(index int) bool {
		_, ok := matchedPages[index]
		return !ok
	}
	for _, nodeType := range []string{types.WikiPageTypeEntity, types.WikiPageTypeConcept} {
		goldIndexes := indexesForGoldType(goldNodes, nodeType, unmatchedGold)
		pageIndexes := indexesForPageType(pageNodes, nodeType, unmatchedPage)
		semantic, err := s.semanticNodeMatches(ctx, goldNodes, pageNodes, goldIndexes, pageIndexes, embeddingModelID, threshold)
		if err != nil {
			return nil, err
		}
		for _, match := range semantic {
			matchedGold[match.goldIndex] = match.pageIndex
			matchedPages[match.pageIndex] = match.goldIndex
			methods[match.goldIndex] = "semantic"
			scores[match.goldIndex] = match.score
		}
	}

	result := &types.WikiNodeScore{PageToGoldID: make(map[string]string)}
	for goldIndex, node := range goldNodes {
		match := types.WikiNodeMatch{
			GoldNodeID: node.ID,
			GoldType:   node.Type,
			GoldName:   node.Name,
			Method:     "unmatched",
			Reason:     "no exact or semantic candidate met the threshold",
		}
		if pageIndex, ok := matchedGold[goldIndex]; ok {
			page := pageNodes[pageIndex]
			match.PageID = page.ID
			match.PageSlug = page.Slug
			match.PageTitle = page.Title
			match.Method = methods[goldIndex]
			match.Reason = ""
			if match.Method == "semantic" {
				score := scores[goldIndex]
				match.Score = &score
			}
			result.PageToGoldID[page.Slug] = node.ID
		}
		result.Matches = append(result.Matches, match)
	}
	result.Entity = nodeMetric(result.Matches, types.WikiPageTypeEntity)
	result.Concept = nodeMetric(result.Matches, types.WikiPageTypeConcept)
	result.Overall = nodeMetric(result.Matches, "")
	return result, nil
}

func normalizeWikiEvaluationName(value string) string {
	value = norm.NFKC.String(value)
	value = cases.Fold().String(value)
	value = strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) })
	return strings.Join(strings.Fields(value), " ")
}

func goldNodeLabels(node types.WikiGoldNode) []string {
	return normalizedUniqueLabels(append([]string{node.Name}, node.Aliases...))
}

func pageNodeLabels(page types.WikiEvaluationPage) []string {
	return normalizedUniqueLabels(append([]string{page.Title}, page.Aliases...))
}

func normalizedUniqueLabels(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	labels := make([]string, 0, len(values))
	for _, value := range values {
		label := normalizeWikiEvaluationName(value)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

func labelsIntersect(left, right []string) bool {
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			return true
		case left[i] < right[j]:
			i++
		default:
			j++
		}
	}
	return false
}

func exactNodeMatches(
	gold []types.WikiGoldNode,
	pages []types.WikiEvaluationPage,
	goldIndexes []int,
	pageIndexes []int,
) map[int]int {
	adjacency := make(map[int][]int, len(goldIndexes))
	for _, goldIndex := range goldIndexes {
		goldLabels := goldNodeLabels(gold[goldIndex])
		for _, pageIndex := range pageIndexes {
			if labelsIntersect(goldLabels, pageNodeLabels(pages[pageIndex])) {
				adjacency[goldIndex] = append(adjacency[goldIndex], pageIndex)
			}
		}
	}
	pageOwner := make(map[int]int)
	var visit func(int, map[int]bool) bool
	visit = func(goldIndex int, seen map[int]bool) bool {
		for _, pageIndex := range adjacency[goldIndex] {
			if seen[pageIndex] {
				continue
			}
			seen[pageIndex] = true
			owner, occupied := pageOwner[pageIndex]
			if !occupied || visit(owner, seen) {
				pageOwner[pageIndex] = goldIndex
				return true
			}
		}
		return false
	}
	for _, goldIndex := range goldIndexes {
		visit(goldIndex, make(map[int]bool))
	}
	matches := make(map[int]int, len(pageOwner))
	for pageIndex, goldIndex := range pageOwner {
		matches[goldIndex] = pageIndex
	}
	return matches
}

type semanticNodeMatch struct {
	goldIndex int
	pageIndex int
	score     float64
}

func (s *wikiEvaluationScorer) semanticNodeMatches(
	ctx context.Context,
	gold []types.WikiGoldNode,
	pages []types.WikiEvaluationPage,
	goldIndexes []int,
	pageIndexes []int,
	modelID string,
	threshold float64,
) ([]semanticNodeMatch, error) {
	if len(goldIndexes) == 0 || len(pageIndexes) == 0 {
		return nil, nil
	}
	if s.embeddings == nil {
		return nil, errors.New("wiki evaluation: embedding provider is required for unmatched nodes")
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, errors.New("wiki evaluation: embedding model is required for unmatched nodes")
	}

	allLabels := make(map[string]struct{})
	goldLabels := make(map[int][]string, len(goldIndexes))
	pageLabels := make(map[int][]string, len(pageIndexes))
	for _, index := range goldIndexes {
		goldLabels[index] = goldNodeLabels(gold[index])
		for _, label := range goldLabels[index] {
			allLabels[label] = struct{}{}
		}
	}
	for _, index := range pageIndexes {
		pageLabels[index] = pageNodeLabels(pages[index])
		for _, label := range pageLabels[index] {
			allLabels[label] = struct{}{}
		}
	}
	texts := make([]string, 0, len(allLabels))
	for label := range allLabels {
		texts = append(texts, label)
	}
	sort.Strings(texts)
	vectors, err := s.embeddings.Embed(ctx, modelID, texts)
	if err != nil {
		return nil, fmt.Errorf("wiki evaluation: embed node labels: %w", err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("wiki evaluation: embedding count %d does not match text count %d", len(vectors), len(texts))
	}
	vectorByText := make(map[string][]float32, len(texts))
	dimension := -1
	for i, vector := range vectors {
		if len(vector) == 0 {
			return nil, fmt.Errorf("wiki evaluation: embedding for %q is empty", texts[i])
		}
		if dimension < 0 {
			dimension = len(vector)
		} else if len(vector) != dimension {
			return nil, fmt.Errorf("wiki evaluation: embedding dimension mismatch for %q", texts[i])
		}
		vectorByText[texts[i]] = vector
	}

	weights := make([][]float64, len(goldIndexes))
	valid := make([][]bool, len(goldIndexes))
	for row, goldIndex := range goldIndexes {
		weights[row] = make([]float64, len(pageIndexes))
		valid[row] = make([]bool, len(pageIndexes))
		for col, pageIndex := range pageIndexes {
			score, scoreErr := maximumLabelCosine(goldLabels[goldIndex], pageLabels[pageIndex], vectorByText)
			if scoreErr != nil {
				return nil, scoreErr
			}
			weights[row][col] = score
			valid[row][col] = score >= threshold
		}
	}
	assignment := maximumWeightAssignment(weights, valid)
	matches := make([]semanticNodeMatch, 0, len(assignment))
	for row, col := range assignment {
		if col < 0 || col >= len(pageIndexes) || !valid[row][col] {
			continue
		}
		matches = append(matches, semanticNodeMatch{
			goldIndex: goldIndexes[row],
			pageIndex: pageIndexes[col],
			score:     weights[row][col],
		})
	}
	return matches, nil
}

func maximumLabelCosine(left, right []string, vectors map[string][]float32) (float64, error) {
	best := -1.0
	for _, leftLabel := range left {
		for _, rightLabel := range right {
			score, err := wikiEvaluationCosineSimilarity(vectors[leftLabel], vectors[rightLabel])
			if err != nil {
				return 0, err
			}
			if score > best {
				best = score
			}
		}
	}
	if best < 0 {
		return 0, nil
	}
	return best, nil
}

func wikiEvaluationCosineSimilarity(left, right []float32) (float64, error) {
	if len(left) == 0 || len(left) != len(right) {
		return 0, errors.New("wiki evaluation: incompatible embedding vectors")
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0, errors.New("wiki evaluation: zero embedding vector")
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm)), nil
}

// maximumWeightAssignment returns a column per row, or -1 when the chosen
// square assignment is a dummy/invalid edge. It uses the Hungarian algorithm.
func maximumWeightAssignment(weights [][]float64, valid [][]bool) []int {
	rows := len(weights)
	if rows == 0 {
		return nil
	}
	cols := len(weights[0])
	result := make([]int, rows)
	for i := range result {
		result[i] = -1
	}
	if cols == 0 {
		return result
	}
	size := max(rows, cols)
	augmented := make([][]float64, size)
	maxWeight := 0.0
	for i := range augmented {
		augmented[i] = make([]float64, size)
		for j := range augmented[i] {
			if i < rows && j < cols {
				if valid[i][j] {
					augmented[i][j] = weights[i][j] + semanticAssignmentEpsilon
				} else {
					augmented[i][j] = -1
				}
			}
			if augmented[i][j] > maxWeight {
				maxWeight = augmented[i][j]
			}
		}
	}

	u := make([]float64, size+1)
	v := make([]float64, size+1)
	p := make([]int, size+1)
	way := make([]int, size+1)
	for i := 1; i <= size; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, size+1)
		used := make([]bool, size+1)
		for j := 1; j <= size; j++ {
			minv[j] = math.Inf(1)
		}
		for {
			used[j0] = true
			i0 := p[j0]
			delta := math.Inf(1)
			j1 := 0
			for j := 1; j <= size; j++ {
				if used[j] {
					continue
				}
				cost := maxWeight - augmented[i0-1][j-1]
				cur := cost - u[i0] - v[j]
				if cur < minv[j]-semanticAssignmentEpsilon {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta-semanticAssignmentEpsilon ||
					(math.Abs(minv[j]-delta) <= semanticAssignmentEpsilon && (j1 == 0 || j < j1)) {
					delta = minv[j]
					j1 = j
				}
			}
			for j := 0; j <= size; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}
	for col := 1; col <= size; col++ {
		row := p[col] - 1
		candidate := col - 1
		if row >= 0 && row < rows && candidate < cols && valid[row][candidate] {
			result[row] = candidate
		}
	}
	return result
}

func indexesForGoldType(nodes []types.WikiGoldNode, nodeType string, include func(int) bool) []int {
	var indexes []int
	for index, node := range nodes {
		if node.Type == nodeType && (include == nil || include(index)) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func indexesForPageType(nodes []types.WikiEvaluationPage, nodeType string, include func(int) bool) []int {
	var indexes []int
	for index, node := range nodes {
		if node.Type == nodeType && (include == nil || include(index)) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func nodeMetric(matches []types.WikiNodeMatch, nodeType string) types.WikiNodeMetric {
	metric := types.WikiNodeMetric{}
	for _, match := range matches {
		if nodeType != "" && match.GoldType != nodeType {
			continue
		}
		metric.GoldTotal++
		switch match.Method {
		case "exact":
			metric.ExactMatched++
		case "semantic":
			metric.SemanticMatched++
		default:
			metric.Unmatched++
		}
	}
	if metric.GoldTotal > 0 {
		metric.Coverage = float64(metric.ExactMatched+metric.SemanticMatched) / float64(metric.GoldTotal)
	}
	return metric
}
