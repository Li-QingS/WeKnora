// Package costledger installs a global model-call recorder hook. Model
// wrappers report calls here; the application wires the real implementation
// at startup, mirroring the langfuse manager pattern.
package costledger

import (
	"context"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

const recordTimeout = 5 * time.Second

// Recorder persists one model call into the ledger.
type Recorder interface {
	Record(ctx context.Context, info *types.ModelCallInfo) error
}

var (
	mu       sync.RWMutex
	recorder Recorder
)

// SetRecorder installs the process-wide recorder. Call once at startup; tests
// may replace it.
func SetRecorder(r Recorder) {
	mu.Lock()
	recorder = r
	mu.Unlock()
}

// GetRecorder returns the installed recorder, or nil when unset.
func GetRecorder() Recorder {
	mu.RLock()
	defer mu.RUnlock()
	return recorder
}

// Record reports a model call. It is a no-op when no recorder is installed,
// so model wrappers can call it unconditionally.
func Record(ctx context.Context, info *types.ModelCallInfo) error {
	r := GetRecorder()
	if r == nil {
		return nil
	}
	// Model completion and billing evidence outlive an HTTP disconnect. Keep
	// attribution values from the request while bounding the database write.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	return r.Record(recordCtx, info)
}
