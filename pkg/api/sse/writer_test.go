package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type flushWriter struct {
	http.ResponseWriter
	flushed bool
}

func (f *flushWriter) Flush() {
	f.flushed = true
}

func TestNewWriter_SetsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	fw := &flushWriter{ResponseWriter: rec}
	_, err := NewWriter(fw, WithAllowOrigin("*"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS header")
	}
}

func TestWriter_WriteJSONAndCloseDone(t *testing.T) {
	rec := httptest.NewRecorder()
	fw := &flushWriter{ResponseWriter: rec}
	w, err := NewWriter(fw)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := w.WriteJSON(map[string]string{"token": "hi"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	w.CloseDone()
	body := rec.Body.String()
	if !strings.Contains(body, `"token":"hi"`) {
		t.Fatalf("body missing token: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("body missing done event: %s", body)
	}
	if !fw.flushed {
		t.Fatal("expected Flush to be called")
	}
}

type nopWriter struct{}

func (nopWriter) Header() http.Header       { return make(http.Header) }
func (nopWriter) Write([]byte) (int, error) { return 0, nil }
func (nopWriter) WriteHeader(int)           {}

func TestNewWriter_Unsupported(t *testing.T) {
	_, err := NewWriter(nopWriter{})
	if err == nil {
		t.Fatal("expected error when ResponseWriter is not a Flusher")
	}
}
