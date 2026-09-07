package router

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"net/http"
	"os"

	"go.opentelemetry.io/otel/attribute"
)

func (r *Router) registerStreamRoutes() {
	h := handlers.NewStreamHandler(r.bus)
	for _, m := range r.reg.Streams {
		var spec manifest.StreamSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			logger.Warn("failed to unmarshal stream spec for %s: %v", m.Metadata.Name, err)
			continue
		}

		if spec.DevOnly && os.Getenv("BFFX_ENV") == "production" {
			logger.Info("Skipping dev-only stream %s in production", m.Metadata.Name)
			continue
		}

		route := "GET " + spec.Route.Path
		authMode := spec.Route.Auth
		mLocal := m // shadow for closure

		r.mux.HandleFunc(route, func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Stream: "+mLocal.Metadata.Name)
			defer span.End()
			span.SetAttributes(
				attribute.String("stream.name", mLocal.Metadata.Name),
				attribute.StringSlice("stream.channels", spec.Channels),
			)
			req = req.WithContext(ctx)

			if authMode == "required" {
				userID := middleware.GetUserID(req.Context())
				if userID == "" {
					errors.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
					return
				}
			}
			h.HandleStream(spec.Channels)(w, req)
		})
		logger.Info("Registered Stream route for %s at %s", m.Metadata.Name, route)
	}

	// Register universal real-time event stream route
	realtimeRoute := "GET " + r.reg.ApiPrefix + "/realtime"
	r.mux.HandleFunc(realtimeRoute, func(w http.ResponseWriter, req *http.Request) {
		ctx, span := observability.StartSpan(req.Context(), "Realtime: universal")
		defer span.End()
		req = req.WithContext(ctx)
		h.HandleUniversalRealtime()(w, req)
	})
	logger.Info("Registered Realtime universal event stream at %s", realtimeRoute)
}
