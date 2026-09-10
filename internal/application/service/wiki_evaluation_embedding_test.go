package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/require"
)

type wikiEvaluationEmbeddingModelService struct {
	interfaces.ModelService
	model embedding.Embedder
}

func (s wikiEvaluationEmbeddingModelService) GetEmbeddingModel(
	context.Context, string,
) (embedding.Embedder, error) {
	return s.model, nil
}

type providerCappedWikiEmbedder struct {
	pooler    embedding.EmbedderPooler
	mu        sync.Mutex
	sizes     []int
	workloads []embedding.CacheWorkload
}

func (e *providerCappedWikiEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	value, err := strconv.Atoi(strings.TrimPrefix(text, "label-"))
	return []float32{float32(value)}, err
}

func (e *providerCappedWikiEmbedder) BatchEmbed(
	ctx context.Context, texts []string,
) ([][]float32, error) {
	if len(texts) > 20 {
		return nil, fmt.Errorf("provider batch limit exceeded: %d", len(texts))
	}
	e.mu.Lock()
	e.sizes = append(e.sizes, len(texts))
	e.workloads = append(e.workloads, embedding.CacheWorkloadFromContext(ctx))
	e.mu.Unlock()
	vectors := make([][]float32, len(texts))
	for index, text := range texts {
		vector, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[index] = vector
	}
	return vectors, nil
}

func (e *providerCappedWikiEmbedder) BatchEmbedWithPool(
	ctx context.Context, model embedding.Embedder, texts []string,
) ([][]float32, error) {
	return e.pooler.BatchEmbedWithPool(ctx, model, texts)
}

func (e *providerCappedWikiEmbedder) GetModelName() string { return "provider-capped" }
func (e *providerCappedWikiEmbedder) GetDimensions() int   { return 1 }
func (e *providerCappedWikiEmbedder) GetModelID() string   { return "provider-capped" }

func TestWikiEmbeddingProviderUsesPoolAwareBatching(t *testing.T) {
	t.Setenv("BATCH_EMBED_SIZE", "20")
	pool, err := ants.NewPool(3)
	require.NoError(t, err)
	t.Cleanup(pool.Release)

	model := &providerCappedWikiEmbedder{pooler: embedding.NewBatchEmbedder(pool)}
	provider := NewWikiEmbeddingProvider(wikiEvaluationEmbeddingModelService{model: model})
	texts := make([]string, 45)
	for index := range texts {
		texts[index] = fmt.Sprintf("label-%d", index)
	}

	vectors, err := provider.Embed(context.Background(), "embedding-model", texts)
	require.NoError(t, err)
	require.Len(t, vectors, len(texts))
	for index, vector := range vectors {
		require.Equal(t, []float32{float32(index)}, vector, "vector order changed at index %d", index)
	}

	model.mu.Lock()
	sizes := append([]int(nil), model.sizes...)
	workloads := append([]embedding.CacheWorkload(nil), model.workloads...)
	model.mu.Unlock()
	sort.Ints(sizes)
	require.Equal(t, []int{5, 20, 20}, sizes)
	for _, workload := range workloads {
		require.Equal(t, embedding.CacheWorkloadWikiEvaluation, workload)
	}
}
