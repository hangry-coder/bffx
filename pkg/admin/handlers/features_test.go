package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/features"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func mkManifest(t *testing.T, kind, name, feature, specYAML string) *manifest.Manifest {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(specYAML), &node); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	return &manifest.Manifest{
		ApiVersion: "bffx.io/v1alpha1",
		Kind:       kind,
		Metadata:   manifest.Metadata{Name: name},
		Feature:    feature,
		Spec:       node,
	}
}

func TestFeaturesHandler_ListFeatures(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: [currentUser]
sections: [{ key: weekly_stats, default_visible: true }]
`))
	reg.Actions = append(reg.Actions, mkManifest(t, "Action", "log_water_action", "mobile", `
group: mobile
route: { method: POST, path: /api/v1/app/water-log, auth: required }
`))

	h := NewFeaturesHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/features", nil)
	rr := httptest.NewRecorder()
	h.ListFeatures(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var tree features.FeatureTree
	if err := json.Unmarshal(rr.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(tree.Groups) != 1 || tree.Groups[0].Name != "mobile" {
		t.Fatalf("expected single mobile group, got %+v", tree.Groups)
	}
	if len(tree.Groups[0].Screens) != 1 || tree.Groups[0].Screens[0].Name != "home" {
		t.Fatalf("expected home screen, got %+v", tree.Groups[0].Screens)
	}
	if len(tree.OrphanActions) != 1 || tree.OrphanActions[0].Name != "log_water_action" {
		t.Fatalf("expected log_water_action orphan, got %+v", tree.OrphanActions)
	}

	// Ensure Content-Type is JSON.
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}
}

func TestFeaturesHandler_GroupFilterUnknownReturnsEmpty(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))
	h := NewFeaturesHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/features?group=does-not-exist", nil)
	rr := httptest.NewRecorder()
	h.ListFeatures(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var tree features.FeatureTree
	if err := json.Unmarshal(rr.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tree.Groups) != 0 {
		t.Fatalf("expected zero groups for unknown filter, got %+v", tree.Groups)
	}
}

type stubRunner struct {
	called  bool
	gotOpts features.RunOpts
	gotName string
	result  features.RunResult
	err     error
}

func (s *stubRunner) RunScreen(_ context.Context, name string, opts features.RunOpts) (features.RunResult, error) {
	s.called = true
	s.gotName = name
	s.gotOpts = opts
	return s.result, s.err
}

func TestFeaturesHandler_RunAs_WithoutRunnerReturns503(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))
	h := NewFeaturesHandler(reg)

	req := httptest.NewRequest(http.MethodPost, "/features/screens/home/run-as", nil)
	req.SetPathValue("name", "home")
	rr := httptest.NewRecorder()
	h.RunAs(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when runner missing, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestFeaturesHandler_RunAs_UnknownScreenReturns404(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	h := NewFeaturesHandler(reg)
	h.SetRunner(&stubRunner{result: features.RunResult{Output: map[string]any{}}})

	req := httptest.NewRequest(http.MethodPost, "/features/screens/ghost/run-as", nil)
	req.SetPathValue("name", "ghost")
	rr := httptest.NewRecorder()
	h.RunAs(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestFeaturesHandler_RunAs_Success(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))

	runner := &stubRunner{
		result: features.RunResult{Output: map[string]any{"appName": "Demo"}},
	}
	h := NewFeaturesHandler(reg)
	h.SetRunner(runner)

	body := strings.NewReader(`{"user_id":"u_abc","locale":"en","query":{"x":"1"},"headers":{"X-Device-ID":"d_xyz"}}`)
	req := httptest.NewRequest(http.MethodPost, "/features/screens/home/run-as", body)
	req.SetPathValue("name", "home")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.RunAs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !runner.called || runner.gotName != "home" {
		t.Fatalf("runner not invoked correctly: called=%v name=%q", runner.called, runner.gotName)
	}
	if runner.gotOpts.UserID != "u_abc" || runner.gotOpts.Locale != "en" {
		t.Fatalf("unexpected opts: %+v", runner.gotOpts)
	}
	if got := runner.gotOpts.Query.Get("x"); got != "1" {
		t.Fatalf("expected query x=1, got %q", got)
	}
	if got := runner.gotOpts.Headers.Get("X-Device-Id"); got != "d_xyz" {
		t.Fatalf("expected device header d_xyz, got %q", got)
	}

	var got struct {
		Output map[string]any `json:"output"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Output["appName"] != "Demo" {
		t.Fatalf("expected appName=Demo, got %+v", got.Output)
	}
}

type stubInvalidator struct {
	gotScreen, gotSection string
	flushed               int
	err                   error
}

func (s *stubInvalidator) InvalidateScreenCache(_ context.Context, screen string) (int, error) {
	s.gotScreen = screen
	s.gotSection = ""
	return s.flushed, s.err
}

func (s *stubInvalidator) InvalidateSectionCache(_ context.Context, screen, section string) (int, error) {
	s.gotScreen = screen
	s.gotSection = section
	return s.flushed, s.err
}

func TestFeaturesHandler_InvalidateScreenCache(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))
	h := NewFeaturesHandler(reg)

	// Without invalidator wired: 503.
	req := httptest.NewRequest(http.MethodPost, "/features/screens/home/cache/invalidate", nil)
	req.SetPathValue("name", "home")
	rr := httptest.NewRecorder()
	h.InvalidateScreenCache(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without invalidator, got %d", rr.Code)
	}

	// With invalidator wired: returns the flushed count.
	inv := &stubInvalidator{flushed: 7}
	h.SetCacheInvalidator(inv)
	req = httptest.NewRequest(http.MethodPost, "/features/screens/home/cache/invalidate", nil)
	req.SetPathValue("name", "home")
	rr = httptest.NewRecorder()
	h.InvalidateScreenCache(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if inv.gotScreen != "home" || inv.gotSection != "" {
		t.Fatalf("invalidator received wrong args: %+v", inv)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(body["flushed"].(float64)) != 7 {
		t.Fatalf("expected flushed=7, got %+v", body)
	}
}

func TestFeaturesHandler_InvalidateSectionCache(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: [{ key: weekly, default_visible: true }]
`))
	h := NewFeaturesHandler(reg)
	inv := &stubInvalidator{flushed: 3}
	h.SetCacheInvalidator(inv)

	req := httptest.NewRequest(http.MethodPost, "/features/screens/home/sections/weekly/cache/invalidate", nil)
	req.SetPathValue("name", "home")
	req.SetPathValue("key", "weekly")
	rr := httptest.NewRecorder()
	h.InvalidateSectionCache(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if inv.gotScreen != "home" || inv.gotSection != "weekly" {
		t.Fatalf("invalidator received wrong args: %+v", inv)
	}
}

func TestFeaturesHandler_RunAs_RejectsBadJSON(t *testing.T) {
	reg := &manifest.Registry{ApiPrefix: "/api/v1"}
	reg.Screens = append(reg.Screens, mkManifest(t, "Screen", "home", "mobile", `
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))
	h := NewFeaturesHandler(reg)
	h.SetRunner(&stubRunner{})

	req := httptest.NewRequest(http.MethodPost, "/features/screens/home/run-as", bytes.NewBufferString("{bad"))
	req.SetPathValue("name", "home")
	req.ContentLength = int64(len("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.RunAs(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}
