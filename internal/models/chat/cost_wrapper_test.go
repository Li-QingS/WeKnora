package chat

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/costledger"
	"github.com/Tencent/WeKnora/internal/types"
)

type fakeCostChat struct {
	resp *types.ChatResponse
	err  error
}

func (f *fakeCostChat) Chat(context.Context, []Message, *ChatOptions) (*types.ChatResponse, error) {
	return f.resp, f.err
}

func (f *fakeCostChat) ChatStream(context.Context, []Message, *ChatOptions) (<-chan types.StreamResponse, error) {
	return nil, f.err
}

func (f *fakeCostChat) GetModelName() string { return "chat-1" }
func (f *fakeCostChat) GetModelID() string   { return "chat-id-1" }

func TestCostChatRecordsUsage(t *testing.T) {
	rec := costledger.NewMemRecorder()
	costledger.SetRecorder(rec)
	defer costledger.SetRecorder(nil)

	inner := &fakeCostChat{resp: &types.ChatResponse{
		Usage: types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}}
	c := &costChat{inner: inner, tenantID: 7}
	if _, err := c.Chat(context.Background(), nil, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	info := rec.Last()
	if info == nil {
		t.Fatal("no record")
	}
	if info.TenantID != 7 || info.ModelID != "chat-id-1" {
		t.Errorf("info=%+v", info)
	}
	if info.PromptTokens != 10 || info.CompletionTokens != 5 || info.TotalTokens != 15 {
		t.Errorf("usage=%+v", info)
	}
	if info.Status != types.ModelCallStatusSuccess {
		t.Errorf("status=%s", info.Status)
	}
}
