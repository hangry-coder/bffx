package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	addcat "github.com/hangry-coder/bffx/pkg/addons/catalog/nutrition"
	"github.com/hangry-coder/bffx/pkg/ai/orchestrator"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

type legacyNutritionCatalog struct {
	provider nutrition.Provider
}

func (l *legacyNutritionCatalog) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if l.provider == nil {
		return nil, fmt.Errorf("no legacy nutrition provider configured")
	}
	return l.provider.FetchNutrition(ctx, query)
}

func (l *legacyNutritionCatalog) Source() string {
	if l.provider != nil {
		return l.provider.Type()
	}
	return "legacy_nutrition"
}

func chatbotInputFromRequest(req *http.Request) (string, string) {
	var sessionID string
	var message string

	if req.Method == http.MethodPost && req.Body != nil {
		var body struct {
			SessionID string `json:"session_id"`
			Message   string `json:"message"`
		}
		bodyData, readErr := io.ReadAll(req.Body)
		if readErr == nil && len(bodyData) > 0 {
			_ = json.Unmarshal(bodyData, &body)
			sessionID = body.SessionID
			message = body.Message
		}
	}

	if sessionID == "" {
		sessionID = req.URL.Query().Get("session_id")
	}
	if message == "" {
		message = req.URL.Query().Get("message")
	}
	return sessionID, message
}

func resolvePipelineCatalog(spec manifest.PipelineSpec, nutritionProvider nutrition.Provider) orchestrator.Catalog {
	if spec.Catalog == nil || spec.Catalog.Adapter != "openfoodfacts" {
		return nil
	}
	if nutritionProvider != nil && nutritionProvider.Type() != "noop" {
		return &legacyNutritionCatalog{provider: nutritionProvider}
	}
	return addcat.NewAddonCatalog()
}

func readPipelineMedia(req *http.Request) ([]byte, error) {
	var rawMedia []byte
	var readErr error

	if req.Method == http.MethodPost && strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data") {
		_ = req.ParseMultipartForm(10 << 20)
		file, _, err := req.FormFile("file")
		if err != nil {
			file, _, err = req.FormFile("image")
		}
		if err == nil && file != nil {
			defer file.Close()
			var buf bytes.Buffer
			_, readErr = buf.ReadFrom(file)
			rawMedia = buf.Bytes()
		}
	}

	if len(rawMedia) == 0 {
		rawMedia, readErr = io.ReadAll(req.Body)
	}
	return rawMedia, readErr
}

func withCacheBypassFromHeader(req *http.Request) *http.Request {
	bypassHeader := req.Header.Get("X-Cache-Bypass")
	if strings.ToLower(bypassHeader) == "true" || bypassHeader == "1" {
		return req.WithContext(context.WithValue(req.Context(), "X-Cache-Bypass", true))
	}
	return req
}

func (r *Router) buildQuotaCheck(spec manifest.PipelineSpec) func(context.Context) error {
	if spec.QuotaProfile == "" || r.billing == nil {
		return nil
	}
	return func(ctx context.Context) error {
		userID := middleware.GetUserID(ctx)
		if userID == "" {
			return orchestrator.PipelineError{
				Code:      "UNAUTHORIZED",
				Message:   "unauthorized: quota profile requires a logged in user",
				Retryable: false,
			}
		}
		activeEnts, err := r.billing.GetActiveEntitlements(ctx, userID)
		if err != nil {
			return orchestrator.PipelineError{
				Code:      "UPSTREAM_TEMPORARY_FAILURE",
				Message:   fmt.Sprintf("failed to fetch user entitlements: %v", err),
				Retryable: true,
			}
		}

		for _, ent := range activeEnts {
			if ent == spec.QuotaProfile {
				return nil
			}
		}
		return orchestrator.PipelineError{
			Code:      "QUOTA_EXCEEDED",
			Message:   fmt.Sprintf("quota profile %q not met", spec.QuotaProfile),
			Retryable: false,
		}
	}
}
