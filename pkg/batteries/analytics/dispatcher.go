package analytics

import (
	"context"
	"sync"
)

// Dispatcher provides an asynchronous, non-blocking way to send analytics events.
// It uses a worker pool to process events from a buffered channel.
type Dispatcher struct {
	provider Provider
	queue    chan eventRequest
	wg       sync.WaitGroup
	workers  int
}

type eventRequest struct {
	ctx        context.Context
	name       string
	properties map[string]any
}

// NewDispatcher creates a new Dispatcher with the given provider and worker count.
func NewDispatcher(p Provider, workers int) *Dispatcher {
	if workers <= 0 {
		workers = 5 // Default to 5 workers
	}
	d := &Dispatcher{
		provider: p,
		queue:    make(chan eventRequest, 1000), // Buffer up to 1000 events
		workers:  workers,
	}
	
	for i := 0; i < workers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	
	return d
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for req := range d.queue {
		d.provider.Track(req.ctx, req.name, req.properties)
	}
}

// Track enqueues an event to be processed by the worker pool.
// It is non-blocking; if the queue is full, the event is dropped.
func (d *Dispatcher) Track(ctx context.Context, name string, properties map[string]any) {
	// Don't block the caller.
	select {
	case d.queue <- eventRequest{
		ctx:        ctx,
		name:       name,
		properties: properties,
	}:
	default:
		// Queue full, drop event to protect system stability.
	}
}

// Close stops the dispatcher and waits for all events to be processed.
func (d *Dispatcher) Close(ctx context.Context) error {
	close(d.queue)
	d.wg.Wait()
	return d.provider.Close(ctx)
}

// Type returns the type of the underlying provider.
func (d *Dispatcher) Type() string {
	return d.provider.Type()
}
