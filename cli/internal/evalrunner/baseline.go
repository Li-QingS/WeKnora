package evalrunner

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Baseline is the committed quality standard for one evaluation exam
// (config_hash + dataset sha256).
type Baseline struct {
	Version    int              `yaml:"version"`
	ConfigHash string           `yaml:"config_hash"`
	Dataset    BaselineDataset  `yaml:"dataset"`
	Metrics    BaselineMetrics  `yaml:"metrics"`
	Metadata   BaselineMetadata `yaml:"metadata"`
}

// BaselineDataset identifies the dataset exam.
type BaselineDataset struct {
	ID     string `yaml:"id"`
	SHA256 string `yaml:"sha256"`
}

// BaselineMetrics carries thresholds for retrieval and generation metrics.
type BaselineMetrics struct {
	Retrieval  RetrievalThresholds  `yaml:"retrieval_metrics"`
	Generation GenerationThresholds `yaml:"generation_metrics"`
}

// MetricThreshold defines the rules for one metric.
type MetricThreshold struct {
	Baseline        float64  `yaml:"baseline"`
	MinValue        *float64 `yaml:"min_value,omitempty"`
	MaxAbsoluteDrop *float64 `yaml:"max_absolute_drop,omitempty"`
	MaxRelativeDrop *float64 `yaml:"max_relative_drop,omitempty"`
}

// RetrievalThresholds mirrors the retrieval metrics in EvalReport.
type RetrievalThresholds struct {
	Precision MetricThreshold `yaml:"precision"`
	Recall    MetricThreshold `yaml:"recall"`
	NDCG3     MetricThreshold `yaml:"ndcg3"`
	NDCG10    MetricThreshold `yaml:"ndcg10"`
	MRR       MetricThreshold `yaml:"mrr"`
	MAP       MetricThreshold `yaml:"map"`
}

// GenerationThresholds mirrors the generation metrics in EvalReport.
type GenerationThresholds struct {
	BLEU1  MetricThreshold `yaml:"bleu1"`
	BLEU2  MetricThreshold `yaml:"bleu2"`
	BLEU4  MetricThreshold `yaml:"bleu4"`
	ROUGE1 MetricThreshold `yaml:"rouge1"`
	ROUGE2 MetricThreshold `yaml:"rouge2"`
	ROUGEL MetricThreshold `yaml:"rougel"`
}

// BaselineMetadata records who approved the baseline and why.
type BaselineMetadata struct {
	ApprovedCommit string `yaml:"approved_commit"`
	ApprovedBy     string `yaml:"approved_by"`
	CreatedAt      string `yaml:"created_at"`
	Note           string `yaml:"note,omitempty"`
}

// BaselineGenOptions carries the approval metadata for generating a baseline.
type BaselineGenOptions struct {
	ApprovedBy     string
	ApprovedCommit string
	Note           string
}

// LoadBaseline reads and validates a baseline YAML file.
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read baseline %s: %v", ErrInvalidConfig, path, err)
	}
	var baseline Baseline
	if err := yaml.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("%w: parse baseline %s: %v", ErrInvalidConfig, path, err)
	}
	if baseline.Version != 1 {
		return nil, fmt.Errorf("%w: baseline version must be 1, got %d", ErrInvalidConfig, baseline.Version)
	}
	if baseline.ConfigHash == "" {
		return nil, fmt.Errorf("%w: baseline config_hash is required", ErrInvalidConfig)
	}
	if baseline.Dataset.SHA256 == "" {
		return nil, fmt.Errorf("%w: baseline dataset.sha256 is required", ErrInvalidConfig)
	}
	return &baseline, nil
}

// GenerateBaseline builds a baseline from a trusted result. Threshold rules
// are intentionally left empty so the author fills them in explicitly.
func GenerateBaseline(result *EvalReport, opts BaselineGenOptions) (*Baseline, error) {
	if result == nil {
		return nil, fmt.Errorf("%w: result is required", ErrInvalidConfig)
	}
	if result.ConfigHash == "" {
		return nil, fmt.Errorf("%w: result config_hash is required", ErrInvalidConfig)
	}
	if result.Dataset.SHA256 == "" {
		return nil, fmt.Errorf("%w: result dataset.sha256 is required", ErrInvalidConfig)
	}
	if opts.ApprovedBy == "" || opts.ApprovedCommit == "" {
		return nil, fmt.Errorf("%w: approved_by and approved_commit are required", ErrInvalidConfig)
	}

	return &Baseline{
		Version:    1,
		ConfigHash: result.ConfigHash,
		Dataset: BaselineDataset{
			ID:     result.Dataset.ID,
			SHA256: result.Dataset.SHA256,
		},
		Metrics: BaselineMetrics{
			Retrieval: RetrievalThresholds{
				Precision: MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "precision")},
				Recall:    MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "recall")},
				NDCG3:     MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "ndcg3")},
				NDCG10:    MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "ndcg10")},
				MRR:       MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "mrr")},
				MAP:       MetricThreshold{Baseline: metricValueOrZero(result, "retrieval_metrics", "map")},
			},
			Generation: GenerationThresholds{
				BLEU1:  MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "bleu1")},
				BLEU2:  MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "bleu2")},
				BLEU4:  MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "bleu4")},
				ROUGE1: MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "rouge1")},
				ROUGE2: MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "rouge2")},
				ROUGEL: MetricThreshold{Baseline: metricValueOrZero(result, "generation_metrics", "rougel")},
			},
		},
		Metadata: BaselineMetadata{
			ApprovedCommit: opts.ApprovedCommit,
			ApprovedBy:     opts.ApprovedBy,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			Note:           opts.Note,
		},
	}, nil
}

// WriteBaseline writes the baseline YAML. Existing files are only overwritten
// when force is true.
func WriteBaseline(path string, baseline *Baseline, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%w: baseline already exists: %s", ErrInvalidConfig, path)
		}
	}
	data, err := yaml.Marshal(baseline)
	if err != nil {
		return fmt.Errorf("%w: encode baseline: %v", ErrInvalidConfig, err)
	}
	if err := atomicWrite(path, data); err != nil {
		return fmt.Errorf("%w: write baseline: %v", ErrInvalidConfig, err)
	}
	return nil
}

func metricValueOrZero(result *EvalReport, group, name string) float64 {
	v, _ := metricValue(result, group, name)
	return v
}

func metricValue(result *EvalReport, group, name string) (float64, bool) {
	if result == nil || result.Metric == nil {
		return 0, false
	}
	groupMap, ok := result.Metric[group].(map[string]any)
	if !ok {
		return 0, false
	}
	value, ok := groupMap[name]
	if !ok {
		return 0, false
	}
	f, ok := value.(float64)
	return f, ok
}
