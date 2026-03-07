// README: Worker utilities — panic recovery wrapper for background goroutines.
package worker

import (
	"context"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// RunWithRecovery runs fn in a loop, restarting it if it panics.
// It logs the panic stack trace and waits restartDelay before restarting.
// If registry is non-nil, it records periodic heartbeats while fn is running.
// The function stops when ctx is cancelled.
func RunWithRecovery(ctx context.Context, name string, fn func(ctx context.Context), restartDelay time.Duration, registry *Registry) {
	for {
		if ctx.Err() != nil {
			return
		}

		// Record heartbeats while the worker function is running.
		heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
		if registry != nil {
			registry.Heartbeat(name)
			go func() {
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						registry.Heartbeat(name)
					case <-heartbeatCtx.Done():
						return
					}
				}
			}()
		}

		func() {
			defer func() {
				heartbeatCancel()
				if r := recover(); r != nil {
					log.Printf("worker %s panicked: %v\n%s", name, r, debug.Stack())
				}
			}()
			fn(ctx)
		}()

		// If fn returned normally (ctx cancelled), exit.
		if ctx.Err() != nil {
			return
		}
		log.Printf("worker %s exited unexpectedly, restarting in %v", name, restartDelay)
		select {
		case <-time.After(restartDelay):
		case <-ctx.Done():
			return
		}
	}
}

// Registry tracks running workers and their heartbeats for health checks.
type Registry struct {
	mu      sync.RWMutex
	workers map[string]time.Time
}

// NewRegistry creates a new worker registry.
func NewRegistry() *Registry {
	return &Registry{workers: make(map[string]time.Time)}
}

// Heartbeat records that a worker is alive.
func (r *Registry) Heartbeat(name string) {
	r.mu.Lock()
	r.workers[name] = time.Now()
	r.mu.Unlock()
}

// Status returns a map of worker names to their last heartbeat time.
func (r *Registry) Status() map[string]time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]time.Time, len(r.workers))
	for k, v := range r.workers {
		out[k] = v
	}
	return out
}

// AllHealthy returns true if all registered workers have heartbeated within maxAge.
func (r *Registry) AllHealthy(maxAge time.Duration) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := time.Now().Add(-maxAge)
	for _, t := range r.workers {
		if t.Before(cutoff) {
			return false
		}
	}
	return true
}
