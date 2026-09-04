package asr

import (
	"context"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// costASR records transcription calls into the model cost ledger.
type costASR struct {
	inner    ASR
	tenantID uint64
}

func (a *costASR) Transcribe(ctx context.Context, audioBytes []byte, fileName string) (*TranscriptionResult, error) {
	info := costledger.NewCallInfo(ctx, a.tenantID, string(types.ModelTypeASR), a.inner.GetModelID(), a.inner.GetModelName())
	result, err := a.inner.Transcribe(ctx, audioBytes, fileName)
	if err == nil {
		// Failed transcription requests are not billed by providers.
		info.UnitType = "requests"
		info.UnitCount = 1
	}
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record asr call: %v", recordErr)
	}
	return result, err
}

func (a *costASR) GetModelName() string { return a.inner.GetModelName() }
func (a *costASR) GetModelID() string   { return a.inner.GetModelID() }

func wrapASRCost(a ASR, err error, tenantID uint64) (ASR, error) {
	if err != nil {
		return a, err
	}
	return &costASR{inner: a, tenantID: tenantID}, nil
}
