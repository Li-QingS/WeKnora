// Package client provides the implementation for interacting with the WeKnora API.
// The Evaluation types mirror the server's evaluation contract: create a run,
// poll its result, and list historical runs.
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// EvaluationTask mirrors the task object returned by POST /evaluation and
// GET /evaluation?task_id=...
type EvaluationTask struct {
	ID        string `json:"id"`
	TenantID  uint64 `json:"tenant_id"`
	DatasetID string `json:"dataset_id"`
	StartTime string `json:"start_time"`
	Status    int    `json:"status"`
	ErrMsg    string `json:"err_msg,omitempty"`
	Total     int    `json:"total,omitempty"`
	Finished  int    `json:"finished,omitempty"`
}

// EvaluationDetail is the data envelope of an evaluation task query.
type EvaluationDetail struct {
	Task   EvaluationTask  `json:"task"`
	Params json.RawMessage `json:"params"`
	Metric json.RawMessage `json:"metric,omitempty"`
}

// EvaluationRequest creates an evaluation run. All model fields are optional;
// empty values fall back to server defaults.
type EvaluationRequest struct {
	DatasetID        string              `json:"dataset_id"`
	KnowledgeBaseID  string              `json:"knowledge_base_id,omitempty"`
	ChatModelID      string              `json:"chat_id,omitempty"`
	RerankModelID    string              `json:"rerank_id,omitempty"`
	EmbeddingModelID string              `json:"embedding_id,omitempty"`
	Chunking         *EvaluationChunking `json:"chunking,omitempty"`
	Params           any                 `json:"params,omitempty"`
}

// EvaluationChunking mirrors the server's optional chunking override.
// Strategy "passthrough" keeps one chunk per corpus passage.
type EvaluationChunking struct {
	Strategy     string   `json:"strategy"`
	ChunkSize    int      `json:"chunk_size"`
	ChunkOverlap int      `json:"chunk_overlap"`
	TokenLimit   int      `json:"token_limit,omitempty"`
	Languages    []string `json:"languages,omitempty"`
}

// EvaluationRun mirrors one item of GET /evaluation/runs.
type EvaluationRun struct {
	ID             string          `json:"id"`
	TenantID       uint64          `json:"tenant_id"`
	DatasetID      string          `json:"dataset_id"`
	Status         int             `json:"status"`
	ErrMsg         string          `json:"err_msg"`
	Total          int             `json:"total"`
	Finished       int             `json:"finished"`
	Params         json.RawMessage `json:"params"`
	Metric         json.RawMessage `json:"metric,omitempty"`
	ConfigHash     string          `json:"config_hash"`
	ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

type evaluationResponse struct {
	Success bool             `json:"success"`
	Data    EvaluationDetail `json:"data"`
}

type evaluationRunListResponse struct {
	Success  bool            `json:"success"`
	Data     []EvaluationRun `json:"data"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// StartEvaluation creates a new evaluation run and returns its initial state.
func (c *Client) StartEvaluation(ctx context.Context, request *EvaluationRequest) (*EvaluationDetail, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/evaluation", request, nil)
	if err != nil {
		return nil, err
	}

	var response evaluationResponse
	if err := parseResponse(resp, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetEvaluationResult retrieves the current state of an evaluation run.
func (c *Client) GetEvaluationResult(ctx context.Context, taskID string) (*EvaluationDetail, error) {
	queryParams := url.Values{}
	queryParams.Add("task_id", taskID)

	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/evaluation", nil, queryParams)
	if err != nil {
		return nil, err
	}

	var response evaluationResponse
	if err := parseResponse(resp, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// ListEvaluationRuns returns the tenant's evaluation runs for a page.
// It returns the runs and the server-reported total count.
func (c *Client) ListEvaluationRuns(ctx context.Context, page, pageSize int) ([]EvaluationRun, int, error) {
	queryParams := url.Values{}
	queryParams.Add("page", strconv.Itoa(page))
	queryParams.Add("page_size", strconv.Itoa(pageSize))

	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/evaluation/runs", nil, queryParams)
	if err != nil {
		return nil, 0, err
	}

	var response evaluationRunListResponse
	if err := parseResponse(resp, &response); err != nil {
		return nil, 0, err
	}
	if response.Data == nil {
		response.Data = []EvaluationRun{}
	}
	return response.Data, response.Total, nil
}
