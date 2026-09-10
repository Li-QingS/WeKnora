package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type wikiEmbeddingProvider struct {
	models interfaces.ModelService
}

func NewWikiEmbeddingProvider(models interfaces.ModelService) *wikiEmbeddingProvider {
	return &wikiEmbeddingProvider{models: models}
}

func (p *wikiEmbeddingProvider) Embed(
	ctx context.Context, modelID string, texts []string,
) ([][]float32, error) {
	if p == nil || p.models == nil {
		return nil, fmt.Errorf("wiki embedding provider is not configured")
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, fmt.Errorf("embedding model id is required")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	model, err := p.models.GetEmbeddingModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("load wiki evaluation embedding model %q: %w", modelID, err)
	}
	// Use the model's standard pool-aware path so provider-specific request
	// limits are respected through BATCH_EMBED_SIZE. Calling BatchEmbed
	// directly sends every unmatched label in one request, which providers
	// with small caps (for example, 20 inputs) reject.
	vectors, err := model.BatchEmbedWithPool(ctx, model, texts)
	if err != nil {
		return nil, fmt.Errorf("embed wiki evaluation labels: %w", err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding model returned %d vectors for %d labels", len(vectors), len(texts))
	}
	return vectors, nil
}
