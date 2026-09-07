package router

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/api/validation"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"fmt"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
)

const (
	maxCRUDListLimit  = 100
	maxCRUDListOffset = 1_000_000
)

func normalizeCRUDPagination(limitStr, offsetStr string) (limit int, offset int) {
	limit, _ = strconv.Atoi(limitStr)
	offset, _ = strconv.Atoi(offsetStr)
	if limit <= 0 {
		limit = 20
	}
	if limit > maxCRUDListLimit {
		limit = maxCRUDListLimit
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxCRUDListOffset {
		offset = maxCRUDListOffset
	}
	return limit, offset
}

func (r *Router) registerCRUDRoutes() {
	for _, m := range r.reg.Resources {
		m := m // shadow for closure
		resourceKey := m.Metadata.Name

		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)

		if !spec.Routes.Crud {
			logger.Info("Skipping CRUD routes for %s (disabled in manifest)", resourceKey)
			continue
		}

		collectionPath := crudCollectionHTTPPath(spec, resourceKey)
		itemPath := collectionPath + "/{id}"
		logger.Info("Registering CRUD routes for resource=%s collection=%s", resourceKey, collectionPath)

		r.mux.HandleFunc("GET "+collectionPath, r.withPolicy(m, "read", func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Resource.List: "+resourceKey)
			defer span.End()
			span.SetAttributes(
				attribute.String("resource.name", resourceKey),
				attribute.String("resource.operation", "list"),
			)
			req = req.WithContext(ctx)

			limit, offset := normalizeCRUDPagination(req.URL.Query().Get("limit"), req.URL.Query().Get("offset"))

			if policyReadIsOwner(spec.Policy.Read) {
				userID := middleware.GetUserID(req.Context())
				items, err := r.store.ListByOwner(req.Context(), resourceKey, userID, limit, offset)
				if err != nil {
					span.RecordError(err)
				}
				errors.WriteJSON(w, http.StatusOK, map[string]any{"items": SanitizeResourceListResponse(resourceKey, items)})
			} else {
				items, err := r.store.List(req.Context(), resourceKey, limit, offset)
				if err != nil {
					span.RecordError(err)
				}
				errors.WriteJSON(w, http.StatusOK, map[string]any{"items": SanitizeResourceListResponse(resourceKey, items)})
			}
		}))

		r.mux.HandleFunc("POST "+collectionPath, r.withPolicy(m, "write", func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Resource.Create: "+resourceKey)
			defer span.End()
			span.SetAttributes(
				attribute.String("resource.name", resourceKey),
				attribute.String("resource.operation", "create"),
			)
			req = req.WithContext(ctx)

			payload, ok := decodeJSONBody(w, req)
			if !ok {
				span.RecordError(fmt.Errorf("failed to decode JSON body"))
				return
			}

			payload = validation.AllowedPayload(&spec, payload)
			if err := r.validator.Validate(&spec, payload, false); err != nil {
				span.RecordError(err)
				errors.WriteError(w, http.StatusBadRequest, err.Error(), "validation_failed")
				return
			}

			// Ownership injection
			userID := middleware.GetUserID(req.Context())
			if userID != "" {
				payload["created_by"] = userID
			}

			// BeforeCreate Hooks
			if len(spec.Hooks.BeforeCreate) > 0 {
				ctx := r.newActionContext(req)
				if err := r.executeHooks(spec.Hooks.BeforeCreate, ctx, payload); err != nil {
					span.RecordError(err)
					errors.WriteError(w, http.StatusBadRequest, err.Error(), "hook_failed")
					return
				}
			}

			result, err := r.store.Create(req.Context(), resourceKey, payload)
			if err != nil || result == nil {
				if err != nil {
					span.RecordError(err)
				} else {
					span.RecordError(fmt.Errorf("failed to create: result is nil"))
				}
				errors.WriteError(w, http.StatusInternalServerError, "failed to create", "create_failed")
				return
			}

			// AfterCreate Hooks
			if len(spec.Hooks.AfterCreate) > 0 {
				ctx := r.newActionContext(req)
				r.executeHooks(spec.Hooks.AfterCreate, ctx, result)
			}

			if spec.Stream {
				r.bus.Publish(req.Context(), events.Event{
					Resource: resourceKey,
					Action:   "created",
					Payload:  result,
				})
			}
			errors.WriteJSON(w, http.StatusCreated, SanitizeResourceResponse(resourceKey, result))
		}))

		r.mux.HandleFunc("GET "+itemPath, r.withPolicy(m, "read", func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Resource.Get: "+resourceKey)
			defer span.End()
			span.SetAttributes(
				attribute.String("resource.name", resourceKey),
				attribute.String("resource.operation", "get"),
				attribute.String("resource.id", req.PathValue("id")),
			)
			req = req.WithContext(ctx)

			id := req.PathValue("id")
			item, err := r.store.Get(req.Context(), resourceKey, id)
			if err != nil {
				span.RecordError(err)
				errors.Write(w, errors.ErrNotFound)
				return
			}
			errors.WriteJSON(w, http.StatusOK, SanitizeResourceResponse(resourceKey, item))
		}))

		r.mux.HandleFunc("PATCH "+itemPath, r.withPolicy(m, "write", func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Resource.Update: "+resourceKey)
			defer span.End()
			span.SetAttributes(
				attribute.String("resource.name", resourceKey),
				attribute.String("resource.operation", "update"),
				attribute.String("resource.id", req.PathValue("id")),
			)
			req = req.WithContext(ctx)

			id := req.PathValue("id")
			payload, ok := decodeJSONBody(w, req)
			if !ok {
				span.RecordError(fmt.Errorf("failed to decode JSON body"))
				return
			}

			payload = validation.AllowedPayload(&spec, payload)
			if err := r.validator.Validate(&spec, payload, true); err != nil {
				span.RecordError(err)
				errors.WriteError(w, http.StatusBadRequest, err.Error(), "validation_failed")
				return
			}

			// Security: Prevent hijacking ID or Ownership
			delete(payload, "id")
			delete(payload, "created_by")

			// BeforeUpdate Hooks
			if len(spec.Hooks.BeforeUpdate) > 0 {
				ctx := r.newActionContext(req)
				if err := r.executeHooks(spec.Hooks.BeforeUpdate, ctx, payload); err != nil {
					span.RecordError(err)
					errors.WriteError(w, http.StatusBadRequest, err.Error(), "hook_failed")
					return
				}
			}

			item, err := r.store.Update(req.Context(), resourceKey, id, payload)
			if err != nil {
				span.RecordError(err)
				errors.Write(w, errors.ErrNotFound)
				return
			}

			// AfterUpdate Hooks
			if len(spec.Hooks.AfterUpdate) > 0 {
				ctx := r.newActionContext(req)
				r.executeHooks(spec.Hooks.AfterUpdate, ctx, item)
			}

			if spec.Stream {
				r.bus.Publish(req.Context(), events.Event{
					Resource: resourceKey,
					Action:   "updated",
					Payload:  item,
				})
			}
			errors.WriteJSON(w, http.StatusOK, SanitizeResourceResponse(resourceKey, item))
		}))

		r.mux.HandleFunc("DELETE "+itemPath, r.withPolicy(m, "write", func(w http.ResponseWriter, req *http.Request) {
			ctx, span := observability.StartSpan(req.Context(), "Resource.Delete: "+resourceKey)
			defer span.End()
			span.SetAttributes(
				attribute.String("resource.name", resourceKey),
				attribute.String("resource.operation", "delete"),
				attribute.String("resource.id", req.PathValue("id")),
			)
			req = req.WithContext(ctx)

			id := req.PathValue("id")

			// BeforeDelete Hooks
			if len(spec.Hooks.BeforeDelete) > 0 {
				ctx := r.newActionContext(req)
				if err := r.executeHooks(spec.Hooks.BeforeDelete, ctx, map[string]any{"id": id}); err != nil {
					span.RecordError(err)
					errors.WriteError(w, http.StatusBadRequest, err.Error(), "hook_failed")
					return
				}
			}

			if err := r.store.Delete(req.Context(), resourceKey, id); err != nil {
				span.RecordError(err)
				errors.Write(w, errors.ErrNotFound)
				return
			}

			// AfterDelete Hooks
			if len(spec.Hooks.AfterDelete) > 0 {
				ctx := r.newActionContext(req)
				r.executeHooks(spec.Hooks.AfterDelete, ctx, map[string]any{"id": id})
			}

			if spec.Stream {
				r.bus.Publish(req.Context(), events.Event{
					Resource: resourceKey,
					Action:   "deleted",
					Payload:  map[string]any{"id": id},
				})
			}
			errors.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		}))
	}
}
