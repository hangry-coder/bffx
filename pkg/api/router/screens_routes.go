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

func (r *Router) registerScreenRoutes() {
	for _, m := range r.reg.Screens {
		var spec manifest.ScreenSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			logger.Warn("failed to unmarshal screen spec for %s: %v", m.Metadata.Name, err)
			continue
		}

		if spec.DevOnly && os.Getenv("BFFX_ENV") == "production" {
			logger.Info("Skipping dev-only screen %s in production", m.Metadata.Name)
			continue
		}

		// REST opt-out: if the manifest declares `route.rest: false` the
		// Screen is gRPC-only. Skip route registration but keep the proto
		// service generated (handled by proto/builder.go).
		if spec.Route.Rest != nil && !*spec.Route.Rest {
			logger.Info("Skipping REST registration for screen %s (route.rest=false)", m.Metadata.Name)
			continue
		}

		method, path := r.reg.GetManifestRoute(m)

		if path == "" {
			logger.Warn("skipping screen %s: no explicit or implicit path found", m.Metadata.Name)
			continue
		}

		route := method + " " + path

		mLocal := m // Capture for closure
		handler := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Screen: "+mLocal.Metadata.Name)
			defer span.End()
			span.SetAttributes(attribute.String("screen.name", mLocal.Metadata.Name))
			req = req.WithContext(ctx)

			// Kill-switch evaluation: a screen-level switch supersedes feature
			// flags and the manifest default. We answer 503 with a JSON envelope
			// so the client can show the reason banner.
			if r.killSwitch != nil {
				if ks, err := r.killSwitch.Get(ctx, mLocal.Metadata.Name, ""); err == nil && ks != nil && ks.IsActiveNow() {
					writeKilledResponse(w, ks)
					return
				}
			}

			var spec manifest.ScreenSpec
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
		}, spec.Route.Auth)

		if spec.Cache != nil {
			handler = r.withTagCache(spec.Cache, handler, ScreenCacheTag(mLocal.Metadata.Name))
		}

		r.mux.HandleFunc(route, handler)
		logger.Info("Registered Screen route for %s at %s", m.Metadata.Name, route)

		// Streaming Screens v1 (Phase 7): polling shim with hash diffing.
		// When the manifest declares `spec.stream: true`, expose an SSE endpoint
		// at `<path>/stream` that emits a new JSON payload only when the
		// rendered screen hash changes. This unblocks live mobile screens
		// (chat lists, live game state, etc.) without requiring an eventbus.
		if spec.Stream {
			streamRoute := "GET " + path + "/stream"
			streamHandler := r.wrapWithAuth(r.streamScreenHandler(mLocal), spec.Route.Auth)
			r.mux.HandleFunc(streamRoute, streamHandler)
			logger.Info("Registered Screen stream route for %s at %s", m.Metadata.Name, streamRoute)
		}

		// Register Section routes if they have a Path
		for _, sec := range spec.Sections {
			if sec.Route != nil && sec.Route.Path != "" {
				secRoute := "GET " + sec.Route.Path
				secLocal := sec // Capture for closure

				secHandler := r.wrapWithAuth(func(w http.ResponseWriter, req *http.Request) {
					ctx, span := observability.StartSpan(req.Context(), "ScreenSection: "+mLocal.Metadata.Name+":"+secLocal.Key)
					defer span.End()
					span.SetAttributes(
						attribute.String("screen.name", mLocal.Metadata.Name),
						attribute.String("section.key", secLocal.Key),
					)
					req = req.WithContext(ctx)

					// Section route honours both whole-screen and section-level
					// kill switches. Section-level takes precedence on the
					// section route; on miss, the screen-level switch applies.
					if r.killSwitch != nil {
						if ks, err := r.killSwitch.Get(ctx, mLocal.Metadata.Name, secLocal.Key); err == nil && ks != nil && ks.IsActiveNow() {
							writeKilledResponse(w, ks)
							return
						}
						if ks, err := r.killSwitch.Get(ctx, mLocal.Metadata.Name, ""); err == nil && ks != nil && ks.IsActiveNow() {
							writeKilledResponse(w, ks)
							return
						}
					}

					var spec manifest.ScreenSpec
					mLocal.UnmarshalSpec(&spec)
					sourceData, errs := r.resolveSources(req, spec.Sources, spec.PartialSuccess)
					if len(errs) > 0 && !spec.PartialSuccess {
						span.RecordError(errs[0])
						errors.Write(w, errs[0])
						return
					}

					output := r.resolveOutput(ctx, sourceData, spec.Output)

					var finalData any = output
					// If we can find the section in the output, return just that
					if val, ok := output[secLocal.Key]; ok {
						finalData = val
					}

					if len(errs) > 0 && spec.PartialSuccess {
						for _, err := range errs {
							span.RecordError(err)
						}
						errors.WritePartial(w, finalData, errs)
						return
					}

					writeJSONWithETag(w, req, http.StatusOK, finalData)
				}, sec.Route.Auth)

				if sec.Cache != nil {
					secHandler = r.withTagCache(
						sec.Cache,
						secHandler,
						ScreenCacheTag(mLocal.Metadata.Name),
						SectionCacheTag(mLocal.Metadata.Name, secLocal.Key),
					)
				}

				r.mux.HandleFunc(secRoute, secHandler)
				logger.Info("Registered Section route for %s:%s at %s", m.Metadata.Name, sec.Key, secRoute)
			}
		}
	}
}
