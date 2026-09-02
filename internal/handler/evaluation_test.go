package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeEvaluationService struct {
	opts   *types.EvaluationOptions
	detail *types.EvaluationDetail
	err    error
}

func (f *fakeEvaluationService) Evaluation(_ context.Context, opts *types.EvaluationOptions) (*types.EvaluationDetail, error) {
	f.opts = opts
	return f.detail, f.err
}

func (f *fakeEvaluationService) ListAvailableDatasets(context.Context) ([]*types.EvaluationDatasetMeta, error) {
	return []*types.EvaluationDatasetMeta{
		{ID: "default", SampleCount: 1},
	}, nil
}

func (f *fakeEvaluationService) DeleteEvaluationRun(context.Context, string) error {
	return f.err
}

func (f *fakeEvaluationService) EvaluationResult(context.Context, string) (*types.EvaluationDetail, error) {
	return f.detail, f.err
}

func (f *fakeEvaluationService) ListEvaluationRuns(
	context.Context,
	*types.EvaluationStatue,
	*types.Pagination,
) (*types.PageResult, error) {
	return nil, nil
}

func newEvaluationHandlerTestRouter(svc *fakeEvaluationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	h := NewEvaluationHandler(svc)
	r.POST("/evaluation", h.Evaluation)
	r.GET("/evaluation/datasets", h.ListAvailableDatasets)
	r.DELETE("/evaluation/runs/:id", h.DeleteEvaluationRun)
	return r
}

func TestEvaluationHandler_MapsOptionsFromRequest(t *testing.T) {
	svc := &fakeEvaluationService{detail: &types.EvaluationDetail{}}
	router := newEvaluationHandlerTestRouter(svc)

	body := []byte(`{
		"dataset_id": "demo",
		"chat_id": "chat-1",
		"embedding_id": "embed-1",
		"rerank_id": "rerank-1",
		"chunking": {
			"strategy": "recursive",
			"chunk_size": 1024,
			"chunk_overlap": 80
		},
		"params": {
			"embedding_top_k": 20,
			"summary_config": {"temperature": 0.5}
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/evaluation", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.opts)
	assert.Equal(t, "demo", svc.opts.DatasetID)
	assert.Equal(t, "chat-1", svc.opts.ChatModelID)
	assert.Equal(t, "embed-1", svc.opts.EmbeddingModelID)
	require.NotNil(t, svc.opts.Chunking)
	assert.Equal(t, "recursive", svc.opts.Chunking.Strategy)
	assert.Equal(t, 1024, svc.opts.Chunking.ChunkSize)
	assert.Equal(t, 80, svc.opts.Chunking.ChunkOverlap)
	require.NotNil(t, svc.opts.Params)
	require.NotNil(t, svc.opts.Params.EmbeddingTopK)
	assert.Equal(t, 20, *svc.opts.Params.EmbeddingTopK)
	require.NotNil(t, svc.opts.Params.SummaryConfig)
	require.NotNil(t, svc.opts.Params.SummaryConfig.Temperature)
	assert.Equal(t, 0.5, *svc.opts.Params.SummaryConfig.Temperature)
}

func TestEvaluationHandler_MapsDatasetErrorsToBadRequest(t *testing.T) {
	for _, sentinel := range []error{
		service.ErrDatasetNotFound,
		service.ErrInvalidDataset,
		service.ErrInvalidEvaluationParams,
	} {
		svc := &fakeEvaluationService{err: fmt.Errorf("wrapped: %w", sentinel)}
		router := newEvaluationHandlerTestRouter(svc)
		req := httptest.NewRequest(http.MethodPost, "/evaluation",
			bytes.NewReader([]byte(`{"dataset_id":"demo"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "sentinel: %v", sentinel)
	}
}

func TestEvaluationHandler_ListsAvailableDatasets(t *testing.T) {
	svc := &fakeEvaluationService{detail: &types.EvaluationDetail{}}
	router := newEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/evaluation/datasets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"default"`)
	assert.Contains(t, w.Body.String(), `"sample_count":1`)
}

func TestEvaluationHandler_DeleteEvaluationRun(t *testing.T) {
	svc := &fakeEvaluationService{detail: &types.EvaluationDetail{}}
	router := newEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodDelete, "/evaluation/runs/run-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"success":true`)

	svc.err = service.ErrEvaluationRunActive
	req = httptest.NewRequest(http.MethodDelete, "/evaluation/runs/run-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
