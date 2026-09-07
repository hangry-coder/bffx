package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Dynamic Response Prototype Registry for Unmarshaling Cache Hits
var (
	responseTypesMu sync.RWMutex
	responseTypes   = make(map[string]reflect.Type)
)

// BuildCacheTTLMap parses active manifests in the registry to build a static FullMethod -> TTL mapping.
func BuildCacheTTLMap(reg *manifest.Registry, wirePackage string) map[string]time.Duration {
	ttls := make(map[string]time.Duration)
	if wirePackage == "" {
		wirePackage = "bffx.v1"
	}

	// 1. Resources
	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := r.UnmarshalSpec(&spec); err == nil {
			if spec.Cache != nil && spec.Cache.TTL > 0 {
				ttl := time.Duration(spec.Cache.TTL) * time.Second
				name := r.Metadata.Name
				// List RPC
				ttls[fmt.Sprintf("/%s.%sService/List%ss", wirePackage, name, name)] = ttl
				// Get RPC
				ttls[fmt.Sprintf("/%s.%sService/Get%s", wirePackage, name, name)] = ttl
			}
		}
	}

	// 2. Actions
	for _, a := range reg.Actions {
		var spec manifest.ActionSpec
		if err := a.UnmarshalSpec(&spec); err == nil {
			if spec.Route.CacheTTL > 0 {
				ttl := time.Duration(spec.Route.CacheTTL) * time.Second
				name := a.Metadata.Name
				ttls[fmt.Sprintf("/%s.ActionService/%s", wirePackage, name)] = ttl
			}
		}
	}

	// 3. Builders
	for _, b := range reg.Builders {
		var spec manifest.BuilderSpec
		if err := b.UnmarshalSpec(&spec); err == nil {
			if spec.Cache != nil && spec.Cache.TTL > 0 {
				ttl := time.Duration(spec.Cache.TTL) * time.Second
				name := b.Metadata.Name
				ttls[fmt.Sprintf("/%s.BuilderService/Get%s", wirePackage, name)] = ttl
			}
		}
	}

	return ttls
}

// UnaryCacheInterceptor intercepts unary requests to enforce high-performance binary caching.
func UnaryCacheInterceptor(provider cache.Provider, ttls map[string]time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if provider == nil || len(ttls) == 0 {
			return handler(ctx, req)
		}

		ttl, isCacheable := ttls[info.FullMethod]
		if !isCacheable || ttl <= 0 {
			return handler(ctx, req)
		}

		userID := middleware.GetUserID(ctx)
		key := grpcCacheKey(userID, info.FullMethod, req)

		// 1. Check cache hit
		if cachedBytes, err := provider.Get(ctx, key); err == nil && len(cachedBytes) > 0 {
			responseTypesMu.RLock()
			t, found := responseTypes[info.FullMethod]
			responseTypesMu.RUnlock()

			if found {
				// Instantiate clean dynamic struct pointer
				newResp := reflect.New(t.Elem()).Interface().(proto.Message)
				if err := proto.Unmarshal(cachedBytes, newResp); err == nil {
					return newResp, nil
				}
			}
		}

		// 2. Cache miss: Execute handler
		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}

		// 3. Store dynamic response prototype and cached binary payload
		if resp != nil {
			msg, ok := resp.(proto.Message)
			if ok {
				// Save prototype type info
				responseTypesMu.Lock()
				responseTypes[info.FullMethod] = reflect.TypeOf(resp)
				responseTypesMu.Unlock()

				// Marshal response dynamically and save to cache
				if responseBytes, err := proto.Marshal(msg); err == nil {
					_ = provider.Set(ctx, key, responseBytes, ttl)
				}
			}
		}

		return resp, nil
	}
}

func grpcCacheKey(userID string, fullMethod string, req interface{}) string {
	h := sha256.New()
	h.Write([]byte("GRPC"))
	h.Write([]byte{0})
	h.Write([]byte(fullMethod))
	h.Write([]byte{0})

	if msg, ok := req.(proto.Message); ok {
		if reqBytes, err := proto.Marshal(msg); err == nil {
			h.Write(reqBytes)
		} else {
			h.Write([]byte(fmt.Sprintf("%v", req)))
		}
	} else {
		h.Write([]byte(fmt.Sprintf("%v", req)))
	}

	owner := userID
	if owner == "" {
		owner = "anon"
	}
	return "bffx:action-cache:" + owner + ":" + hex.EncodeToString(h.Sum(nil))
}

// TraceIDInterceptor extracts or injects trace IDs and request IDs into incoming RPC contexts.
func TraceIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		var traceID, requestID string
		if ok {
			if vals := md.Get(observability.MetadataTraceID); len(vals) > 0 {
				traceID = vals[0]
			}
			if traceID == "" {
				if vals := md.Get(observability.MetadataCorrelationID); len(vals) > 0 {
					traceID = vals[0]
				}
			}
			if vals := md.Get(observability.MetadataRequestID); len(vals) > 0 {
				requestID = vals[0]
			}
			if traceID == "" {
				if vals := md.Get(observability.MetadataTraceParent); len(vals) > 0 {
					// W3C traceparent (e.g. 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01)
					parts := strings.Split(vals[0], "-")
					if len(parts) >= 2 {
						traceID = parts[1]
					}
				}
			}
		}

		if traceID == "" {
			traceID = uuid.New().String()
		}
		if requestID == "" {
			requestID = traceID
		}

		ctx = middleware.WithTraceID(ctx, traceID)
		ctx = middleware.WithRequestID(ctx, requestID)

		// Set client IP/UA if provided
		if ok {
			if ips := md.Get("x-real-ip"); len(ips) > 0 {
				ctx = middleware.WithIP(ctx, ips[0])
			} else if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
				ctx = middleware.WithIP(ctx, strings.Split(ips[0], ",")[0])
			}
			if uas := md.Get("user-agent"); len(uas) > 0 {
				ctx = middleware.WithUserAgent(ctx, uas[0])
			}
		}

		return handler(ctx, req)
	}
}

// RecoveryInterceptor catches panics, logs the stack trace, and converts them to internal gRPC status codes.
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("GRPC PANIC RECOVERED: %v\n%s", r, logger.Stack())
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// AuthInterceptor enforces authentication, token validations, and anonymous guests auto-provisioning.
func AuthInterceptor(authProvider auth.Provider, store storage.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		var authHeader, deviceID string
		if ok {
			if vals := md.Get("authorization"); len(vals) > 0 {
				authHeader = vals[0]
			}
			if vals := md.Get("x-device-id"); len(vals) > 0 {
				deviceID = vals[0]
			}
		}

		if deviceID != "" {
			if !middleware.ValidDeviceID(deviceID) {
				return nil, status.Errorf(codes.InvalidArgument, "invalid x-device-id")
			}
			ctx = middleware.WithDeviceID(ctx, deviceID)
		}

		if authHeader == "" {
			// Handle anonymous guest login if auto-provision is not disabled
			if os.Getenv("BFFX_DISABLE_ANONYMOUS_AUTO_PROVISION") != "true" && deviceID != "" && store != nil {
				user, err := store.GetByField(ctx, "User", "device_id", deviceID)
				if err != nil {
					logger.InfoCtx(ctx, "Provisioning new anonymous gRPC user for device: %s", deviceID)
					newID := uuid.New().String()
					user, _ = store.Create(ctx, "User", map[string]any{
						"id":         newID,
						"created_by": newID,
						"device_id":  deviceID,
						"name":       "Guest " + middleware.SafePrefixRunes(deviceID, 4),
						"role":       "guest",
						"status":     "active",
					})
				}
				if user != nil {
					claims := map[string]any{
						"sub":  user["id"],
						"role": user["role"],
						"anon": true,
					}
					ctx = middleware.WithClaims(ctx, claims)
				}
			}
			return handler(ctx, req)
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := authProvider.ValidateToken(ctx, token)
		if err != nil {
			logger.ErrorCtx(ctx, "gRPC Invalid JWT token: %v", err)
			return nil, status.Errorf(codes.Unauthenticated, "invalid or expired token")
		}

		ctx = middleware.WithClaims(ctx, claims)
		return handler(ctx, req)
	}
}
