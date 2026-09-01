package embedding

import (
	"context"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// costEmbedder records embedding calls into the model cost ledger.
type costEmbedder struct {
	inner    Embedder
	tenantID uint64
}

func (c *costEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	info := costledger.NewCallInfo(ctx, c.tenantID, string(types.ModelTypeEmbedding), c.inner.GetModelID(), c.inner.GetModelName())
	info.PromptTokens = costledger.ApproxTokens([]string{text})
	info.TotalTokens = info.PromptTokens
	result, err := c.inner.Embed(ctx, text)
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record embedding call: %v", recordErr)
	}
	return result, err
}

func (c *costEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	info := costledger.NewCallInfo(ctx, c.tenantID, string(types.ModelTypeEmbedding), c.inner.GetModelID(), c.inner.GetModelName())
	info.PromptTokens = costledger.ApproxTokens(texts)
	info.TotalTokens = info.PromptTokens
	result, err := c.inner.BatchEmbed(ctx, texts)
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record batch embedding call: %v", recordErr)
	}
	return result, err
}

func (c *costEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	return c.inner.BatchEmbedWithPool(ctx, c, texts)
}

func (c *costEmbedder) GetModelName() string { return c.inner.GetModelName() }
func (c *costEmbedder) GetDimensions() int   { return c.inner.GetDimensions() }
func (c *costEmbedder) GetModelID() string   { return c.inner.GetModelID() }

func wrapEmbeddingCost(e Embedder, tenantID uint64) Embedder {
	return &costEmbedder{inner: e, tenantID: tenantID}
}
