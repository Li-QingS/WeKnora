package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

type wikiEvaluationReportRenderer struct{}

func NewWikiEvaluationReportRenderer() *wikiEvaluationReportRenderer {
	return &wikiEvaluationReportRenderer{}
}

func (r *wikiEvaluationReportRenderer) JSON(detail *types.WikiEvaluationDetail) ([]byte, error) {
	normalized, err := normalizedWikiEvaluationDetail(detail)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal wiki evaluation report: %w", err)
	}
	return append(data, '\n'), nil
}

func (r *wikiEvaluationReportRenderer) Markdown(detail *types.WikiEvaluationDetail) ([]byte, error) {
	normalized, err := normalizedWikiEvaluationDetail(detail)
	if err != nil {
		return nil, err
	}
	if normalized.Run == nil {
		return nil, fmt.Errorf("wiki evaluation report requires a run")
	}

	var out bytes.Buffer
	run := normalized.Run
	fmt.Fprintln(&out, "# Wiki Evaluation Report")
	fmt.Fprintln(&out)
	markdownKeyValueTable(&out, [][2]string{
		{"Run ID", run.ID},
		{"Status", evaluationStatusLabel(run.Status)},
		{"Stage", string(run.Stage)},
		{"Failure stage", string(run.FailureStage)},
		{"Dataset", run.DatasetID},
		{"Started at", formatReportTime(run.StartTime)},
		{"Finished at", formatOptionalReportTime(run.FinishedAt)},
		{"Error", run.ErrMsg},
	})

	if normalized.ConfigSnapshot != nil {
		config := normalized.ConfigSnapshot
		fmt.Fprintln(&out, "## Configuration")
		fmt.Fprintln(&out)
		markdownKeyValueTable(&out, [][2]string{
			{"Dataset SHA-256", config.Dataset.SHA256},
			{"Documents", strconv.Itoa(config.Dataset.SampleCount)},
			{"Gold schema", config.Gold.SchemaVersion},
			{"Gold SHA-256", config.Gold.ContentSHA256},
			{"Gold nodes", strconv.Itoa(config.Gold.NodeCount)},
			{"Gold edges", strconv.Itoa(config.Gold.EdgeCount)},
			{"Chat model", reportModel(config.ChatModel)},
			{"Embedding model", reportModel(config.EmbeddingModel)},
			{"Semantic threshold", formatReportFloat(config.Threshold)},
			{"App version", config.Version.AppVersion},
			{"Git commit", config.Version.GitCommit},
			{"Git dirty", strconv.FormatBool(config.Version.GitDirty)},
			{"Go version", config.Version.GoVersion},
		})
	}

	if normalized.Metric == nil {
		fmt.Fprintln(&out, "## Result")
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "No quality metric was produced for this run.")
		return out.Bytes(), nil
	}

	metric := normalized.Metric
	fmt.Fprintln(&out, "## Metrics")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Metric | Gold total | Exact | Semantic | Unmatched | Coverage |")
	fmt.Fprintln(&out, "|---|---:|---:|---:|---:|---:|")
	markdownNodeMetricRow(&out, "Entity", metric.Entity)
	markdownNodeMetricRow(&out, "Concept", metric.Concept)
	markdownNodeMetricRow(&out, "Overall", metric.Overall)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Graph | Correct | Missing | Extra | Precision | Recall | F1 | Scorable |")
	fmt.Fprintln(&out, "|---|---:|---:|---:|---:|---:|---:|---|")
	fmt.Fprintf(&out, "| Directed edges | %d | %d | %d | %s | %s | %s | %t |\n\n",
		metric.Graph.Correct, metric.Graph.Missing, metric.Graph.Extra,
		formatReportFloat(metric.Graph.Precision), formatReportFloat(metric.Graph.Recall),
		formatReportFloat(metric.Graph.F1), metric.Graph.Scorable)
	if metric.Graph.Note != "" {
		fmt.Fprintf(&out, "%s\n\n", escapeMarkdownCell(metric.Graph.Note))
	}
	markdownCost(&out, "Generation cost", metric.GenerationCost)
	markdownCost(&out, "Scoring cost", metric.ScoringCost)

	if normalized.Result != nil {
		result := normalized.Result
		fmt.Fprintln(&out, "## Node matches")
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "| Type | Gold ID | Gold name | Method | Page | Score | Reason |")
		fmt.Fprintln(&out, "|---|---|---|---|---|---:|---|")
		for _, match := range result.NodeMatches {
			score := ""
			if match.Score != nil {
				score = formatReportFloat(*match.Score)
			}
			page := match.PageTitle
			if page == "" {
				page = match.PageSlug
			}
			fmt.Fprintf(&out, "| %s | %s | %s | %s | %s | %s | %s |\n",
				escapeMarkdownCell(match.GoldType), escapeMarkdownCell(match.GoldNodeID),
				escapeMarkdownCell(match.GoldName), escapeMarkdownCell(match.Method),
				escapeMarkdownCell(page), score, escapeMarkdownCell(match.Reason))
		}
		fmt.Fprintln(&out)
		markdownEdgeTable(&out, "Correct edges", result.CorrectEdges)
		markdownEdgeTable(&out, "Missing edges", result.MissingEdges)
		markdownEdgeTable(&out, "Extra edges", result.ExtraEdges)
		markdownEdgeTable(&out, "Unscored edges", result.UnscoredEdges)
	}

	return out.Bytes(), nil
}

func normalizedWikiEvaluationDetail(detail *types.WikiEvaluationDetail) (*types.WikiEvaluationDetail, error) {
	if detail == nil {
		return nil, fmt.Errorf("wiki evaluation report detail is required")
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("copy wiki evaluation report: %w", err)
	}
	var clone types.WikiEvaluationDetail
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("copy wiki evaluation report: %w", err)
	}
	if clone.Result == nil {
		return &clone, nil
	}
	sort.Slice(clone.Result.NodeMatches, func(i, j int) bool {
		left, right := clone.Result.NodeMatches[i], clone.Result.NodeMatches[j]
		if left.GoldType != right.GoldType {
			return left.GoldType < right.GoldType
		}
		return left.GoldNodeID < right.GoldNodeID
	})
	sortWikiEdgeRefs(clone.Result.CorrectEdges)
	sortWikiEdgeRefs(clone.Result.MissingEdges)
	sortWikiEdgeRefs(clone.Result.ExtraEdges)
	sortWikiEdgeRefs(clone.Result.UnscoredEdges)
	sort.Slice(clone.Result.FrozenPages, func(i, j int) bool {
		if clone.Result.FrozenPages[i].Type != clone.Result.FrozenPages[j].Type {
			return clone.Result.FrozenPages[i].Type < clone.Result.FrozenPages[j].Type
		}
		return clone.Result.FrozenPages[i].Slug < clone.Result.FrozenPages[j].Slug
	})
	return &clone, nil
}

func sortWikiEdgeRefs(edges []types.WikiEdgeRef) {
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

func markdownKeyValueTable(out *bytes.Buffer, rows [][2]string) {
	fmt.Fprintln(out, "| Field | Value |")
	fmt.Fprintln(out, "|---|---|")
	for _, row := range rows {
		if row[1] != "" {
			fmt.Fprintf(out, "| %s | %s |\n", escapeMarkdownCell(row[0]), escapeMarkdownCell(row[1]))
		}
	}
	fmt.Fprintln(out)
}

func markdownNodeMetricRow(out *bytes.Buffer, name string, metric types.WikiNodeMetric) {
	fmt.Fprintf(out, "| %s | %d | %d | %d | %d | %s |\n", name, metric.GoldTotal,
		metric.ExactMatched, metric.SemanticMatched, metric.Unmatched, formatReportFloat(metric.Coverage))
}

func markdownCost(out *bytes.Buffer, title string, cost *types.EvaluationCostMetrics) {
	if cost == nil {
		return
	}
	fmt.Fprintf(out, "### %s\n\n", title)
	rows := [][2]string{
		{"Model calls", strconv.FormatInt(cost.ModelCalls, 10)},
		{"Prompt tokens", strconv.FormatInt(cost.PromptTokens, 10)},
		{"Completion tokens", strconv.FormatInt(cost.CompletionTokens, 10)},
		{"Total tokens", strconv.FormatInt(cost.TotalTokens, 10)},
		{"Cache read tokens", strconv.FormatInt(cost.CacheReadTokens, 10)},
		{"Cache write tokens", strconv.FormatInt(cost.CacheWriteTokens, 10)},
	}
	if cost.EstimatedCostUSD != nil {
		rows = append(rows, [2]string{"Estimated cost USD", formatReportFloat(*cost.EstimatedCostUSD)})
	}
	markdownKeyValueTable(out, rows)
}

func markdownEdgeTable(out *bytes.Buffer, title string, edges []types.WikiEdgeRef) {
	fmt.Fprintf(out, "## %s (%d)\n\n", title, len(edges))
	fmt.Fprintln(out, "| Source | Target | Reason |")
	fmt.Fprintln(out, "|---|---|---|")
	for _, edge := range edges {
		fmt.Fprintf(out, "| %s | %s | %s |\n", escapeMarkdownCell(edge.Source),
			escapeMarkdownCell(edge.Target), escapeMarkdownCell(edge.Reason))
	}
	fmt.Fprintln(out)
}

func evaluationStatusLabel(status types.EvaluationStatue) string {
	switch status {
	case types.EvaluationStatuePending:
		return "pending"
	case types.EvaluationStatueRunning:
		return "running"
	case types.EvaluationStatueSuccess:
		return "success"
	case types.EvaluationStatueFailed:
		return "failed"
	case types.EvaluationStatueInterrupted:
		return "interrupted"
	default:
		return strconv.Itoa(int(status))
	}
}

func reportModel(model types.ModelSnapshot) string {
	parts := []string{model.Name, model.ID, model.Provider, model.Type}
	nonEmpty := parts[:0]
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, " / ")
}

func formatReportTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func formatOptionalReportTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func formatReportFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func escapeMarkdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}
