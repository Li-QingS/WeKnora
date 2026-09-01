package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartEvaluation(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/evaluation" {
			t.Fatalf("path=%s, want /api/v1/evaluation", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"task": {"id":"run-1","tenant_id":1,"dataset_id":"demo","status":0,"total":0,"finished":0},
				"params": {},
				"metric": null
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	detail, err := client.StartEvaluation(context.Background(), &EvaluationRequest{
		DatasetID:        "demo",
		ChatModelID:      "chat-1",
		EmbeddingModelID: "embed-1",
	})
	if err != nil {
		t.Fatalf("StartEvaluation: %v", err)
	}
	if detail.Task.ID != "run-1" {
		t.Errorf("task id=%q, want run-1", detail.Task.ID)
	}
	if gotBody["dataset_id"] != "demo" {
		t.Errorf("body dataset_id=%v, want demo", gotBody["dataset_id"])
	}
	if gotBody["chat_id"] != "chat-1" {
		t.Errorf("body chat_id=%v, want chat-1", gotBody["chat_id"])
	}
	if gotBody["embedding_id"] != "embed-1" {
		t.Errorf("body embedding_id=%v, want embed-1", gotBody["embedding_id"])
	}
}

func TestGetEvaluationResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/evaluation" {
			t.Fatalf("path=%s, want /api/v1/evaluation", r.URL.Path)
		}
		if r.URL.Query().Get("task_id") != "run-1" {
			t.Fatalf("task_id=%q, want run-1", r.URL.Query().Get("task_id"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"task": {"id":"run-1","dataset_id":"demo","status":2,"total":1,"finished":1},
				"params": {"embedding_top_k": 30},
				"metric": {"retrieval_metrics":{"precision":1},"generation_metrics":{"bleu1":0.2}}
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	detail, err := client.GetEvaluationResult(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetEvaluationResult: %v", err)
	}
	if detail.Task.Status != 2 {
		t.Errorf("status=%d, want 2", detail.Task.Status)
	}
	if !strings.Contains(string(detail.Params), `"embedding_top_k"`) {
		t.Errorf("params=%s, want embedding_top_k", string(detail.Params))
	}
	if !strings.Contains(string(detail.Metric), `"precision":1`) {
		t.Errorf("metric=%s, want precision", string(detail.Metric))
	}
}

func TestListEvaluationRuns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/evaluation/runs" {
			t.Fatalf("path=%s, want /api/v1/evaluation/runs", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("page_size") != "10" {
			t.Fatalf("query=%s, want page=1&page_size=10", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [{
				"id":"run-1",
				"dataset_id":"demo",
				"status":2,
				"config_hash":"abc123",
				"config_snapshot":{"dataset":{"id":"demo"}}
			}],
			"total": 1,
			"page": 1,
			"page_size": 10
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	runs, total, err := client.ListEvaluationRuns(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("ListEvaluationRuns: %v", err)
	}
	if total != 1 {
		t.Errorf("total=%d, want 1", total)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" || runs[0].ConfigHash != "abc123" {
		t.Errorf("runs=%+v, want one run with id run-1 and config_hash abc123", runs)
	}
}

func TestStartEvaluationHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":1003,"message":"not found"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.StartEvaluation(context.Background(), &EvaluationRequest{DatasetID: "demo"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("apiErr=%+v, want 404", apiErr)
	}
}
