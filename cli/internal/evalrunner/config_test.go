package evalrunner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "eval.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigValidWithDefaults(t *testing.T) {
	path := writeConfig(t, `
dataset_id: demo
models:
  chat: chat-1
retrieval:
  embedding_top_k: 30
execution:
  wait: true
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DatasetID != "demo" || cfg.Models.Chat != "chat-1" {
		t.Errorf("cfg=%+v", cfg)
	}
	if cfg.Execution.ReportDir != "artifacts/evaluation" {
		t.Errorf("report_dir=%q, want default", cfg.Execution.ReportDir)
	}
	timeout, err := cfg.Timeout()
	if err != nil || timeout != 30*time.Minute {
		t.Errorf("timeout=%v err=%v, want 30m", timeout, err)
	}
	interval, err := cfg.Interval()
	if err != nil || interval != 2*time.Second {
		t.Errorf("interval=%v err=%v, want 2s", interval, err)
	}
	if !cfg.Wait() {
		t.Error("wait default should be true")
	}
}

func TestLoadConfigChunking(t *testing.T) {
	path := writeConfig(t, `
dataset_id: enterprise_rag
chunking:
  strategy: recursive
  chunk_size: 1024
  chunk_overlap: 80
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Chunking.Enabled() {
		t.Fatal("chunking should be enabled")
	}
	if cfg.Chunking.Strategy != "recursive" ||
		cfg.Chunking.ChunkSize != 1024 ||
		cfg.Chunking.ChunkOverlap != 80 {
		t.Errorf("chunking=%+v", cfg.Chunking)
	}
}

func TestLoadConfigBadChunking(t *testing.T) {
	path := writeConfig(t, "dataset_id: demo\nchunking:\n  strategy: recursive\n  chunk_size: 512\n  chunk_overlap: 600")
	_, err := LoadConfig(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigBadYAML(t *testing.T) {
	path := writeConfig(t, "dataset_id: [unclosed")
	_, err := LoadConfig(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigUnsafeDatasetID(t *testing.T) {
	path := writeConfig(t, "dataset_id: ../escape")
	_, err := LoadConfig(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigBadThreshold(t *testing.T) {
	path := writeConfig(t, "dataset_id: demo\nretrieval:\n  vector_threshold: 1.5")
	_, err := LoadConfig(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigBadDuration(t *testing.T) {
	path := writeConfig(t, "dataset_id: demo\nexecution:\n  timeout: nope")
	_, err := LoadConfig(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err=%v, want ErrInvalidConfig", err)
	}
}
