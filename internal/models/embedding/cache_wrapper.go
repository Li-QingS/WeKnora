package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"github.com/Tencent/WeKnora/internal/types"
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
	key := c.keyFor(ctx, text)
	if vector, ok, err := c.cache.Get(ctx, &key); err == nil && ok && c.validVector(vector) {
		recordCacheHit(c.modelID, c.modelName)
		_ = c.cache.IncrementHit(ctx, &key)
		return vector, nil
	}
	recordCacheMiss(c.modelID, c.modelName)
	vector, err := c.inner.Embed(ctx, text)
	recordProviderCall(c.modelID, c.modelName)
	if err != nil {
		return nil, err
	}
	if err := c.validateVector(vector); err != nil {
		return nil, err
	}
	_ = c.cache.Set(ctx, &key, vector)
	return vector, nil
}

func (c *cachedEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	type batchGroup struct {
		key     types.EmbeddingCacheKey
		text    string
		indexes []int
	}
	groups := make([]*batchGroup, 0, len(texts))
	byHash := make(map[string]*batchGroup, len(texts))
	for i, text := range texts {
		key := c.keyFor(ctx, text)
		group, ok := byHash[key.TextHash]
		if !ok {
			group = &batchGroup{key: key, text: text}
			byHash[key.TextHash] = group
			groups = append(groups, group)
		}
		group.indexes = append(group.indexes, i)
	}

	missing := make([]*batchGroup, 0, len(groups))
	missingTexts := make([]string, 0, len(groups))
	for _, group := range groups {
		vector, ok, err := c.cache.Get(ctx, &group.key)
		if err == nil && ok && c.validVector(vector) {
			for _, index := range group.indexes {
				recordCacheHit(c.modelID, c.modelName)
				results[index] = vector
			}
			_ = c.cache.IncrementHit(ctx, &group.key)
			continue
		}
		for range group.indexes {
			recordCacheMiss(c.modelID, c.modelName)
		}
		missing = append(missing, group)
		missingTexts = append(missingTexts, group.text)
	}
	if len(missing) == 0 {
		return results, nil
	}

	vectors, err := c.inner.BatchEmbed(ctx, missingTexts)
	recordProviderCall(c.modelID, c.modelName)
	if err != nil {
		return nil, err
	}
	if len(vectors) != len(missing) {
		return nil, fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(vectors), len(missing))
	}
	for i, group := range missing {
		if err := c.validateVector(vectors[i]); err != nil {
			return nil, fmt.Errorf("embedding result %d: %w", i, err)
		}
		for _, index := range group.indexes {
			results[index] = vectors[i]
		}
		_ = c.cache.Set(ctx, &group.key, vectors[i])
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
		CustomHeaders:             config.CustomHeaders,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func wrapEmbeddingCache(e Embedder, tenantID uint64, pooler EmbedderPooler, namespaces ...string) Embedder {
	cache := GetEmbeddingCache()
	if cache == nil || e == nil {
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
