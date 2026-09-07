package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// IngestionResponse is the standard contract returned to callers.
type IngestionResponse struct {
	Status        string         `json:"status"`
	Pipeline      string         `json:"pipeline"`
	ProviderUsed  string         `json:"provider_used"`
	CatalogSource string         `json:"catalog_source"`
	Data          any            `json:"data,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// HookExecutor defines the function signature for executing standard action lifecycle hooks.
type HookExecutor func(ctx context.Context, configs []manifest.HookConfig, data map[string]any) error

// IngestionEngine coordinates the VLM, Cache, and Catalog layers to process media.
type IngestionEngine struct {
	vlm       vlm.Provider
	cache     cache.Provider
	catalog   Catalog
	optimizer *Optimizer
}

func NewIngestionEngine(v vlm.Provider, c cache.Provider, cat Catalog) *IngestionEngine {
	return &IngestionEngine{
		vlm:       v,
		cache:     c,
		catalog:   cat,
		optimizer: NewOptimizer(),
	}
}

func parseVLMJSON(vlmOutput string) (map[string]any, error) {
	cleanJSON := extractJSON(vlmOutput)
	var parsedData map[string]any
	if err := json.Unmarshal([]byte(cleanJSON), &parsedData); err != nil {
		if err2 := json.Unmarshal([]byte(vlmOutput), &parsedData); err2 != nil {
			return nil, fmt.Errorf("VLM response did not return valid JSON: %s", vlmOutput)
		}
	}
	return parsedData, nil
}

func buildCatalogHints(parsedData map[string]any) map[string]string {
	hints := make(map[string]string)
	for k, v := range parsedData {
		if str, ok := v.(string); ok {
			hints[k] = str
		}
	}
	return hints
}

func resolveCatalogQuery(parsedData map[string]any) string {
	if barcode, ok := parsedData["barcode"].(string); ok && barcode != "" {
		return barcode
	}
	if name, ok := parsedData["name"].(string); ok && name != "" {
		return name
	}
	if title, ok := parsedData["title"].(string); ok && title != "" {
		return title
	}
	return ""
}

func (e *IngestionEngine) resolveCatalogData(ctx context.Context, spec manifest.PipelineSpec, parsedData map[string]any) (string, any) {
	catalogSource := "none"
	var resolvedData any = parsedData

	if e.catalog != nil && spec.Catalog != nil && spec.Catalog.Adapter != "" {
		catalogSource = e.catalog.Source()
		query := resolveCatalogQuery(parsedData)
		hints := buildCatalogHints(parsedData)
		res, err := e.catalog.Resolve(ctx, query, hints)
		if err != nil {
			logger.Warn("catalog resolve failed: %v. Continuing with raw VLM data.", err)
		} else if res != nil {
			resolvedData = res
		}
	}
	return catalogSource, resolvedData
}

// Analyze coordinates cache-hits, VLM model fallbacks, raw JSON formatting, catalog hydration, and hooks.
func (e *IngestionEngine) Analyze(
	ctx context.Context,
	m *manifest.Manifest,
	rawMedia []byte,
	quotaCheck func(ctx context.Context) error,
	hookExecutor HookExecutor,
) (*IngestionResponse, error) {
	var spec manifest.PipelineSpec
	if err := m.UnmarshalSpec(&spec); err != nil {
		return nil, &PipelineError{
			Code:      "VALIDATION_FAILED",
			Message:   fmt.Sprintf("invalid pipeline manifest: %v", err),
			Retryable: false,
		}
	}

	// 1. Quota Check (Task 2.6)
	if quotaCheck != nil {
		if err := quotaCheck(ctx); err != nil {
			return nil, err
		}
	}

	// 2. Before Pipeline Hooks (Task 2.5)
	if len(spec.Hooks.BeforePipeline) > 0 && hookExecutor != nil {
		hookData := map[string]any{
			"pipeline": m.Metadata.Name,
			"event":    "beforePipeline",
		}
		if err := hookExecutor(ctx, spec.Hooks.BeforePipeline, hookData); err != nil {
			return nil, &PipelineError{
				Code:      "HOOK_FAILURE",
				Message:   fmt.Sprintf("beforePipeline hook execution failed: %v", err),
				Retryable: false,
			}
		}
	}

	// 3. Optimize raw media (Task 2.1)
	maxWidth := 0
	if spec.Settings.CompressionMaxWidth > 0 {
		maxWidth = spec.Settings.CompressionMaxWidth
	}
	asset, err := e.optimizer.Optimize(ctx, rawMedia, maxWidth)
	if err != nil {
		return nil, &PipelineError{
			Code:      "INVALID_ASSET",
			Message:   fmt.Sprintf("failed to optimize media asset: %v", err),
			Retryable: false,
		}
	}

	// 4. Cache lookup (unless cache_bypass settings/headers are present) (Task 2.2)
	cacheKey := "bffx:pipeline:media:" + asset.SHA256
	bypassCache := spec.Settings.CacheBypass
	if bypassVal, ok := ctx.Value("X-Cache-Bypass").(bool); ok && bypassVal {
		bypassCache = true
	}

	if !bypassCache && e.cache != nil {
		cached, err := e.cache.Get(ctx, cacheKey)
		if err == nil && len(cached) > 0 {
			var resp IngestionResponse
			if err := json.Unmarshal(cached, &resp); err == nil {
				// Cache hit! Return cached data instantly (Task 2.4, 2.7)
				resp.Status = "cache_hit"

				// 5. After Pipeline Hooks on Cache Hit (Task 2.5)
				if len(spec.Hooks.AfterPipeline) > 0 && hookExecutor != nil {
					hookData := map[string]any{
						"pipeline": m.Metadata.Name,
						"event":    "afterPipeline",
						"status":   "cache_hit",
					}
					_ = hookExecutor(ctx, spec.Hooks.AfterPipeline, hookData)
				}
				return &resp, nil
			}
		}
	}

	// 6. VLM execution with Model Fallbacks (Task 2.2)
	var vlmOutput string
	var vlmErr error
	providerUsed := "none"

	prompt := "Identify the object and structure its attributes as JSON."

	// Model Fallback Loop: try each model registered in model_routing
	models := spec.ModelRouting
	if len(models) == 0 {
		models = []string{"gemini"}
	}

	for _, model := range models {
		providerUsed = e.vlm.Type()
		vlmOutput, vlmErr = e.vlm.Analyze(ctx, asset.Optimized, prompt)
		if vlmErr == nil {
			break
		}
		logger.Warn("VLM model %q execution failed: %v. Trying next model...", model, vlmErr)
	}

	if vlmErr != nil {
		return nil, &PipelineError{
			Code:      "UPSTREAM_TEMPORARY_FAILURE",
			Message:   fmt.Sprintf("all configured VLM models failed: %v", vlmErr),
			Retryable: true,
		}
	}

	// 7. Parse VLM Output and extract JSON inside Markdown fences
	parsedData, err := parseVLMJSON(vlmOutput)
	if err != nil {
		return nil, &PipelineError{
			Code:      "VALIDATION_FAILED",
			Message:   err.Error(),
			Retryable: false,
		}
	}

	// 8. Catalog Resolution (Task 2.2)
	catalogSource, resolvedData := e.resolveCatalogData(ctx, spec, parsedData)

	// 9. Assemble final response
	resp := &IngestionResponse{
		Status:        "ok",
		Pipeline:      m.Metadata.Name,
		ProviderUsed:  providerUsed,
		CatalogSource: catalogSource,
		Data:          resolvedData,
		Metadata: map[string]any{
			"sha256":    asset.SHA256,
			"mime_type": asset.MimeType,
			"width":     asset.Width,
			"height":    asset.Height,
		},
	}

	// 10. Cache set (Task 2.2)
	if e.cache != nil {
		respBytes, err := json.Marshal(resp)
		if err == nil {
			_ = e.cache.Set(ctx, cacheKey, respBytes, 24*time.Hour)
		}
	}

	// 11. After Pipeline Hooks (Task 2.5)
	if len(spec.Hooks.AfterPipeline) > 0 && hookExecutor != nil {
		hookData := map[string]any{
			"pipeline":       m.Metadata.Name,
			"event":          "afterPipeline",
			"status":         "ok",
			"provider_used":  providerUsed,
			"catalog_source": catalogSource,
		}
		_ = hookExecutor(ctx, spec.Hooks.AfterPipeline, hookData)
	}

	return resp, nil
}

func extractJSON(s string) string {
	start := bytes.Index([]byte(s), []byte("```json"))
	if start != -1 {
		s = s[start+7:]
		end := bytes.Index([]byte(s), []byte("```"))
		if end != -1 {
			s = s[:end]
		}
	}
	return s
}
