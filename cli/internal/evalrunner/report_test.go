package evalrunner

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	sdk "github.com/Tencent/WeKnora/client"
)

func TestBuildReport(t *testing.T) {
	detail := &sdk.EvaluationDetail{
		Task: sdk.EvaluationTask{
			ID:       "run-1",
			Status:   2,
			Total:    1,
			Finished: 1,
		},
		Params: json.RawMessage(`{"embedding_top_k":30}`),
		Metric: json.RawMessage(`{"retrieval_metrics":{"precision":1},"generation_metrics":{}}`),
	}
	run := &sdk.EvaluationRun{
		ID:         "run-1",
		DatasetID:  "demo",
		ConfigHash: "abc123",
		ConfigSnapshot: json.RawMessage(`{
			"dataset":{"id":"demo","sha256":"hash123","sample_count":1},
			"models":[{"id":"chat-1","name":"Chat","provider":"zhipu","type":"KnowledgeQA"}],
			"version":{"app_version":"0.7.2","git_commit":"abc"}
		}`),
	}

	report, err := BuildReport(detail, run, "make eval-baseline CONFIG=./evaluation/configs/default.yaml")
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if report.RunID != "run-1" || report.ConfigHash != "abc123" {
		t.Errorf("report=%+v", report)
	}
	if report.Dataset.SHA256 != "hash123" || report.Dataset.SampleCount != 1 {
		t.Errorf("dataset=%+v", report.Dataset)
	}
	if len(report.Models) != 1 || report.Models[0].ID != "chat-1" {
		t.Errorf("models=%+v", report.Models)
	}
	if report.Version["app_version"] != "0.7.2" {
		t.Errorf("version=%v", report.Version)
	}
}

func TestWriteReports(t *testing.T) {
	dir := t.TempDir()
	report := &EvalReport{
		RunID:       "run-1",
		ConfigHash:  "abc123",
		Status:      2,
		StatusLabel: "success",
		Dataset:     DatasetReport{ID: "demo", SHA256: "hash123", SampleCount: 1},
		Models:      []ModelReport{},
		Params:      map[string]any{"embedding_top_k": float64(30)},
		Metric:      map[string]any{"retrieval_metrics": map[string]any{"precision": float64(1)}},
		Reproduce:   "make eval-baseline CONFIG=./evaluation/configs/default.yaml",
		GeneratedAt: "2026-09-01T00:00:00Z",
	}

	paths, err := WriteReports(dir, report)
	if err != nil {
		t.Fatalf("WriteReports: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%v, want 2 files", paths)
	}
	jsonData, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var decoded EvalReport
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if decoded.RunID != "run-1" || decoded.ConfigHash != "abc123" {
		t.Errorf("decoded=%+v", decoded)
	}
	mdData, err := os.ReadFile(paths[1])
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	md := string(mdData)
	for _, want := range []string{"run-1", "abc123", "make eval-baseline", "success"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestWriteReportsWithComparison(t *testing.T) {
	dir := t.TempDir()
	report := &EvalReport{
		RunID:       "run-1",
		ConfigHash:  "abc123",
		Status:      2,
		StatusLabel: "success",
		Dataset:     DatasetReport{ID: "demo", SHA256: "hash123", SampleCount: 1},
		Models:      []ModelReport{},
		Params:      map[string]any{},
		Metric:      map[string]any{},
		Reproduce:   "make eval-ci",
		GeneratedAt: "2026-09-01T00:00:00Z",
		Comparison: &Comparison{
			Pass: false,
			Items: []ComparisonItem{
				{
					Group:    "retrieval_metrics",
					Name:     "recall",
					Baseline: 0.9,
					Current:  0.7,
					Delta:    0.2,
					Pass:     false,
					Reason:   "below min_value",
				},
			},
			FailedCount: 1,
		},
	}

	paths, err := WriteReports(dir, report)
	if err != nil {
		t.Fatalf("WriteReports: %v", err)
	}
	md, err := os.ReadFile(paths[1])
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	content := string(md)
	for _, want := range []string{"Baseline comparison", "recall", "FAIL", "Failed metrics: 1"} {
		if !strings.Contains(content, want) {
			t.Errorf("markdown missing %q:\n%s", want, content)
		}
	}
}
