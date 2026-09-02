// Package evalrunner implements the WP2 reproducible evaluation runner:
// config loading, orchestration, polling and report generation.
package evalrunner

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrInvalidConfig marks any configuration or dataset-local validation failure.
var ErrInvalidConfig = errors.New("invalid eval config")

var configDatasetIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// RunnerConfig is the YAML configuration for one baseline run.
type RunnerConfig struct {
	DatasetID  string           `yaml:"dataset_id"`
	Models     RunnerModels     `yaml:"models"`
	Chunking   RunnerChunking   `yaml:"chunking"`
	Retrieval  RunnerRetrieval  `yaml:"retrieval"`
	Generation RunnerGeneration `yaml:"generation"`
	Execution  RunnerExecution  `yaml:"execution"`
}

// RunnerModels names the models by ID or name. Empty values use server defaults.
type RunnerModels struct {
	Chat      string `yaml:"chat"`
	Embedding string `yaml:"embedding"`
	Rerank    string `yaml:"rerank"`
}

// RunnerRetrieval holds optional retrieval parameter overrides.
type RunnerRetrieval struct {
	VectorThreshold  *float64 `yaml:"vector_threshold"`
	KeywordThreshold *float64 `yaml:"keyword_threshold"`
	EmbeddingTopK    *int     `yaml:"embedding_top_k"`
	RerankTopK       *int     `yaml:"rerank_top_k"`
	RerankThreshold  *float64 `yaml:"rerank_threshold"`
}

// RunnerChunking holds the optional chunking override. Zero values are
// resolved by the server to its normal defaults, so omitting the whole
// section keeps the historical passage-per-chunk evaluation behavior.
type RunnerChunking struct {
	Strategy     string   `yaml:"strategy"`
	ChunkSize    int      `yaml:"chunk_size"`
	ChunkOverlap int      `yaml:"chunk_overlap"`
	TokenLimit   int      `yaml:"token_limit"`
	Languages    []string `yaml:"languages"`
}

// RunnerGeneration holds optional generation parameter overrides.
type RunnerGeneration struct {
	MaxTokens           *int     `yaml:"max_tokens"`
	RepeatPenalty       *float64 `yaml:"repeat_penalty"`
	TopK                *int     `yaml:"top_k"`
	TopP                *float64 `yaml:"top_p"`
	Temperature         *float64 `yaml:"temperature"`
	Seed                *int     `yaml:"seed"`
	MaxCompletionTokens *int     `yaml:"max_completion_tokens"`
}

// RunnerExecution controls polling and report output.
type RunnerExecution struct {
	Wait      *bool  `yaml:"wait"`
	Timeout   string `yaml:"timeout"`
	Interval  string `yaml:"interval"`
	ReportDir string `yaml:"report_dir"`
}

// LoadConfig reads, parses and validates a YAML config file.
func LoadConfig(path string) (*RunnerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidConfig, path, err)
	}
	var cfg RunnerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrInvalidConfig, path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the config and fills defaults for execution settings.
func (c *RunnerConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: nil config", ErrInvalidConfig)
	}
	if c.DatasetID == "" {
		return fmt.Errorf("%w: dataset_id is required", ErrInvalidConfig)
	}
	if !configDatasetIDPattern.MatchString(c.DatasetID) {
		return fmt.Errorf("%w: unsafe dataset_id %q", ErrInvalidConfig, c.DatasetID)
	}
	if err := c.Chunking.validate(); err != nil {
		return err
	}
	if err := c.Retrieval.validate(); err != nil {
		return err
	}
	if err := c.Generation.validate(); err != nil {
		return err
	}
	if c.Execution.ReportDir == "" {
		c.Execution.ReportDir = "artifacts/evaluation"
	}
	if c.Execution.Timeout == "" {
		c.Execution.Timeout = "30m"
	}
	if c.Execution.Interval == "" {
		c.Execution.Interval = "2s"
	}
	if _, err := time.ParseDuration(c.Execution.Timeout); err != nil {
		return fmt.Errorf("%w: invalid execution.timeout %q: %v", ErrInvalidConfig, c.Execution.Timeout, err)
	}
	if _, err := time.ParseDuration(c.Execution.Interval); err != nil {
		return fmt.Errorf("%w: invalid execution.interval %q: %v", ErrInvalidConfig, c.Execution.Interval, err)
	}
	return nil
}

// Enabled reports whether the config pins an explicit chunking override.
func (c RunnerChunking) Enabled() bool {
	return c.Strategy != "" || c.ChunkSize != 0 || c.ChunkOverlap != 0 ||
		c.TokenLimit != 0 || len(c.Languages) > 0
}

func (c RunnerChunking) validate() error {
	if !c.Enabled() {
		return nil
	}
	strategy := strings.ToLower(strings.TrimSpace(c.Strategy))
	if strategy == "" {
		strategy = "recursive"
	}
	switch strategy {
	case "passthrough", "auto", "heading", "heuristic", "recursive", "legacy":
	default:
		return fmt.Errorf("%w: unsupported chunking strategy %q", ErrInvalidConfig, c.Strategy)
	}
	if c.ChunkSize < 0 {
		return fmt.Errorf("%w: chunk_size must be non-negative", ErrInvalidConfig)
	}
	if c.ChunkOverlap < 0 {
		return fmt.Errorf("%w: chunk_overlap must be non-negative", ErrInvalidConfig)
	}
	if c.ChunkOverlap > 0 && c.ChunkSize > 0 && c.ChunkOverlap >= c.ChunkSize {
		return fmt.Errorf("%w: chunk_overlap must be less than chunk_size", ErrInvalidConfig)
	}
	if c.TokenLimit < 0 {
		return fmt.Errorf("%w: token_limit must be non-negative", ErrInvalidConfig)
	}
	return nil
}

// Wait returns whether the runner should block until the run is terminal.
// The config default is true.
func (c *RunnerConfig) Wait() bool {
	if c == nil || c.Execution.Wait == nil {
		return true
	}
	return *c.Execution.Wait
}

// Timeout returns the parsed wait timeout.
func (c *RunnerConfig) Timeout() (time.Duration, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: nil config", ErrInvalidConfig)
	}
	return time.ParseDuration(c.Execution.Timeout)
}

// Interval returns the parsed poll interval.
func (c *RunnerConfig) Interval() (time.Duration, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: nil config", ErrInvalidConfig)
	}
	return time.ParseDuration(c.Execution.Interval)
}

func (r RunnerRetrieval) validate() error {
	if r.VectorThreshold != nil && (*r.VectorThreshold < 0 || *r.VectorThreshold > 1) {
		return fmt.Errorf("%w: vector_threshold must be in [0,1]", ErrInvalidConfig)
	}
	if r.KeywordThreshold != nil && (*r.KeywordThreshold < 0 || *r.KeywordThreshold > 1) {
		return fmt.Errorf("%w: keyword_threshold must be in [0,1]", ErrInvalidConfig)
	}
	if r.EmbeddingTopK != nil && *r.EmbeddingTopK <= 0 {
		return fmt.Errorf("%w: embedding_top_k must be positive", ErrInvalidConfig)
	}
	if r.RerankTopK != nil && *r.RerankTopK <= 0 {
		return fmt.Errorf("%w: rerank_top_k must be positive", ErrInvalidConfig)
	}
	if r.RerankThreshold != nil && (*r.RerankThreshold < 0 || *r.RerankThreshold > 1) {
		return fmt.Errorf("%w: rerank_threshold must be in [0,1]", ErrInvalidConfig)
	}
	return nil
}

func (g RunnerGeneration) validate() error {
	if g.MaxTokens != nil && *g.MaxTokens < 0 {
		return fmt.Errorf("%w: max_tokens must be non-negative", ErrInvalidConfig)
	}
	if g.RepeatPenalty != nil && *g.RepeatPenalty < 0 {
		return fmt.Errorf("%w: repeat_penalty must be non-negative", ErrInvalidConfig)
	}
	if g.TopK != nil && *g.TopK < 0 {
		return fmt.Errorf("%w: top_k must be non-negative", ErrInvalidConfig)
	}
	if g.TopP != nil && (*g.TopP < 0 || *g.TopP > 1) {
		return fmt.Errorf("%w: top_p must be in [0,1]", ErrInvalidConfig)
	}
	if g.Temperature != nil && (*g.Temperature < 0 || *g.Temperature > 2) {
		return fmt.Errorf("%w: temperature must be in [0,2]", ErrInvalidConfig)
	}
	if g.MaxCompletionTokens != nil && *g.MaxCompletionTokens <= 0 {
		return fmt.Errorf("%w: max_completion_tokens must be positive", ErrInvalidConfig)
	}
	return nil
}
