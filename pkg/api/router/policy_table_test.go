package router

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
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

func TestPolicyMatrix(t *testing.T) {
	s := storage.NewMemoryStore()
	jwt := auth.NewJWTService("test-secret")

	// Helper to create token
	createToken := func(userID, role string) string {
		token, _ := jwt.GenerateToken(userID, role, "test-device", "test-fp", false, 1*time.Hour)
		return token
	}

	adminToken := createToken("admin-1", "admin")
	user1Token := createToken("user-1", "user")
	user2Token := createToken("user-2", "user")

	var publicSpec, requiredSpec, adminSpec, ownerSpec, actionAdminSpec, builderAdminSpec yaml.Node

	unmarshal := func(y string) yaml.Node {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(y), &n); err != nil {
			t.Fatalf("failed to unmarshal yaml: %v", err)
		}
		return n
	}

	publicSpec = unmarshal("routes:\n  crud: true\npolicy:\n  read: public\n  write: public\nfields:\n  - {name: name, type: string}")
	requiredSpec = unmarshal("routes:\n  crud: true\npolicy:\n  read: authenticated\n  write: authenticated\nfields:\n  - {name: name, type: string}")
	adminSpec = unmarshal("routes:\n  crud: true\npolicy:\n  read: admin\n  write: admin\nfields:\n  - {name: name, type: string}")
	ownerSpec = unmarshal("routes:\n  crud: true\npolicy:\n  read: owner\n  write: owner\nfields:\n  - {name: name, type: string}")
	actionAdminSpec = unmarshal("route:\n  method: POST\n  path: /api/v1/admin-action\n  auth: optional\n  roles: [admin]")
	builderAdminSpec = unmarshal("route:\n  method: GET\n  path: /api/v1/admin-builder\n  auth: optional\n  roles: [admin]\nsources: []\noutput: {}")

	reg := &manifest.Registry{
		ApiPrefix: "/api/v1",
		Project: &manifest.Manifest{
			Metadata: manifest.Metadata{Name: "TestApp"},
			Spec:     unmarshal("app:\n  apiPrefix: /api/v1"),
		},
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "PublicItem"}, Spec: publicSpec},
			{Metadata: manifest.Metadata{Name: "RequiredItem"}, Spec: requiredSpec},
			{Metadata: manifest.Metadata{Name: "AdminItem"}, Spec: adminSpec},
			{Metadata: manifest.Metadata{Name: "OwnerItem"}, Spec: ownerSpec},
		},
		Actions: []*manifest.Manifest{
			{Kind: "Action", Metadata: manifest.Metadata{Name: "AdminAction"}, Spec: actionAdminSpec},
		},
		Builders: []*manifest.Manifest{
			{Kind: "Builder", Metadata: manifest.Metadata{Name: "AdminBuilder"}, Spec: builderAdminSpec},
		},
	}

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
		ActionHandlers: map[string]handlers.ActionHandler{
			"AdminAction": func(actx *handlers.ActionContext, w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			},
		},
	})
	handler := r.Setup()

	// Seed some owner data
	ctx := context.Background()
	item1, _ := s.Create(ctx, "OwnerItem", map[string]any{"name": "User 1 Item", "created_by": "user-1"})
	s.Create(ctx, "OwnerItem", map[string]any{"name": "User 2 Item", "created_by": "user-2"})
	id1 := item1["id"].(string)

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		expectedStatus int
	}{
		// Public Access
		{"PublicReadNoAuth", "GET", "/api/v1/publicitems", "", http.StatusOK},
		{"PublicWriteNoAuth", "POST", "/api/v1/publicitems", "", http.StatusCreated},

		// Authenticated Access
		{"RequiredReadNoAuth", "GET", "/api/v1/requireditems", "", http.StatusUnauthorized},
		{"RequiredReadWithAuth", "GET", "/api/v1/requireditems", user1Token, http.StatusOK},

		// Admin Access
		{"AdminReadWithUserAuth", "GET", "/api/v1/adminitems", user1Token, http.StatusForbidden},
		{"AdminReadWithAdminAuth", "GET", "/api/v1/adminitems", adminToken, http.StatusOK},

		// Owner Access (LIST/CREATE)
		{"OwnerListNoAuth", "GET", "/api/v1/owneritems", "", http.StatusUnauthorized},
		{"OwnerListWithAuth", "GET", "/api/v1/owneritems", user1Token, http.StatusOK},
		{"OwnerCreateWithAuth", "POST", "/api/v1/owneritems", user1Token, http.StatusCreated},

		// Owner Access (Specific Resource)
		{"OwnerGetOwnItem", "GET", "/api/v1/owneritems/" + id1, user1Token, http.StatusOK},
		{"OwnerGetOtherItem", "GET", "/api/v1/owneritems/" + id1, user2Token, http.StatusForbidden},
		{"OwnerUpdateOwnItem", "PATCH", "/api/v1/owneritems/" + id1, user1Token, http.StatusOK},
		{"OwnerUpdateOtherItem", "PATCH", "/api/v1/owneritems/" + id1, user2Token, http.StatusForbidden},
		{"AdminGetAnyOwnerItem", "GET", "/api/v1/owneritems/" + id1, adminToken, http.StatusOK},

		// Declarative role guards on action/builder routes
		{"AdminActionNoAuth", "POST", "/api/v1/admin-action", "", http.StatusUnauthorized},
		{"AdminActionWithUserRole", "POST", "/api/v1/admin-action", user1Token, http.StatusForbidden},
		{"AdminActionWithAdminRole", "POST", "/api/v1/admin-action", adminToken, http.StatusOK},
		{"AdminBuilderNoAuth", "GET", "/api/v1/admin-builder", "", http.StatusUnauthorized},
		{"AdminBuilderWithUserRole", "GET", "/api/v1/admin-builder", user1Token, http.StatusForbidden},
		{"AdminBuilderWithAdminRole", "GET", "/api/v1/admin-builder", adminToken, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.method != "GET" {
				body = bytes.NewBufferString(`{"name": "test"}`)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("%s %s: expected status %d, got %d. Body: %s", tt.method, tt.path, tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
