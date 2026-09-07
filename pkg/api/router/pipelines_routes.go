package router

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/hangry-coder/bffx/pkg/ai/chat"
	"github.com/hangry-coder/bffx/pkg/ai/orchestrator"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func (r *Router) registerChatbotPipelineRoute(m *manifest.Manifest, spec manifest.PipelineSpec, route string) {
	h := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
		sessionID, message := chatbotInputFromRequest(req)
		if sessionID == "" || message == "" {
			errors.WriteError(w, http.StatusBadRequest, "session_id and message are required", "INVALID_INPUT")
			return
		}

		sseWriter, err := chat.NewWriter(w)
		if err != nil {
			errors.WriteError(w, http.StatusInternalServerError, err.Error(), "STREAM_ERROR")
			return
		}

		maxHistory := 10
		if spec.Settings.MaxSlidingHistory > 0 {
			maxHistory = spec.Settings.MaxSlidingHistory
		}
		mem := orchestrator.NewMemoryManager(r.tagCache, maxHistory, 24*time.Hour)

		var activeVLM vlm.Provider = r.vlm
		if activeVLM == nil {
			activeVLM = vlm.NewNoopProvider()
		}
		engine := orchestrator.NewChatEngine([]vlm.Provider{activeVLM}, mem)

		hookData := map[string]any{
			"session_id": sessionID,
			"message":    message,
		}
		ac := r.newActionContext(req)
		ac.Context = req.Context()
		_ = r.executeHooks(spec.Hooks.BeforePipeline, ac, hookData)

		fullResponse, _, err := engine.ChatStream(req.Context(), sessionID, message, spec.Settings.SystemPrompt, func(token string) error {
			return chat.WriteToken(sseWriter, token, sessionID, activeVLM.Type())
		})
		if err != nil {
			chat.WriteError(sseWriter, err)
			return
		}

		hookData["response"] = fullResponse
		_ = r.executeHooks(spec.Hooks.AfterPipeline, ac, hookData)
		sseWriter.CloseDone()
	}, spec.Route.Auth)

	r.mux.HandleFunc(route, h.ServeHTTP)
	logger.Info("Registered Chatbot Pipeline route for %s at %s (SSE)", m.Metadata.Name, route)
}

func (r *Router) registerIngestionPipelineRoute(m *manifest.Manifest, spec manifest.PipelineSpec, route string) {
	cat := resolvePipelineCatalog(spec, r.nutrition)
	engine := orchestrator.NewIngestionEngine(r.vlm, r.tagCache, cat)

	h := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
		rawMedia, readErr := readPipelineMedia(req)
		if readErr != nil || len(rawMedia) == 0 {
			errors.Write(w, orchestrator.PipelineError{
				Code:      "INVALID_ASSET",
				Message:   "media body is empty or unreadable",
				Retryable: false,
			})
			return
		}

		quotaCheck := r.buildQuotaCheck(spec)
		hookExecutor := func(ctx context.Context, configs []manifest.HookConfig, data map[string]any) error {
			ac := r.newActionContext(req)
			ac.Context = ctx
			return r.executeHooks(configs, ac, data)
		}

		req = withCacheBypassFromHeader(req)
		if spec.Execution == "async" {
			userID := middleware.GetUserID(req.Context())
			jobInput := map[string]any{
				"pipeline": m.Metadata.Name,
				"media":    string(rawMedia),
			}
			job, err := r.jobStore.Enqueue(req.Context(), userID, "pipeline.ingestion", m.Metadata.Name, jobInput)
			if err != nil {
				errors.Write(w, err)
				return
			}
			if r.queue != nil {
				_ = r.queue.Push(req.Context(), job)
			}
			errors.WriteJSON(w, http.StatusAccepted, map[string]any{
				"job_id": job.ID,
				"status": job.Status,
			})
			return
		}

		resp, err := engine.Analyze(req.Context(), m, rawMedia, quotaCheck, hookExecutor)
		if err != nil {
			errors.Write(w, err)
			return
		}
		errors.WriteJSON(w, http.StatusOK, resp)
	}, spec.Route.Auth)

	r.mux.HandleFunc(route, h.ServeHTTP)
	logger.Info("Registered Pipeline route for %s at %s", m.Metadata.Name, route)
}

func (r *Router) registerPipelineRoutes() {
	for _, m := range r.reg.Pipelines {
		var spec manifest.PipelineSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			logger.Warn("failed to unmarshal pipeline spec for %s: %v", m.Metadata.Name, err)
			continue
		}

		if spec.DevOnly && os.Getenv("BFFX_ENV") == "production" {
			logger.Info("Skipping dev-only pipeline %s in production", m.Metadata.Name)
			continue
		}

		method, path := r.reg.GetManifestRoute(m)
		if method == "" || path == "" {
			logger.Info("Skipping HTTP route registration for pipeline %s (missing method/path)", m.Metadata.Name)
			continue
		}
		route := method + " " + path

		if spec.Type == "chatbot" {
			r.registerChatbotPipelineRoute(m, spec, route)
			continue
		}
		r.registerIngestionPipelineRoute(m, spec, route)
	}
}
