package router

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestNewGRPCProxyHTTPRequest_GETMergesPayloadIntoQuery(t *testing.T) {
	t.Setenv("BFFX_APP_SECRET", "test-secret-32-chars-minimum!!!!!!")

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer tok",
		"x-device-id", "device-1",
	))

	req, err := NewGRPCProxyHTTPRequest(ctx, http.MethodGet, "/api/v1/fasting/overview", []byte(`{"tz":"Europe/London","week_offset":2}`))
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodGet {
		t.Fatalf("method: got %s want GET", req.Method)
	}
	if req.Body != nil {
		t.Fatal("GET proxy must not send a body")
	}
	if !strings.Contains(req.URL.RawQuery, "week_offset=2") {
		t.Fatalf("missing week_offset in query: %s", req.URL.RawQuery)
	}
	if !strings.Contains(req.URL.RawQuery, "tz=Europe") {
		t.Fatalf("missing tz in query: %s", req.URL.RawQuery)
	}
	if req.Header.Get("Authorization") != "Bearer tok" {
		t.Fatalf("authorization header: %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("X-Device-ID") != "device-1" {
		t.Fatalf("device header: %q", req.Header.Get("X-Device-ID"))
	}
	if req.Header.Get("X-App-Secret") != "test-secret-32-chars-minimum!!!!!!" {
		t.Fatalf("app secret not injected: %q", req.Header.Get("X-App-Secret"))
	}
}

func TestNewGRPCProxyHTTPRequest_POSTKeepsBody(t *testing.T) {
	ctx := context.Background()
	payload := []byte(`{"started_at":"2026-06-05T12:00:00Z"}`)

	req, err := NewGRPCProxyHTTPRequest(ctx, http.MethodPost, "/api/v1/fasting/fast/start", payload)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost {
		t.Fatalf("method: got %s", req.Method)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(payload) {
		t.Fatalf("body: got %s want %s", body, payload)
	}
}

func TestNewGRPCProxyHTTPRequest_RespectsClientAppSecret(t *testing.T) {
	t.Setenv("BFFX_APP_SECRET", "server-secret")

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-app-secret", "client-secret"))
	req, err := NewGRPCProxyHTTPRequest(ctx, http.MethodGet, "/api/v1/app/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("X-App-Secret") != "client-secret" {
		t.Fatalf("should not override client secret, got %q", req.Header.Get("X-App-Secret"))
	}
}

func TestNewGRPCProxyHTTPRequest_EndToEndGET(t *testing.T) {
	t.Setenv("BFFX_APP_SECRET", "")

	var seenMethod, seenQuery, seenSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenQuery = r.URL.RawQuery
		seenSecret = r.Header.Get("X-App-Secret")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	path := srv.URL + "/overview"
	req, err := NewGRPCProxyHTTPRequest(context.Background(), http.MethodGet, path, []byte(`{"week_offset":1}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-App-Secret", "from-client")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if seenMethod != http.MethodGet {
		t.Fatalf("server saw method %s", seenMethod)
	}
	if !strings.Contains(seenQuery, "week_offset=1") {
		t.Fatalf("server query %q", seenQuery)
	}
	if seenSecret != "from-client" {
		t.Fatalf("server secret %q", seenSecret)
	}
}
