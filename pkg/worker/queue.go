package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const queueKey = "bffx:jobs"

// Queue is the interface for the async job queue.
type Queue interface {
	Push(ctx context.Context, job Job) error
	Pop(ctx context.Context) (Job, error)
}

// RedisQueue implements Queue using Redis list operations.
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue creates a RedisQueue from a Redis URL (e.g. "redis://localhost:6379").
// Returns an error if the URL is empty or the connection cannot be established.
func NewRedisQueue(ctx context.Context, redisURL string) (*RedisQueue, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is empty")
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisQueue{client: client}, nil
}

// Push serializes the job to JSON and pushes it to the left of the list.
func (q *RedisQueue) Push(ctx context.Context, job Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return q.client.LPush(ctx, queueKey, data).Err()
}

// Pop blocks until a job is available, then deserializes and returns it.
// Uses BRPOP (blocking right pop) so the worker consumes in FIFO order.
func (q *RedisQueue) Pop(ctx context.Context) (Job, error) {
	result, err := q.client.BRPop(ctx, 0, queueKey).Result()
	if err != nil {
		return Job{}, fmt.Errorf("brpop: %w", err)
	}

	// result is [key, value]
	if len(result) < 2 {
		return Job{}, fmt.Errorf("unexpected brpop result length")
	}

	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return Job{}, fmt.Errorf("unmarshal job: %w", err)
	}

	return job, nil
}
// MemoryQueue implements Queue using an in-memory channel.
type MemoryQueue struct {
	ch chan Job
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		ch: make(chan Job, 100),
	}
}

func (q *MemoryQueue) Push(ctx context.Context, job Job) error {
	select {
	case q.ch <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("memory queue full")
	}
}

func (q *MemoryQueue) Pop(ctx context.Context) (Job, error) {
	select {
	case job := <-q.ch:
		return job, nil
	case <-ctx.Done():
		return Job{}, ctx.Err()
	}
}
