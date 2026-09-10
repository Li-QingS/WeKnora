package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/provider"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClampPromptCacheKey(t *testing.T) {
	assert.Equal(t, "", clampPromptCacheKey(""))
	assert.Equal(t, "sess-1", clampPromptCacheKey("sess-1"))
	long := strings.Repeat("a", 80)
	got := clampPromptCacheKey(long)
	assert.Equal(t, 64, len([]rune(got)))
	assert.Equal(t, strings.Repeat("a", 64), got)
}

func TestApplyPromptCacheToJSONBody_OpenAIKey(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
		},
	}
	policy := promptCachePolicyFor(provider.ProviderOpenAI, "")
	body, forceRaw, err := applyPromptCacheToJSONBody(req, policy, "sess-abc", CacheRetentionShort)
	require.NoError(t, err)
	require.True(t, forceRaw)
	payload := body.(map[string]any)
	assert.Equal(t, "sess-abc", payload["prompt_cache_key"])
	assert.NotContains(t, payload, "prompt_cache_retention")
}

func TestApplyPromptCacheToJSONBody_OpenAILongRetention(t *testing.T) {
	req := openai.ChatCompletionRequest{Model: "gpt-4o"}
	policy := promptCachePolicyFor(provider.ProviderOpenAI, "")
	body, forceRaw, err := applyPromptCacheToJSONBody(req, policy, "sess-abc", CacheRetentionLong)
	require.NoError(t, err)
	require.True(t, forceRaw)
	payload := body.(map[string]any)
	assert.Equal(t, "24h", payload["prompt_cache_retention"])
}

func TestApplyPromptCacheToJSONBody_NoneLeavesBodyUntouched(t *testing.T) {
	req := &openai.ChatCompletionRequest{Model: "gpt-4o"}
	policy := promptCachePolicyFor(provider.ProviderOpenAI, "")
	body, forceRaw, err := applyPromptCacheToJSONBody(req, policy, "sess-abc", CacheRetentionNone)
	require.NoError(t, err)
	assert.False(t, forceRaw)
	assert.Equal(t, req, body)
}

func TestApplyPromptCacheToJSONBody_AliyunCacheControlBreakpoints(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "qwen-plus",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "stable system"},
			{Role: "user", Content: "turn question"},
		},
		Tools: []openai.Tool{{
			Type:     openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{Name: "search", Description: "d", Parameters: json.RawMessage(`{}`)},
		}},
	}
	policy := promptCachePolicyFor(provider.ProviderAliyun, "")
	body, forceRaw, err := applyPromptCacheToJSONBody(req, policy, "", CacheRetentionShort)
	require.NoError(t, err)
	require.True(t, forceRaw)
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(wire, &payload))

	messages := payload["messages"].([]any)
	sys := messages[0].(map[string]any)
	sysParts := sys["content"].([]any)
	sysCache := sysParts[0].(map[string]any)["cache_control"].(map[string]any)
	assert.Equal(t, "ephemeral", sysCache["type"])

	user := messages[1].(map[string]any)
	userParts := user["content"].([]any)
	userCache := userParts[0].(map[string]any)["cache_control"].(map[string]any)
	assert.Equal(t, "ephemeral", userCache["type"])

	tools := payload["tools"].([]any)
	lastTool := tools[len(tools)-1].(map[string]any)
	toolCache := lastTool["cache_control"].(map[string]any)
	assert.Equal(t, "ephemeral", toolCache["type"])
}

func TestPromptCachePolicyFor_GenericAliyunWorkspace(t *testing.T) {
	policy := promptCachePolicyFor(
		provider.ProviderGeneric,
		"https://llm-example.cn-beijing.maas.aliyuncs.com/compatible-mode/v1",
	)
	assert.True(t, policy.sendCacheControl)

	assert.False(t, promptCachePolicyFor(
		provider.ProviderGeneric,
		"https://maas.aliyuncs.com.example.test/compatible-mode/v1",
	).sendCacheControl)
}

func TestApplyPromptCacheToJSONBody_CustomTextBreakpointPreservesPrompt(t *testing.T) {
	prompt := "稳定前缀\n<chunks>\ndynamic"
	offset := strings.Index(prompt, "<chunks>")
	req := openai.ChatCompletionRequest{
		Model:    "provider-model-vnext",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
	}
	body, forceRaw, err := applyPromptCacheToJSONBody(
		req,
		promptCachePolicy{sendCacheControl: true},
		"",
		CacheRetentionShort,
		PromptCacheBreakpoint{MessageIndex: 0, ByteOffset: offset},
	)
	require.NoError(t, err)
	require.True(t, forceRaw)

	payload := body.(map[string]any)
	message := payload["messages"].([]any)[0].(map[string]any)
	parts := message["content"].([]any)
	require.Len(t, parts, 2)
	stable := parts[0].(map[string]any)
	dynamic := parts[1].(map[string]any)
	assert.Equal(t, prompt, stable["text"].(string)+dynamic["text"].(string))
	assert.Equal(t, prompt[:offset], stable["text"])
	assert.Contains(t, stable, "cache_control")
	assert.Equal(t, prompt[offset:], dynamic["text"])
	assert.NotContains(t, dynamic, "cache_control", "dynamic suffix must not create a unique full-request cache")
}

func TestApplyPromptCacheToJSONBody_LimitsCustomBreakpoints(t *testing.T) {
	messages := make([]openai.ChatCompletionMessage, 6)
	breakpoints := make([]PromptCacheBreakpoint, 6)
	for index := range messages {
		messages[index] = openai.ChatCompletionMessage{Role: "user", Content: "stable|dynamic"}
		breakpoints[index] = PromptCacheBreakpoint{MessageIndex: index, ByteOffset: len("stable|")}
	}
	body, _, err := applyPromptCacheToJSONBody(
		openai.ChatCompletionRequest{Model: "provider-model-vnext", Messages: messages},
		promptCachePolicy{sendCacheControl: true},
		"",
		CacheRetentionShort,
		breakpoints...,
	)
	require.NoError(t, err)
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	assert.Equal(t, maxCacheControlBreakpoints, strings.Count(string(wire), `"cache_control"`))
}

func TestBuildOutbound_OpenAIPromptCacheKeyFromSession(t *testing.T) {
	c := newOutboundChat(t, string(provider.ProviderOpenAI), "gpt-4o", nil)
	ctx := types.WithSessionID(context.Background(), "sess-live")
	body, _, useRaw, err := c.buildOutbound(ctx, []Message{{Role: "user", Content: "hi"}}, &ChatOptions{}, false)
	require.NoError(t, err)
	require.True(t, useRaw)
	payload := body.(map[string]any)
	assert.Equal(t, "sess-live", payload["prompt_cache_key"])
}

func TestBuildOutbound_GenericAliyunWorkspaceAppliesCustomBreakpoint(t *testing.T) {
	c := newOutboundChat(t, string(provider.ProviderGeneric), "provider-model-vnext", nil)
	c.baseURL = "https://llm-example.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
	prompt := "stable prefix<dynamic>"
	body, _, useRaw, err := c.buildOutbound(
		context.Background(),
		[]Message{{Role: "user", Content: prompt}},
		&ChatOptions{PromptCacheBreakpoints: []PromptCacheBreakpoint{{
			MessageIndex: 0,
			ByteOffset:   len("stable prefix"),
		}}},
		false,
	)
	require.NoError(t, err)
	require.True(t, useRaw)
	payload := body.(map[string]any)
	messages := payload["messages"].([]any)
	require.Len(t, messages, 2)
	stable := messages[0].(map[string]any)
	dynamic := messages[1].(map[string]any)
	assert.Equal(t, "user", stable["role"])
	assert.Equal(t, "user", dynamic["role"])
	stableParts := stable["content"].([]any)
	require.Len(t, stableParts, 1)
	assert.Contains(t, stableParts[0].(map[string]any), "cache_control")
	assert.Equal(t, prompt, stableParts[0].(map[string]any)["text"].(string)+dynamic["content"].(string))
}

func TestBuildOutbound_GenericAliyunWorkspaceWithoutBreakpointStaysOnSDKPath(t *testing.T) {
	c := newOutboundChat(t, string(provider.ProviderGeneric), "any-chat-model", nil)
	c.baseURL = "https://llm-example.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
	body, _, useRaw, err := c.buildOutbound(
		context.Background(),
		[]Message{{Role: "user", Content: "ordinary chat"}},
		&ChatOptions{},
		false,
	)
	require.NoError(t, err)
	assert.False(t, useRaw)
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(wire), "cache_control")
}

func TestBuildOutbound_GenericEndpointLeavesCustomBreakpointOffWire(t *testing.T) {
	c := newOutboundChat(t, string(provider.ProviderGeneric), "provider-model-vnext", nil)
	c.baseURL = "https://example.com/v1"
	prompt := "stable prefix<dynamic>"
	body, _, useRaw, err := c.buildOutbound(
		context.Background(),
		[]Message{{Role: "user", Content: prompt}},
		&ChatOptions{PromptCacheBreakpoints: []PromptCacheBreakpoint{{
			MessageIndex: 0,
			ByteOffset:   len("stable prefix"),
		}}},
		false,
	)
	require.NoError(t, err)
	assert.False(t, useRaw)
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(wire), "cache_control")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(wire, &payload))
	assert.Equal(t, prompt, payload["messages"].([]any)[0].(map[string]any)["content"])
}

func TestBuildOutbound_GenericAliyunWorkspaceRejectsInvalidBreakpoint(t *testing.T) {
	c := newOutboundChat(t, string(provider.ProviderGeneric), "provider-model-vnext", nil)
	c.baseURL = "https://llm-example.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
	body, _, useRaw, err := c.buildOutbound(
		context.Background(),
		[]Message{{Role: "user", Content: "short prompt"}},
		&ChatOptions{PromptCacheBreakpoints: []PromptCacheBreakpoint{{
			MessageIndex: 0,
			ByteOffset:   999,
		}}},
		false,
	)
	require.NoError(t, err)
	assert.False(t, useRaw)
	wire, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(wire), "cache_control")
}

func TestAttachPromptCacheHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", nil)
	require.NoError(t, err)
	attachPromptCacheHeaders(req, promptCachePolicy{sendAffinity: true}, "sess-1")
	assert.Equal(t, "sess-1", req.Header.Get("session_id"))
	assert.Equal(t, "sess-1", req.Header.Get("x-client-request-id"))
	assert.Equal(t, "sess-1", req.Header.Get("x-session-affinity"))
}
