package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	jwt := auth.NewJWTService("test-secret")

	t.Run("Auth", func(t *testing.T) {
		mw := Auth("/api/v1", jwt, nil, nil, nil)

		// 1. Missing header -> anonymous
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		nextCalled := false
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			claims := GetClaims(r.Context())
			// If store is nil and no header, it might just proceed without claims or with empty claims
			// Actually Auth middleware doesn't inject claims if no header AND store is nil
			if claims != nil {
				assert.True(t, claims["anon"].(bool))
			}
		})).ServeHTTP(rr, req)
		assert.True(t, nextCalled)

		// 2. Valid token
		token, _ := jwt.GenerateToken("user-1", "user", "device-1", "fp-1", false, 1*time.Hour)
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		nextCalled = false
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			claims := GetClaims(r.Context())
			assert.NotNil(t, claims)
			assert.Equal(t, "user-1", claims["sub"])
		})).ServeHTTP(rr, req)
		assert.True(t, nextCalled)

		// 3. Expired token
		token, _ = jwt.GenerateToken("user-1", "user", "device-1", "fp-1", false, -1*time.Hour)
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not be called for expired token")
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		// 4. Revoked token
		mockRev := &mockChecker{revoked: true}
		mw = Auth("/api/v1", jwt, nil, nil, mockRev)
		token, _ = jwt.GenerateToken("user-1", "user", "device-1", "fp-1", false, 1*time.Hour)
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not be called for revoked token")
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		// 5. literal "null" token -> 401
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer null")
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not be called for literal 'null' token")
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		// 6. literal "undefined" token -> 401
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer undefined")
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not be called for literal 'undefined' token")
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("RateLimit", func(t *testing.T) {
		// 1. Nil redis (skipped)
		mw := RateLimit(nil, 10, 1)
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// 2. Unreachable redis (should fail-open in dev)
		mw = RateLimit(redis.NewClient(&redis.Options{Addr: "localhost:1"}), 1, 1)
		req = httptest.NewRequest("GET", "/", nil)
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// 3. Bypass for exempt paths
		mw = RateLimit(nil, 1, 1)
		for _, path := range []string{"/admin", "/admin/", "/admin/dashboard", "/api/admin/login", "/health", "/metrics"} {
			for i := 0; i < 5; i++ {
				req := httptest.NewRequest("GET", path, nil)
				rr := httptest.NewRecorder()
				called := false
				mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					w.WriteHeader(http.StatusOK)
				})).ServeHTTP(rr, req)
				assert.True(t, called, "handler should be called for path %s", path)
				assert.Equal(t, http.StatusOK, rr.Code, "path %s should bypass rate limiter", path)
			}
		}
	})

	t.Run("CORS", func(t *testing.T) {
		os.Setenv("BFFX_ENV", "development")
		mw := CORS([]string{"http://localhost:3000"})
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "http://localhost:3000", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("WorkerAuth", func(t *testing.T) {
		mw := WorkerAuth("secret")
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "secret")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Invalid
		req = httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "wrong")
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Locale", func(t *testing.T) {
		mw := Locale("en")
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Language", "fr")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "fr", r.Context().Value(LocaleKey))
		})).ServeHTTP(rr, req)
	})

	t.Run("RequestID", func(t *testing.T) {
		mw := RequestID
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NotEmpty(t, r.Context().Value(requestIDKey{}))
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
			assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
		})).ServeHTTP(rr, req)
	})

	t.Run("RequestID_UsesCorrelationAlias", func(t *testing.T) {
		mw := RequestID
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Correlation-ID", "corr-123")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "corr-123", GetTraceID(r.Context()))
			assert.Equal(t, "corr-123", GetRequestID(r.Context()))
		})).ServeHTTP(rr, req)
		assert.Equal(t, "corr-123", rr.Header().Get("X-Trace-ID"))
		assert.Equal(t, "corr-123", rr.Header().Get("X-Correlation-ID"))
		assert.Equal(t, "corr-123", rr.Header().Get("X-Request-ID"))
	})

	t.Run("RequestID_PrefersTraceOverCorrelation", func(t *testing.T) {
		mw := RequestID
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Trace-ID", "trace-abc")
		req.Header.Set("X-Correlation-ID", "corr-def")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "trace-abc", GetTraceID(r.Context()))
			assert.Equal(t, "trace-abc", GetRequestID(r.Context()))
		})).ServeHTTP(rr, req)
		assert.Equal(t, "trace-abc", rr.Header().Get("X-Trace-ID"))
		assert.Equal(t, "trace-abc", rr.Header().Get("X-Correlation-ID"))
		assert.Equal(t, "trace-abc", rr.Header().Get("X-Request-ID"))
	})

	t.Run("RequestID_PreservesExplicitRequestID", func(t *testing.T) {
		mw := RequestID
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Correlation-ID", "corr-321")
		req.Header.Set("X-Request-ID", "req-999")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "corr-321", GetTraceID(r.Context()))
			assert.Equal(t, "req-999", GetRequestID(r.Context()))
		})).ServeHTTP(rr, req)
		assert.Equal(t, "corr-321", rr.Header().Get("X-Trace-ID"))
		assert.Equal(t, "corr-321", rr.Header().Get("X-Correlation-ID"))
		assert.Equal(t, "req-999", rr.Header().Get("X-Request-ID"))
	})

	t.Run("Logger", func(t *testing.T) {
		mw := Logger
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Metrics", func(t *testing.T) {
		mw := MetricsAccess
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("SecurityHeaders", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		mw := SecurityHeaders(next)
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
		assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	})

	t.Run("EnforceAuth_APIDocsRequiresAuth", func(t *testing.T) {
		mw := EnforceAuth("/api/v1")
		req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("GetClaims_Empty", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		claims := GetClaims(req.Context())
		assert.Nil(t, claims)
	})

	t.Run("MetricsAccess_Production", func(t *testing.T) {
		os.Setenv("BFFX_ENV", "production")
		defer os.Unsetenv("BFFX_ENV")

		mw := MetricsAccess
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()

		nextCalled := false
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
		assert.False(t, nextCalled)
	})

	t.Run("WithClaims", func(t *testing.T) {
		ctx := context.Background()
		claims := map[string]any{"foo": "bar"}
		newCtx := WithClaims(ctx, claims)
		assert.Equal(t, claims, GetClaims(newCtx))
	})

	t.Run("EnforceAuth", func(t *testing.T) {
		mw := EnforceAuth("/api/v1")

		// 1. Unauthenticated -> 401
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		// 2. Authenticated -> 200
		ctx := WithClaims(context.Background(), map[string]any{"sub": "user-1"})
		req = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Recovery", func(t *testing.T) {
		mw := Recovery
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("RecoveryWithStore", func(t *testing.T) {
		store := storage.NewMemoryStore()
		mw := RecoveryWithStore(store)
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("critical db panic")
		})).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		records, err := store.List(context.Background(), "Incident", 10, 0)
		assert.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Contains(t, records[0]["title"].(string), "critical db panic")
		assert.Equal(t, "Critical", records[0]["severity"])
		assert.Equal(t, false, records[0]["resolved"])
	})

	t.Run("GetTraceID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), traceIDKey{}, "trace-123")
		assert.Equal(t, "trace-123", GetTraceID(ctx))
	})

	t.Run("GetLocale_Context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LocaleKey, "fr")
		assert.Equal(t, "fr", GetLocale(ctx))
	})

	t.Run("MaxBytes", func(t *testing.T) {
		mw := MaxBytes(10)
		body := bytes.NewBufferString("this is too long")
		req := httptest.NewRequest("POST", "/", body)
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := r.Body.Read(make([]byte, 20))
			assert.Error(t, err)
		})).ServeHTTP(rr, req)
	})

	t.Run("AppSecret", func(t *testing.T) {
		mw := AppSecret("/api/v1", "secret")

		// 1. Valid
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-App-Secret", "secret")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		// 2. Invalid
		req = httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("X-App-Secret", "wrong")
		rr = httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Next handler should not be called")
		})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("Timeout", func(t *testing.T) {
		mw := Timeout(10 * time.Millisecond)
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(50 * time.Millisecond):
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
				// Expected cancellation
			}
		})).ServeHTTP(rr, req)
	})

	t.Run("DeviceIDMatrix", func(t *testing.T) {
		assert.True(t, ValidDeviceID("device-1234"), "should accept standard ID")
		assert.False(t, ValidDeviceID("short"), "should reject short ID")
		assert.False(t, ValidDeviceID(string(make([]byte, 200))), "should reject too long ID")
		assert.False(t, ValidDeviceID("invalid@char"), "should reject special chars")
	})

	t.Run("CORS_Disallowed", func(t *testing.T) {
		os.Setenv("BFFX_ENV", "production")
		defer os.Unsetenv("BFFX_ENV")
		mw := CORS([]string{"http://trusted.com"})
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://evil.com")
		rr := httptest.NewRecorder()
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"), "disallowed origin should not get header")
	})

	t.Run("ProxyHeaders", func(t *testing.T) {
		os.Setenv("BFFX_TRUST_X_FORWARDED_FOR", "true")
		defer os.Unsetenv("BFFX_TRUST_X_FORWARDED_FOR")

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
		ip := ClientIP(req, TrustForwardedHeaders())
		assert.Equal(t, "1.1.1.1", ip)

		os.Setenv("BFFX_TRUST_X_FORWARDED_FOR", "false")
		ip = ClientIP(req, TrustForwardedHeaders())
		assert.NotEqual(t, "1.1.1.1", ip, "should ignore XFF when trust is false")
	})
}

type mockChecker struct {
	revoked bool
}

func (m *mockChecker) IsRevoked(jti string) bool { return m.revoked }
func (m *mockChecker) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return nil
}

func (m *mockChecker) SetFailClosed(b bool) {}
