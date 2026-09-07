package router

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/api/validation"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// protoResourceMarshalOpts keeps gRPC resource payloads on manifest snake_case keys.
// Default protojson uses camelCase JSON names (loggedAt); storage expects logged_at.
var protoResourceMarshalOpts = protojson.MarshalOptions{UseProtoNames: true}

func (r *Router) newActionContextFromGRPC(ctx context.Context) *handlers.ActionContext {
	userID := middleware.GetUserID(ctx)
	var user map[string]any
	if userID != "" {
		user, _ = r.store.Get(ctx, "User", userID)
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
		Claims:       middleware.GetClaims(ctx),
		Context:      ctx,
	}
}

func (r *Router) resourceSpec(resourceKey string) (manifest.ResourceSpec, error) {
	m, ok := r.reg.GetResource(resourceKey)
	if !ok {
		return manifest.ResourceSpec{}, fmt.Errorf("resource %s not found", resourceKey)
	}
	var spec manifest.ResourceSpec
	if err := m.UnmarshalSpec(&spec); err != nil {
		return manifest.ResourceSpec{}, err
	}
	return spec, nil
}

// MarshalProtoResourcePayload converts a typed gRPC request message into a manifest-shaped payload map.
func (r *Router) MarshalProtoResourcePayload(resourceKey string, msg proto.Message) (map[string]any, error) {
	spec, err := r.resourceSpec(resourceKey)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := protoResourceMarshalOpts.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return nil, err
	}
	return alignGRPCPayloadToSpec(&spec, payload), nil
}

// CreateResourceGRPC mirrors HTTP POST CRUD create: validation, ownership, hooks, then persist.
func (r *Router) CreateResourceGRPC(ctx context.Context, resourceKey string, payload map[string]any) (map[string]any, error) {
	ctx, span := observability.StartSpan(ctx, "Resource.CreateGRPC: "+resourceKey)
	defer span.End()
	span.SetAttributes(
		attribute.String("resource.name", resourceKey),
		attribute.String("resource.operation", "create"),
	)

	spec, err := r.resourceSpec(resourceKey)
	if err != nil {
		span.RecordError(err)
		return nil, status.Errorf(codes.NotFound, "%s", err.Error())
	}

	if payload == nil {
		payload = map[string]any{}
	}
	if id, ok := payload["id"].(string); ok && strings.TrimSpace(id) == "" {
		delete(payload, "id")
	}

	payload = alignGRPCPayloadToSpec(&spec, payload)
	payload = validation.AllowedPayload(&spec, payload)
	if err := r.validator.Validate(&spec, payload, false); err != nil {
		span.RecordError(err)
		return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	userID := middleware.GetUserID(ctx)
	if userID != "" {
		payload["created_by"] = userID
	}

	if len(spec.Hooks.BeforeCreate) > 0 {
		actCtx := r.newActionContextFromGRPC(ctx)
		if err := r.executeHooks(spec.Hooks.BeforeCreate, actCtx, payload); err != nil {
			span.RecordError(err)
			return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
		}
	}

	result, err := r.store.Create(ctx, resourceKey, payload)
	if err != nil || result == nil {
		if err != nil {
			span.RecordError(err)
			return nil, status.Errorf(codes.Internal, "failed to create")
		}
		span.RecordError(fmt.Errorf("create returned nil result"))
		return nil, status.Errorf(codes.Internal, "failed to create")
	}

	if len(spec.Hooks.AfterCreate) > 0 {
		actCtx := r.newActionContextFromGRPC(ctx)
		r.executeHooks(spec.Hooks.AfterCreate, actCtx, result)
	}

	if spec.Stream {
		r.bus.Publish(ctx, events.Event{
			Resource: resourceKey,
			Action:   "created",
			Payload:  result,
		})
	}

	return SanitizeResourceResponse(resourceKey, result), nil
}

// UpdateResourceGRPC mirrors HTTP PATCH CRUD update: validation, hooks, then persist.
func (r *Router) UpdateResourceGRPC(ctx context.Context, resourceKey, id string, payload map[string]any) (map[string]any, error) {
	ctx, span := observability.StartSpan(ctx, "Resource.UpdateGRPC: "+resourceKey)
	defer span.End()
	span.SetAttributes(
		attribute.String("resource.name", resourceKey),
		attribute.String("resource.operation", "update"),
		attribute.String("resource.id", id),
	)

	spec, err := r.resourceSpec(resourceKey)
	if err != nil {
		span.RecordError(err)
		return nil, status.Errorf(codes.NotFound, "%s", err.Error())
	}

	if payload == nil {
		payload = map[string]any{}
	}
	payload = alignGRPCPayloadToSpec(&spec, payload)
	payload = validation.AllowedPayload(&spec, payload)
	if err := r.validator.Validate(&spec, payload, true); err != nil {
		span.RecordError(err)
		return nil, status.Errorf(codes.InvalidArgument, "%s", err.Error())
	}

	delete(payload, "id")
	delete(payload, "created_by")

	if len(spec.Hooks.BeforeUpdate) > 0 {
		actCtx := r.newActionContextFromGRPC(ctx)
		if err := r.executeHooks(spec.Hooks.BeforeUpdate, actCtx, payload); err != nil {
			span.RecordError(err)
			return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
		}
	}

	result, err := r.store.Update(ctx, resourceKey, id, payload)
	if err != nil || result == nil {
		if err != nil {
			span.RecordError(err)
			return nil, status.Errorf(codes.NotFound, "not found")
		}
		span.RecordError(fmt.Errorf("update returned nil result"))
		return nil, status.Errorf(codes.Internal, "failed to update")
	}

	if len(spec.Hooks.AfterUpdate) > 0 {
		actCtx := r.newActionContextFromGRPC(ctx)
		r.executeHooks(spec.Hooks.AfterUpdate, actCtx, result)
	}

	if spec.Stream {
		r.bus.Publish(ctx, events.Event{
			Resource: resourceKey,
			Action:   "updated",
			Payload:  result,
		})
	}

	return SanitizeResourceResponse(resourceKey, result), nil
}

// alignGRPCPayloadToSpec maps proto JSON camelCase keys (loggedAt) to manifest snake_case (logged_at).
func alignGRPCPayloadToSpec(spec *manifest.ResourceSpec, payload map[string]any) map[string]any {
	if spec == nil || len(payload) == 0 {
		return payload
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	for _, f := range spec.Fields {
		if _, ok := out[f.Name]; ok {
			continue
		}
		if v, ok := out[protoJSONFieldName(f.Name)]; ok {
			out[f.Name] = v
			delete(out, protoJSONFieldName(f.Name))
		}
	}
	return out
}

func protoJSONFieldName(field string) string {
	parts := strings.Split(field, "_")
	if len(parts) == 0 {
		return field
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}
