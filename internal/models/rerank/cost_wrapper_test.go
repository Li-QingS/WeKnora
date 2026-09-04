package rerank

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
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

var errFakeProvider = errors.New("provider down")

type failingCostReranker struct{}

func (f *failingCostReranker) Rerank(context.Context, string, []string) ([]RankResult, error) {
	return nil, errFakeProvider
}

func (f *failingCostReranker) GetModelName() string { return "rerank-1" }
func (f *failingCostReranker) GetModelID() string   { return "rerank-id-1" }

func TestCostRerankerDoesNotBillFailures(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	r := &costReranker{inner: &failingCostReranker{}, tenantID: 7}
	if _, err := r.Rerank(context.Background(), "q", []string{"a", "b", "c"}); err == nil {
		t.Fatal("expected Rerank error")
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
