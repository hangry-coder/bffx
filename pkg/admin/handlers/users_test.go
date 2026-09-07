package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestUserHandler_ListAndDetail(t *testing.T) {
	store := storage.NewMemoryStore()
	reg := &manifest.Registry{}

	// Seed some mock users
	created, _ := store.Create(context.Background(), "User", map[string]any{
		"email":         "test@bffx.io",
		"password":      "secret123",
		"otp_code":      "999999",
		"password_hash": "hash123",
		"name":          "BFFX Test User",
	})
	userID := created["id"].(string)

	handler := handlers.NewUserHandler(store, reg)

	t.Run("ListUsers filters out sensitive fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/users", nil)
		w := httptest.NewRecorder()

		handler.ListUsers(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var users []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(users) != 1 {
			t.Fatalf("expected 1 user, got %d", len(users))
		}

		user := users[0]
		if user["email"] != "test@bffx.io" {
			t.Errorf("expected email to be test@bffx.io, got %v", user["email"])
		}
		if _, ok := user["password"]; ok {
			t.Error("password field not filtered")
		}
		if _, ok := user["otp_code"]; ok {
			t.Error("otp_code field not filtered")
		}
		if _, ok := user["password_hash"]; ok {
			t.Error("password_hash field not filtered")
		}
	})

	t.Run("GetUserDetail filters out sensitive fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/users/"+userID, nil)
		req.SetPathValue("id", userID)
		w := httptest.NewRecorder()

		handler.GetUserDetail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var detail map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		userPart, ok := detail["user"].(map[string]any)
		if !ok {
			t.Fatalf("missing user section in response: %v", detail)
		}

		if userPart["name"] != "BFFX Test User" {
			t.Errorf("expected name to be BFFX Test User, got %v", userPart["name"])
		}
		if _, ok := userPart["password"]; ok {
			t.Error("password field not filtered in detail view")
		}
		if _, ok := userPart["otp_code"]; ok {
			t.Error("otp_code field not filtered in detail view")
		}
		if _, ok := userPart["password_hash"]; ok {
			t.Error("password_hash field not filtered in detail view")
		}
	})
}
