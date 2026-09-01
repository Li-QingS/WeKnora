package vlm

import (
	"context"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// costVLM records vision-language calls into the model cost ledger.
type costVLM struct {
	inner    VLM
	tenantID uint64
}

func (v *costVLM) Predict(ctx context.Context, imgBytes [][]byte, prompt string) (string, error) {
	info := costledger.NewCallInfo(ctx, v.tenantID, string(types.ModelTypeVLLM), v.inner.GetModelID(), v.inner.GetModelName())
	info.UnitType = "requests"
	info.UnitCount = 1
	info.PromptTokens = costledger.ApproxTokens([]string{prompt})
	info.TotalTokens = info.PromptTokens

	result, err := v.inner.Predict(ctx, imgBytes, prompt)
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record vlm call: %v", recordErr)
	}
	return result, err
}

func (v *costVLM) GetModelName() string { return v.inner.GetModelName() }
func (v *costVLM) GetModelID() string   { return v.inner.GetModelID() }

func wrapVLMCost(v VLM, err error, tenantID uint64) (VLM, error) {
	if err != nil {
		return v, err
	}
	return &costVLM{inner: v, tenantID: tenantID}, nil
}
