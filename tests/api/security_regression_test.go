package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/router"
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

func setupSecurityTestRouter(t *testing.T) (http.Handler, *storage.MemoryStore, *auth.JWTService) {
	t.Helper()
	st := storage.NewMemoryStore()

	var userSpec yaml.Node
	userYAML := `
routes:
  crud: true
policy:
  read: owner
  write: authenticated
fields:
  - {name: email, type: string, required: true}
  - {name: password, type: string, required: true}
  - {name: role, type: string}
  - {name: secret_token, type: string}
`
	if err := yaml.Unmarshal([]byte(userYAML), &userSpec); err != nil {
		t.Fatalf("user spec: %v", err)
	}

	reg := &manifest.Registry{
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     yaml.Node{},
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}, Spec: userSpec},
		},
	}

	bus := events.NewMemoryBus()
	notify := notifications.NewManager(st)
	emailMgr := email.NewManager()
	hub := comm.NewHub(st, notify, emailMgr, comm.NewTemplateManager(reg))
	bundle := i18n.NewBundle("en")
	flagProvider := providers.NewBffxProvider(st, reg)
	jwtSvc := auth.NewJWTService("test-secret-at-least-32-characters-long")

	r := router.NewRouter(router.RouterConfig{
		Store:         st,
		Telemetry:     st,
		JobStore:      worker.NewMemoryJobStore(),
		Registry:      reg,
		AuthProvider:  auth.NewJWTProvider(jwtSvc),
		JWTService:    jwtSvc,
		EventBus:      bus,
		Notifications: notify,
		Email:         emailMgr,
		CommHub:       hub,
		I18n:          bundle,
		FlagProvider:  flagProvider,
	})
	handler := r.Setup()
	return handler, st, jwtSvc
}

func TestSecurityRegression_UserRoleEscalationBlocked(t *testing.T) {
	handler, _, _ := setupSecurityTestRouter(t)

	// 1. Unauthenticated CRUD POST User with role: admin must be blocked (401/403)
	payload := `{"email":"admin_esc@example.com","password":"password123","role":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
		t.Errorf("expected 401 or 403 for unauthenticated CRUD post, got %d; body=%s", rr.Code, rr.Body.String())
	}
}

func TestSecurityRegression_CRUDPasswordAndSecretLeak(t *testing.T) {
	handler, st, jwtSvc := setupSecurityTestRouter(t)

	pwdHash, _ := auth.HashPassword("my-insecure-password-123")
	userRow, err := st.Create(context.Background(), "User", map[string]any{
		"email":        "test_owner@example.com",
		"password":     pwdHash,
		"role":         "user",
		"secret_token": "highly-sensitive-token-should-be-stripped",
	})
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	userID := userRow["id"].(string)

	_, err = st.Update(context.Background(), "User", userID, map[string]any{
		"created_by": userID,
	})
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	// Generate owner token
	token, err := jwtSvc.GenerateToken(userID, "user", "device1", "agent1", false, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// 1. GET /api/v1/users/{id} (as owner) - verify password, password_hash, secret_token are stripped
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET user, got %d; body=%s", rr.Code, rr.Body.String())
	}

	var fetchedUser map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &fetchedUser); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, field := range []string{"password", "password_hash", "secret_token", "otp_code", "secret", "token"} {
		if val, exists := fetchedUser[field]; exists && val != nil {
			t.Errorf("security leak: field %q found in GET response with value: %v", field, val)
		}
	}

	// 2. LIST /api/v1/users (as owner) - verify it is filtered / stripped
	// Note: User policy is read: owner. Let's see if ListByOwner is used.
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	rrList := httptest.NewRecorder()
	handler.ServeHTTP(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for LIST users, got %d; body=%s", rrList.Code, rrList.Body.String())
	}

	var listResult struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResult); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	if len(listResult.Items) == 0 {
		t.Fatal("expected at least 1 user item in list")
	}

	for _, item := range listResult.Items {
		for _, field := range []string{"password", "password_hash", "secret_token", "otp_code", "secret", "token"} {
			if val, exists := item[field]; exists && val != nil {
				t.Errorf("security leak: field %q found in LIST response item with value: %v", field, val)
			}
		}
	}
}
