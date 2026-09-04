package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeCostEmbedder struct{}

func (f *fakeCostEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{1, 2, 3}, nil
}

func (f *fakeCostEmbedder) BatchEmbed(context.Context, []string) ([][]float32, error) {
	return [][]float32{{1, 2, 3}}, nil
}

func (f *fakeCostEmbedder) BatchEmbedWithPool(context.Context, Embedder, []string) ([][]float32, error) {
	return nil, nil
}

func (f *fakeCostEmbedder) GetModelName() string { return "embed-1" }
func (f *fakeCostEmbedder) GetDimensions() int   { return 3 }
func (f *fakeCostEmbedder) GetModelID() string   { return "embed-id-1" }

func TestCostEmbedderRecordsApproxTokens(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	c := &costEmbedder{inner: &fakeCostEmbedder{}, tenantID: 7}
	if _, err := c.BatchEmbed(context.Background(), []string{"hello world"}); err != nil {
		t.Fatalf("BatchEmbed: %v", err)
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.TenantID != 7 || info.ModelID != "embed-id-1" {
		t.Errorf("info=%+v", info)
	}
	if info.PromptTokens <= 0 || info.TotalTokens != info.PromptTokens {
		t.Errorf("tokens=%+v", info)
	}
}

var errFakeProvider = errors.New("provider down")

type failingCostEmbedder struct{}

func (f *failingCostEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, errFakeProvider
}

func (f *failingCostEmbedder) BatchEmbed(context.Context, []string) ([][]float32, error) {
	return nil, errFakeProvider
}

func (f *failingCostEmbedder) BatchEmbedWithPool(context.Context, Embedder, []string) ([][]float32, error) {
	return nil, errFakeProvider
}

func (f *failingCostEmbedder) GetModelName() string { return "embed-1" }
func (f *failingCostEmbedder) GetDimensions() int   { return 3 }
func (f *failingCostEmbedder) GetModelID() string   { return "embed-id-1" }

func TestCostEmbedderDoesNotBillFailures(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	c := &costEmbedder{inner: &failingCostEmbedder{}, tenantID: 7}
	if _, err := c.Embed(context.Background(), "hello world"); err == nil {
		t.Fatal("expected Embed error")
	}
	if _, err := c.BatchEmbed(context.Background(), []string{"hello world"}); err == nil {
		t.Fatal("expected BatchEmbed error")
	}
	calls := rec.Calls()
	if len(calls) != 2 {
		t.Fatalf("calls=%d, want 2", len(calls))
	}
	for _, info := range calls {
		if info.Status != types.ModelCallStatusFailed {
			t.Errorf("status=%s", info.Status)
		}
		if info.PromptTokens != 0 || info.TotalTokens != 0 {
			t.Errorf("failed call billed tokens: %+v", info)
		}
	}
}
