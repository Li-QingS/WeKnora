package chat

import (
	"context"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// costChat records every chat call into the model cost ledger.
type costChat struct {
	inner    Chat
	tenantID uint64
}

func (c *costChat) GetModelName() string { return c.inner.GetModelName() }
func (c *costChat) GetModelID() string   { return c.inner.GetModelID() }

func (c *costChat) Chat(ctx context.Context, messages []Message, opts *ChatOptions) (*types.ChatResponse, error) {
	info := costledger.NewCallInfo(ctx, c.tenantID, string(types.ModelTypeKnowledgeQA), c.inner.GetModelID(), c.inner.GetModelName())
	resp, err := c.inner.Chat(ctx, messages, opts)
	if resp != nil {
		fillTokenUsage(info, resp.Usage)
	}
	costledger.Finish(info, err)
	if recordErr := costledger.Record(ctx, info); recordErr != nil {
		logger.Warnf(ctx, "[cost] failed to record chat call: %v", recordErr)
	}
	return resp, err
}

func (c *costChat) ChatStream(
	ctx context.Context,
	messages []Message,
	opts *ChatOptions,
) (<-chan types.StreamResponse, error) {
	info := costledger.NewCallInfo(ctx, c.tenantID, string(types.ModelTypeKnowledgeQA), c.inner.GetModelID(), c.inner.GetModelName())
	ch, err := c.inner.ChatStream(ctx, messages, opts)
	if err != nil {
		costledger.Finish(info, err)
		if recordErr := costledger.Record(ctx, info); recordErr != nil {
			logger.Warnf(ctx, "[cost] failed to record chat stream failure: %v", recordErr)
		}
		return ch, err
	}
	if ch == nil {
		costledger.Finish(info, nil)
		_ = costledger.Record(ctx, info)
		return nil, nil
	}

	wrapped := make(chan types.StreamResponse)
	go func() {
		defer close(wrapped)
		for resp := range ch {
			if resp.Usage != nil {
				fillTokenUsage(info, *resp.Usage)
			}
			wrapped <- resp
		}
		costledger.Finish(info, nil)
		if recordErr := costledger.Record(ctx, info); recordErr != nil {
			logger.Warnf(ctx, "[cost] failed to record chat stream: %v", recordErr)
		}
	}()
	return wrapped, nil
}

func fillTokenUsage(info *types.ModelCallInfo, usage types.TokenUsage) {
	info.PromptTokens = usage.PromptTokens
	info.CompletionTokens = usage.CompletionTokens
	info.TotalTokens = usage.TotalTokens
	info.CacheReadTokens = usage.CacheReadTokens
	info.CacheWriteTokens = usage.CacheWriteTokens
	info.CacheMissTokens = usage.CacheMissTokens
}

func wrapChatCost(c Chat, err error, tenantID uint64) (Chat, error) {
	if err != nil {
		return c, err
	}
	return &costChat{inner: c, tenantID: tenantID}, nil
}
