package evalrunner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func sampleBaselineReport() *EvalReport {
	return &EvalReport{
		ConfigHash: "hash123",
		Dataset:    DatasetReport{ID: "demo", SHA256: "ds123", SampleCount: 15},
		Metric: map[string]any{
			"retrieval_metrics": map[string]any{
				"precision": float64(1.0),
				"recall":    float64(0.9),
				"ndcg3":     float64(1.0),
				"ndcg10":    float64(1.0),
				"mrr":       float64(1.0),
				"map":       float64(1.0),
			},
			"generation_metrics": map[string]any{
				"bleu1":  float64(0.2),
				"rouge1": float64(0.3),
			},
		},
	}
}

func TestGenerateBaseline(t *testing.T) {
	baseline, err := GenerateBaseline(sampleBaselineReport(), BaselineGenOptions{
		ApprovedBy:     "lqs",
		ApprovedCommit: "abc123",
		Note:           "demo smoke",
	})
	if err != nil {
		t.Fatalf("GenerateBaseline: %v", err)
	}
	if baseline.Version != 1 || baseline.ConfigHash != "hash123" {
		t.Errorf("baseline=%+v", baseline)
	}
	if baseline.Dataset.SHA256 != "ds123" {
		t.Errorf("dataset=%+v", baseline.Dataset)
	}
	if baseline.Metrics.Retrieval.Recall.Baseline != 0.9 {
		t.Errorf("recall baseline=%v, want 0.9", baseline.Metrics.Retrieval.Recall.Baseline)
	}
	if baseline.Metrics.Generation.BLEU1.Baseline != 0.2 {
		t.Errorf("bleu1 baseline=%v, want 0.2", baseline.Metrics.Generation.BLEU1.Baseline)
	}
	if baseline.Metadata.ApprovedBy != "lqs" || baseline.Metadata.ApprovedCommit != "abc123" {
		t.Errorf("metadata=%+v", baseline.Metadata)
	}
}

func TestGenerateBaselineRequiresApproval(t *testing.T) {
	_, err := GenerateBaseline(sampleBaselineReport(), BaselineGenOptions{ApprovedBy: "lqs"})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestWriteAndLoadBaseline(t *testing.T) {
	baseline, err := GenerateBaseline(sampleBaselineReport(), BaselineGenOptions{
		ApprovedBy: "lqs", ApprovedCommit: "abc123",
	})
	if err != nil {
		t.Fatalf("GenerateBaseline: %v", err)
	}
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := WriteBaseline(path, baseline, false); err != nil {
		t.Fatalf("WriteBaseline: %v", err)
	}
	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if loaded.ConfigHash != "hash123" || loaded.Dataset.SHA256 != "ds123" {
		t.Errorf("loaded=%+v", loaded)
	}
}

func TestWriteBaselineRejectsOverwriteWithoutForce(t *testing.T) {
	baseline, err := GenerateBaseline(sampleBaselineReport(), BaselineGenOptions{
		ApprovedBy: "lqs", ApprovedCommit: "abc123",
	})
	if err != nil {
		t.Fatalf("GenerateBaseline: %v", err)
	}
	path := filepath.Join(t.TempDir(), "demo.yaml")
	if err := WriteBaseline(path, baseline, false); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteBaseline(path, baseline, false); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("second write err=%v, want ErrInvalidConfig", err)
	}
	if err := WriteBaseline(path, baseline, true); err != nil {
		t.Fatalf("force write: %v", err)
	}
}

func TestLoadBaselineRejectsBadVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("version: 2\nconfig_hash: x\ndataset:\n  sha256: y\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadBaseline(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}
