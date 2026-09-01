package costledger

import (
	"context"
	"sync"

	"github.com/Tencent/WeKnora/internal/types"
)

// MemRecorder is an in-memory Recorder used by tests and lightweight tools.
type MemRecorder struct {
	mu    sync.Mutex
	calls []*types.ModelCallInfo
}

// NewMemRecorder creates an empty MemRecorder.
func NewMemRecorder() *MemRecorder {
	return &MemRecorder{}
}

// Record appends a copy of the call info.
func (m *MemRecorder) Record(_ context.Context, info *types.ModelCallInfo) error {
	if m == nil || info == nil {
		return nil
	}
	copied := *info
	m.mu.Lock()
	m.calls = append(m.calls, &copied)
	m.mu.Unlock()
	return nil
}

// Calls returns the recorded infos.
func (m *MemRecorder) Calls() []*types.ModelCallInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*types.ModelCallInfo, len(m.calls))
	copy(out, m.calls)
	return out
}

// Last returns the most recent call info, or nil.
func (m *MemRecorder) Last() *types.ModelCallInfo {
	calls := m.Calls()
	if len(calls) == 0 {
		return nil
	}
	return calls[len(calls)-1]
}
