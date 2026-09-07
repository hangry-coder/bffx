package router

import (
	"net/http"
	"os"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"

	"go.opentelemetry.io/otel/attribute"
)

func (r *Router) registerBuilderRoutes() {
	for _, m := range r.reg.Builders {
		var spec manifest.BuilderSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			logger.Warn("failed to unmarshal builder spec for %s: %v", m.Metadata.Name, err)
			continue
		}

		if spec.DevOnly && os.Getenv("BFFX_ENV") == "production" {
			logger.Info("Skipping dev-only builder %s in production", m.Metadata.Name)
			continue
		}

		method, path := r.reg.GetManifestRoute(m)
		if path == "" {
			logger.Warn("skipping builder %s: no explicit or implicit path found", m.Metadata.Name)
			continue
		}
		route := method + " " + path
		mLocal := m // Capture for closure
		handler := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Builder: "+mLocal.Metadata.Name)
			defer span.End()
			span.SetAttributes(attribute.String("builder.name", mLocal.Metadata.Name))
			req = req.WithContext(ctx)

			var spec manifest.BuilderSpec
			mLocal.UnmarshalSpec(&spec)

			sourceData, errs := r.resolveSources(req, spec.Sources, spec.PartialSuccess)
			if len(errs) > 0 && !spec.PartialSuccess {
				span.RecordError(errs[0])
				errors.Write(w, errs[0])
				return
			}

			output := r.resolveOutput(ctx, sourceData, spec.Output)

			if len(errs) > 0 && spec.PartialSuccess {
				for _, err := range errs {
					span.RecordError(err)
				}
				errors.WritePartial(w, output, errs)
				return
			}

			writeJSONWithETag(w, req, http.StatusOK, output)
		}, spec.Route.Auth, spec.Route.Roles...)

		if spec.Cache != nil {
			handler = r.withTagCache(spec.Cache, handler)
		}

		r.mux.HandleFunc(route, handler)
		logger.Info("Registered Builder route for %s at %s", m.Metadata.Name, route)
	}
}
