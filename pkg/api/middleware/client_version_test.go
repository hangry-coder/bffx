package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestClientVersion_BelowMinimum_ReturnsErrorEnvelope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	spec := manifest.LifecycleSpec{}
	spec.Platforms = map[string]manifest.PlatformLifecycle{
		"default": {MinVersion: "2.0.0"},
	}

	handler := RequestID(ClientVersion(spec)(next))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-BFFX-Client-Version", "1.9.9")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUpgradeRequired)
	}
	if rr.Header().Get("X-BFFX-Upgrade-Required") != "true" {
		t.Fatalf("missing X-BFFX-Upgrade-Required header")
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%q", err, rr.Body.String())
	}
	if body.Error.Code != "client_upgrade_required" {
		t.Fatalf("code = %q, want client_upgrade_required", body.Error.Code)
	}
}

func TestClientVersion_AdminUIExempt(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	spec := manifest.LifecycleSpec{}
	spec.Semver.Enforce = true
	spec.Platforms = map[string]manifest.PlatformLifecycle{
		"default": {MinVersion: "9.0.0"},
	}

	handler := ClientVersion(spec)(next)

	for _, path := range []string{"/admin/", "/admin", "/admin/api/admin/login"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("path %q status = %d, want 200 without client version header", path, rr.Code)
		}
	}
}

func TestClientVersion_SuggestUpgrade(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	spec := manifest.LifecycleSpec{}
	spec.Platforms = map[string]manifest.PlatformLifecycle{
		"ios": {MinVersion: "1.0.0", SuggestVersion: "2.0.0"},
	}

	handler := ClientVersion(spec)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-BFFX-Client-Version", "1.5.0")
	req.Header.Set("X-BFFX-Platform", "ios")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Header().Get("X-BFFX-Upgrade-Suggested") != "true" {
		t.Fatalf("missing X-BFFX-Upgrade-Suggested header")
	}
}
