package embedding

import (
	"context"
	"sync"
)

type embeddingCacheFlight struct {
	done   chan struct{}
	vector []float32
	err    error
}

func (f *embeddingCacheFlight) wait(ctx context.Context) ([]float32, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.done:
		if f.err != nil {
			return nil, f.err
		}
		return append([]float32(nil), f.vector...), nil
	}
}

type embeddingCacheFlightRegistry struct {
	mu      sync.Mutex
	flights map[string]*embeddingCacheFlight
}

func newEmbeddingCacheFlightRegistry() *embeddingCacheFlightRegistry {
	return &embeddingCacheFlightRegistry{flights: make(map[string]*embeddingCacheFlight)}
}

// claim returns the existing flight for a cache key, or creates a new flight
// and marks the caller as its leader. Batch callers can claim several keys and
// send all leader inputs in one provider request.
func (r *embeddingCacheFlightRegistry) claim(key string) (*embeddingCacheFlight, bool) {
	flights, leaders := r.claimMany([]string{key})
	return flights[0], leaders[0]
}

// claimMany reserves a whole batch while holding one lock. Two overlapping
// batches therefore cannot split ownership of the same set of keys and turn a
// single provider batch into several partial calls.
func (r *embeddingCacheFlightRegistry) claimMany(keys []string) ([]*embeddingCacheFlight, []bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flights := make([]*embeddingCacheFlight, len(keys))
	leaders := make([]bool, len(keys))
	for i, key := range keys {
		if flight := r.flights[key]; flight != nil {
			flights[i] = flight
			continue
		}
		flight := &embeddingCacheFlight{done: make(chan struct{})}
		r.flights[key] = flight
		flights[i] = flight
		leaders[i] = true
	}
	return flights, leaders
}

func (r *embeddingCacheFlightRegistry) complete(
	key string,
	flight *embeddingCacheFlight,
	vector []float32,
	err error,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if flight == nil || r.flights[key] != flight {
		return
	}
	flight.vector = append([]float32(nil), vector...)
	flight.err = err
	close(flight.done)
	delete(r.flights, key)
}

var embeddingCacheFlights = newEmbeddingCacheFlightRegistry()
