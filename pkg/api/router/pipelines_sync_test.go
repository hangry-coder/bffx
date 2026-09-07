package router

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries/cache"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestSyncIngestionPipeline_NoNutritionBattery(t *testing.T) {
	s := storage.NewMemoryStore()
	bus := events.NewMemoryBus()
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(s, nil)
	jwt := auth.NewJWTService("test-secret")

	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: MealVision
spec:
  type: ingestion
  execution: sync
  route:
    method: POST
    path: /api/v1/meals/scan
    auth: optional
  model_routing:
    - noop
  catalog:
    adapter: openfoodfacts
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	require.NoError(t, yaml.Unmarshal([]byte(pipelineYaml), &m))

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "PipelinesCalorie"},
		},
		Pipelines: []*manifest.Manifest{m},
	}

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	r := NewRouter(RouterConfig{
		Store:        s,
		Telemetry:    s,
		Registry:     reg,
		AuthProvider: jwt,
		JWTService:   jwt,
		EventBus:     bus,
		I18n:         bundle,
		FlagProvider: flagProvider,
		VlmProvider:  &mockVLMForRouter{response: `{"name": "Apple", "barcode": "12345"}`},
		TagCache:     cache.NewMemoryProvider(),
	})

	handler := r.Setup()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/meals/scan", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "image/png")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Status        string `json:"status"`
		ProviderUsed  string `json:"provider_used"`
		CatalogSource string `json:"catalog_source"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "mock_vlm_router", resp.ProviderUsed)
	assert.Equal(t, "openfoodfacts", resp.CatalogSource)
}
