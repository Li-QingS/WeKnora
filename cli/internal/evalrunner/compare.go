package evalrunner

import (
	"errors"
	"fmt"
)

// ErrBaselineMismatch means the result and baseline do not describe the same
// evaluation exam (config_hash or dataset sha256 differ).
var ErrBaselineMismatch = errors.New("baseline mismatch")

// Comparison is the machine-readable result of comparing one run against a
// baseline.
type Comparison struct {
	Pass          bool             `json:"pass"`
	ConfigHash    string           `json:"config_hash"`
	DatasetSHA256 string           `json:"dataset_sha256"`
	Items         []ComparisonItem `json:"items"`
	FailedCount   int              `json:"failed_count"`
}

// ComparisonItem is one metric's baseline/current/delta and verdict.
type ComparisonItem struct {
	Group    string  `json:"group"`
	Name     string  `json:"name"`
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	Delta    float64 `json:"delta"`
	Pass     bool    `json:"pass"`
	Reason   string  `json:"reason,omitempty"`
}

// CompareResult validates the exam identity and compares every metric in the
// baseline against the current result.
func CompareResult(result *EvalReport, baseline *Baseline) (*Comparison, error) {
	if result == nil || baseline == nil {
		return nil, fmt.Errorf("%w: result and baseline are required", ErrInvalidConfig)
	}
	if result.ConfigHash != baseline.ConfigHash {
		return nil, fmt.Errorf("%w: config_hash %q != baseline %q", ErrBaselineMismatch, result.ConfigHash, baseline.ConfigHash)
	}
	if result.Dataset.SHA256 != baseline.Dataset.SHA256 {
		return nil, fmt.Errorf("%w: dataset sha256 %q != baseline %q", ErrBaselineMismatch, result.Dataset.SHA256, baseline.Dataset.SHA256)
	}

	comparison := &Comparison{
		ConfigHash:    result.ConfigHash,
		DatasetSHA256: result.Dataset.SHA256,
		Items:         []ComparisonItem{},
	}

	retrieval := []struct {
		name string
		t    MetricThreshold
	}{
		{"precision", baseline.Metrics.Retrieval.Precision},
		{"recall", baseline.Metrics.Retrieval.Recall},
		{"ndcg3", baseline.Metrics.Retrieval.NDCG3},
		{"ndcg10", baseline.Metrics.Retrieval.NDCG10},
		{"mrr", baseline.Metrics.Retrieval.MRR},
		{"map", baseline.Metrics.Retrieval.MAP},
	}
	generation := []struct {
		name string
		t    MetricThreshold
	}{
		{"bleu1", baseline.Metrics.Generation.BLEU1},
		{"bleu2", baseline.Metrics.Generation.BLEU2},
		{"bleu4", baseline.Metrics.Generation.BLEU4},
		{"rouge1", baseline.Metrics.Generation.ROUGE1},
		{"rouge2", baseline.Metrics.Generation.ROUGE2},
		{"rougel", baseline.Metrics.Generation.ROUGEL},
	}

	for _, group := range []string{"retrieval_metrics", "generation_metrics"} {
		var rows []struct {
			name string
			t    MetricThreshold
		}
		if group == "retrieval_metrics" {
			rows = retrieval
		} else {
			rows = generation
		}
		for _, row := range rows {
			current, ok := metricValue(result, group, row.name)
			if !ok {
				return nil, fmt.Errorf("%w: result is missing %s.%s", ErrInvalidConfig, group, row.name)
			}
			item := compareMetric(group, row.name, current, row.t)
			comparison.Items = append(comparison.Items, item)
			if !item.Pass {
				comparison.FailedCount++
			}
		}
	}
	comparison.Pass = comparison.FailedCount == 0
	return comparison, nil
}

func compareMetric(group, name string, current float64, t MetricThreshold) ComparisonItem {
	item := ComparisonItem{
		Group:    group,
		Name:     name,
		Baseline: t.Baseline,
		Current:  current,
		Delta:    t.Baseline - current,
		Pass:     true,
	}
	if t.MinValue != nil && current < *t.MinValue {
		item.Pass = false
		item.Reason = fmt.Sprintf("current %.4f below min_value %.4f", current, *t.MinValue)
		return item
	}
	if t.MaxAbsoluteDrop != nil && item.Delta > *t.MaxAbsoluteDrop {
		item.Pass = false
		item.Reason = fmt.Sprintf("delta %.4f above max_absolute_drop %.4f", item.Delta, *t.MaxAbsoluteDrop)
		return item
	}
	if t.MaxRelativeDrop != nil && t.Baseline != 0 {
		relative := item.Delta / t.Baseline
		if relative > *t.MaxRelativeDrop {
			item.Pass = false
			item.Reason = fmt.Sprintf("relative drop %.4f above max_relative_drop %.4f", relative, *t.MaxRelativeDrop)
			return item
		}
	}
	return item
}
