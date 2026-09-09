package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeModelCallService struct {
	listResult *types.PageResult
	summary    []*types.ModelCallSummaryItem
	prices     []*types.ModelPrice
	upserted   *types.ModelPrice
	listFilter *types.ModelCallFilter
	err        error
}

func (f *fakeModelCallService) List(
	_ context.Context,
	filter *types.ModelCallFilter,
	_ *types.Pagination,
) (*types.PageResult, error) {
	f.listFilter = filter
	return f.listResult, f.err
}

func (f *fakeModelCallService) Summary(
	context.Context,
	*types.ModelCallFilter,
) ([]*types.ModelCallSummaryItem, error) {
	return f.summary, f.err
}

func (f *fakeModelCallService) UpsertPrice(context.Context, *types.ModelPrice) error {
	return f.err
}

func (f *fakeModelCallService) GetPrice(context.Context, string) (*types.ModelPrice, error) {
	return &types.ModelPrice{ModelID: "m1"}, f.err
}

func (f *fakeModelCallService) ListPrices(context.Context) ([]*types.ModelPrice, error) {
	return f.prices, f.err
}

func newModelCallTestRouter(svc *fakeModelCallService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "user-1")
		c.Next()
	})
	h := NewModelCallHandler(svc)
	r.GET("/model-calls", h.List)
	r.GET("/model-calls/summary", h.Summary)
	r.GET("/model-prices", h.ListPrices)
	r.GET("/model-prices/:modelId", h.GetPrice)
	r.PUT("/model-prices/:modelId", h.UpsertPrice)
	return r
}

func TestModelCallList(t *testing.T) {
	svc := &fakeModelCallService{
		listResult: types.NewPageResult(1, &types.Pagination{Page: 1, PageSize: 10}, []*types.ModelCallRecord{
			{ID: "call-1", ModelID: "m1"},
		}),
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-calls?model_id=m1", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"call-1"`)
}

func TestModelCallListExpandsSameDayRange(t *testing.T) {
	svc := &fakeModelCallService{listResult: types.NewPageResult(
		0,
		&types.Pagination{Page: 1, PageSize: 10},
		[]*types.ModelCallRecord{},
	)}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-calls?from=2026-09-08&to=2026-09-08", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.listFilter)
	require.NotNil(t, svc.listFilter.From)
	require.NotNil(t, svc.listFilter.To)
	assert.Equal(t, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), *svc.listFilter.From)
	assert.Equal(t, time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), *svc.listFilter.To)
	assert.True(t, svc.listFilter.ToExclusive)
}

func TestParseModelCallFilterKeepsRFC3339UpperBoundInclusive(t *testing.T) {
	svc := &fakeModelCallService{listResult: types.NewPageResult(
		0,
		&types.Pagination{Page: 1, PageSize: 10},
		[]*types.ModelCallRecord{},
	)}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-calls?to=2026-09-08T23%3A59%3A59%2B08%3A00", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.listFilter)
	require.NotNil(t, svc.listFilter.To)
	assert.False(t, svc.listFilter.ToExclusive)
}

func TestModelCallSummary(t *testing.T) {
	svc := &fakeModelCallService{summary: []*types.ModelCallSummaryItem{
		{ModelID: "m1", Calls: 3},
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-calls/summary", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"calls":3`)
}

func TestModelCallUpsertPrice(t *testing.T) {
	svc := &fakeModelCallService{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/model-prices/m1",
		bytes.NewReader([]byte(`{"input_price_per_million":1.5,"currency":"USD"}`)))
	req.Header.Set("Content-Type", "application/json")
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"model_id":"m1"`)
}

func TestModelPriceErrorMapping(t *testing.T) {
	svc := &fakeModelCallService{err: service.ErrInvalidModelPrice}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/model-prices/m1",
		bytes.NewReader([]byte(`{"input_price_per_million":-1,"currency":"USD"}`)))
	req.Header.Set("Content-Type", "application/json")
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	svc.err = service.ErrModelPriceNotFound
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/model-prices/missing", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	svc.err = context.DeadlineExceeded
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/model-prices/m1", nil)
	newModelCallTestRouter(svc).ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
