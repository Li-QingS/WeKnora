package vlm

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
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
