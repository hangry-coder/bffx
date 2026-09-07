package blob

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalProvider_PutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sec := "unit-test-hmac-secret-32bytes!!"
	lp := NewLocalProvider(sec, dir)
	base := "http://example.com"

	res, err := lp.PresignPut(context.Background(), base, "user/1/a.txt", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "PUT" || res.URL == "" {
		t.Fatalf("unexpected %+v", res)
	}

	req := httptest.NewRequest(http.MethodPut, res.URL, bytes.NewBufferString("hello"))
	rec := httptest.NewRecorder()
	lp.ServePut(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status %d body %s", rec.Code, rec.Body.String())
	}

	got, err := os.ReadFile(filepath.Join(dir, "user", "1", "a.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("file: %v %q", err, got)
	}

	gres, err := lp.PresignGet(context.Background(), base, "user/1/a.txt", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	greq := httptest.NewRequest(http.MethodGet, gres.URL, nil)
	grec := httptest.NewRecorder()
	lp.ServeGet(grec, greq)
	if grec.Code != http.StatusOK {
		t.Fatalf("get status %d", grec.Code)
	}
	body, _ := io.ReadAll(grec.Body)
	if string(body) != "hello" {
		t.Fatalf("get body %q", body)
	}
}
