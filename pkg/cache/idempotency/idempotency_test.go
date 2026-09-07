package idempotency

import "testing"

func TestIsDurable(t *testing.T) {
	mem := NewMemoryStore()
	if IsDurable(mem) {
		t.Fatal("memory store should not be durable")
	}
}
