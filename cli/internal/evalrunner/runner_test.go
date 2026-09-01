package evalrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	sdk "github.com/Tencent/WeKnora/client"
)

type fakeEvalClient struct {
	models      []sdk.Model
	startDetail *sdk.EvaluationDetail
	run         *sdk.EvaluationRun
	statuses    []int
	startErr    error
	pollErr     error
	listErr     error
	metricJSON  string
}

func (f *fakeEvalClient) ListModels(context.Context) ([]sdk.Model, error) {
	return f.models, nil
}

func (f *fakeEvalClient) StartEvaluation(context.Context, *sdk.EvaluationRequest) (*sdk.EvaluationDetail, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	return f.startDetail, nil
}

func (f *fakeEvalClient) GetEvaluationResult(_ context.Context, id string) (*sdk.EvaluationDetail, error) {
	if f.pollErr != nil {
		return nil, f.pollErr
	}
	status := 1
	if len(f.statuses) > 0 {
		status = f.statuses[0]
		f.statuses = f.statuses[1:]
	}
	return &sdk.EvaluationDetail{
		Task:   sdk.EvaluationTask{ID: id, Status: status, Total: 1, Finished: 1},
		Params: json.RawMessage(`{}`),
		Metric: json.RawMessage(f.metricJSON),
	}, nil
}

func (f *fakeEvalClient) ListEvaluationRuns(context.Context, int, int) ([]sdk.EvaluationRun, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	if f.run == nil {
		return []sdk.EvaluationRun{}, 0, nil
	}
	return []sdk.EvaluationRun{*f.run}, 1, nil
}

func testRunOptions(reportDir string) RunOptions {
	return RunOptions{
		Wait:      true,
		Timeout:   2 * time.Second,
		Interval:  10 * time.Millisecond,
		ReportDir: reportDir,
		Reproduce: "make eval-baseline CONFIG=./evaluation/configs/default.yaml",
	}
}

func TestRunSuccess(t *testing.T) {
	dir := t.TempDir()
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{
			Task: sdk.EvaluationTask{ID: "run-1", Status: 0},
		},
		statuses: []int{1, 2},
		run: &sdk.EvaluationRun{
			ID:             "run-1",
			ConfigHash:     "abc123",
			ConfigSnapshot: json.RawMessage(`{"dataset":{"id":"demo","sha256":"hash","sample_count":1}}`),
		},
	}

	result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, testRunOptions(dir))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.ReportPaths) != 2 {
		t.Fatalf("report paths=%v, want 2", result.ReportPaths)
	}
	if result.Report == nil || result.Report.ConfigHash != "abc123" {
		t.Errorf("report=%+v", result.Report)
	}
}

func TestRunFailedAndInterruptedWriteReports(t *testing.T) {
	for _, status := range []int{3, 4} {
		dir := t.TempDir()
		client := &fakeEvalClient{
			startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
			statuses:    []int{status},
			run: &sdk.EvaluationRun{
				ID:             "run-1",
				ConfigHash:     "abc123",
				ConfigSnapshot: json.RawMessage(`{}`),
			},
		}
		result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, testRunOptions(dir))
		if !errors.Is(err, ErrRunFailed) {
			t.Fatalf("status %d: err=%v, want ErrRunFailed", status, err)
		}
		if len(result.ReportPaths) != 2 {
			t.Fatalf("status %d: report paths=%v, want 2", status, result.ReportPaths)
		}
	}
}

func TestRunTimeout(t *testing.T) {
	dir := t.TempDir()
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
		statuses:    []int{1, 1, 1},
		run: &sdk.EvaluationRun{
			ID:             "run-1",
			ConfigHash:     "abc123",
			ConfigSnapshot: json.RawMessage(`{}`),
		},
	}
	opts := testRunOptions(dir)
	opts.Timeout = 60 * time.Millisecond

	result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, opts)
	if !errors.Is(err, ErrRunFailed) {
		t.Fatalf("err=%v, want ErrRunFailed", err)
	}
	if result != nil && len(result.ReportPaths) != 0 {
		t.Fatalf("timeout must not write a report, got %v", result.ReportPaths)
	}
}

func TestRunNonWait(t *testing.T) {
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
	}
	opts := testRunOptions(t.TempDir())
	opts.Wait = false

	result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TaskID != "run-1" || len(result.ReportPaths) != 0 {
		t.Errorf("result=%+v", result)
	}
}

func TestRunWithExistingTaskID(t *testing.T) {
	dir := t.TempDir()
	client := &fakeEvalClient{
		statuses: []int{2},
		run: &sdk.EvaluationRun{
			ID:             "run-1",
			ConfigHash:     "abc123",
			ConfigSnapshot: json.RawMessage(`{"dataset":{"id":"demo","sha256":"hash","sample_count":1}}`),
		},
	}
	opts := testRunOptions(dir)
	opts.TaskID = "run-1"

	result, err := Run(context.Background(), nil, client, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Report == nil || result.Report.ConfigHash != "abc123" {
		t.Errorf("report=%+v", result.Report)
	}
	if len(result.ReportPaths) != 2 {
		t.Errorf("report paths=%v, want 2", result.ReportPaths)
	}
}

func TestRunWithBaselinePass(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.yaml")
	baseline := baselineForRunner("ds123")
	if err := WriteBaseline(baselinePath, baseline, false); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
		statuses:    []int{2},
		metricJSON:  runnerMetricJSON(0.95),
		run: &sdk.EvaluationRun{
			ID:             "run-1",
			ConfigHash:     "hash123",
			ConfigSnapshot: json.RawMessage(`{"dataset":{"id":"demo","sha256":"ds123","sample_count":1}}`),
		},
	}
	opts := testRunOptions(dir)
	opts.Baseline = baselinePath

	result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Comparison == nil || !result.Comparison.Pass {
		t.Errorf("comparison=%+v, want pass", result.Comparison)
	}
}

func TestRunWithBaselineRegression(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.yaml")
	baseline := baselineForRunner("ds123")
	if err := WriteBaseline(baselinePath, baseline, false); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
		statuses:    []int{2},
		metricJSON:  runnerMetricJSON(0.7),
		run: &sdk.EvaluationRun{
			ID:             "run-1",
			ConfigHash:     "hash123",
			ConfigSnapshot: json.RawMessage(`{"dataset":{"id":"demo","sha256":"ds123","sample_count":1}}`),
		},
	}
	opts := testRunOptions(dir)
	opts.Baseline = baselinePath

	result, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, opts)
	if !errors.Is(err, ErrRegression) {
		t.Fatalf("err=%v, want ErrRegression", err)
	}
	if result == nil || result.Comparison == nil || result.Comparison.Pass {
		t.Errorf("result=%+v, want failing comparison", result)
	}
}

func TestRunModelResolutionError(t *testing.T) {
	client := &fakeEvalClient{models: []sdk.Model{}}
	cfg := &RunnerConfig{DatasetID: "demo", Models: RunnerModels{Chat: "missing-chat"}}
	_, err := Run(context.Background(), cfg, client, testRunOptions(t.TempDir()))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestRunServiceUnavailable(t *testing.T) {
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1"}},
		startErr:    errors.New("connection refused"),
	}
	_, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, testRunOptions(t.TempDir()))
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("err=%v, want ErrServiceUnavailable", err)
	}
}

func TestRunStart400IsConfigError(t *testing.T) {
	client := &fakeEvalClient{
		startErr: &sdk.APIError{StatusCode: http.StatusBadRequest, Body: "dataset not found"},
	}
	_, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, testRunOptions(t.TempDir()))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestRunHistoryRecordMissing(t *testing.T) {
	client := &fakeEvalClient{
		startDetail: &sdk.EvaluationDetail{Task: sdk.EvaluationTask{ID: "run-1", Status: 0}},
		statuses:    []int{2},
	}
	_, err := Run(context.Background(), &RunnerConfig{DatasetID: "demo"}, client, testRunOptions(t.TempDir()))
	if !errors.Is(err, ErrRunFailed) {
		t.Fatalf("err=%v, want ErrRunFailed", err)
	}
}

func runnerMetricJSON(recall float64) string {
	return fmt.Sprintf(`{
		"retrieval_metrics": {
			"precision": 1,
			"recall": %f,
			"ndcg3": 1,
			"ndcg10": 1,
			"mrr": 1,
			"map": 1
		},
		"generation_metrics": {
			"bleu1": 0.2,
			"bleu2": 0.15,
			"bleu4": 0.1,
			"rouge1": 0.3,
			"rouge2": 0.2,
			"rougel": 0.28
		}
	}`, recall)
}

func baselineForRunner(dsHash string) *Baseline {
	min := 0.8
	abs := 0.1
	return &Baseline{
		Version:    1,
		ConfigHash: "hash123",
		Dataset:    BaselineDataset{ID: "demo", SHA256: dsHash},
		Metrics: BaselineMetrics{
			Retrieval: RetrievalThresholds{
				Recall: MetricThreshold{Baseline: 0.9, MinValue: &min, MaxAbsoluteDrop: &abs},
			},
		},
	}
}
