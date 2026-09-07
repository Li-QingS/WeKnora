package types

import (
	"encoding/json"
	"time"
)

const DefaultWikiSemanticThreshold = 0.80

// EvaluationReportFormat is a supported persisted-result report rendering.
type EvaluationReportFormat string

const (
	EvaluationReportJSON     EvaluationReportFormat = "json"
	EvaluationReportMarkdown EvaluationReportFormat = "markdown"
)

func (f EvaluationReportFormat) IsValid() bool {
	return f == EvaluationReportJSON || f == EvaluationReportMarkdown
}

// WikiEvaluationOptions carries all inputs accepted by the Wiki evaluation API.
type WikiEvaluationOptions struct {
	DatasetID                 string  `json:"dataset_id"`
	ChatModelID               string  `json:"chat_id"`
	EmbeddingModelID          string  `json:"embedding_id"`
	SemanticThreshold         float64 `json:"semantic_threshold"`
	SemanticThresholdProvided bool    `json:"-"`
}

func (o *WikiEvaluationOptions) UnmarshalJSON(data []byte) error {
	var payload struct {
		DatasetID         string   `json:"dataset_id"`
		ChatModelID       string   `json:"chat_id"`
		EmbeddingModelID  string   `json:"embedding_id"`
		SemanticThreshold *float64 `json:"semantic_threshold"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	o.DatasetID = payload.DatasetID
	o.ChatModelID = payload.ChatModelID
	o.EmbeddingModelID = payload.EmbeddingModelID
	o.SemanticThresholdProvided = payload.SemanticThreshold != nil
	if payload.SemanticThreshold == nil {
		o.SemanticThreshold = 0
	} else {
		o.SemanticThreshold = *payload.SemanticThreshold
	}
	return nil
}

// WikiGold is the versioned reference node/link graph for one dataset.
type WikiGold struct {
	SchemaVersion string         `json:"schema_version"`
	DatasetID     string         `json:"dataset_id"`
	DatasetSHA256 string         `json:"dataset_sha256"`
	ContentSHA256 string         `json:"content_sha256"`
	Nodes         []WikiGoldNode `json:"nodes"`
	Edges         []WikiGoldEdge `json:"edges"`
}

type WikiGoldNode struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type WikiGoldEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type WikiGoldSnapshot struct {
	SchemaVersion string `json:"schema_version"`
	DatasetSHA256 string `json:"dataset_sha256"`
	ContentSHA256 string `json:"content_sha256"`
	NodeCount     int    `json:"node_count"`
	EdgeCount     int    `json:"edge_count"`
}

type WikiGenerationSnapshot struct {
	IndexingStrategy IndexingStrategy `json:"indexing_strategy"`
	WikiConfig       WikiConfig       `json:"wiki_config"`
}

type WikiEvaluationConfigSnapshot struct {
	Dataset        DatasetSnapshot        `json:"dataset"`
	Gold           WikiGoldSnapshot       `json:"gold"`
	ChatModel      ModelSnapshot          `json:"chat_model"`
	EmbeddingModel ModelSnapshot          `json:"embedding_model"`
	Threshold      float64                `json:"semantic_threshold"`
	Wiki           WikiGenerationSnapshot `json:"wiki"`
	Version        VersionSignature       `json:"version"`
}

type WikiEvaluationDatasetMeta struct {
	ID            string           `json:"id"`
	SHA256        string           `json:"sha256"`
	DocumentCount int              `json:"document_count"`
	Gold          WikiGoldSnapshot `json:"gold"`
}

type WikiEvaluationPage struct {
	ID       string   `json:"id"`
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Aliases  []string `json:"aliases"`
	OutLinks []string `json:"out_links"`
}

type WikiNodeMatch struct {
	GoldNodeID string   `json:"gold_node_id"`
	GoldType   string   `json:"gold_type"`
	GoldName   string   `json:"gold_name"`
	PageID     string   `json:"page_id,omitempty"`
	PageSlug   string   `json:"page_slug,omitempty"`
	PageTitle  string   `json:"page_title,omitempty"`
	Method     string   `json:"method"`
	Score      *float64 `json:"score,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type WikiNodeMetric struct {
	GoldTotal       int     `json:"gold_total"`
	ExactMatched    int     `json:"exact_matched"`
	SemanticMatched int     `json:"semantic_matched"`
	Unmatched       int     `json:"unmatched"`
	Coverage        float64 `json:"coverage"`
}

type WikiEdgeRef struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Reason string `json:"reason,omitempty"`
}

type WikiGraphMetric struct {
	Correct   int     `json:"correct"`
	Missing   int     `json:"missing"`
	Extra     int     `json:"extra"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	Scorable  bool    `json:"scorable"`
	Note      string  `json:"note,omitempty"`
}

type WikiNodeScore struct {
	Entity       WikiNodeMetric    `json:"entity"`
	Concept      WikiNodeMetric    `json:"concept"`
	Overall      WikiNodeMetric    `json:"overall"`
	Matches      []WikiNodeMatch   `json:"matches"`
	PageToGoldID map[string]string `json:"-"`
}

type WikiGraphScore struct {
	Metric        WikiGraphMetric `json:"metric"`
	CorrectEdges  []WikiEdgeRef   `json:"correct_edges"`
	MissingEdges  []WikiEdgeRef   `json:"missing_edges"`
	ExtraEdges    []WikiEdgeRef   `json:"extra_edges"`
	UnscoredEdges []WikiEdgeRef   `json:"unscored_edges"`
}

type WikiEvaluationMetric struct {
	Entity         WikiNodeMetric         `json:"entity"`
	Concept        WikiNodeMetric         `json:"concept"`
	Overall        WikiNodeMetric         `json:"overall"`
	Graph          WikiGraphMetric        `json:"graph"`
	GenerationCost *EvaluationCostMetrics `json:"generation_cost,omitempty"`
	ScoringCost    *EvaluationCostMetrics `json:"scoring_cost,omitempty"`
}

type WikiEvaluationResult struct {
	Metric        WikiEvaluationMetric `json:"metric"`
	NodeMatches   []WikiNodeMatch      `json:"node_matches"`
	CorrectEdges  []WikiEdgeRef        `json:"correct_edges"`
	MissingEdges  []WikiEdgeRef        `json:"missing_edges"`
	ExtraEdges    []WikiEdgeRef        `json:"extra_edges"`
	UnscoredEdges []WikiEdgeRef        `json:"unscored_edges"`
	FrozenPages   []WikiEvaluationPage `json:"frozen_pages"`
}

type WikiEvaluationDetail struct {
	Run            *EvaluationRun                `json:"run"`
	Params         *WikiEvaluationOptions        `json:"params"`
	ConfigSnapshot *WikiEvaluationConfigSnapshot `json:"config_snapshot,omitempty"`
	Metric         *WikiEvaluationMetric         `json:"metric,omitempty"`
	Result         *WikiEvaluationResult         `json:"result,omitempty"`
}

// WikiEvaluationReportMetadata is included in rendered reports.
type WikiEvaluationReportMetadata struct {
	GeneratedAt time.Time `json:"generated_at"`
}
