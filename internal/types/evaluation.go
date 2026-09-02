package types

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yanyiwu/gojieba"
)

// Jieba is a global instance of Chinese text segmentation tool
var Jieba *gojieba.Jieba = newJieba()

func newJieba() *gojieba.Jieba {
	dictDir := os.Getenv("JIEBA_DICT_DIR")
	if dictDir == "" {
		return gojieba.NewJieba()
	}

	return gojieba.NewJieba(
		filepath.Join(dictDir, "jieba.dict.utf8"),
		filepath.Join(dictDir, "hmm_model.utf8"),
		filepath.Join(dictDir, "user.dict.utf8"),
		filepath.Join(dictDir, "idf.utf8"),
		filepath.Join(dictDir, "stop_words.utf8"),
	)
}

// EvaluationStatue represents the status of an evaluation task
type EvaluationStatue int

const (
	EvaluationStatuePending     EvaluationStatue = iota // Task is waiting to start
	EvaluationStatueRunning                             // Task is in progress
	EvaluationStatueSuccess                             // Task completed successfully
	EvaluationStatueFailed                              // Task failed
	EvaluationStatueInterrupted                         // Task interrupted by service restart
)

// EvaluationRun is the persisted representation of an evaluation task.
type EvaluationRun struct {
	ID             string           `gorm:"primaryKey;type:varchar(128)" json:"id"`
	TenantID       uint64           `gorm:"index" json:"tenant_id"`
	DatasetID      string           `gorm:"type:varchar(128)" json:"dataset_id"`
	Status         EvaluationStatue `json:"status"`
	StartTime      time.Time        `json:"start_time"`
	ErrMsg         string           `json:"err_msg"`
	Total          int              `json:"total"`
	Finished       int              `json:"finished"`
	Params         json.RawMessage  `gorm:"type:jsonb;default:'{}'" json:"params"`
	Metric         json.RawMessage  `gorm:"type:jsonb" json:"metric,omitempty"`
	HeartbeatAt    *time.Time       `json:"heartbeat_at,omitempty"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	ConfigHash     string           `gorm:"type:varchar(64)" json:"config_hash"`
	ConfigSnapshot json.RawMessage  `gorm:"type:jsonb;default:'{}'" json:"config_snapshot"`
	TemporaryKBID  string           `gorm:"type:varchar(128)" json:"temporary_kb_id"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// TableName returns the database table name for EvaluationRun.
func (EvaluationRun) TableName() string {
	return "evaluation_runs"
}

// EvaluationConfigSnapshot captures the effective, non-sensitive evaluation
// configuration so a historical run can be explained and compared.
type EvaluationConfigSnapshot struct {
	Dataset  DatasetSnapshot   `json:"dataset"`
	Models   []ModelSnapshot   `json:"models"`
	Chunking *ChunkingSnapshot `json:"chunking,omitempty"`
	Version  VersionSignature  `json:"version"`
}

// DatasetSnapshot identifies the dataset and its content fingerprint.
type DatasetSnapshot struct {
	ID          string `json:"id"`
	SHA256      string `json:"sha256"`
	SampleCount int    `json:"sample_count"`
}

// EvaluationDatasetMeta is the lightweight metadata shown in dataset pickers.
type EvaluationDatasetMeta struct {
	ID          string `json:"id"`
	SHA256      string `json:"sha256"`
	SampleCount int    `json:"sample_count"`
}

// ChunkingSnapshot records the chunking parameters that produced a run.
// It is only present when the caller pinned an explicit chunking strategy;
// legacy runs without it keep the historical passage-per-chunk behavior.
type ChunkingSnapshot struct {
	Strategy     string   `json:"strategy"`
	ChunkSize    int      `json:"chunk_size"`
	ChunkOverlap int      `json:"chunk_overlap"`
	TokenLimit   int      `json:"token_limit,omitempty"`
	Languages    []string `json:"languages,omitempty"`
}

// ModelSnapshot contains only the semantic model identity, never credentials.
type ModelSnapshot struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Type     string `json:"type"`
}

// VersionSignature records the build provenance of the application.
type VersionSignature struct {
	AppVersion string `json:"app_version"`
	GitCommit  string `json:"git_commit"`
	GitDirty   bool   `json:"git_dirty"`
	GoVersion  string `json:"go_version"`
}

// EvaluationTask contains information about an evaluation task
type EvaluationTask struct {
	ID        string `json:"id"`         // Unique task ID
	TenantID  uint64 `json:"tenant_id"`  // Tenant/Organization ID
	DatasetID string `json:"dataset_id"` // Dataset ID for evaluation

	StartTime time.Time        `json:"start_time"`        // Task start time
	Status    EvaluationStatue `json:"status"`            // Current task status
	ErrMsg    string           `json:"err_msg,omitempty"` // Error message if failed

	Total    int `json:"total,omitempty"`    // Total items to evaluate
	Finished int `json:"finished,omitempty"` // Completed items count
}

// EvaluationDetail contains detailed evaluation information
type EvaluationDetail struct {
	Task   *EvaluationTask `json:"task"`             // Evaluation task info
	Params *ChatManage     `json:"params"`           // Evaluation parameters
	Metric *MetricResult   `json:"metric,omitempty"` // Evaluation metrics
}

// String returns JSON representation of EvaluationTask
func (e *EvaluationTask) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// MetricInput contains input data for metric calculation
type MetricInput struct {
	RetrievalGT  [][]int // Ground truth for retrieval
	RetrievalIDs []int   // Retrieved IDs

	GeneratedTexts string // Generated text for evaluation
	GeneratedGT    string // Ground truth text for comparison
}

// MetricResult contains evaluation metrics
type MetricResult struct {
	RetrievalMetrics  RetrievalMetrics          `json:"retrieval_metrics"`  // Retrieval performance metrics
	GenerationMetrics GenerationMetrics         `json:"generation_metrics"` // Text generation quality metrics
	CostMetrics       *EvaluationCostMetrics    `json:"cost_metrics,omitempty"`
	LatencyMetrics    *EvaluationLatencyMetrics `json:"latency_metrics,omitempty"`
}

// EvaluationCostMetrics summarizes model ledger rows attributed to one run.
type EvaluationCostMetrics struct {
	ModelCalls       int64    `json:"model_calls"`
	PromptTokens     int64    `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
	TotalTokens      int64    `json:"total_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`
	EstimatedCostUSD *float64 `json:"estimated_cost_usd"`
}

// EvaluationLatencyMetrics reports wall-clock and average model-call latency.
type EvaluationLatencyMetrics struct {
	DurationMS        int64   `json:"duration_ms"`
	AvgMsPerSample    float64 `json:"avg_ms_per_sample"`
	ModelCalls        int64   `json:"model_calls"`
	AvgMsPerModelCall float64 `json:"avg_ms_per_model_call"`
}

// RetrievalMetrics contains metrics for retrieval evaluation
type RetrievalMetrics struct {
	Precision float64 `json:"precision"` // Precision score
	Recall    float64 `json:"recall"`    // Recall score

	NDCG3  float64 `json:"ndcg3"`  // Normalized Discounted Cumulative Gain at 3
	NDCG10 float64 `json:"ndcg10"` // Normalized Discounted Cumulative Gain at 10
	MRR    float64 `json:"mrr"`    // Mean Reciprocal Rank
	MAP    float64 `json:"map"`    // Mean Average Precision
}

// GenerationMetrics contains metrics for text generation evaluation
type GenerationMetrics struct {
	BLEU1 float64 `json:"bleu1"` // BLEU-1 score
	BLEU2 float64 `json:"bleu2"` // BLEU-2 score
	BLEU4 float64 `json:"bleu4"` // BLEU-4 score

	ROUGE1 float64 `json:"rouge1"` // ROUGE-1 score
	ROUGE2 float64 `json:"rouge2"` // ROUGE-2 score
	ROUGEL float64 `json:"rougel"` // ROUGE-L score
}

// EvalState represents different stages of evaluation process
type EvalState int

const (
	StateBegin             EvalState = iota // Evaluation started
	StateAfterQaPairs                       // After loading QA pairs
	StateAfterDataset                       // After processing dataset
	StateAfterEmbedding                     // After generating embeddings
	StateAfterVectorSearch                  // After vector search
	StateAfterRerank                        // After reranking
	StateAfterComplete                      // After completion
	StateEnd                                // Evaluation ended
)

// EvaluationOptions carries all inputs accepted by the evaluation API.
type EvaluationOptions struct {
	DatasetID        string                    `json:"dataset_id"`
	KnowledgeBaseID  string                    `json:"knowledge_base_id"`
	ChatModelID      string                    `json:"chat_id"`
	RerankModelID    string                    `json:"rerank_id"`
	EmbeddingModelID string                    `json:"embedding_id"`
	Chunking         *EvaluationChunkingConfig `json:"chunking,omitempty"`
	Params           *EvaluationParamsOverride `json:"params,omitempty"`
}

// EvaluationChunkingConfig is the chunking configuration accepted by the
// evaluation API. Strategy "passthrough" preserves one chunk per corpus
// passage; empty/nil disables the override entirely for backward
// compatibility.
type EvaluationChunkingConfig struct {
	Strategy     string   `json:"strategy"`
	ChunkSize    int      `json:"chunk_size"`
	ChunkOverlap int      `json:"chunk_overlap"`
	TokenLimit   int      `json:"token_limit,omitempty"`
	Languages    []string `json:"languages,omitempty"`
}

// EvaluationParamsOverride holds optional evaluation parameters. Pointer
// fields distinguish "not provided" from explicit zero values.
type EvaluationParamsOverride struct {
	VectorThreshold  *float64               `json:"vector_threshold,omitempty"`
	KeywordThreshold *float64               `json:"keyword_threshold,omitempty"`
	EmbeddingTopK    *int                   `json:"embedding_top_k,omitempty"`
	MaxRounds        *int                   `json:"max_rounds,omitempty"`
	RerankTopK       *int                   `json:"rerank_top_k,omitempty"`
	RerankThreshold  *float64               `json:"rerank_threshold,omitempty"`
	EnableRewrite    *bool                  `json:"enable_rewrite,omitempty"`
	SummaryConfig    *SummaryConfigOverride `json:"summary_config,omitempty"`
}

// SummaryConfigOverride contains the generation parameters a runner may
// override. Prompt text is intentionally not overridable through the API.
type SummaryConfigOverride struct {
	MaxTokens           *int     `json:"max_tokens,omitempty"`
	Temperature         *float64 `json:"temperature,omitempty"`
	TopK                *int     `json:"top_k,omitempty"`
	TopP                *float64 `json:"top_p,omitempty"`
	RepeatPenalty       *float64 `json:"repeat_penalty,omitempty"`
	FrequencyPenalty    *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64 `json:"presence_penalty,omitempty"`
	Seed                *int     `json:"seed,omitempty"`
	MaxCompletionTokens *int     `json:"max_completion_tokens,omitempty"`
}

// EvaluationDataset is a loaded, validated dataset with a deterministic
// content fingerprint.
type EvaluationDataset struct {
	ID          string
	SHA256      string
	SampleCount int
	Pairs       []*QAPair
}
