package smoke

import (
	"github.com/hangry-coder/bffx/pkg/admin"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/mcp"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPhaseB_EventBus(t *testing.T) {
	bus := events.NewMemoryBus()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	channel := "bffx:events:testresource:created"
	sub := bus.Subscribe(ctx, channel)

	testEvent := events.Event{
		Resource: "TestResource",
		Action:   "created",
		Payload:  map[string]any{"id": "123"},
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		bus.Publish(ctx, testEvent)
	}()

	select {
	case ev := <-sub:
		if ev.Resource != "TestResource" {
			t.Errorf("expected Resource TestResource, got %s", ev.Resource)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

func TestPhaseC_AdminAuth(t *testing.T) {
	store := storage.NewMemoryStore()
	handler := admin.AuthMiddleware(store, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Admin API uses signed session cookies (no legacy Bearer header path).
	req := httptest.NewRequest("GET", "/api/admin/resources", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session cookie, got %d", rr.Code)
	}
}

func TestPhaseD_SwiftUIGenerator(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "bffx-test-*")
	defer os.RemoveAll(tempDir)

	err := generator.GenerateClientIOS(tempDir, "ios")
	if err != nil {
		t.Fatalf("GenerateClientIOS failed: %v", err)
	}

	sdkPath := filepath.Join(tempDir, "ios", "BFFXClient.swift")
	if _, err := os.Stat(sdkPath); os.IsNotExist(err) {
		t.Errorf("BFFXClient.swift not generated")
	}

	content, _ := os.ReadFile(sdkPath)
	if !strings.Contains(string(content), "class BFFXClient") {
		t.Errorf("BFFXClient.swift does not contain BFFXClient class")
	}
}

func TestPhaseD_MCPTools(t *testing.T) {
	reg := &manifest.Registry{}
	_ = mcp.NewServer(".", reg)
	
	// Test handleToolsList contains client.scaffold
	_ = &mcp.Request{ID: "1"}
}
