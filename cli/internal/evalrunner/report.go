package evalrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/Tencent/WeKnora/client"
)

// EvalReport is the machine-readable report written to evaluation-result.json.
type EvalReport struct {
	RunID       string          `json:"run_id"`
	ConfigHash  string          `json:"config_hash"`
	Status      int             `json:"status"`
	StatusLabel string          `json:"status_label"`
	Dataset     DatasetReport   `json:"dataset"`
	Models      []ModelReport   `json:"models"`
	Chunking    *ChunkingReport `json:"chunking,omitempty"`
	Params      map[string]any  `json:"params"`
	Metric      map[string]any  `json:"metric,omitempty"`
	ErrMsg      string          `json:"err_msg,omitempty"`
	Finished    int             `json:"finished"`
	Total       int             `json:"total"`
	Version     map[string]any  `json:"version,omitempty"`
	Reproduce   string          `json:"reproduce"`
	GeneratedAt string          `json:"generated_at"`
	Comparison  *Comparison     `json:"comparison,omitempty"`
}

// LoadResult reads an evaluation-result.json written by WriteReports.
func LoadResult(path string) (*EvalReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read result %s: %v", ErrInvalidConfig, path, err)
	}
	var report EvalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("%w: parse result %s: %v", ErrInvalidConfig, path, err)
	}
	return &report, nil
}

// DatasetReport identifies the dataset used by a run.
type DatasetReport struct {
	ID          string `json:"id"`
	SHA256      string `json:"sha256"`
	SampleCount int    `json:"sample_count"`
}

// ModelReport identifies one model in the config snapshot.
type ModelReport struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Type     string `json:"type"`
}

// ChunkingReport identifies the chunking parameters recorded for a run.
type ChunkingReport struct {
	Strategy     string   `json:"strategy"`
	ChunkSize    int      `json:"chunk_size,omitempty"`
	ChunkOverlap int      `json:"chunk_overlap,omitempty"`
	TokenLimit   int      `json:"token_limit,omitempty"`
	Languages    []string `json:"languages,omitempty"`
}

type configSnapshot struct {
	Dataset  DatasetReport   `json:"dataset"`
	Models   []ModelReport   `json:"models"`
	Chunking *ChunkingReport `json:"chunking,omitempty"`
	Version  map[string]any  `json:"version"`
}

// BuildReport assembles the final report from the terminal task detail and
// the matching history run record.
func BuildReport(detail *sdk.EvaluationDetail, run *sdk.EvaluationRun, reproduce string) (*EvalReport, error) {
	report := &EvalReport{
		RunID:       detail.Task.ID,
		ConfigHash:  run.ConfigHash,
		Status:      detail.Task.Status,
		StatusLabel: statusLabel(detail.Task.Status),
		ErrMsg:      detail.Task.ErrMsg,
		Finished:    detail.Task.Finished,
		Total:       detail.Task.Total,
		Reproduce:   reproduce,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Models:      []ModelReport{},
		Params:      map[string]any{},
	}
	if err := json.Unmarshal(detail.Params, &report.Params); err != nil && len(detail.Params) > 0 {
		return nil, fmt.Errorf("decode params: %w", err)
	}
	if len(detail.Metric) > 0 {
		report.Metric = map[string]any{}
		if err := json.Unmarshal(detail.Metric, &report.Metric); err != nil {
			return nil, fmt.Errorf("decode metric: %w", err)
		}
	}
	if len(run.ConfigSnapshot) > 0 {
		var snapshot configSnapshot
		if err := json.Unmarshal(run.ConfigSnapshot, &snapshot); err != nil {
			return nil, fmt.Errorf("decode config snapshot: %w", err)
		}
		if snapshot.Dataset.ID != "" || snapshot.Dataset.SHA256 != "" {
			report.Dataset = snapshot.Dataset
		} else {
			report.Dataset = DatasetReport{ID: run.DatasetID}
		}
		report.Models = snapshot.Models
		report.Chunking = snapshot.Chunking
		report.Version = snapshot.Version
	}
	if report.Dataset.ID == "" {
		report.Dataset.ID = run.DatasetID
	}
	if report.Models == nil {
		report.Models = []ModelReport{}
	}
	return report, nil
}

// WriteReports writes evaluation-result.json and evaluation-report.md into
// dir and returns their paths.
func WriteReports(dir string, report *EvalReport) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create report dir: %w", err)
	}
	jsonPath := filepath.Join(dir, "evaluation-result.json")
	mdPath := filepath.Join(dir, "evaluation-report.md")

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode report: %w", err)
	}
	if err := atomicWrite(jsonPath, append(jsonData, '\n')); err != nil {
		return nil, err
	}
	if err := atomicWrite(mdPath, []byte(renderMarkdown(report))); err != nil {
		return nil, err
	}
	return []string{jsonPath, mdPath}, nil
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return fmt.Errorf("create temp report: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp report: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename report: %w", err)
	}
	return nil
}

func renderMarkdown(report *EvalReport) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Evaluation Report\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", report.RunID)
	fmt.Fprintf(&b, "- Config hash: `%s`\n", report.ConfigHash)
	fmt.Fprintf(&b, "- Status: %s (%d)\n", report.StatusLabel, report.Status)
	fmt.Fprintf(&b, "- Dataset: `%s` (sha256 `%s`, %d samples)\n",
		report.Dataset.ID, report.Dataset.SHA256, report.Dataset.SampleCount)
	if report.Chunking != nil {
		fmt.Fprintf(&b, "- Chunking: strategy `%s`, chunk_size %d, chunk_overlap %d\n",
			report.Chunking.Strategy, report.Chunking.ChunkSize, report.Chunking.ChunkOverlap)
		if len(report.Chunking.Languages) > 0 {
			fmt.Fprintf(&b, "- Chunking languages: %v\n", report.Chunking.Languages)
		}
	}
	fmt.Fprintf(&b, "- Progress: %d/%d\n", report.Finished, report.Total)
	if report.ErrMsg != "" {
		fmt.Fprintf(&b, "- Error: %s\n", report.ErrMsg)
	}
	fmt.Fprintf(&b, "\n## Reproduce\n\n```bash\n%s\n```\n", report.Reproduce)
	if len(report.Metric) > 0 {
		fmt.Fprintf(&b, "\n## Metrics\n\n")
		for _, group := range []string{
			"retrieval_metrics",
			"generation_metrics",
			"cost_metrics",
			"latency_metrics",
		} {
			if metrics, ok := report.Metric[group].(map[string]any); ok {
				fmt.Fprintf(&b, "### %s\n\n", strings.ReplaceAll(group, "_", " "))
				fmt.Fprintf(&b, "| Metric | Value |\n| --- | --- |\n")
				for k, v := range metrics {
					fmt.Fprintf(&b, "| %s | %s |\n", k, reportMetricValue(v))
				}
				fmt.Fprintf(&b, "\n")
			}
		}
	}
	if report.Comparison != nil {
		fmt.Fprintf(&b, "\n## Baseline comparison\n\n")
		fmt.Fprintf(&b, "| Group | Metric | Baseline | Current | Delta | Pass |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- |\n")
		for _, item := range report.Comparison.Items {
			pass := "PASS"
			if !item.Pass {
				pass = "FAIL"
			}
			fmt.Fprintf(&b, "| %s | %s | %.4f | %.4f | %.4f | %s |\n",
				item.Group, item.Name, item.Baseline, item.Current, item.Delta, pass)
		}
		fmt.Fprintf(&b, "\nFailed metrics: %d\n", report.Comparison.FailedCount)
	}
	fmt.Fprintf(&b, "Generated at %s\n", report.GeneratedAt)
	return b.String()
}

func reportMetricValue(value any) string {
	if value == nil {
		return "unknown"
	}
	if number, ok := value.(float64); ok {
		return fmt.Sprintf("%.6f", number)
	}
	if number, ok := value.(int64); ok {
		return fmt.Sprintf("%d", number)
	}
	return fmt.Sprintf("%v", value)
}

func statusLabel(status int) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "running"
	case 2:
		return "success"
	case 3:
		return "failed"
	case 4:
		return "interrupted"
	default:
		return "unknown"
	}
}
