package evalcmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/Tencent/WeKnora/client"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/evalrunner"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

func writeEvalConfig(t *testing.T, reportDir string) string {
	t.Helper()
	content := "dataset_id: demo\nexecution:\n  wait: true\n  timeout: 2s\n  interval: 10ms\n  report_dir: " + reportDir + "\n"
	path := filepath.Join(t.TempDir(), "eval.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestRunMissingConfig(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	cmd := NewCmdRun(&cmdutil.Factory{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}
}

func TestRunDryRunDoesNotTouchServer(t *testing.T) {
	out, _ := iostreams.SetForTest(t)
	cmd := NewCmdRun(&cmdutil.Factory{})
	cmd.SetArgs([]string{"--config", "/nonexistent.yaml", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Errorf("output=%s, want dry_run envelope", out.String())
	}
}

func TestRunInvalidConfig(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	cmd := NewCmdRun(&cmdutil.Factory{})
	cmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml")})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalConfigError {
		t.Fatalf("err=%v, want eval.config_error", err)
	}
}

func TestRunServiceUnavailable(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	cfgPath := writeEvalConfig(t, t.TempDir())
	factory := &cmdutil.Factory{
		Client: func() (*sdk.Client, error) {
			return nil, errors.New("no credentials")
		},
	}
	cmd := NewCmdRun(factory)
	cmd.SetArgs([]string{"--config", cfgPath})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalServiceUnavailable {
		t.Fatalf("err=%v, want eval.service_unavailable", err)
	}
}

func TestRunSuccess(t *testing.T) {
	out, _ := iostreams.SetForTest(t)
	reportDir := t.TempDir()
	cfgPath := writeEvalConfig(t, reportDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/evaluation":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {"task":{"id":"run-1","dataset_id":"demo","status":0,"total":0,"finished":0},"params":{}}
				}`))
				return
			}
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {"task":{"id":"run-1","dataset_id":"demo","status":2,"total":1,"finished":1},"params":{},"metric":{}}
				}`))
				return
			}
		case "/api/v1/evaluation/runs":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": [{
					"id":"run-1",
					"dataset_id":"demo",
					"status":2,
					"config_hash":"abc123",
					"config_snapshot":{"dataset":{"id":"demo","sha256":"hash","sample_count":1}}
				}],
				"total": 1,
				"page": 1,
				"page_size": 10
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	factory := &cmdutil.Factory{
		Client: func() (*sdk.Client, error) {
			return sdk.NewClient(server.URL, sdk.WithBearerToken("x")), nil
		},
	}
	cmd := NewCmdRun(factory)
	cmd.SetArgs([]string{"--config", cfgPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"task_id":"run-1"`) {
		t.Errorf("output=%s, want task_id run-1", out.String())
	}
	if _, err := os.Stat(filepath.Join(reportDir, "evaluation-result.json")); err != nil {
		t.Fatalf("report not written: %v", err)
	}
	var report struct {
		ConfigHash string `json:"config_hash"`
	}
	data, _ := os.ReadFile(filepath.Join(reportDir, "evaluation-result.json"))
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ConfigHash != "abc123" {
		t.Errorf("config_hash=%q, want abc123", report.ConfigHash)
	}
}

func TestRunCommandBaselineRegression(t *testing.T) {
	_, _ = iostreams.SetForTest(t)
	dir := t.TempDir()
	cfgPath := writeEvalConfig(t, dir)

	min := 0.8
	abs := 0.1
	baseline := &evalrunner.Baseline{
		Version:    1,
		ConfigHash: "abc123",
		Dataset:    evalrunner.BaselineDataset{ID: "demo", SHA256: "ds123"},
		Metrics: evalrunner.BaselineMetrics{
			Retrieval: evalrunner.RetrievalThresholds{
				Recall: evalrunner.MetricThreshold{Baseline: 0.9, MinValue: &min, MaxAbsoluteDrop: &abs},
			},
		},
	}
	baselinePath := filepath.Join(dir, "baseline.yaml")
	if err := evalrunner.WriteBaseline(baselinePath, baseline, false); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/evaluation":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {"task":{"id":"run-1","dataset_id":"demo","status":0},"params":{}}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": {
					"task":{"id":"run-1","dataset_id":"demo","status":2,"total":1,"finished":1},
					"params":{},
					"metric":{"retrieval_metrics":{"precision":1,"recall":0.7,"ndcg3":1,"ndcg10":1,"mrr":1,"map":1},"generation_metrics":{"bleu1":0.2,"bleu2":0.15,"bleu4":0.1,"rouge1":0.3,"rouge2":0.2,"rougel":0.28}}
				}
			}`))
			return
		case "/api/v1/evaluation/runs":
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": [{
					"id":"run-1",
					"dataset_id":"demo",
					"status":2,
					"config_hash":"abc123",
					"config_snapshot":{"dataset":{"id":"demo","sha256":"ds123","sample_count":1}}
				}],
				"total": 1,
				"page": 1,
				"page_size": 10
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	factory := &cmdutil.Factory{
		Client: func() (*sdk.Client, error) {
			return sdk.NewClient(server.URL, sdk.WithBearerToken("x")), nil
		},
	}
	cmd := NewCmdRun(factory)
	cmd.SetArgs([]string{"--config", cfgPath, "--baseline", baselinePath})
	err := cmd.Execute()
	var ce *cmdutil.Error
	if !errors.As(err, &ce) || ce.Code != cmdutil.CodeEvalRegression {
		t.Fatalf("err=%v, want eval.regression", err)
	}
}
