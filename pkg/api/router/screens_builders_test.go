package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"

	"github.com/alicebob/miniredis/v2"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestScreens_RegisterAndExecute(t *testing.T) {
	var spec yaml.Node
	yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/home
sources:
  - app
  - currentUser
  - i18n
  - navigation
  - onboarding
  - flags
  - {kind: Resource, name: Note}
output:
  appName: app.name
  user: currentUser
  nav: navigation
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{}, // Needs App.AuthStrategy for onboarding source
		},
		Screens: []*manifest.Manifest{
			{
				Kind:     "Screen",
				Metadata: manifest.Metadata{Name: "HomeScreen"},
				Spec:     spec,
			},
		},
	}

	s := storage.NewMemoryStore()
	created, _ := s.Create(context.Background(), "User", map[string]any{"name": "Test User"})
	userID := created["id"].(string)

	r := &Router{
		reg:          reg,
		mux:          http.NewServeMux(),
		store:        s,
		i18n:         i18n.NewBundle("en"),
		featureFlags: featureflags.NewFlagService(reg, providers.NewBffxProvider(s, reg)),
	}

	r.registerScreenRoutes()

	req := httptest.NewRequest("GET", "/api/v1/screens/home", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": userID}))
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "TestApp", resp["appName"])
	assert.NotNil(t, resp["user"])
}

func TestBuilders_RegisterAndExecute(t *testing.T) {
	var spec yaml.Node
	yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/builders/test
sources:
  - app
output:
  appName: app.name
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Builders: []*manifest.Manifest{
			{
				Kind:     "Builder",
				Metadata: manifest.Metadata{Name: "TestBuilder"},
				Spec:     spec,
			},
		},
	}

	r := &Router{
		reg: reg,
		mux: http.NewServeMux(),
	}

	r.registerBuilderRoutes()

	req := httptest.NewRequest("GET", "/api/v1/builders/test", nil)
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "TestApp", resp["appName"])
}

func TestScreens_Cache_Integration(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tCache := cache.NewRedisCache(rdb)

	var spec yaml.Node
	yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/home
cache:
  ttl: 60
  allow_anonymous: true
  vary: ["X-App-Version"]
sources:
  - app
output:
  name: app.name
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}},
		Screens: []*manifest.Manifest{{Kind: "Screen", Metadata: manifest.Metadata{Name: "Home"}, Spec: spec}},
	}

	r := &Router{
		reg:      reg,
		mux:      http.NewServeMux(),
		tagCache: tCache,
	}
	r.registerScreenRoutes()

	// 1. First request: MISS
	req := httptest.NewRequest("GET", "/api/v1/screens/home", nil)
	rr := httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, "MISS", rr.Header().Get("X-BFFX-Cache"))
	etag := rr.Header().Get("ETag")
	assert.NotEmpty(t, etag)

	// 2. Second request: HIT
	rr = httptest.NewRecorder()
	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, "HIT", rr.Header().Get("X-BFFX-Cache"))
	assert.Equal(t, etag, rr.Header().Get("ETag"))

	// 3. ETag check: 304
	req304 := httptest.NewRequest("GET", "/api/v1/screens/home", nil)
	req304.Header.Set("If-None-Match", etag)
	rr304 := httptest.NewRecorder()
	r.mux.ServeHTTP(rr304, req304)
	assert.Equal(t, http.StatusNotModified, rr304.Code)

	// 4. Query Variance Check
	reqVar1 := httptest.NewRequest("GET", "/api/v1/screens/home?a=1&b=2", nil)
	rrVar1 := httptest.NewRecorder()
	r.mux.ServeHTTP(rrVar1, reqVar1)
	assert.Equal(t, "MISS", rrVar1.Header().Get("X-BFFX-Cache"))

	// Identical query but different order: Should HIT due to normalization
	reqVar2 := httptest.NewRequest("GET", "/api/v1/screens/home?b=2&a=1", nil)
	rrVar2 := httptest.NewRecorder()
	r.mux.ServeHTTP(rrVar2, reqVar2)
	assert.Equal(t, "HIT", rrVar2.Header().Get("X-BFFX-Cache"))
}
