package embedding

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type fakeCache struct {
	mu     sync.Mutex
	values map[string][]float32
}

func newFakeCache() *fakeCache {
	return &fakeCache{values: map[string][]float32{}}
}

func (f *fakeCache) key(k *types.EmbeddingCacheKey) string {
	return k.TextHash
}

func (f *fakeCache) Get(_ context.Context, k *types.EmbeddingCacheKey) ([]float32, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.values[f.key(k)]
	return v, ok, nil
}

func (f *fakeCache) Set(_ context.Context, k *types.EmbeddingCacheKey, v []float32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[f.key(k)] = append([]float32(nil), v...)
	return nil
}

func (f *fakeCache) IncrementHit(context.Context, *types.EmbeddingCacheKey) error {
	return nil
}

type countingEmbedder struct {
	embedCalls     int
	batchCalls     int
	lastBatch      []string
	batchResult    [][]float32
	useBatchResult bool
}

func (e *countingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.embedCalls++
	return []float32{float32(len(text))}, nil
}

func (e *countingEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	e.batchCalls++
	e.lastBatch = append([]string(nil), texts...)
	if e.useBatchResult {
		return e.batchResult, nil
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = []float32{float32(len(text))}
	}
	return out, nil
}

func (e *countingEmbedder) BatchEmbedWithPool(_ context.Context, _ Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(context.Background(), texts)
}

func (e *countingEmbedder) GetModelName() string { return "embed-1" }
func (e *countingEmbedder) GetDimensions() int   { return 1 }
func (e *countingEmbedder) GetModelID() string   { return "embed-id-1" }

func TestCachedEmbedderSingleHit(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}

	first, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("first Embed: %v", err)
	}
	second, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("second Embed: %v", err)
	}
	if inner.embedCalls != 1 {
		t.Fatalf("embed calls=%d, want 1", inner.embedCalls)
	}
	if len(first) != 1 || first[0] != second[0] {
		t.Errorf("vectors=%v/%v", first, second)
	}
	stats := CacheStats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("stats=%+v, want 1/1", stats)
	}
}

func TestCacheStatsPerModel(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &countingEmbedder{}
	embedder := wrapEmbeddingCache(inner, 7, nil)
	if _, err := embedder.Embed(context.Background(), "hello"); err != nil {
		t.Fatalf("first Embed: %v", err)
	}
	if _, err := embedder.Embed(context.Background(), "hello"); err != nil {
		t.Fatalf("second Embed: %v", err)
	}

	stats := CacheStats()
	if !stats.Enabled {
		t.Fatal("cache should report enabled")
	}
	if len(stats.Models) != 1 {
		t.Fatalf("models=%+v, want one model", stats.Models)
	}
	model := stats.Models[0]
	if model.ModelID != "embed-id-1" || model.ModelName != "embed-1" {
		t.Fatalf("model stats=%+v", model)
	}
	if model.Hits != 1 || model.Misses != 1 || model.ProviderCalls != 1 {
		t.Fatalf("model stats=%+v", model)
	}
}

func TestCachedEmbedderBatchPartialHit(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}

	if _, err := c.BatchEmbed(context.Background(), []string{"a"}); err != nil {
		t.Fatalf("warm batch: %v", err)
	}
	results, err := c.BatchEmbed(context.Background(), []string{"a", "bb", "a"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if inner.batchCalls != 2 {
		t.Fatalf("batch calls=%d, want 2 (warm + one miss)", inner.batchCalls)
	}
	if len(results) != 3 {
		t.Fatalf("results len=%d", len(results))
	}
	if results[0][0] != 1 || results[1][0] != 2 || results[2][0] != 1 {
		t.Errorf("results=%v", results)
	}
}

func TestCachedEmbedderBatchDeduplicatesColdInputs(t *testing.T) {
	cache := newFakeCache()
	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}

	results, err := c.BatchEmbed(context.Background(), []string{"a", "a", "bb"})
	if err != nil {
		t.Fatalf("BatchEmbed: %v", err)
	}
	if !reflect.DeepEqual(inner.lastBatch, []string{"a", "bb"}) {
		t.Fatalf("provider inputs=%v, want unique inputs", inner.lastBatch)
	}
	if len(results) != 3 || results[0][0] != 1 || results[1][0] != 1 || results[2][0] != 2 {
		t.Fatalf("results=%v", results)
	}
}

func TestCachedEmbedderBatchRejectsWrongProviderCount(t *testing.T) {
	cache := newFakeCache()
	inner := &countingEmbedder{
		useBatchResult: true,
		batchResult:    [][]float32{{1}},
	}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}

	_, err := c.BatchEmbed(context.Background(), []string{"a", "bb"})
	if err == nil || !strings.Contains(err.Error(), "returned 1 vectors for 2 inputs") {
		t.Fatalf("error=%v, want provider count mismatch", err)
	}
}

func TestCachedEmbedderTreatsInvalidCachedVectorAsMiss(t *testing.T) {
	cache := newFakeCache()
	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}
	key := c.keyFor(context.Background(), "hello")
	if err := cache.Set(context.Background(), &key, []float32{1, 2}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	vector, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if inner.embedCalls != 1 || !reflect.DeepEqual(vector, []float32{5}) {
		t.Fatalf("embed calls=%d vector=%v, want provider refresh", inner.embedCalls, vector)
	}
}

func TestEmbeddingCacheNamespaceTracksVectorSettings(t *testing.T) {
	base := Config{BaseURL: "https://one.example/v1", ModelName: "embed-1", APIKey: "secret-one", Dimensions: 1}
	changedEndpoint := base
	changedEndpoint.BaseURL = "https://two.example/v1"
	changedCredential := base
	changedCredential.APIKey = "secret-two"

	if embeddingCacheNamespace(base) == embeddingCacheNamespace(changedEndpoint) {
		t.Fatal("cache namespace must change with the model endpoint")
	}
	if embeddingCacheNamespace(base) != embeddingCacheNamespace(changedCredential) {
		t.Fatal("cache namespace must not depend on credentials")
	}
}

func TestEmbeddingCacheNamespaceIgnoresReservedHeaders(t *testing.T) {
	base := Config{
		BaseURL:       "https://one.example/v1",
		ModelName:     "embed-1",
		Dimensions:    1,
		CustomHeaders: map[string]string{"X-Route": "blue", "Authorization": "secret-one"},
	}
	changedCredential := base
	changedCredential.CustomHeaders = map[string]string{"X-Route": "blue", "Authorization": "secret-two"}
	changedRoute := base
	changedRoute.CustomHeaders = map[string]string{"X-Route": "green", "Authorization": "secret-one"}

	if embeddingCacheNamespace(base) != embeddingCacheNamespace(changedCredential) {
		t.Fatal("cache namespace must ignore reserved headers that never reach the provider")
	}
	if embeddingCacheNamespace(base) == embeddingCacheNamespace(changedRoute) {
		t.Fatal("cache namespace must change with effective provider headers")
	}
}

type identityEmbedder struct {
	*countingEmbedder
	modelID   string
	dimension int
}

func (e *identityEmbedder) GetModelID() string { return e.modelID }
func (e *identityEmbedder) GetDimensions() int { return e.dimension }

func TestWrapEmbeddingCacheRequiresStableIdentity(t *testing.T) {
	SetEmbeddingCache(newFakeCache())
	defer SetEmbeddingCache(nil)

	tests := []struct {
		name      string
		tenantID  uint64
		modelID   string
		dimension int
	}{
		{name: "missing tenant", modelID: "model-1", dimension: 1},
		{name: "missing model id", tenantID: 7, dimension: 1},
		{name: "missing dimension", tenantID: 7, modelID: "model-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &identityEmbedder{
				countingEmbedder: &countingEmbedder{},
				modelID:          tt.modelID,
				dimension:        tt.dimension,
			}
			if got := wrapEmbeddingCache(inner, tt.tenantID, nil); got != inner {
				t.Fatal("expected passthrough without a stable cache identity")
			}
		})
	}
}

func TestWrapEmbeddingCacheNoCachePassthrough(t *testing.T) {
	SetEmbeddingCache(nil)
	inner := &countingEmbedder{}
	if got := wrapEmbeddingCache(inner, 7, nil); got != inner {
		t.Fatal("expected passthrough when cache is nil")
	}
}
