package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/events"

	"github.com/stretchr/testify/assert"
)

func TestNewStreamHandler_UnsupportedWriter(t *testing.T) {
	bus := events.NewMemoryBus()
	h := NewStreamHandler(bus)

	inner := httptest.NewRecorder()
	w := struct{ http.ResponseWriter }{inner}

	req := httptest.NewRequest("GET", "/", nil)
	h.HandleStream([]string{"ch"})(w, req)

	assert.Equal(t, http.StatusInternalServerError, inner.Code)
}

func TestStreamHandler_HandleStream_EventAndDisconnect(t *testing.T) {
	bus := events.NewMemoryBus()
	h := NewStreamHandler(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleStream([]string{"bffx:events:note:created"})(rr, req)
		close(done)
	}()

	time.Sleep(15 * time.Millisecond)
	_ = bus.Publish(context.Background(), events.Event{
		Resource: "Note",
		Action:   "created",
		Payload:  map[string]any{"id": "1"},
	})

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit")
	}

	body := rr.Body.String()
	assert.Contains(t, body, `"resource":"Note"`)
}
