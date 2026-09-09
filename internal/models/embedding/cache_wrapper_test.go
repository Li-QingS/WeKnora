package embedding

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestCanonicalizeCacheTextIsConservative(t *testing.T) {
	input := "  \ufeffCafe\u0301  \r\n\tindented  \t\r\n\r\nend  \n"
	want := "Café\n\tindented\n\nend"
	if got := CanonicalizeCacheText(input); got != want {
		t.Fatalf("CanonicalizeCacheText()=%q, want %q", got, want)
	}

	preserved := "alpha  beta\n\n  code indentation"
	if got := CanonicalizeCacheText(preserved); got != preserved {
		t.Fatalf("canonicalization changed meaningful inner layout: %q", got)
	}
}

func TestCachedEmbedderNormalizesEquivalentFormatting(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7, modelID: inner.GetModelID(), modelName: inner.GetModelName()}
	first, err := c.Embed(context.Background(), "hello\nworld")
	if err != nil {
		t.Fatalf("canonical Embed: %v", err)
	}
	second, err := c.Embed(context.Background(), " \ufeffhello\r\nworld  \t\n")
	if err != nil {
		t.Fatalf("format variant Embed: %v", err)
	}
	if inner.embedCalls != 1 {
		t.Fatalf("provider calls=%d, want 1", inner.embedCalls)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("vectors differ: %v / %v", first, second)
	}
	stats := CacheStats()
	if stats.Hits != 1 || stats.NormalizedHits != 1 || stats.Misses != 1 {
		t.Fatalf("stats=%+v, want hit=1 normalized=1 miss=1", stats)
	}
}

func TestCachedEmbedderPromotesLegacyExactKey(t *testing.T) {
	cache := newFakeCache()
	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}
	raw := "hello\r\nworld  "
	legacy := c.legacyKeyFor(context.Background(), raw)
	if err := cache.Set(context.Background(), &legacy, []float32{42}); err != nil {
		t.Fatalf("seed legacy cache: %v", err)
	}

	vector, err := c.Embed(context.Background(), raw)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if inner.embedCalls != 0 || !reflect.DeepEqual(vector, []float32{42}) {
		t.Fatalf("provider calls=%d vector=%v", inner.embedCalls, vector)
	}
	primary := c.keyFor(context.Background(), raw)
	if _, ok, err := cache.Get(context.Background(), &primary); err != nil || !ok {
		t.Fatalf("legacy hit was not promoted: ok=%v err=%v", ok, err)
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

func TestCachedEmbedderBatchDeduplicatesFormattingVariants(t *testing.T) {
	cache := newFakeCache()
	ResetCacheStats()
	inner := &countingEmbedder{}
	c := &cachedEmbedder{inner: inner, cache: cache, tenantID: 7}

	results, err := c.BatchEmbed(context.Background(), []string{"hello\nworld", " \ufeffhello\r\nworld  \n"})
	if err != nil {
		t.Fatalf("BatchEmbed: %v", err)
	}
	if !reflect.DeepEqual(inner.lastBatch, []string{"hello\nworld"}) {
		t.Fatalf("provider inputs=%q, want one canonical input", inner.lastBatch)
	}
	if len(results) != 2 || !reflect.DeepEqual(results[0], results[1]) {
		t.Fatalf("results=%v", results)
	}
	results[0][0] = -1
	if results[1][0] == -1 {
		t.Fatal("batch results share a mutable vector slice")
	}
	stats := CacheStats()
	if stats.Misses != 1 || stats.CoalescedRequests != 1 || stats.ProviderCalls != 1 {
		t.Fatalf("stats=%+v, want one miss, one merged input and one provider call", stats)
	}
}

type blockingEmbedder struct {
	calls   atomic.Int64
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type blockingBatchEmbedder struct {
	calls   atomic.Int64
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (e *blockingBatchEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text))}, nil
}

func (e *blockingBatchEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	e.calls.Add(1)
	e.once.Do(func() { close(e.started) })
	<-e.release
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = []float32{float32(len(text))}
	}
	return result, nil
}

func (e *blockingBatchEmbedder) BatchEmbedWithPool(ctx context.Context, _ Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(ctx, texts)
}

func (e *blockingBatchEmbedder) GetModelName() string { return "blocking-batch" }
func (e *blockingBatchEmbedder) GetDimensions() int   { return 1 }
func (e *blockingBatchEmbedder) GetModelID() string   { return "blocking-batch-model" }

func (e *blockingEmbedder) Embed(context.Context, string) ([]float32, error) {
	e.calls.Add(1)
	e.once.Do(func() { close(e.started) })
	<-e.release
	return []float32{7}, nil
}

func (e *blockingEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{7}
	}
	return result, nil
}

func (e *blockingEmbedder) BatchEmbedWithPool(ctx context.Context, _ Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(ctx, texts)
}

func (e *blockingEmbedder) GetModelName() string { return "blocking" }
func (e *blockingEmbedder) GetDimensions() int   { return 1 }
func (e *blockingEmbedder) GetModelID() string   { return "blocking-model" }

func TestCachedEmbedderCoalescesConcurrentColdRequests(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &blockingEmbedder{started: make(chan struct{}), release: make(chan struct{})}
	c := &cachedEmbedder{
		inner: inner, cache: cache, tenantID: 7,
		modelID: inner.GetModelID(), modelName: inner.GetModelName(),
	}
	const callers = 32
	begin := make(chan struct{})
	ready := sync.WaitGroup{}
	ready.Add(callers)
	done := sync.WaitGroup{}
	done.Add(callers)
	results := make([][]float32, callers)
	errors := make([]error, callers)
	for i := range callers {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-begin
			results[index], errors[index] = c.Embed(context.Background(), "same query")
		}(i)
	}
	ready.Wait()
	close(begin)
	select {
	case <-inner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider call did not start")
	}
	// Keep the leader blocked long enough for the other ready goroutines to
	// join the flight; the test still has a strict overall timeout above.
	time.Sleep(25 * time.Millisecond)
	close(inner.release)
	done.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	if calls := inner.calls.Load(); calls != 1 {
		t.Fatalf("provider calls=%d, want 1", calls)
	}
	results[0][0] = -1
	for i := 1; i < callers; i++ {
		if results[i][0] == -1 {
			t.Fatalf("caller %d shares mutable result with caller 0", i)
		}
	}
	stats := CacheStats()
	if stats.Misses != 1 || stats.ProviderCalls != 1 || stats.CoalescedRequests != callers-1 {
		t.Fatalf("stats=%+v", stats)
	}
	t.Logf("callers=%d provider_calls=%d coalesced=%d reduction=%.3f%%",
		callers, inner.calls.Load(), stats.CoalescedRequests,
		100*(1-float64(inner.calls.Load())/callers))
}

func TestCachedEmbedderCoalescesConcurrentBatches(t *testing.T) {
	cache := newFakeCache()
	SetEmbeddingCache(cache)
	defer SetEmbeddingCache(nil)
	ResetCacheStats()

	inner := &blockingBatchEmbedder{started: make(chan struct{}), release: make(chan struct{})}
	c := &cachedEmbedder{
		inner: inner, cache: cache, tenantID: 7,
		modelID: inner.GetModelID(), modelName: inner.GetModelName(),
	}
	const callers = 16
	begin := make(chan struct{})
	var ready, done sync.WaitGroup
	ready.Add(callers)
	done.Add(callers)
	results := make([][][]float32, callers)
	errs := make([]error, callers)
	for i := range callers {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-begin
			results[index], errs[index] = c.BatchEmbed(context.Background(), []string{"alpha", "beta"})
		}(i)
	}
	ready.Wait()
	close(begin)
	select {
	case <-inner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider batch did not start")
	}
	time.Sleep(25 * time.Millisecond)
	close(inner.release)
	done.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
		if got := results[i]; len(got) != 2 || got[0][0] != 5 || got[1][0] != 4 {
			t.Fatalf("caller %d result=%v", i, got)
		}
	}
	if calls := inner.calls.Load(); calls != 1 {
		t.Fatalf("provider batch calls=%d, want 1", calls)
	}
	results[0][0][0] = -1
	for i := 1; i < callers; i++ {
		if results[i][0][0] == -1 {
			t.Fatalf("caller %d shares a mutable vector with caller 0", i)
		}
	}
	stats := CacheStats()
	wantCoalesced := int64((callers - 1) * 2)
	if stats.Misses != 2 || stats.ProviderCalls != 1 || stats.CoalescedRequests != wantCoalesced {
		t.Fatalf("stats=%+v, want misses=2 provider_calls=1 coalesced=%d", stats, wantCoalesced)
	}
	t.Logf("callers=%d inputs_per_batch=2 provider_calls=%d coalesced=%d call_reduction=%.3f%%",
		callers, inner.calls.Load(), stats.CoalescedRequests,
		100*(1-float64(inner.calls.Load())/callers))
}

type failingCache struct{}

func (failingCache) Get(context.Context, *types.EmbeddingCacheKey) ([]float32, bool, error) {
	return nil, false, errors.New("cache unavailable")
}

func (failingCache) Set(context.Context, *types.EmbeddingCacheKey, []float32) error {
	return errors.New("cache unavailable")
}

func (failingCache) IncrementHit(context.Context, *types.EmbeddingCacheKey) error {
	return errors.New("cache unavailable")
}

func TestCachedEmbedderCacheFailureFallsBackToProvider(t *testing.T) {
	ResetCacheStats()
	inner := &countingEmbedder{}
	c := &cachedEmbedder{
		inner: inner, cache: failingCache{}, tenantID: 7,
		modelID: inner.GetModelID(), modelName: inner.GetModelName(),
	}

	vector, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed returned a cache error: %v", err)
	}
	if inner.embedCalls != 1 || !reflect.DeepEqual(vector, []float32{5}) {
		t.Fatalf("provider calls=%d vector=%v", inner.embedCalls, vector)
	}
	stats := CacheStats()
	if stats.Misses != 1 || stats.ProviderCalls != 1 {
		t.Fatalf("stats=%+v", stats)
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
