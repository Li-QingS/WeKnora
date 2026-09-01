package evalrunner

import (
	"errors"
	"testing"
)

func fullMetricReport(configHash, dsHash string, recall float64) *EvalReport {
	return &EvalReport{
		ConfigHash: configHash,
		Dataset:    DatasetReport{ID: "demo", SHA256: dsHash},
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

func thresholdBaseline(configHash, dsHash string) *Baseline {
	min := 0.8
	abs := 0.02
	rel := 0.5
	return &Baseline{
		Version:    1,
		ConfigHash: configHash,
		Dataset:    BaselineDataset{ID: "demo", SHA256: dsHash},
		Metrics: BaselineMetrics{
			Retrieval: RetrievalThresholds{
				Recall: MetricThreshold{Baseline: 0.9, MinValue: &min, MaxAbsoluteDrop: &abs},
			},
			Generation: GenerationThresholds{
				BLEU1: MetricThreshold{Baseline: 0.2, MaxRelativeDrop: &rel},
			},
		},
	}
}

func TestCompareResultPass(t *testing.T) {
	result := fullMetricReport("hash", "ds", 0.95)
	baseline := thresholdBaseline("hash", "ds")
	cmp, err := CompareResult(result, baseline)
	if err != nil {
		t.Fatalf("CompareResult: %v", err)
	}
	if !cmp.Pass || cmp.FailedCount != 0 {
		t.Errorf("cmp=%+v, want pass", cmp)
	}
}

func TestCompareResultFailsMinValue(t *testing.T) {
	result := fullMetricReport("hash", "ds", 0.7)
	baseline := thresholdBaseline("hash", "ds")
	cmp, err := CompareResult(result, baseline)
	if err != nil {
		t.Fatalf("CompareResult: %v", err)
	}
	if cmp.Pass || cmp.FailedCount != 1 {
		t.Fatalf("cmp=%+v, want one failure", cmp)
	}
	for _, item := range cmp.Items {
		if item.Name == "recall" && !item.Pass {
			return
		}
	}
	t.Fatal("recall item should fail")
}

func TestCompareResultFailsAbsoluteDrop(t *testing.T) {
	result := fullMetricReport("hash", "ds", 0.85)
	baseline := thresholdBaseline("hash", "ds")
	cmp, err := CompareResult(result, baseline)
	if err != nil {
		t.Fatalf("CompareResult: %v", err)
	}
	if cmp.Pass || cmp.FailedCount != 1 {
		t.Fatalf("cmp=%+v, want one failure", cmp)
	}
}

func TestCompareResultFailsRelativeDrop(t *testing.T) {
	result := fullMetricReport("hash", "ds", 1.0)
	result.Metric["generation_metrics"].(map[string]any)["bleu1"] = float64(0.05)
	baseline := thresholdBaseline("hash", "ds")
	cmp, err := CompareResult(result, baseline)
	if err != nil {
		t.Fatalf("CompareResult: %v", err)
	}
	if cmp.Pass || cmp.FailedCount != 1 {
		t.Fatalf("cmp=%+v, want one failure", cmp)
	}
}

func TestCompareResultMismatch(t *testing.T) {
	result := fullMetricReport("hash", "ds", 0.95)
	baseline := thresholdBaseline("other-hash", "ds")
	_, err := CompareResult(result, baseline)
	if !errors.Is(err, ErrBaselineMismatch) {
		t.Fatalf("err=%v, want ErrBaselineMismatch", err)
	}
}

func TestCompareResultMissingMetric(t *testing.T) {
	result := fullMetricReport("hash", "ds", 0.95)
	delete(result.Metric["generation_metrics"].(map[string]any), "bleu2")
	baseline := thresholdBaseline("hash", "ds")
	_, err := CompareResult(result, baseline)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}
