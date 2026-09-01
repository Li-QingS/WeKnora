package evalcmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/evalrunner"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

func compareResultReport(recall float64) *evalrunner.EvalReport {
	return &evalrunner.EvalReport{
		ConfigHash: "hash123",
		Dataset:    evalrunner.DatasetReport{ID: "demo", SHA256: "ds123", SampleCount: 15},
		Metric: map[string]any{
			"retrieval_metrics": map[string]any{
				"precision": float64(1.0),
				"recall":    recall,
				"ndcg3":     float64(1.0),
				"ndcg10":    float64(1.0),
				"mrr":       float64(1.0),
				"map":       float64(1.0),
			},
			"generation_metrics": map[string]any{
				"bleu1":  float64(0.2),
				"bleu2":  float64(0.15),
				"bleu4":  float64(0.1),
				"rouge1": float64(0.3),
				"rouge2": float64(0.2),
				"rougel": float64(0.28),
			},
		},
	}
}

func writeCompareFixture(t *testing.T, report *evalrunner.EvalReport, baseline *evalrunner.Baseline) (string, string) {
	t.Helper()
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "evaluation-result.json")
	baselinePath := filepath.Join(dir, "baseline.yaml")
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(resultPath, data, 0o600); err != nil {
		t.Fatalf("write result: %v", err)
	}
	if err := evalrunner.WriteBaseline(baselinePath, baseline, false); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	return resultPath, baselinePath
}

func compareBaseline() *evalrunner.Baseline {
	min := 0.8
	abs := 0.1
	return &evalrunner.Baseline{
		Version:    1,
		ConfigHash: "hash123",
		Dataset:    evalrunner.BaselineDataset{ID: "demo", SHA256: "ds123"},
		Metrics: evalrunner.BaselineMetrics{
			Retrieval: evalrunner.RetrievalThresholds{
				Recall: evalrunner.MetricThreshold{Baseline: 0.9, MinValue: &min, MaxAbsoluteDrop: &abs},
			},
		},
	}
}

func TestCompareCommandPass(t *testing.T) {
	out, _ := iostreams.SetForTest(t)
	resultPath, baselinePath := writeCompareFixture(t, compareResultReport(0.95), compareBaseline())
	cmd := NewCmdCompare()
	cmd.SetArgs([]string{"--result", resultPath, "--baseline", baselinePath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare pass: %v", err)
	}
	if !strings.Contains(out.String(), `"pass":true`) {
		t.Errorf("output=%s, want pass true", out.String())
	}
}

func TestCompareCommandDegraded(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	resultPath, baselinePath := writeCompareFixture(t, compareResultReport(0.7), compareBaseline())
	cmd := NewCmdCompare()
	cmd.SetArgs([]string{"--result", resultPath, "--baseline", baselinePath})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalRegression {
		t.Fatalf("err=%v, want eval.regression", err)
	}
}

func TestCompareCommandMissingArgs(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	cmd := NewCmdCompare()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}
}

func TestCompareCommandMismatch(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	report := compareResultReport(0.95)
	report.ConfigHash = "other"
	resultPath, baselinePath := writeCompareFixture(t, report, compareBaseline())
	cmd := NewCmdCompare()
	cmd.SetArgs([]string{"--result", resultPath, "--baseline", baselinePath})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}
}
