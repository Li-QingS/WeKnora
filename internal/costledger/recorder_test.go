package costledger

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type fakeRecorder struct {
	calls int
	err   error
}

func (f *fakeRecorder) Record(context.Context, *types.ModelCallInfo) error {
	f.calls++
	return f.err
}

func TestRecordNoopWithoutRecorder(t *testing.T) {
	SetRecorder(nil)
	if err := Record(context.Background(), &types.ModelCallInfo{}); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestRecordCallsInstalledRecorder(t *testing.T) {
	f := &fakeRecorder{}
	SetRecorder(f)
	defer SetRecorder(nil)

	if err := Record(context.Background(), &types.ModelCallInfo{ModelID: "m1"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("calls=%d, want 1", f.calls)
	}
}

func TestRecordPropagatesRecorderError(t *testing.T) {
	want := errors.New("db down")
	f := &fakeRecorder{err: want}
	SetRecorder(f)
	defer SetRecorder(nil)

	err := Record(context.Background(), &types.ModelCallInfo{})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v, want %v", err, want)
	}
}

func TestNewCallInfoCarriesRequestGroup(t *testing.T) {
	ctx := types.WithRequestGroupID(context.Background(), "evaluation_run_1")
	info := NewCallInfo(ctx, 1, string(types.ModelTypeEmbedding), "embed-1", "Embed")
	if info.RequestGroupID != "evaluation_run_1" {
		t.Fatalf("request group = %q, want evaluation_run_1", info.RequestGroupID)
	}
}
