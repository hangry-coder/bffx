package router

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func TestScreenStream_EmitsSnapshotWithHash(t *testing.T) {
	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
name: Live
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/live }
stream: true
stream_interval_ms: 50
sources: [app]
output:
  appName: app.name
`), &spec)
	if spec.Kind == yaml.DocumentNode && len(spec.Content) > 0 {
		spec = *spec.Content[0]
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "Streamed"}, Spec: yaml.Node{}},
		Screens: []*manifest.Manifest{{
			Kind:     "Screen",
			Metadata: manifest.Metadata{Name: "live"},
			Spec:     spec,
		}},
		ApiPrefix: "/api/v1",
	}
	store := storage.NewMemoryStore()
	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        store,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(store, reg)),
	}
	r.registerScreenRoutes()

	srv := httptest.NewServer(r.mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/screens/live/stream", nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("unexpected content-type: %q", got)
	}

	// Read the initial frame and verify it has an id (hash) + payload.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 8*1024), 1024*1024)
	sawID, sawEvent, sawData := false, false, false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "id: ") {
			sawID = true
		}
		if strings.HasPrefix(line, "event: screen") {
			sawEvent = true
		}
		if strings.HasPrefix(line, "data: ") {
			sawData = true
			if !strings.Contains(line, `"appName":"Streamed"`) {
				t.Fatalf("expected payload to include appName, got %q", line)
			}
		}
		if sawID && sawEvent && sawData {
			return
		}
	}
	t.Fatalf("never saw a complete screen frame (id=%v event=%v data=%v)", sawID, sawEvent, sawData)
}
