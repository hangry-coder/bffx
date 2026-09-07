package chat

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteToken(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := NewWriter(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteToken(w, "hello", "sess", "gemini"); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"token":"hello"`) || !strings.Contains(body, `"session_id":"sess"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
