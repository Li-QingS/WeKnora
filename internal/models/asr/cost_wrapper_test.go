package asr

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
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

var errFakeProvider = errors.New("provider down")

type failingCostASR struct{}

func (f *failingCostASR) Transcribe(context.Context, []byte, string) (*TranscriptionResult, error) {
	return nil, errFakeProvider
}

func (f *failingCostASR) GetModelName() string { return "asr-1" }
func (f *failingCostASR) GetModelID() string   { return "asr-id-1" }

func TestCostASRDoesNotBillFailures(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	a := &costASR{inner: &failingCostASR{}, tenantID: 7}
	if _, err := a.Transcribe(context.Background(), []byte{1}, "a.wav"); err == nil {
		t.Fatal("expected Transcribe error")
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.Status != types.ModelCallStatusFailed {
		t.Errorf("status=%s", info.Status)
	}
	if info.UnitType != "" || info.UnitCount != 0 {
		t.Errorf("failed call billed units: %+v", info)
	}
}
