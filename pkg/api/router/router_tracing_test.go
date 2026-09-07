package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/api/validation"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gopkg.in/yaml.v3"
)

func TestRouter_Tracing_Screen_Success(t *testing.T) {
	// Setup in-memory span recorder for OpenTelemetry
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/home
sources:
  - app
  - currentUser
  - {kind: Resource, name: Note}
output:
  appName: app.name
  user: currentUser
  notes: Note
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Screens: []*manifest.Manifest{
			{
				Kind:     "Screen",
				Metadata: manifest.Metadata{Name: "HomeScreen"},
				Spec:     spec,
			},
		},
	}

	memStore := storage.NewMemoryStore()
	// Wrap with tracing store
	tracingStore := storage.NewTracingStore(memStore)

	// Create user and a note
	createdUser, _ := memStore.Create(context.Background(), "User", map[string]any{"name": "Developer"})
	userID := createdUser["id"].(string)

	_, _ = memStore.Create(context.Background(), "Note", map[string]any{"content": "First note"})

	r := &Router{
		reg:   reg,
		mux:   http.NewServeMux(),
		store: tracingStore,
	}
	r.registerScreenRoutes()

	// Reset recorder
	sr.Reset()

	req := httptest.NewRequest("GET", "/api/v1/screens/home", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": userID}))
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	spans := sr.Ended()

	// Find expected spans
	var screenSpan, resolveSourcesSpan, resolveAppSpan, resolveUserSpan, dbGetUserSpan, resolveNoteSpan, dbListNotesSpan, resolveOutputSpan sdktrace.ReadOnlySpan

	for _, s := range spans {
		switch s.Name() {
		case "Screen: HomeScreen":
			screenSpan = s
		case "ResolveSources":
			resolveSourcesSpan = s
		case "ResolveSource: app":
			resolveAppSpan = s
		case "ResolveSource: currentUser":
			resolveUserSpan = s
		case "Storage.Get":
			dbGetUserSpan = s
		case "ResolveSource: Resource:Note":
			resolveNoteSpan = s
		case "Storage.List":
			dbListNotesSpan = s
		case "ResolveOutput":
			resolveOutputSpan = s
		}
	}

	// 1. Ensure all spans are captured
	require.NotNil(t, screenSpan, "screen span not found")
	require.NotNil(t, resolveSourcesSpan, "resolveSources span not found")
	require.NotNil(t, resolveAppSpan, "resolveApp span not found")
	require.NotNil(t, resolveUserSpan, "resolveUser span not found")
	require.NotNil(t, dbGetUserSpan, "dbGetUser span not found")
	require.NotNil(t, resolveNoteSpan, "resolveNote span not found")
	require.NotNil(t, dbListNotesSpan, "dbListNotes span not found")
	require.NotNil(t, resolveOutputSpan, "resolveOutput span not found")

	// 2. Assert parent-child hierarchy!
	assert.Equal(t, screenSpan.SpanContext().SpanID(), resolveSourcesSpan.Parent().SpanID(), "ResolveSources must be child of Screen")
	assert.Equal(t, screenSpan.SpanContext().SpanID(), resolveOutputSpan.Parent().SpanID(), "ResolveOutput must be child of Screen")

	assert.Equal(t, resolveSourcesSpan.SpanContext().SpanID(), resolveAppSpan.Parent().SpanID(), "ResolveSource: app must be child of ResolveSources")
	assert.Equal(t, resolveSourcesSpan.SpanContext().SpanID(), resolveUserSpan.Parent().SpanID(), "ResolveSource: currentUser must be child of ResolveSources")
	assert.Equal(t, resolveSourcesSpan.SpanContext().SpanID(), resolveNoteSpan.Parent().SpanID(), "ResolveSource: Resource:Note must be child of ResolveSources")

	// Context propagation to DB: dbGetUserSpan and dbListNotesSpan should be nested under their respective source spans!
	assert.Equal(t, resolveUserSpan.SpanContext().SpanID(), dbGetUserSpan.Parent().SpanID(), "DB GetUser must be child of ResolveSource: currentUser (context propagation verification)")
	assert.Equal(t, resolveNoteSpan.SpanContext().SpanID(), dbListNotesSpan.Parent().SpanID(), "DB ListNotes must be child of ResolveSource: Resource:Note (context propagation verification)")

	// 3. Verify attributes
	var screenNameAttr, sourceNameAttr, dbResourceAttr string
	for _, attr := range screenSpan.Attributes() {
		if attr.Key == attribute.Key("screen.name") {
			screenNameAttr = attr.Value.AsString()
		}
	}
	assert.Equal(t, "HomeScreen", screenNameAttr)

	for _, attr := range resolveUserSpan.Attributes() {
		if attr.Key == attribute.Key("source.name") {
			sourceNameAttr = attr.Value.AsString()
		}
	}
	assert.Equal(t, "currentUser", sourceNameAttr)

	for _, attr := range dbListNotesSpan.Attributes() {
		if attr.Key == attribute.Key("db.resource") {
			dbResourceAttr = attr.Value.AsString()
		}
	}
	assert.Equal(t, "Note", dbResourceAttr)
}

func TestRouter_Tracing_Builder(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/builders/card
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
				Metadata: manifest.Metadata{Name: "CardBuilder"},
				Spec:     spec,
			},
		},
	}

	r := &Router{
		reg: reg,
		mux: http.NewServeMux(),
	}
	r.registerBuilderRoutes()

	sr.Reset()

	req := httptest.NewRequest("GET", "/api/v1/builders/card", nil)
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	spans := sr.Ended()

	var builderSpan, resolveSourcesSpan, resolveAppSpan, resolveOutputSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Builder: CardBuilder":
			builderSpan = s
		case "ResolveSources":
			resolveSourcesSpan = s
		case "ResolveSource: app":
			resolveAppSpan = s
		case "ResolveOutput":
			resolveOutputSpan = s
		}
	}

	require.NotNil(t, builderSpan)
	require.NotNil(t, resolveSourcesSpan)
	require.NotNil(t, resolveAppSpan)
	require.NotNil(t, resolveOutputSpan)

	assert.Equal(t, builderSpan.SpanContext().SpanID(), resolveSourcesSpan.Parent().SpanID())
	assert.Equal(t, builderSpan.SpanContext().SpanID(), resolveOutputSpan.Parent().SpanID())
	assert.Equal(t, resolveSourcesSpan.SpanContext().SpanID(), resolveAppSpan.Parent().SpanID())

	var builderNameAttr string
	for _, attr := range builderSpan.Attributes() {
		if attr.Key == attribute.Key("builder.name") {
			builderNameAttr = attr.Value.AsString()
		}
	}
	assert.Equal(t, "CardBuilder", builderNameAttr)
}

func TestRouter_Tracing_PartialSuccess_Error(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
route:
  method: GET
  path: /api/v1/screens/dashboard
partial_success: true
sources:
  - app
  - currentUser
output:
  appName: app.name
  user: currentUser
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Screens: []*manifest.Manifest{
			{
				Kind:     "Screen",
				Metadata: manifest.Metadata{Name: "Dashboard"},
				Spec:     spec,
			},
		},
	}

	memStore := storage.NewMemoryStore()
	tracingStore := storage.NewTracingStore(memStore)

	r := &Router{
		reg:   reg,
		mux:   http.NewServeMux(),
		store: tracingStore,
	}
	r.registerScreenRoutes()

	sr.Reset()

	// Query with a missing user to trigger error under partial success (returns HTTP 206)
	req := httptest.NewRequest("GET", "/api/v1/screens/dashboard", nil)
	req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "missing-user"}))
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusPartialContent, rr.Code)

	spans := sr.Ended()

	var screenSpan, resolveUserSpan, dbGetUserSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Screen: Dashboard":
			screenSpan = s
		case "ResolveSource: currentUser":
			resolveUserSpan = s
		case "Storage.Get":
			dbGetUserSpan = s
		}
	}

	require.NotNil(t, screenSpan)
	require.NotNil(t, resolveUserSpan)
	require.NotNil(t, dbGetUserSpan)

	// Verify that error was recorded on the span hierarchy since the user fetch failed!
	assert.NotEmpty(t, dbGetUserSpan.Events(), "Storage Get should have recorded the error event")
	assert.NotEmpty(t, resolveUserSpan.Events(), "ResolveSource: currentUser should have recorded the error event")
	assert.NotEmpty(t, screenSpan.Events(), "Screen: Dashboard should have recorded the error event")
}

func TestRouter_Tracing_CRUD(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
routes:
  crud: true
policy:
  read: public
  write: public
fields:
  - {name: text, type: string, required: true}
`), &spec)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Resources: []*manifest.Manifest{
			{
				Kind:     "Resource",
				Metadata: manifest.Metadata{Name: "Post"},
				Spec:     spec,
			},
		},
	}

	memStore := storage.NewMemoryStore()
	tracingStore := storage.NewTracingStore(memStore)

	r := &Router{
		reg:       reg,
		mux:       http.NewServeMux(),
		store:     tracingStore,
		validator: validation.NewValidator(),
	}
	r.registerCRUDRoutes()

	// 1. Test POST /api/v1/posts
	sr.Reset()
	postReq := httptest.NewRequest("POST", "/api/v1/posts", bytes.NewBufferString(`{"text": "Hello, OpenTelemetry!"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postRR := httptest.NewRecorder()
	r.mux.ServeHTTP(postRR, postReq)
	assert.Equal(t, http.StatusCreated, postRR.Code)

	var createdItem map[string]any
	assert.NoError(t, json.Unmarshal(postRR.Body.Bytes(), &createdItem))
	postID := createdItem["id"].(string)

	spans := sr.Ended()
	var createSpan, dbCreateSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Resource.Create: Post":
			createSpan = s
		case "Storage.Create":
			dbCreateSpan = s
		}
	}
	require.NotNil(t, createSpan)
	require.NotNil(t, dbCreateSpan)
	assert.Equal(t, createSpan.SpanContext().SpanID(), dbCreateSpan.Parent().SpanID(), "Storage.Create must be nested under Resource.Create")

	var resName, resOp string
	for _, attr := range createSpan.Attributes() {
		if attr.Key == "resource.name" {
			resName = attr.Value.AsString()
		} else if attr.Key == "resource.operation" {
			resOp = attr.Value.AsString()
		}
	}
	assert.Equal(t, "Post", resName)
	assert.Equal(t, "create", resOp)

	// 2. Test GET /api/v1/posts
	sr.Reset()
	listReq := httptest.NewRequest("GET", "/api/v1/posts", nil)
	listRR := httptest.NewRecorder()
	r.mux.ServeHTTP(listRR, listReq)
	assert.Equal(t, http.StatusOK, listRR.Code)

	spans = sr.Ended()
	var listSpan, dbListSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Resource.List: Post":
			listSpan = s
		case "Storage.List":
			dbListSpan = s
		}
	}
	require.NotNil(t, listSpan)
	require.NotNil(t, dbListSpan)
	assert.Equal(t, listSpan.SpanContext().SpanID(), dbListSpan.Parent().SpanID(), "Storage.List must be nested under Resource.List")

	// 3. Test GET /api/v1/posts/{id}
	sr.Reset()
	getReq := httptest.NewRequest("GET", "/api/v1/posts/"+postID, nil)
	getRR := httptest.NewRecorder()
	r.mux.ServeHTTP(getRR, getReq)
	assert.Equal(t, http.StatusOK, getRR.Code)

	spans = sr.Ended()
	var getSpan, dbGetSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Resource.Get: Post":
			getSpan = s
		case "Storage.Get":
			dbGetSpan = s
		}
	}
	require.NotNil(t, getSpan)
	require.NotNil(t, dbGetSpan)
	assert.Equal(t, getSpan.SpanContext().SpanID(), dbGetSpan.Parent().SpanID(), "Storage.Get must be nested under Resource.Get")

	// 4. Test PATCH /api/v1/posts/{id}
	sr.Reset()
	patchReq := httptest.NewRequest("PATCH", "/api/v1/posts/"+postID, bytes.NewBufferString(`{"text": "Hello, Updated!"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRR := httptest.NewRecorder()
	r.mux.ServeHTTP(patchRR, patchReq)
	assert.Equal(t, http.StatusOK, patchRR.Code)

	spans = sr.Ended()
	var updateSpan, dbUpdateSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Resource.Update: Post":
			updateSpan = s
		case "Storage.Update":
			dbUpdateSpan = s
		}
	}
	require.NotNil(t, updateSpan)
	require.NotNil(t, dbUpdateSpan)
	assert.Equal(t, updateSpan.SpanContext().SpanID(), dbUpdateSpan.Parent().SpanID(), "Storage.Update must be nested under Resource.Update")

	// 5. Test DELETE /api/v1/posts/{id}
	sr.Reset()
	deleteReq := httptest.NewRequest("DELETE", "/api/v1/posts/"+postID, nil)
	deleteRR := httptest.NewRecorder()
	r.mux.ServeHTTP(deleteRR, deleteReq)
	assert.Equal(t, http.StatusOK, deleteRR.Code)

	spans = sr.Ended()
	var deleteSpan, dbDeleteSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.Name() {
		case "Resource.Delete: Post":
			deleteSpan = s
		case "Storage.Delete":
			dbDeleteSpan = s
		}
	}
	require.NotNil(t, deleteSpan)
	require.NotNil(t, dbDeleteSpan)
	assert.Equal(t, deleteSpan.SpanContext().SpanID(), dbDeleteSpan.Parent().SpanID(), "Storage.Delete must be nested under Resource.Delete")
}

func TestRouter_Tracing_Streams(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)

	var spec yaml.Node
	_ = yaml.Unmarshal([]byte(`
route:
  path: /api/v1/live
  auth: optional
channels:
  - updates
  - alerts
`), &spec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Streams: []*manifest.Manifest{
			{
				Kind:     "Stream",
				Metadata: manifest.Metadata{Name: "LiveStream"},
				Spec:     spec,
			},
		},
	}

	bus := events.NewMemoryBus()
	r := &Router{
		reg: reg,
		mux: http.NewServeMux(),
		bus: bus,
	}
	r.registerStreamRoutes()

	sr.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // instantly cancel to prevent streaming block
	req := httptest.NewRequest("GET", "/api/v1/live", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	r.mux.ServeHTTP(rr, req)

	spans := sr.Ended()
	var streamSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == "Stream: LiveStream" {
			streamSpan = s
			break
		}
	}

	require.NotNil(t, streamSpan)
	var streamName string
	var streamChannels []string
	for _, attr := range streamSpan.Attributes() {
		if attr.Key == "stream.name" {
			streamName = attr.Value.AsString()
		} else if attr.Key == "stream.channels" {
			streamChannels = attr.Value.AsStringSlice()
		}
	}
	assert.Equal(t, "LiveStream", streamName)
	assert.Equal(t, []string{"updates", "alerts"}, streamChannels)
}
