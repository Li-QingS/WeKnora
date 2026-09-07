package handler

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

type WikiEvaluationHandler struct {
	service interfaces.WikiEvaluationService
}

func NewWikiEvaluationHandler(service interfaces.WikiEvaluationService) *WikiEvaluationHandler {
	return &WikiEvaluationHandler{service: service}
}

func (h *WikiEvaluationHandler) Start(c *gin.Context) {
	var request types.WikiEvaluationOptions
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(apperrors.NewBadRequestError("Invalid Wiki evaluation parameters").WithDetails(err.Error()))
		return
	}
	detail, err := h.service.Start(c.Request.Context(), &request)
	if err != nil {
		if stderrors.Is(err, service.ErrDatasetNotFound) || stderrors.Is(err, service.ErrInvalidDataset) ||
			stderrors.Is(err, service.ErrWikiGoldNotFound) || stderrors.Is(err, service.ErrInvalidWikiGold) {
			c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": detail})
}

func (h *WikiEvaluationHandler) Get(c *gin.Context) {
	detail, err := h.service.Get(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": detail})
}

func (h *WikiEvaluationHandler) List(c *gin.Context) {
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		c.Error(apperrors.NewBadRequestError("Invalid pagination parameters").WithDetails(err.Error()))
		return
	}
	var status *types.EvaluationStatue
	if raw := c.Query("status"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < int(types.EvaluationStatuePending) || value > int(types.EvaluationStatueInterrupted) {
			c.Error(apperrors.NewBadRequestError("Invalid status filter"))
			return
		}
		parsed := types.EvaluationStatue(value)
		status = &parsed
	}
	result, err := h.service.ListRuns(c.Request.Context(), status, &pagination)
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true, "data": result.Data, "total": result.Total,
		"page": result.Page, "page_size": result.PageSize,
	})
}

func (h *WikiEvaluationHandler) ListDatasets(c *gin.Context) {
	datasets, err := h.service.ListDatasets(c.Request.Context())
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": datasets})
}

func (h *WikiEvaluationHandler) Report(c *gin.Context) {
	format := types.EvaluationReportFormat(c.DefaultQuery("format", string(types.EvaluationReportJSON)))
	data, contentType, err := h.service.RenderReport(
		c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), format,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}
	extension := string(format)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="wiki-evaluation-%s.%s"`,
		secutils.SanitizeForLog(c.Param("id")), extension))
	c.Data(http.StatusOK, contentType, data)
}

func (h *WikiEvaluationHandler) Delete(c *gin.Context) {
	err := h.service.DeleteRun(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WikiEvaluationHandler) handleError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrEvaluationTaskNotFound):
		c.Error(apperrors.NewNotFoundError("Wiki evaluation run not found"))
	case stderrors.Is(err, service.ErrEvaluationRunActive):
		c.Error(apperrors.NewBadRequestError("Running evaluation cannot be deleted"))
	case !types.EvaluationReportFormat(c.Query("format")).IsValid() && c.Query("format") != "":
		c.Error(apperrors.NewBadRequestError(err.Error()))
	default:
		c.Error(apperrors.NewInternalServerError(err.Error()))
	}
}
