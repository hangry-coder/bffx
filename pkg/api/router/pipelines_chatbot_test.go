package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hangry-coder/bffx/pkg/ai/chat"
	"github.com/hangry-coder/bffx/pkg/ai/orchestrator"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries/cache"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
)

type mockVLMForRouter struct {
	response string
}

func (m *mockVLMForRouter) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	return m.response, nil
}

func (m *mockVLMForRouter) Type() string {
	return "mock_vlm_router"
}

func TestChatbotPipeline_SSEStreaming(t *testing.T) {
	s := storage.NewMemoryStore()
	bus := events.NewMemoryBus()
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(s, nil)
	jwt := auth.NewJWTService("test-secret")

	// Construct chatbot pipeline manifest
	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: Coach
spec:
  type: chatbot
  route:
    method: POST
    path: /api/v1/coach
    auth: optional
  settings:
    max_sliding_history: 5
    system_prompt: "Be a gym coach"
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
		},
		Pipelines: []*manifest.Manifest{m},
	}

	mockCache := cache.NewMemoryProvider()
	mockModel := &mockVLMForRouter{response: "Let's do 10 pushups!"}

	r := NewRouter(RouterConfig{
		Store:        s,
		Telemetry:    s,
		JobStore:     worker.NewMemoryJobStore(),
		Registry:     reg,
		AuthProvider: jwt,
		JWTService:   jwt,
		EventBus:     bus,
		I18n:         bundle,
		FlagProvider: flagProvider,
		TagCache:     mockCache,
		VlmProvider:  mockModel,
	})

	handler := r.Setup()

	// 1. Send SSE request using POST JSON body
	reqBody := `{"session_id": "test-session", "message": "Give me a workout plan"}`
	req := httptest.NewRequest("POST", "/api/v1/coach", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))

	// 2. Parse chunk lines
	bodyStr := rr.Body.String()
	t.Logf("Raw SSE Response: %q", bodyStr)
	lines := strings.Split(bodyStr, "\n")

	var tokens []string
	hasDoneEvent := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataPayload := strings.TrimPrefix(line, "data:")
			dataPayload = strings.TrimSpace(dataPayload)

			if dataPayload == "{}" {
				continue
			}

			var chunk chat.TokenChunk
			if err := json.Unmarshal([]byte(dataPayload), &chunk); err == nil && chunk.Token != "" {
				tokens = append(tokens, chunk.Token)
				assert.Equal(t, "test-session", chunk.SessionID)
				assert.Equal(t, "mock_vlm_router", chunk.ProviderUsed)
			}
		}
		if strings.HasPrefix(line, "event: done") {
			hasDoneEvent = true
		}
	}

	assert.True(t, hasDoneEvent, "SSE stream must close with 'event: done'")
	rebuilt := strings.Join(tokens, "")
	assert.Equal(t, "Let's do 10 pushups!", rebuilt)

	// 3. Verify conversation session is stored in history memory
	mem := orchestrator.NewMemoryManager(mockCache, 5, 1*time.Hour)
	history, err := mem.GetHistory(context.Background(), "test-session")
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "Give me a workout plan", history[0].Content)
	assert.Equal(t, "model", history[1].Role)
	assert.Equal(t, "Let's do 10 pushups!", history[1].Content)
}
