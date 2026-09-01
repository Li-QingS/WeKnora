package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ModelCallHandler exposes model call ledger queries and price management.
type ModelCallHandler struct {
	service interfaces.ModelCallService
}

// NewModelCallHandler constructs the handler.
func NewModelCallHandler(service interfaces.ModelCallService) *ModelCallHandler {
	return &ModelCallHandler{service: service}
}

// List godoc
// @Summary      模型调用明细
// @Description  按租户分页查询模型调用记录
// @Tags         模型调用
// @Accept       json
// @Produce      json
// @Router       /model-calls [get]
func (h *ModelCallHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		c.Error(errors.NewBadRequestError("Invalid pagination parameters").WithDetails(err.Error()))
		return
	}
	filter, err := parseModelCallFilter(c)
	if err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	result, err := h.service.List(ctx, filter, &pagination)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result.Data,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// Summary godoc
// @Summary      模型调用汇总
// @Description  按模型聚合调用次数、Token 与估算费用
// @Tags         模型调用
// @Produce      json
// @Router       /model-calls/summary [get]
func (h *ModelCallHandler) Summary(c *gin.Context) {
	ctx := c.Request.Context()
	filter, err := parseModelCallFilter(c)
	if err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	items, err := h.service.Summary(ctx, filter)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if items == nil {
		items = []*types.ModelCallSummaryItem{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// ListPrices godoc
// @Summary      价格列表
// @Description  当前租户的模型价格列表
// @Tags         模型调用
// @Produce      json
// @Router       /model-prices [get]
func (h *ModelCallHandler) ListPrices(c *gin.Context) {
	ctx := c.Request.Context()
	prices, err := h.service.ListPrices(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if prices == nil {
		prices = []*types.ModelPrice{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": prices})
}

// GetPrice godoc
// @Summary      单模型价格
// @Description  获取指定模型的当前价格
// @Tags         模型调用
// @Produce      json
// @Router       /model-prices/{modelId} [get]
func (h *ModelCallHandler) GetPrice(c *gin.Context) {
	ctx := c.Request.Context()
	price, err := h.service.GetPrice(ctx, c.Param("modelId"))
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewNotFoundError("Model price not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": price})
}

// UpsertPrice godoc
// @Summary      写入/更新价格
// @Description  设置指定模型的估算价格
// @Tags         模型调用
// @Accept       json
// @Produce      json
// @Router       /model-prices/{modelId} [put]
func (h *ModelCallHandler) UpsertPrice(c *gin.Context) {
	ctx := c.Request.Context()
	var request modelPriceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(errors.NewBadRequestError("Invalid price parameters").WithDetails(err.Error()))
		return
	}
	price := &types.ModelPrice{
		ModelID:                   c.Param("modelId"),
		InputPricePerMillion:      request.InputPricePerMillion,
		OutputPricePerMillion:     request.OutputPricePerMillion,
		CacheReadPricePerMillion:  request.CacheReadPricePerMillion,
		CacheWritePricePerMillion: request.CacheWritePricePerMillion,
		UnitType:                  request.UnitType,
		UnitPrice:                 request.UnitPrice,
		Currency:                  request.Currency,
		UpdatedBy:                 userIDFromCtx(ctx),
	}
	if err := h.service.UpsertPrice(ctx, price); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": price})
}

type modelPriceRequest struct {
	InputPricePerMillion      *float64 `json:"input_price_per_million"`
	OutputPricePerMillion     *float64 `json:"output_price_per_million"`
	CacheReadPricePerMillion  *float64 `json:"cache_read_price_per_million"`
	CacheWritePricePerMillion *float64 `json:"cache_write_price_per_million"`
	UnitType                  string   `json:"unit_type"`
	UnitPrice                 *float64 `json:"unit_price"`
	Currency                  string   `json:"currency"`
}

func parseModelCallFilter(c *gin.Context) (*types.ModelCallFilter, error) {
	filter := &types.ModelCallFilter{
		ModelID:   c.Query("model_id"),
		ModelType: c.Query("model_type"),
		Status:    c.Query("status"),
	}
	if raw := c.Query("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("invalid from timestamp")
		}
		filter.From = &parsed
	}
	if raw := c.Query("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("invalid to timestamp")
		}
		filter.To = &parsed
	}
	return filter, nil
}

func userIDFromCtx(ctx context.Context) string {
	if userID, ok := types.UserIDFromContext(ctx); ok {
		return userID
	}
	return ""
}
