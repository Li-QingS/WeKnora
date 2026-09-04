package chat

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestApplyRawPromptCacheUsageDialects locks the provider-vocabulary mapping:
// every known way an OpenAI-compatible response can report prompt-cache usage
// must land in the same read/write/miss model, and the invariant
// read + write + miss == prompt_tokens must hold.
func TestApplyRawPromptCacheUsageDialects(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantRead   int
		wantWrite  int
		wantMiss   int
		wantReport bool
		wantStatus types.PromptCacheStatus
	}{
		{
			name:     "openai nested cached_tokens only",
			body:     `{"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":60}}}`,
			wantRead: 60, wantWrite: 0, wantMiss: 40,
			wantReport: true, wantStatus: types.PromptCacheStatusHit,
		},
		{
			name:     "deepseek top-level hit/miss",
			body:     `{"usage":{"prompt_tokens":100,"prompt_cache_hit_tokens":70,"prompt_cache_miss_tokens":30}}`,
			wantRead: 70, wantWrite: 0, wantMiss: 30,
			wantReport: true, wantStatus: types.PromptCacheStatusHit,
		},
		{
			name:     "anthropic top-level read/write",
			body:     `{"usage":{"prompt_tokens":100,"cache_read_input_tokens":50,"cache_creation_input_tokens":20}}`,
			wantRead: 50, wantWrite: 20, wantMiss: 30,
			wantReport: true, wantStatus: types.PromptCacheStatusHit,
		},
		{
			name:     "qwen nested cache_creation_input_tokens",
			body:     `{"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":40,"cache_creation_input_tokens":25}}}`,
			wantRead: 40, wantWrite: 25, wantMiss: 35,
			wantReport: true, wantStatus: types.PromptCacheStatusHit,
		},
		{
			name:     "nested cache_write_tokens alias",
			body:     `{"usage":{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":40,"cache_write_tokens":25}}}`,
			wantRead: 40, wantWrite: 25, wantMiss: 35,
			wantReport: true, wantStatus: types.PromptCacheStatusHit,
		},
		{
			name:     "no cache fields leaves usage untouched",
			body:     `{"usage":{"prompt_tokens":100}}`,
			wantRead: 0, wantWrite: 0, wantMiss: 0,
			wantReport: false, wantStatus: types.PromptCacheStatusUnreported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := &types.TokenUsage{PromptTokens: 100, CacheStatus: types.PromptCacheStatusUnreported}
			applyRawPromptCacheUsage([]byte(tt.body), usage)

			if usage.CacheReadTokens != tt.wantRead {
				t.Errorf("read = %d, want %d", usage.CacheReadTokens, tt.wantRead)
			}
			if usage.CacheWriteTokens != tt.wantWrite {
				t.Errorf("write = %d, want %d", usage.CacheWriteTokens, tt.wantWrite)
			}
			if usage.CacheMissTokens != tt.wantMiss {
				t.Errorf("miss = %d, want %d", usage.CacheMissTokens, tt.wantMiss)
			}
			if usage.CacheReported != tt.wantReport {
				t.Errorf("reported = %v, want %v", usage.CacheReported, tt.wantReport)
			}
			if usage.CacheStatus != tt.wantStatus {
				t.Errorf("status = %s, want %s", usage.CacheStatus, tt.wantStatus)
			}
			if got := usage.CacheReadTokens + usage.CacheWriteTokens + usage.CacheMissTokens; tt.wantReport && got != 100 {
				t.Errorf("invariant read+write+miss = %d, want 100", got)
			}
		})
	}
}
