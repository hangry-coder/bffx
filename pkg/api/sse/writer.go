package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Writer emits standards-compliant SSE frames and flushes after each write.
type Writer struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewWriter prepares an http.ResponseWriter for SSE streaming.
func NewWriter(w http.ResponseWriter, opts ...Option) (*Writer, error) {
	cfg := applyConfig(opts)
	applyHeaders(w, cfg)

	flusher, ok := unwrapFlusher(w)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported by client connection")
	}

	return &Writer{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteJSON marshals v and writes a default `data:` frame.
func (s *Writer) WriteJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.WriteRaw(data)
}

// WriteRaw writes a `data:` frame with the given payload bytes.
func (s *Writer) WriteRaw(data []byte) error {
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteEvent writes a named SSE event with a data payload.
func (s *Writer) WriteEvent(event string, data []byte) error {
	return s.WriteEventID("", event, data)
}

// WriteEventID writes an SSE event optionally including an id field.
func (s *Writer) WriteEventID(id, event string, data []byte) error {
	if id != "" {
		if _, err := fmt.Fprintf(s.w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if event != "" {
		if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// WriteComment writes an SSE comment line (often used as heartbeat).
func (s *Writer) WriteComment(comment string) error {
	if _, err := fmt.Fprintf(s.w, ": %s\n\n", comment); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// CloseDone emits the canonical terminal `event: done` frame.
func (s *Writer) CloseDone() {
	_, _ = fmt.Fprintf(s.w, "event: done\ndata: {}\n\n")
	s.flusher.Flush()
}
