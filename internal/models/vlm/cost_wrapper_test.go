package vlm

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeCostVLM struct{}

func (f *fakeCostVLM) Predict(context.Context, [][]byte, string) (string, error) {
	return "answer", nil
}

func (f *fakeCostVLM) GetModelName() string { return "vlm-1" }
func (f *fakeCostVLM) GetModelID() string   { return "vlm-id-1" }

func TestCostVLMRecordsRequest(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	v := &costVLM{inner: &fakeCostVLM{}, tenantID: 7}
	if _, err := v.Predict(context.Background(), [][]byte{{1}}, "prompt"); err != nil {
		t.Fatalf("Predict: %v", err)
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.UnitType != "requests" || info.UnitCount != 1 {
		t.Errorf("unit=%s/%d", info.UnitType, info.UnitCount)
	}
	if info.TenantID != 7 || info.ModelID != "vlm-id-1" {
		t.Errorf("info=%+v", info)
	}
}

var errFakeProvider = errors.New("provider down")

type failingCostVLM struct{}

func (f *failingCostVLM) Predict(context.Context, [][]byte, string) (string, error) {
	return "", errFakeProvider
}

func (f *failingCostVLM) GetModelName() string { return "vlm-1" }
func (f *failingCostVLM) GetModelID() string   { return "vlm-id-1" }

func TestCostVLMDoesNotBillFailures(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	v := &costVLM{inner: &failingCostVLM{}, tenantID: 7}
	if _, err := v.Predict(context.Background(), [][]byte{{1}}, "prompt"); err == nil {
		t.Fatal("expected Predict error")
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
	if info.PromptTokens != 0 || info.TotalTokens != 0 {
		t.Errorf("failed call billed tokens: %+v", info)
	}
}
