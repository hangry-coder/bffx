package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
	"gopkg.in/yaml.v3"
)

type mockRevocationChecker struct {
	revoked map[string]bool
	mu      sync.RWMutex
}

func (m *mockRevocationChecker) IsRevoked(jti string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revoked[jti]
}

func (m *mockRevocationChecker) Revoke(jti string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revoked == nil {
		m.revoked = make(map[string]bool)
	}
	m.revoked[jti] = true
}

func TestAuthMatrix(t *testing.T) {
	s := storage.NewMemoryStore()

	var userSpec yaml.Node
	yaml.Unmarshal([]byte("routes:\n  crud: true\nfields:\n  - {name: email, type: string}\n  - {name: password, type: string}"), &userSpec)

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
		},
	}

	checker := &mockRevocationChecker{}
	jwt := auth.NewJWTService("test-secret")
	fmt.Printf("DEBUG: Test created JWT at %p\n", jwt)
	jwt.SetRevocationChecker(func(jti string) bool {
		return checker.IsRevoked(jti)
	})

	r := NewRouter(RouterConfig{
		Store:         s,
		Registry:      reg,
		AuthProvider:  jwt,
		JWTService:    jwt,
		EventBus:      events.NewMemoryBus(),
		Notifications: notifications.NewManager(s),
		Email:         email.NewManager(),
		CommHub:       comm.NewHub(s, nil, nil, nil),
		I18n:          i18n.NewBundle("en"),
		FlagProvider:  providers.NewBffxProvider(s, reg),
		JobStore:      worker.NewMemoryJobStore(),
	})
	// Inject the jwt service with the revocation checker
	r.jwt = jwt

	handler := r.Setup()

	t.Run("AnonymousToUserUpgrade", func(t *testing.T) {
		// 1. Get Anonymous Token
		body := `{"device_id": "test-device-123"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/anonymous", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("anonymous session failed: %s", rr.Body.String())
		}

		var anonResp struct {
			Token string
			User  struct {
				ID   string
				Role string
			}
		}
		json.Unmarshal(rr.Body.Bytes(), &anonResp)

		if anonResp.User.Role != "guest" {
			t.Errorf("expected role guest, got %s", anonResp.User.Role)
		}

		// 2. Upgrade to User
		signupBody := fmt.Sprintf(`{"email": "upgrade@example.com", "password": "password", "anonymous_token": "%s"}`, anonResp.Token)
		req = httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewBufferString(signupBody))
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("upgrade failed: %s", rr.Body.String())
		}

		var upgradeResp struct {
			Token string
			User  struct {
				ID   string
				Role string
			}
		}
		json.Unmarshal(rr.Body.Bytes(), &upgradeResp)

		if upgradeResp.User.ID != anonResp.User.ID {
			t.Errorf("ID mismatch: %s != %s", upgradeResp.User.ID, anonResp.User.ID)
		}
		if upgradeResp.User.Role != "user" {
			t.Errorf("expected role user, got %s", upgradeResp.User.Role)
		}
	})

	t.Run("TokenRevocation", func(t *testing.T) {
		// 0. Create user
		user, err := s.Create(context.Background(), "User", map[string]any{"id": "user-rev-1", "role": "user"})
		if err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		fmt.Printf("DEBUG: Created user: %+v\n", user)
		userID := fmt.Sprintf("%v", user["id"])

		// 1. Generate Token
		token, err := jwt.GenerateToken(userID, "user", "dev-1", "fp-1", false, 1*time.Hour)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Verify locally first
		_, err = jwt.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("Local validation failed: %v", err)
		}

		claims, err := jwt.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}
		jti := claims["jti"].(string)
		t.Logf("Revoking JTI: %s", jti)

		// 2. Verify token works
		req := httptest.NewRequest("GET", "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("initial request failed: %d Body: %s", rr.Code, rr.Body.String())
		}

		// 3. Revoke token
		checker.Revoke(jti)

		// 4. Verify token fails
		req = httptest.NewRequest("GET", "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 after revocation, got %d Body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("MalformedTokens", func(t *testing.T) {
		tests := []struct {
			name  string
			token string
		}{
			{"Garbage", "not-a-token"},
			{"WrongSecret", func() string {
				wrongJwt := auth.NewJWTService("wrong-secret")
				t, _ := wrongJwt.GenerateToken("u1", "user", "", "", false, 1*time.Hour)
				return t
			}()},
			{"Expired", func() string {
				t, _ := jwt.GenerateToken("u1", "user", "", "", false, -1*time.Hour)
				return t
			}()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/v1/me", nil)
				req.Header.Set("Authorization", "Bearer "+tt.token)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				if rr.Code != http.StatusUnauthorized {
					t.Errorf("expected 401 for %s, got %d", tt.name, rr.Code)
				}
			})
		}
	})
}
