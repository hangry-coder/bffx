package router

import (
	"context"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func (r *Router) newActionContext(req *http.Request) *handlers.ActionContext {
	userID := middleware.GetUserID(req.Context())
	var user map[string]any
	if userID != "" {
		user, _ = r.store.Get(req.Context(), "User", userID)
	}

	return &handlers.ActionContext{
		Store:        r.store,
		Telemetry:    r.telemetry,
		Auth:         r.jwt,
		JobStore:     r.jobStore,
		Queue:        r.queue,
		Notify:       r.notify,
		Comm:         r.comm,
		Analytics:    r.analytics,
		Bundle:       r.i18n,
		Localizer:    r.localizer,
		Vlm:          r.vlm,
		Nutrition:    r.nutrition,
		OTP:          r.otp,
		FeatureFlags: r.featureFlags,
		Ads:          r.ads,
		User:         user,
		Claims:       middleware.GetClaims(req.Context()),
		Context:      req.Context(),
		Request:      req,
	}
}

func (r *Router) executeHooks(configs []manifest.HookConfig, ctx *handlers.ActionContext, data map[string]any) error {
	for _, cfg := range configs {
		hookName := cfg.Action
		hook, ok := r.hooks[hookName]
		if !ok {
			logger.Warn("Hook %s not found, skipping", hookName)
			continue
		}

		if cfg.Mode == "async" {
			asyncCtx := context.WithoutCancel(ctx.Context)
			asyncAC := *ctx
			asyncAC.Context = asyncCtx
			go func(h HookFunc, c *handlers.ActionContext, d map[string]any, name string) {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Async hook %s panicked: %v\n%s", name, r, logger.Stack())
					}
				}()
				if err := h(c, d); err != nil {
					logger.Warn("Async hook %s failed: %v", name, err)
				}
			}(hook, &asyncAC, data, hookName)
		} else {
			if err := hook(ctx, data); err != nil {
				return err
			}
		}
	}
	return nil
}
