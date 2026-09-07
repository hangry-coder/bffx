package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/router"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestCacheInvalidation_Integration(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tCache := cache.NewRedisCache(rdb)

	baseStore := storage.NewMemoryStore()
	store := &storage.InvalidatingStore{
		Store: baseStore,
		OnWrite: func(ctx context.Context, resource string, id string) {
			tags := []string{"resource:" + resource}
			if id != "" {
				tags = append(tags, "resource:"+resource+":"+id)
			}
			_, _ = tCache.InvalidateTags(ctx, tags)
		},
	}

	var screenSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/notes
cache:
  ttl: 60
  allow_anonymous: true
  tags_from: ["resource:Note"]
sources:
  - app
output:
  appName: app.name
`), &screenSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Path:     "/tmp/bffx/project.yaml",
		},
		Screens: []*manifest.Manifest{{Kind: "Screen", Metadata: manifest.Metadata{Name: "Notes"}, Spec: screenSpec}},
	}

	bus := events.NewMemoryBus()
	notify := notifications.NewManager(store)
	emailMgr := email.NewManager()
	commHub := comm.NewHub(store, notify, emailMgr, nil)
	bundle := i18n.NewBundle("en")

	jwtSvc := auth.NewJWTService("test-secret")
	authProv := auth.NewJWTProvider(jwtSvc)

	r := router.NewRouter(router.RouterConfig{
		Registry:      reg,
		Store:         store,
		Telemetry:     store,
		JobStore:      worker.NewMemoryJobStore(),
		AuthProvider:  authProv,
		JWTService:    jwtSvc,
		WorkerSecret:  "worker-secret",
		EventBus:      bus,
		Notifications: notify,
		Email:         emailMgr,
		CommHub:       commHub,
		I18n:          bundle,
		TagCache:      tCache,
	})
	handler := r.Setup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/screens/notes", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first read: status=%d body=%s", rr.Code, rr.Body.String())
	}
	assert.Equal(t, "MISS", rr.Header().Get("X-BFFX-Cache"))

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, "HIT", rr.Header().Get("X-BFFX-Cache"))

	_, _ = store.Create(context.Background(), "Note", map[string]any{"title": "New Note"})

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, "MISS", rr.Header().Get("X-BFFX-Cache"))
}

