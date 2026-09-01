package embedding

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
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
