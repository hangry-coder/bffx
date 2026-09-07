package router

import (
	"context"
	"fmt"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
)

// actionRESTEnabled returns whether the REST handler for an Action should
// be registered. Precedence (highest to lowest):
//  1. spec.route.rest (explicit per-Action override)
//  2. BFFX_ACTIONS_REST_ENABLED env var (global kill-switch for prod)
//  3. default: true
func actionRESTEnabled(rest *bool) bool {
	if rest != nil {
		return *rest
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("BFFX_ACTIONS_REST_ENABLED"))); v != "" {
		switch v {
		case "false", "0", "no", "off":
			return false
		}
	}
	return true
}

type actionStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *actionStatusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *actionStatusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (r *Router) registerActionRoutes() {
	for _, m := range r.reg.Actions {
		var spec manifest.ActionSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			logger.Warn("failed to unmarshal action spec for %s: %v", m.Metadata.Name, err)
			continue
		}

		if spec.DevOnly && os.Getenv("BFFX_ENV") == "production" {
			logger.Info("Skipping dev-only action %s in production", m.Metadata.Name)
			continue
		}

		handler, ok := r.actionHandlers[m.Metadata.Name]
		if !ok {
			logger.Warn("no handler found for action %s", m.Metadata.Name)
			continue
		}

		method, path := r.reg.GetManifestRoute(m)

		if method == "" || path == "" {
			logger.Info("Skipping HTTP route registration for action %s (missing method/path)", m.Metadata.Name)
			continue
		}

		if !actionRESTEnabled(spec.Route.Rest) {
			logger.Info("Skipping REST registration for action %s (REST disabled)", m.Metadata.Name)
			continue
		}

		route := method + " " + path
		var h http.Handler = r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Action: "+m.Metadata.Name)
			defer span.End()
			req = req.WithContext(ctx)

			aw := &actionStatusWriter{ResponseWriter: w, status: 0}
			actCtx := r.newActionContext(req)
			handler(actCtx, aw, req)
			// GET actions that mutate should set route.invalidate_cache; mutating verbs are
			// handled by [middleware.InvalidateActionCacheOnMutation] on the mux.
			if spec.Route.InvalidateCache && r.tagCache != nil && aw.status >= 200 && aw.status < 300 {
				middleware.InvalidateActionCache(req.Context(), r.tagCache)
			}
		}, spec.Route.Auth, spec.Route.Roles...)

		if spec.Route.Cache != nil && r.tagCache != nil {
			h = http.HandlerFunc(r.withTagCache(spec.Route.Cache, h.ServeHTTP))
		} else if spec.Route.CacheTTL > 0 && r.tagCache != nil {
			h = middleware.Cache(r.tagCache, middleware.CacheOptions{
				TTL:            time.Duration(spec.Route.CacheTTL) * time.Second,
				VaryHeaders:    []string{"Accept-Language", "X-App-Version", "X-App-Secret"},
				AllowAnonymous: spec.Route.Auth != "required",
			})(h)
		}

		r.mux.HandleFunc(route, h.ServeHTTP)
		logger.Info("Registered Action route for %s at %s", m.Metadata.Name, route)
	}
}

func (r *Router) wrapWithAuth(handler http.HandlerFunc, mode string, requiredRoles ...string) http.HandlerFunc {
	allowedRoles := make(map[string]struct{}, len(requiredRoles))
	for _, role := range requiredRoles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role != "" {
			allowedRoles[role] = struct{}{}
		}
	}
	return func(w http.ResponseWriter, req *http.Request) {
		claims := middleware.GetClaims(req.Context())
		if mode == "required" {
			if claims == nil {
				errors.Write(w, errors.ErrUnauthorized)
				return
			}
		}
		if len(allowedRoles) > 0 {
			if claims == nil {
				errors.Write(w, errors.ErrUnauthorized)
				return
			}
			role, _ := claims["role"].(string)
			role = strings.ToLower(strings.TrimSpace(role))
			if _, ok := allowedRoles[role]; !ok {
				errors.WriteError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
		handler(w, req)
	}
}

// ExecuteAction executes a registered action handler directly.
// This is used for programmatically running actions, e.g. scheduled cron jobs.
func (r *Router) ExecuteAction(ctx context.Context, actionName string, w http.ResponseWriter, req *http.Request) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteAction: "+actionName)
	defer span.End()

	handler, ok := r.actionHandlers[actionName]
	if !ok {
		return fmt.Errorf("action handler %q not found", actionName)
	}

	if req == nil {
		var err error
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, "/_cron/"+actionName, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
	} else {
		req = req.WithContext(ctx)
	}

	if w == nil {
		w = &dummyWriter{}
	}

	aw := &actionStatusWriter{ResponseWriter: w, status: 0}
	actCtx := r.newActionContext(req)
	handler(actCtx, aw, req)
	return nil
}

type dummyWriter struct {
	header http.Header
}

func (d *dummyWriter) Header() http.Header {
	if d.header == nil {
		d.header = make(http.Header)
	}
	return d.header
}

func (d *dummyWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (d *dummyWriter) WriteHeader(statusCode int) {}
