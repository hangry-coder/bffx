package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// FakeVLM mocks the vlm.Provider interface.
type FakeVLM struct {
	TypeStr   string
	MockFunc  func(ctx context.Context, image []byte, prompt string) (string, error)
	CallCount int
}

func (f *FakeVLM) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	f.CallCount++
	if f.MockFunc != nil {
		return f.MockFunc(ctx, image, prompt)
	}
	return `{"name": "Apples", "barcode": "12345"}`, nil
}

func (f *FakeVLM) Type() string {
	if f.TypeStr != "" {
		return f.TypeStr
	}
	return "mock_vlm"
}

// FakeCache mocks the cache.Provider interface.
type FakeCache struct {
	Store map[string][]byte
}

func NewFakeCache() *FakeCache {
	return &FakeCache{Store: make(map[string][]byte)}
}

func (f *FakeCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, ok := f.Store[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return val, nil
}

func (f *FakeCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	f.Store[key] = value
	return nil
}

func (f *FakeCache) Delete(ctx context.Context, key string) error {
	delete(f.Store, key)
	return nil
}

func (f *FakeCache) Flush(ctx context.Context) error {
	f.Store = make(map[string][]byte)
	return nil
}

func (f *FakeCache) Tags(tags ...string) cache.Tagger { return nil }

func (f *FakeCache) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	return nil
}

func (f *FakeCache) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	return 0, nil
}

func (f *FakeCache) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	return 0, nil
}

func (f *FakeCache) Type() string { return "mock_cache" }

func (f *FakeCache) Ping(ctx context.Context) error { return nil }

// FakeCatalog mocks the Catalog interface.
type FakeCatalog struct {
	MockResolve func(ctx context.Context, query string, hints map[string]string) (any, error)
}

func (f *FakeCatalog) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if f.MockResolve != nil {
		return f.MockResolve(ctx, query, hints)
	}
	return map[string]any{"resolved_name": "Hydrated Organic Apples", "query": query}, nil
}

func (f *FakeCatalog) Source() string { return "fake_catalog" }

func TestIngestionEngine_Optimize(t *testing.T) {
	// Create a simple 10x10 red PNG image
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err)

	optimizer := NewOptimizer()
	asset, err := optimizer.Optimize(context.Background(), buf.Bytes(), 5)
	require.NoError(t, err)
	assert.Equal(t, 5, asset.Width)
	assert.Equal(t, 5, asset.Height)
	assert.NotEmpty(t, asset.SHA256)
}

func TestIngestionEngine_Analyze_CacheHit(t *testing.T) {
	fakeVlm := &FakeVLM{}
	fakeCache := NewFakeCache()
	fakeCatalog := &FakeCatalog{}

	engine := NewIngestionEngine(fakeVlm, fakeCache, fakeCatalog)

	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: CachePipeline
spec:
  type: ingestion
  model_routing:
    - gemini
  route:
    method: POST
    path: /api/v1/meals/scan
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	// Pre-populate cache with a mock response
	rawMedia := []byte("some raw image data")
	optimizer := NewOptimizer()
	asset, _ := optimizer.Optimize(context.Background(), rawMedia, 0)
	cacheKey := "bffx:pipeline:media:" + asset.SHA256

	cachedResponse := IngestionResponse{
		Status:        "ok",
		Pipeline:      "CachePipeline",
		ProviderUsed:  "mock_vlm",
		CatalogSource: "fake_catalog",
		Data:          map[string]any{"resolved_name": "Hydrated Organic Apples"},
	}
	cachedBytes, _ := json.Marshal(cachedResponse)
	fakeCache.Store[cacheKey] = cachedBytes

	resp, err := engine.Analyze(context.Background(), m, rawMedia, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "cache_hit", resp.Status)
	assert.Equal(t, "CachePipeline", resp.Pipeline)
	assert.Equal(t, 0, fakeVlm.CallCount) // VLM not called on cache hit!
}

func TestIngestionEngine_Analyze_CacheMiss_VLMSuccess(t *testing.T) {
	fakeVlm := &FakeVLM{
		MockFunc: func(ctx context.Context, image []byte, prompt string) (string, error) {
			return `{"name": "Organic Apples", "barcode": "9999"}`, nil
		},
	}
	fakeCache := NewFakeCache()
	fakeCatalog := &FakeCatalog{
		MockResolve: func(ctx context.Context, query string, hints map[string]string) (any, error) {
			return map[string]any{"catalog_name": "Resolved " + query}, nil
		},
	}

	engine := NewIngestionEngine(fakeVlm, fakeCache, fakeCatalog)

	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: SuccessPipeline
spec:
  type: ingestion
  model_routing:
    - gemini
  route:
    method: POST
    path: /api/v1/meals/scan
  catalog:
    adapter: openfoodfacts
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	rawMedia := []byte("new raw image data")
	resp, err := engine.Analyze(context.Background(), m, rawMedia, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "mock_vlm", resp.ProviderUsed)
	assert.Equal(t, "fake_catalog", resp.CatalogSource)
	
	dataMap, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Resolved 9999", dataMap["catalog_name"])

	// Verify it was saved to cache
	optimizer := NewOptimizer()
	asset, _ := optimizer.Optimize(context.Background(), rawMedia, 0)
	cacheKey := "bffx:pipeline:media:" + asset.SHA256
	assert.Contains(t, fakeCache.Store, cacheKey)
}

func TestIngestionEngine_Analyze_ModelFallbackLoop(t *testing.T) {
	attempts := 0
	fakeVlm := &FakeVLM{
		MockFunc: func(ctx context.Context, image []byte, prompt string) (string, error) {
			attempts++
			if attempts == 1 {
				return "", errors.New("model timeout")
			}
			return `{"name": "Success"}`, nil
		},
	}

	engine := NewIngestionEngine(fakeVlm, nil, nil)

	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: FallbackPipeline
spec:
  type: ingestion
  model_routing:
    - gemini-pro
    - gemini-flash
  route:
    method: POST
    path: /api/v1/meals/scan
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	resp, err := engine.Analyze(context.Background(), m, []byte("media"), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, 2, fakeVlm.CallCount) // Gemini-pro failed, Gemini-flash succeeded!
}

func TestIngestionEngine_Analyze_QuotaFailure(t *testing.T) {
	engine := NewIngestionEngine(&FakeVLM{}, nil, nil)

	pipelineYaml := `
apiVersion: bffx.io/v1alpha1
kind: Pipeline
metadata:
  name: QuotaPipeline
spec:
  type: ingestion
  model_routing:
    - gemini
  route:
    method: POST
    path: /api/v1/meals/scan
`
	m := &manifest.Manifest{Kind: "Pipeline"}
	err := yaml.Unmarshal([]byte(pipelineYaml), &m)
	require.NoError(t, err)

	quotaCheck := func(ctx context.Context) error {
		return &PipelineError{
			Code:      "QUOTA_EXCEEDED",
			Message:   "usage limit exceeded",
			Retryable: false,
		}
	}

	_, err = engine.Analyze(context.Background(), m, []byte("media"), quotaCheck, nil)
	assert.Error(t, err)
	
	pErr, ok := err.(*PipelineError)
	require.True(t, ok)
	assert.Equal(t, "QUOTA_EXCEEDED", pErr.Code)
}
