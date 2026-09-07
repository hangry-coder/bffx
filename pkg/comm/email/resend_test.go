package email

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResend_NonOK_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	t.Cleanup(ts.Close)

	p := &ResendProvider{
		APIKey:     "re_test",
		APIURL:     ts.URL,
		HTTPClient: ts.Client(),
	}
	err := p.Send(context.Background(), "a@b.com", "s", "b")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrResendNonOK) {
		t.Fatalf("want %v, got %v", ErrResendNonOK, err)
	}
	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("error should mention status and body: %v", err)
	}
}

func TestResend_OK_PayloadShape(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	t.Cleanup(ts.Close)

	p := &ResendProvider{
		APIKey:     "re_test",
		From:       "me@example.com",
		APIURL:     ts.URL,
		HTTPClient: ts.Client(),
	}
	if err := p.Send(context.Background(), "to@example.com", "subj", "<p>html</p>"); err != nil {
		t.Fatal(err)
	}
	if got["from"] != "me@example.com" {
		t.Fatalf("from: %v", got["from"])
	}
	to, _ := got["to"].([]any)
	if len(to) != 1 || to[0] != "to@example.com" {
		t.Fatalf("to: %v", got["to"])
	}
}
