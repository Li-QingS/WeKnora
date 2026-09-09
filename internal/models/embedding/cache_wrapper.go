package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

// cachedEmbedder reuses embedding vectors for identical inputs.
type cachedEmbedder struct {
	inner     Embedder
	cache     EmbeddingCache
	tenantID  uint64
	pooler    EmbedderPooler
	modelID   string
	modelName string
	namespace string
}

func (c *cachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	lookup := c.lookupFor(ctx, text)
	flightKey := c.flightKey(lookup.key)
	flight, leader := embeddingCacheFlights.claim(flightKey)
	if !leader {
		vector, err := flight.wait(ctx)
		if err == nil {
			recordCoalescedRequest(c.modelID, c.modelName)
		}
		return vector, err
	}

	if vector, ok, primary := c.lookupCache(ctx, &lookup); ok {
		recordCacheHit(c.modelID, c.modelName)
		if primary && lookup.normalized {
			recordNormalizedCacheHit(c.modelID, c.modelName)
		}
		embeddingCacheFlights.complete(flightKey, flight, vector, nil)
		return append([]float32(nil), vector...), nil
	}

	recordCacheMiss(c.modelID, c.modelName)
	vector, err := c.inner.Embed(ctx, lookup.text)
	recordProviderCall(c.modelID, c.modelName)
	if err != nil {
		embeddingCacheFlights.complete(flightKey, flight, nil, err)
		return nil, err
	}
	if err := c.validateVector(vector); err != nil {
		embeddingCacheFlights.complete(flightKey, flight, nil, err)
		return nil, err
	}
	_ = c.cache.Set(ctx, &lookup.key, vector)
	embeddingCacheFlights.complete(flightKey, flight, vector, nil)
	return append([]float32(nil), vector...), nil
}

func (c *cachedEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	type batchGroup struct {
		lookup          embeddingCacheLookup
		indexes         []int
		normalizedCount int
		flightKey       string
		flight          *embeddingCacheFlight
		leader          bool
	}
	groups := make([]*batchGroup, 0, len(texts))
	byHash := make(map[string]*batchGroup, len(texts))
	for i, text := range texts {
		lookup := c.lookupFor(ctx, text)
		group, ok := byHash[lookup.key.TextHash]
		if !ok {
			group = &batchGroup{lookup: lookup}
			byHash[lookup.key.TextHash] = group
			groups = append(groups, group)
		} else {
			group.lookup.legacyKeys = appendUniqueCacheKeys(group.lookup.legacyKeys, lookup.legacyKeys...)
		}
		group.indexes = append(group.indexes, i)
		if lookup.normalized {
			group.normalizedCount++
		}
	}

	missingLeaders := make([]*batchGroup, 0, len(groups))
	missingTexts := make([]string, 0, len(groups))
	flightKeys := make([]string, len(groups))
	for i, group := range groups {
		group.flightKey = c.flightKey(group.lookup.key)
		flightKeys[i] = group.flightKey
	}
	flights, leaders := embeddingCacheFlights.claimMany(flightKeys)
	for i, group := range groups {
		group.flight, group.leader = flights[i], leaders[i]
		if !group.leader {
			continue
		}
		vector, ok, primary := c.lookupCache(ctx, &group.lookup)
		if ok {
			for _, index := range group.indexes {
				recordCacheHit(c.modelID, c.modelName)
				results[index] = append([]float32(nil), vector...)
			}
			if primary {
				for range group.normalizedCount {
					recordNormalizedCacheHit(c.modelID, c.modelName)
				}
			}
			embeddingCacheFlights.complete(group.flightKey, group.flight, vector, nil)
			continue
		}
		recordCacheMiss(c.modelID, c.modelName)
		for range len(group.indexes) - 1 {
			recordCoalescedRequest(c.modelID, c.modelName)
		}
		missingLeaders = append(missingLeaders, group)
		missingTexts = append(missingTexts, group.lookup.text)
	}
	if len(missingLeaders) > 0 {
		vectors, err := c.inner.BatchEmbed(ctx, missingTexts)
		recordProviderCall(c.modelID, c.modelName)
		if err == nil && len(vectors) != len(missingLeaders) {
			err = fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(vectors), len(missingLeaders))
		}
		if err == nil {
			for i := range missingLeaders {
				if validationErr := c.validateVector(vectors[i]); validationErr != nil {
					err = fmt.Errorf("embedding result %d: %w", i, validationErr)
					break
				}
			}
		}
		if err != nil {
			for _, group := range missingLeaders {
				embeddingCacheFlights.complete(group.flightKey, group.flight, nil, err)
			}
			return nil, err
		}
		for i, group := range missingLeaders {
			for _, index := range group.indexes {
				results[index] = append([]float32(nil), vectors[i]...)
			}
			_ = c.cache.Set(ctx, &group.lookup.key, vectors[i])
			embeddingCacheFlights.complete(group.flightKey, group.flight, vectors[i], nil)
		}
	}

	for _, group := range groups {
		if group.leader {
			continue
		}
		vector, err := group.flight.wait(ctx)
		if err != nil {
			return nil, err
		}
		for _, index := range group.indexes {
			recordCoalescedRequest(c.modelID, c.modelName)
			results[index] = append([]float32(nil), vector...)
		}
	}
	return results, nil
}

func (c *cachedEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	if c.pooler == nil {
		return c.inner.BatchEmbedWithPool(ctx, c, texts)
	}
	return c.pooler.BatchEmbedWithPool(ctx, c, texts)
}

func (c *cachedEmbedder) GetModelName() string { return c.inner.GetModelName() }
func (c *cachedEmbedder) GetDimensions() int   { return c.inner.GetDimensions() }
func (c *cachedEmbedder) GetModelID() string   { return c.inner.GetModelID() }

func (c *cachedEmbedder) keyFor(ctx context.Context, text string) types.EmbeddingCacheKey {
	return c.lookupFor(ctx, text).key
}

func (c *cachedEmbedder) legacyKeyFor(ctx context.Context, text string) types.EmbeddingCacheKey {
	return c.keyForText(ctx, text)
}

type embeddingCacheLookup struct {
	key        types.EmbeddingCacheKey
	legacyKeys []types.EmbeddingCacheKey
	text       string
	normalized bool
}

func (c *cachedEmbedder) lookupFor(ctx context.Context, text string) embeddingCacheLookup {
	canonical := CanonicalizeCacheText(text)
	// Keep the provider's legacy behavior for an all-whitespace non-empty
	// value instead of turning it into a new empty-input error path.
	if canonical == "" && text != "" {
		canonical = text
	}
	lookup := embeddingCacheLookup{
		key:        c.keyForText(ctx, canonical),
		text:       canonical,
		normalized: canonical != text,
	}
	if lookup.normalized {
		lookup.legacyKeys = []types.EmbeddingCacheKey{c.keyForText(ctx, text)}
	}
	return lookup
}

func (c *cachedEmbedder) keyForText(ctx context.Context, text string) types.EmbeddingCacheKey {
	tenantID := c.tenantID
	if t, ok := types.TenantIDFromContext(ctx); ok && t > 0 {
		tenantID = t
	}
	sum := sha256.Sum256([]byte(c.namespace + "\x00" + text))
	return types.EmbeddingCacheKey{
		TenantID:  tenantID,
		ModelID:   c.inner.GetModelID(),
		Dimension: c.inner.GetDimensions(),
		TextHash:  hex.EncodeToString(sum[:]),
	}
}

// lookupCache checks the canonical key first, then exact legacy keys for
// non-canonical inputs. A legacy hit is promoted lazily so later formatting
// variants need only one lookup. The third return value reports a primary-key
// hit, which lets metrics count true normalization wins without claiming a
// legacy exact hit as an optimization.
func (c *cachedEmbedder) lookupCache(
	ctx context.Context,
	lookup *embeddingCacheLookup,
) ([]float32, bool, bool) {
	if lookup == nil {
		return nil, false, false
	}
	if vector, ok, err := c.cache.Get(ctx, &lookup.key); err == nil && ok && c.validVector(vector) {
		_ = c.cache.IncrementHit(ctx, &lookup.key)
		return vector, true, true
	}
	for i := range lookup.legacyKeys {
		legacy := &lookup.legacyKeys[i]
		if vector, ok, err := c.cache.Get(ctx, legacy); err == nil && ok && c.validVector(vector) {
			_ = c.cache.IncrementHit(ctx, legacy)
			_ = c.cache.Set(ctx, &lookup.key, vector)
			return vector, true, false
		}
	}
	return nil, false, false
}

func (c *cachedEmbedder) flightKey(key types.EmbeddingCacheKey) string {
	return fmt.Sprintf("%d\x00%s\x00%d\x00%s", key.TenantID, key.ModelID, key.Dimension, key.TextHash)
}

func appendUniqueCacheKeys(
	destination []types.EmbeddingCacheKey,
	keys ...types.EmbeddingCacheKey,
) []types.EmbeddingCacheKey {
	for _, key := range keys {
		found := false
		for _, existing := range destination {
			if existing == key {
				found = true
				break
			}
		}
		if !found {
			destination = append(destination, key)
		}
	}
	return destination
}

func (c *cachedEmbedder) validVector(vector []float32) bool {
	return c.validateVector(vector) == nil
}

func (c *cachedEmbedder) validateVector(vector []float32) error {
	if len(vector) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	if dimension := c.inner.GetDimensions(); dimension > 0 && len(vector) != dimension {
		return fmt.Errorf("embedding vector dimension %d does not match configured dimension %d", len(vector), dimension)
	}
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding vector contains a non-finite value")
		}
	}
	return nil
}

type cacheNamespaceConfig struct {
	Source                    types.ModelSource `json:"source"`
	BaseURL                   string            `json:"base_url"`
	ModelName                 string            `json:"model_name"`
	TruncatePromptTokens      int               `json:"truncate_prompt_tokens"`
	Dimensions                int               `json:"dimensions"`
	SupportsDimensionOverride bool              `json:"supports_dimension_override"`
	Provider                  string            `json:"provider"`
	ExtraConfig               map[string]string `json:"extra_config,omitempty"`
	CustomHeaders             map[string]string `json:"custom_headers,omitempty"`
}

// embeddingCacheNamespace changes whenever vector-affecting model settings
// change. Credentials and concurrency settings are deliberately excluded.
func embeddingCacheNamespace(config Config) string {
	payload, _ := json.Marshal(cacheNamespaceConfig{
		Source:                    config.Source,
		BaseURL:                   config.BaseURL,
		ModelName:                 config.ModelName,
		TruncatePromptTokens:      config.TruncatePromptTokens,
		Dimensions:                config.Dimensions,
		SupportsDimensionOverride: config.SupportsDimensionOverride,
		Provider:                  config.Provider,
		ExtraConfig:               config.ExtraConfig,
		CustomHeaders:             effectiveEmbeddingHeaders(config.CustomHeaders),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// effectiveEmbeddingHeaders returns only headers that can reach the provider.
// Reserved authentication and protocol headers are ignored by ApplyCustomHeaders,
// so including them here would split identical vectors into separate cache keys.
func effectiveEmbeddingHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	filtered := make(map[string]string, len(headers))
	for key, value := range headers {
		name := strings.TrimSpace(key)
		if name == "" || utils.IsReservedHeader(name) {
			continue
		}
		filtered[name] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func wrapEmbeddingCache(e Embedder, tenantID uint64, pooler EmbedderPooler, namespaces ...string) Embedder {
	cache := GetEmbeddingCache()
	if cache == nil || e == nil {
		return e
	}
	// A persistent cache key must identify both the tenant and the exact vector
	// space. Temporary connection tests and incomplete model configurations do
	// not provide that identity, so they deliberately bypass persistence.
	if tenantID == 0 || strings.TrimSpace(e.GetModelID()) == "" || e.GetDimensions() <= 0 {
		return e
	}
	namespace := ""
	if len(namespaces) > 0 {
		namespace = namespaces[0]
	}
	return &cachedEmbedder{
		inner:     e,
		cache:     cache,
		tenantID:  tenantID,
		pooler:    pooler,
		modelID:   e.GetModelID(),
		modelName: e.GetModelName(),
		namespace: namespace,
	}
}
