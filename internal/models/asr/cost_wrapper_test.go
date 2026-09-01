package asr

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
)

type fakeCostASR struct{}

func (f *fakeCostASR) Transcribe(context.Context, []byte, string) (*TranscriptionResult, error) {
	return &TranscriptionResult{Text: "hello"}, nil
}

func (f *fakeCostASR) GetModelName() string { return "asr-1" }
func (f *fakeCostASR) GetModelID() string   { return "asr-id-1" }

func TestCostASRRecordsRequest(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	a := &costASR{inner: &fakeCostASR{}, tenantID: 7}
	if _, err := a.Transcribe(context.Background(), []byte{1}, "a.wav"); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.UnitType != "requests" || info.UnitCount != 1 {
		t.Errorf("unit=%s/%d", info.UnitType, info.UnitCount)
	}
	if info.TenantID != 7 || info.ModelID != "asr-id-1" {
		t.Errorf("info=%+v", info)
	}
}
