package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/hangry-coder/bffx/pkg/addons/billing"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/auth/revocation"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// Auth provides the primary authentication middleware, handling X-Device-ID validation,
// anonymous auto-provisioning, and JWT Bearer token validation with revocation checks.
func Auth(apiPrefix string, authProvider auth.Provider, billing *billing.Manager, store storage.Store, rev revocation.Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deviceID := strings.TrimSpace(r.Header.Get("X-Device-ID"))
			ctx := r.Context()
			if deviceID != "" {
				if !ValidDeviceID(deviceID) {
					errors.WriteError(w, http.StatusBadRequest, "invalid X-Device-ID")
					return
				}
				ctx = context.WithValue(ctx, deviceIDKey{}, deviceID)
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Device probe: do not auto-provision guest rows (would make "first seen" meaningless).
				if r.URL.Path == apiPrefix+"/auth/handshake" || r.URL.Path == apiPrefix+"/app/handshake" {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if os.Getenv("BFFX_DISABLE_ANONYMOUS_AUTO_PROVISION") == "true" {
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if deviceID != "" && store != nil {
					// Anonymous Auto-Provisioning
					user, err := store.GetByField(ctx, "User", "device_id", deviceID)
					if err != nil {
						logger.InfoCtx(ctx, "Provisioning new anonymous user for device: %s", deviceID)
						newID := uuid.New().String()
						var createErr error
						user, createErr = store.Create(ctx, "User", map[string]any{
							"id":         newID,
							"created_by": newID,
							"device_id":  deviceID,
							"name":       "Guest " + SafePrefixRunes(deviceID, 4),
							"role":       "guest",
							"status":     "active",
						})
						if createErr != nil {
							logger.ErrorCtx(ctx, "Failed to create anonymous user: %v", createErr)
						}
					}

					if user != nil {
						claims := map[string]any{
							"sub":  user["id"],
							"role": user["role"],
							"anon": true,
						}
						ctx = context.WithValue(ctx, claimsKey{}, claims)
					}
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			token = strings.TrimSpace(token)
			if token == "" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if token == "null" || token == "undefined" {
				errors.Write(w, errors.ErrUnauthorized)
				return
			}

			claims, err := authProvider.ValidateToken(ctx, token)
			if err != nil {
				logger.ErrorCtx(ctx, "Invalid JWT token: %v", err)
				errors.Write(w, errors.ErrUnauthorized)
				return
			}

			// Check revocation
			if rev != nil {
				if jti, ok := claims["jti"].(string); ok && jti != "" {
					if rev.IsRevoked(jti) {
						logger.WarnCtx(ctx, "Token revoked: %s", jti)
						errors.Write(w, errors.ErrUnauthorized)
						return
					}
				}
			}

			// Optional legacy UA binding (disabled by default — User-Agent is spoofable).
			if os.Getenv("BFFX_JWT_BIND_USER_AGENT") == "true" {
				if tokenFpt, ok := claims["fpt"].(string); ok && tokenFpt != "" {
					currentFpt := r.Header.Get("User-Agent")
					if currentFpt != tokenFpt {
						logger.ErrorCtx(ctx, "Fingerprint mismatch: token.fpt vs User-Agent")
						errors.Write(w, errors.ErrUnauthorized)
						return
					}
				}
			}

			// Resolve entitlements and inject into claims map
			if billing != nil {
				userID := authProvider.UserIDFromClaims(claims)
				if userID != "" {
					ents, _ := billing.GetActiveEntitlements(ctx, userID)
					claims["entitlements"] = ents
				}
			}

			ctx = context.WithValue(ctx, claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetDeviceID(ctx context.Context) string {
	if v, ok := ctx.Value(deviceIDKey{}).(string); ok {
		return v
	}
	return ""
}

func GetClaims(ctx context.Context) map[string]any {
	if v, ok := ctx.Value(claimsKey{}).(map[string]any); ok {
		return v
	}
	return nil
}

func WithClaims(ctx context.Context, claims map[string]any) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

func WithDeviceID(ctx context.Context, deviceID string) context.Context {
	return context.WithValue(ctx, deviceIDKey{}, deviceID)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipKey{}, ip)
}

func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, userAgentKey{}, ua)
}

func GetUserID(ctx context.Context) string {
	claims := GetClaims(ctx)
	if claims != nil {
		if sub, ok := claims["sub"].(string); ok {
			return sub
		}
	}
	return ""
}
