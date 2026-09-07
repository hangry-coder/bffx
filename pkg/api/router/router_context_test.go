package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"

	"gopkg.in/yaml.v3"
)

type ctxMarkerKey struct{}

type capturingStore struct {
	*storage.MemoryStore
	mu      sync.Mutex
	lastCtx context.Context
}

func (c *capturingStore) touch(ctx context.Context) {
	c.mu.Lock()
	c.lastCtx = ctx
	c.mu.Unlock()
}

func (c *capturingStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.List(ctx, resource, limit, offset)
}

func (c *capturingStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.Get(ctx, resource, id)
}

func (c *capturingStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.GetByField(ctx, resource, field, value)
}

func (c *capturingStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.Create(ctx, resource, payload)
}

func (c *capturingStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.Update(ctx, resource, id, payload)
}

func (c *capturingStore) Delete(ctx context.Context, resource, id string) error {
	c.touch(ctx)
	return c.MemoryStore.Delete(ctx, resource, id)
}

func (c *capturingStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.ListByOwner(ctx, resource, ownerID, limit, offset)
}

func (c *capturingStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]storage.Change, error) {
	c.touch(ctx)
	return c.MemoryStore.Reconcile(ctx, reg)
}

func (c *capturingStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.GetChildren(ctx, resource, id)
}

func (c *capturingStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	c.touch(ctx)
	return c.MemoryStore.GetAncestors(ctx, resource, id)
}

func (c *capturingStore) Query(ctx context.Context, resource string) storage.QueryBuilder {
	c.touch(ctx)
	return &captureQueryBuilder{c: c, inner: c.MemoryStore.Query(ctx, resource)}
}

type captureQueryBuilder struct {
	c     *capturingStore
	inner storage.QueryBuilder
}

func (q *captureQueryBuilder) Where(field, op string, value any) storage.QueryBuilder {
	q.inner = q.inner.Where(field, op, value)
	return q
}

func (q *captureQueryBuilder) WhereIn(field string, values []any) storage.QueryBuilder {
	q.inner = q.inner.WhereIn(field, values)
	return q
}

func (q *captureQueryBuilder) OrderBy(field string, desc bool) storage.QueryBuilder {
	q.inner = q.inner.OrderBy(field, desc)
	return q
}

func (q *captureQueryBuilder) Limit(n int) storage.QueryBuilder {
	q.inner = q.inner.Limit(n)
	return q
}

func (q *captureQueryBuilder) Offset(n int) storage.QueryBuilder {
	q.inner = q.inner.Offset(n)
	return q
}

func (q *captureQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	q.c.touch(ctx)
	return q.inner.Execute(ctx)
}

func (q *captureQueryBuilder) Count(ctx context.Context) (int, error) {
	q.c.touch(ctx)
	return q.inner.Count(ctx)
}

func TestRouter_PropagatesContextToStore(t *testing.T) {
	cap := &capturingStore{MemoryStore: storage.NewMemoryStore()}

	var userSpec, noteSpec yaml.Node
	yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: email, type: string, required: true}
  - {name: password, type: string, required: true}
`), &userSpec)
	yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: authenticated
  write: authenticated
fields:
  - {name: text, type: string, required: true}
`), &noteSpec)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
			{Metadata: manifest.Metadata{Name: "Note"}, Spec: noteSpec},
		},
	}

	bus := events.NewMemoryBus()
	notify := notifications.NewManager(cap)
	emailMgr := email.NewManager()
	hub := comm.NewHub(cap, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(cap, reg)
	r := NewRouter(RouterConfig{
		Store:         cap,
		Telemetry:     cap,
		JobStore:      worker.NewMemoryJobStore(),
		Registry:      reg,
		AuthProvider:  auth.NewJWTService("test-secret"),
		JWTService:    auth.NewJWTService("test-secret"),
		EventBus:      bus,
		Notifications: notify,
		Email:         emailMgr,
		CommHub:       hub,
		I18n:          bundle,
		FlagProvider:  flagProvider,
	})
	handler := r.Setup()

	marker := ctxMarkerKey{}
	base := context.WithValue(context.Background(), marker, "signup-ctx")

	signupBody := `{"email": "ctx@example.com", "password": "password123", "name": "Ctx User"}`
	req := httptest.NewRequestWithContext(base, "POST", "/api/v1/auth/signup", bytes.NewBufferString(signupBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup failed: %d %s", rr.Code, rr.Body.String())
	}

	cap.mu.Lock()
	got := cap.lastCtx
	cap.mu.Unlock()
	if got == nil {
		t.Fatal("expected store to record a context")
	}
	if v := got.Value(marker); v != "signup-ctx" {
		t.Fatalf("store saw wrong context: want signup-ctx marker, got %v", v)
	}

	loginBody := `{"email": "ctx@example.com", "password": "password123"}`
	req = httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %v", rr.Body.String())
	}
	var loginResp struct{ Token string }
	json.Unmarshal(rr.Body.Bytes(), &loginResp)

	noteCtx := context.WithValue(context.Background(), marker, "note-ctx")
	noteBody := `{"text": "hi"}`
	req = httptest.NewRequestWithContext(noteCtx, "POST", "/api/v1/notes", bytes.NewBufferString(noteBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST note failed: %d %s", rr.Code, rr.Body.String())
	}

	cap.mu.Lock()
	got = cap.lastCtx
	cap.mu.Unlock()
	if v := got.Value(marker); v != "note-ctx" {
		t.Fatalf("CRUD path context: want note-ctx, got %v", v)
	}
}
