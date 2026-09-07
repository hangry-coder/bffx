package events

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event represents a system event.
type Event struct {
	Resource  string         `json:"resource"`
	Action    string         `json:"action"` // created, updated, deleted
	Payload   map[string]any `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

// Bus is the interface for publishing and subscribing to events.
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, channel string) <-chan Event
	Watch(ctx context.Context, pattern string) <-chan Event
}

// RedisBus implements Bus using Redis Pub/Sub.
type RedisBus struct {
	client *redis.Client
}

func NewRedisBus(client *redis.Client) *RedisBus {
	return &RedisBus{client: client}
}

func (b *RedisBus) Publish(ctx context.Context, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	channel := fmt.Sprintf("bffx:events:%s:%s", strings.ToLower(event.Resource), event.Action)
	// Also publish to a general resource channel
	generalChannel := fmt.Sprintf("bffx:events:%s", strings.ToLower(event.Resource))

	err = b.client.Publish(ctx, channel, data).Err()
	if err != nil {
		return err
	}

	return b.client.Publish(ctx, generalChannel, data).Err()
}

func (b *RedisBus) Subscribe(ctx context.Context, channel string) <-chan Event {
	pubsub := b.client.Subscribe(ctx, channel)
	ch := make(chan Event)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Redis subscriber panicked: %v\n%s", r, logger.Stack())
			}
		}()
		defer pubsub.Close()
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					return
				}

				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					continue
				}
				ch <- event
			}
		}
	}()

	return ch
}

func (b *RedisBus) Watch(ctx context.Context, pattern string) <-chan Event {
	pubsub := b.client.PSubscribe(ctx, pattern)
	ch := make(chan Event)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Redis watcher panicked: %v\n%s", r, logger.Stack())
			}
		}()
		defer pubsub.Close()
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err != nil {
					return
				}

				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					continue
				}
				ch <- event
			}
		}
	}()

	return ch
}

// MemoryBus implements Bus for testing or local development without Redis.
type MemoryBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Event
}

func NewMemoryBus() *MemoryBus {
	return &MemoryBus{subscribers: make(map[string][]chan Event)}
}

func (b *MemoryBus) Publish(_ context.Context, event Event) error {
	channel := fmt.Sprintf("bffx:events:%s:%s", strings.ToLower(event.Resource), event.Action)
	generalChannel := fmt.Sprintf("bffx:events:%s", strings.ToLower(event.Resource))

	b.mu.RLock()
	var targets []chan Event
	targets = append(targets, b.subscribers[channel]...)
	targets = append(targets, b.subscribers[generalChannel]...)
	b.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

func (b *MemoryBus) Subscribe(_ context.Context, channel string) <-chan Event {
	ch := make(chan Event, 10)
	b.mu.Lock()
	b.subscribers[channel] = append(b.subscribers[channel], ch)
	b.mu.Unlock()
	return ch
}

func (b *MemoryBus) Watch(ctx context.Context, pattern string) <-chan Event {
	// MemoryBus pattern matching is simplified to prefix match
	ch := make(chan Event, 10)
	prefix := strings.TrimSuffix(pattern, "*")

	// We'd need to store patterns or re-evaluate on every publish.
	// For MemoryBus (dev), we'll just subscribe to the prefix if it's specific.
	b.mu.Lock()
	b.subscribers[prefix] = append(b.subscribers[prefix], ch)
	b.mu.Unlock()
	return ch
}
