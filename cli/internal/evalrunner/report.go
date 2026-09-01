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
	RunID       string         `json:"run_id"`
	ConfigHash  string         `json:"config_hash"`
	Status      int            `json:"status"`
	StatusLabel string         `json:"status_label"`
	Dataset     DatasetReport  `json:"dataset"`
	Models      []ModelReport  `json:"models"`
	Params      map[string]any `json:"params"`
	Metric      map[string]any `json:"metric,omitempty"`
	ErrMsg      string         `json:"err_msg,omitempty"`
	Finished    int            `json:"finished"`
	Total       int            `json:"total"`
	Version     map[string]any `json:"version,omitempty"`
	Reproduce   string         `json:"reproduce"`
	GeneratedAt string         `json:"generated_at"`
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

type configSnapshot struct {
	Dataset DatasetReport  `json:"dataset"`
	Models  []ModelReport  `json:"models"`
	Version map[string]any `json:"version"`
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
	fmt.Fprintf(&b, "- Progress: %d/%d\n", report.Finished, report.Total)
	if report.ErrMsg != "" {
		fmt.Fprintf(&b, "- Error: %s\n", report.ErrMsg)
	}
	fmt.Fprintf(&b, "\n## Reproduce\n\n```bash\n%s\n```\n", report.Reproduce)
	if len(report.Metric) > 0 {
		fmt.Fprintf(&b, "\n## Metrics\n\n")
		for _, group := range []string{"retrieval_metrics", "generation_metrics"} {
			if metrics, ok := report.Metric[group].(map[string]any); ok {
				fmt.Fprintf(&b, "### %s\n\n", strings.ReplaceAll(group, "_", " "))
				fmt.Fprintf(&b, "| Metric | Value |\n| --- | --- |\n")
				for k, v := range metrics {
					fmt.Fprintf(&b, "| %s | %v |\n", k, v)
				}
				fmt.Fprintf(&b, "\n")
			}
		}
	}
	fmt.Fprintf(&b, "Generated at %s\n", report.GeneratedAt)
	return b.String()
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
