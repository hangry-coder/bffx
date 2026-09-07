package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

type testMemoryKillSwitch struct {
	entries map[string]runtimecontracts.KillSwitch
}

func newTestMemoryKillSwitch() *testMemoryKillSwitch {
	return &testMemoryKillSwitch{entries: make(map[string]runtimecontracts.KillSwitch)}
}

func (s *testMemoryKillSwitch) key(screen, section string) string {
	return screen + "::" + section
}

func (s *testMemoryKillSwitch) Set(_ context.Context, screen, section string, upd runtimecontracts.KillSwitchUpdate) (runtimecontracts.KillSwitch, error) {
	ks := runtimecontracts.KillSwitch{
		Screen: screen, Section: section, Enabled: upd.Enabled,
		Reason: upd.Reason, ExpiresAt: upd.ExpiresAt, UpdatedAt: time.Now(),
	}
	s.entries[s.key(screen, section)] = ks
	return ks, nil
}

func (s *testMemoryKillSwitch) Get(_ context.Context, screen, section string) (*runtimecontracts.KillSwitch, error) {
	ks, ok := s.entries[s.key(screen, section)]
	if !ok {
		return nil, nil
	}
	if ks.ExpiresAt != nil && time.Now().After(*ks.ExpiresAt) {
		return nil, nil
	}
	out := ks
	return &out, nil
}

func killSwitchTestSetup(t *testing.T) (*Router, *testMemoryKillSwitch) {
	t.Helper()
	var spec yaml.Node
	if err := yaml.Unmarshal([]byte(`
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: [app]
output:
  appName: app.name
sections:
  - { key: weekly, default_visible: true, route: { path: /api/v1/screens/home/weekly, auth: optional } }
`), &spec); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if spec.Kind == yaml.DocumentNode && len(spec.Content) > 0 {
		spec = *spec.Content[0]
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: yaml.Node{}},
		Screens: []*manifest.Manifest{{
			Kind:     "Screen",
			Metadata: manifest.Metadata{Name: "home"},
			Spec:     spec,
		}},
		ApiPrefix: "/api/v1",
	}
	store := storage.NewMemoryStore()
	ks := newTestMemoryKillSwitch()

	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        store,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(store, reg)),
		killSwitch:   ks,
	}
	r.registerScreenRoutes()
	return r, ks
}

func doScreenGET(r *Router, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)
	return rr
}

func TestScreenKillSwitch_NoSwitch_ReturnsScreen(t *testing.T) {
	r, _ := killSwitchTestSetup(t)
	rr := doScreenGET(r, "/api/v1/screens/home")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 without switch, got %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["appName"] != "TestApp" {
		t.Fatalf("expected appName=TestApp, got %+v", body)
	}
}

func TestScreenKillSwitch_ScreenLevelActive_Returns503Envelope(t *testing.T) {
	r, ks := killSwitchTestSetup(t)
	if _, err := ks.Set(context.Background(), "home", "", runtimecontracts.KillSwitchUpdate{Enabled: true, Reason: "incident-42"}); err != nil {
		t.Fatal(err)
	}
	rr := doScreenGET(r, "/api/v1/screens/home")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["killed"] != true {
		t.Fatalf("expected killed=true, got %+v", body)
	}
	if body["reason"] != "incident-42" {
		t.Fatalf("expected reason=incident-42, got %+v", body)
	}
}

func TestScreenKillSwitch_ScreenLevelKillsSectionRoute(t *testing.T) {
	r, ks := killSwitchTestSetup(t)
	if _, err := ks.Set(context.Background(), "home", "", runtimecontracts.KillSwitchUpdate{Enabled: true, Reason: "wholesale"}); err != nil {
		t.Fatal(err)
	}
	rr := doScreenGET(r, "/api/v1/screens/home/weekly")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on section route, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestScreenKillSwitch_SectionLevelOnly_ScreenRouteStillServes(t *testing.T) {
	r, ks := killSwitchTestSetup(t)
	if _, err := ks.Set(context.Background(), "home", "weekly", runtimecontracts.KillSwitchUpdate{Enabled: true, Reason: "section bug"}); err != nil {
		t.Fatal(err)
	}

	// Screen route is unaffected.
	rr := doScreenGET(r, "/api/v1/screens/home")
	if rr.Code != http.StatusOK {
		t.Fatalf("screen route should be 200 when only section is killed, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Section route returns 503.
	rr = doScreenGET(r, "/api/v1/screens/home/weekly")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("section route should be 503, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestScreenKillSwitch_AutoRevertExpired(t *testing.T) {
	r, ks := killSwitchTestSetup(t)
	past := time.Now().Add(-1 * time.Minute)
	if _, err := ks.Set(context.Background(), "home", "", runtimecontracts.KillSwitchUpdate{Enabled: true, ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	rr := doScreenGET(r, "/api/v1/screens/home")
	if rr.Code != http.StatusOK {
		t.Fatalf("expired switch should auto-revert (200), got %d body=%s", rr.Code, rr.Body.String())
	}
}
