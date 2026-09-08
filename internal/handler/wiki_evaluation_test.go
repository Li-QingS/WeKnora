package handler

import (
	"bytes"
	"context"
	"errors"
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

type fakeWikiEvaluationService struct {
	opts     *types.WikiEvaluationOptions
	detail   *types.WikiEvaluationDetail
	datasets []*types.WikiEvaluationDatasetMeta
	runs     *types.PageResult
	report   []byte
	mimeType string
	err      error
}

func (f *fakeWikiEvaluationService) Start(
	_ context.Context,
	opts *types.WikiEvaluationOptions,
) (*types.WikiEvaluationDetail, error) {
	f.opts = opts
	return f.detail, f.err
}

func (f *fakeWikiEvaluationService) Get(context.Context, string) (*types.WikiEvaluationDetail, error) {
	return f.detail, f.err
}

func (f *fakeWikiEvaluationService) ListDatasets(context.Context) ([]*types.WikiEvaluationDatasetMeta, error) {
	return f.datasets, f.err
}

func (f *fakeWikiEvaluationService) ListRuns(
	context.Context,
	*types.EvaluationStatue,
	*types.Pagination,
) (*types.PageResult, error) {
	return f.runs, f.err
}

func (f *fakeWikiEvaluationService) RenderReport(
	context.Context,
	string,
	types.EvaluationReportFormat,
) ([]byte, string, error) {
	return f.report, f.mimeType, f.err
}

func (f *fakeWikiEvaluationService) DeleteRun(context.Context, string) error {
	return f.err
}

func newWikiEvaluationHandlerTestRouter(svc *fakeWikiEvaluationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	h := NewWikiEvaluationHandler(svc)
	r.POST("/evaluation/wiki/runs", h.Start)
	r.GET("/evaluation/wiki/datasets", h.ListDatasets)
	r.GET("/evaluation/wiki/runs", h.List)
	r.GET("/evaluation/wiki/runs/:id", h.Get)
	r.GET("/evaluation/wiki/runs/:id/report", h.Report)
	r.DELETE("/evaluation/wiki/runs/:id", h.Delete)
	return r
}

func TestWikiEvaluationHandler_StartMapsOptionsAndReturnsAccepted(t *testing.T) {
	svc := &fakeWikiEvaluationService{detail: &types.WikiEvaluationDetail{}}
	router := newWikiEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/evaluation/wiki/runs", bytes.NewReader([]byte(`{
		"dataset_id":"enterprise_rag",
		"chat_id":"chat-1",
		"embedding_id":"embedding-1",
		"semantic_threshold":0.82
	}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.NotNil(t, svc.opts)
	assert.Equal(t, "enterprise_rag", svc.opts.DatasetID)
	assert.Equal(t, "chat-1", svc.opts.ChatModelID)
	assert.Equal(t, "embedding-1", svc.opts.EmbeddingModelID)
	assert.Equal(t, 0.82, svc.opts.SemanticThreshold)
}

func TestWikiEvaluationHandler_StartDistinguishesInputAndServerErrors(t *testing.T) {
	svc := &fakeWikiEvaluationService{err: service.ErrInvalidWikiEvaluationParams}
	router := newWikiEvaluationHandlerTestRouter(svc)
	body := []byte(`{"dataset_id":"enterprise_rag","chat_id":"chat-1","embedding_id":"embedding-1"}`)

	req := httptest.NewRequest(http.MethodPost, "/evaluation/wiki/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	svc.err = errors.New("database unavailable")
	req = httptest.NewRequest(http.MethodPost, "/evaluation/wiki/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWikiEvaluationHandler_ReportReturnsPersistedArtifact(t *testing.T) {
	svc := &fakeWikiEvaluationService{
		report:   []byte("# Wiki evaluation"),
		mimeType: "text/markdown; charset=utf-8",
	}
	router := newWikiEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/evaluation/wiki/runs/run-1/report?format=markdown", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "# Wiki evaluation", w.Body.String())
	assert.Equal(t, "text/markdown; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="wiki-evaluation-run-1.markdown"`, w.Header().Get("Content-Disposition"))
}

func TestWikiEvaluationHandler_MapsInvalidReportAndMissingRun(t *testing.T) {
	svc := &fakeWikiEvaluationService{err: errors.New("unsupported report format")}
	router := newWikiEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/evaluation/wiki/runs/run-1/report?format=pdf", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	svc.err = service.ErrEvaluationTaskNotFound
	req = httptest.NewRequest(http.MethodGet, "/evaluation/wiki/runs/missing", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWikiEvaluationHandler_ListsAndDeletesRuns(t *testing.T) {
	svc := &fakeWikiEvaluationService{
		runs: &types.PageResult{Total: 1, Page: 2, PageSize: 10, Data: []*types.EvaluationRun{{ID: "run-1"}}},
	}
	router := newWikiEvaluationHandlerTestRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/evaluation/wiki/runs?page=2&page_size=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)
	assert.Contains(t, w.Body.String(), `"run-1"`)

	req = httptest.NewRequest(http.MethodDelete, "/evaluation/wiki/runs/run-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
