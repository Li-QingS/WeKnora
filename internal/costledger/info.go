package costledger

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// NewCallInfo builds a ModelCallInfo with context-derived attribution and the
// model's tenant as fallback for background calls.
func NewCallInfo(
	ctx context.Context,
	fallbackTenant uint64,
	modelType string,
	modelID string,
	modelName string,
) *types.ModelCallInfo {
	info := &types.ModelCallInfo{
		TenantID:  fallbackTenant,
		ModelType: modelType,
		ModelID:   modelID,
		ModelName: modelName,
		StartedAt: time.Now(),
	}
	if tenantID, ok := types.TenantIDFromContext(ctx); ok && tenantID > 0 {
		info.TenantID = tenantID
	}
	if purpose, _ := types.LLMCallMetadataFromContext(ctx); purpose != "" {
		info.Purpose = purpose
	}
	if sessionID, ok := types.SessionIDFromContext(ctx); ok {
		info.SessionID = sessionID
	}
	info.RequestGroupID = types.RequestGroupIDFromContext(ctx)
	if userID, ok := types.UserIDFromContext(ctx); ok {
		info.UserID = userID
	}
	if principal, ok := types.PrincipalFromContext(ctx); ok {
		info.PrincipalType = principal.Type
		info.PrincipalID = principal.ID
	}
	return info
}

// Finish fills terminal status, duration and error details.
func Finish(info *types.ModelCallInfo, err error) {
	if info == nil {
		return
	}
	info.FinishedAt = time.Now()
	info.DurationMS = info.FinishedAt.Sub(info.StartedAt).Milliseconds()
	if err == nil {
		info.Status = types.ModelCallStatusSuccess
		return
	}
	info.Status = types.ModelCallStatusFailed
	info.ErrorType = errorTypeOf(err)
	info.ErrorMessage = truncate(err.Error(), 500)
}

// ApproxTokens estimates input tokens from rune count, matching the rule of
// thumb used by the Langfuse embedding wrapper.
func ApproxTokens(texts []string) int {
	total := 0
	for _, text := range texts {
		runes := len([]rune(text))
		if runes == 0 {
			continue
		}
		total += runes/4 + 1
	}
	return total
}

func errorTypeOf(err error) string {
	if err == nil {
		return ""
	}
	name := fmt.Sprintf("%T", err)
	if name == "*errors.errorString" || name == "*fmt.wrapError" {
		return "error"
	}
	return name
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
