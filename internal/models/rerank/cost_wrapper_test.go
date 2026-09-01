package rerank

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
)

type fakeCostReranker struct{}

func (f *fakeCostReranker) Rerank(context.Context, string, []string) ([]RankResult, error) {
	return nil, nil
}

func (f *fakeCostReranker) GetModelName() string { return "rerank-1" }
func (f *fakeCostReranker) GetModelID() string   { return "rerank-id-1" }

func TestCostRerankerRecordsUnits(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	r := &costReranker{inner: &fakeCostReranker{}, tenantID: 7}
	if _, err := r.Rerank(context.Background(), "q", []string{"a", "b", "c"}); err != nil {
		t.Fatalf("Rerank: %v", err)
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.UnitType != "documents" || info.UnitCount != 3 {
		t.Errorf("unit=%s/%d", info.UnitType, info.UnitCount)
	}
	if info.TenantID != 7 || info.ModelID != "rerank-id-1" {
		t.Errorf("info=%+v", info)
	}
}
