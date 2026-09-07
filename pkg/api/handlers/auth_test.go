package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestAuthHandler(t *testing.T) {
	s := storage.NewMemoryStore()
	jwt := auth.NewJWTService("test-secret")

	// Registry for refresh token tests
	var rtSpec yaml.Node
	yaml.Unmarshal([]byte(`
fields:
  - {name: user_id, type: string}
  - {name: device_id, type: string}
  - {name: token, type: string}
  - {name: expires_at, type: string}
  - {name: used, type: bool}
  - {name: family_id, type: string}
`), &rtSpec)

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}},
			{Metadata: manifest.Metadata{Name: "RefreshToken"}, Spec: rtSpec},
		},
	}

	h := NewAuthHandler(s, jwt, jwt, reg)

	assertAPIError := func(t *testing.T, body []byte, errorCode string) {
		var res map[string]any
		err := json.Unmarshal(body, &res)
		assert.NoError(t, err)
		errObj, ok := res["error"].(map[string]any)
		assert.True(t, ok, "response should have an 'error' object")
		if ok {
			assert.Equal(t, errorCode, errObj["code"], "API error_code mismatch")
		}
	}

	t.Run("Anonymous", func(t *testing.T) {
		// Valid
		body := `{"device_id": "test-device-1234"}`
		req := httptest.NewRequest("POST", "/auth/anonymous", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.AnonymousSession(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.NotEmpty(t, res["token"])

		// Missing device_id
		req = httptest.NewRequest("POST", "/auth/anonymous", strings.NewReader(`{}`))
		rr = httptest.NewRecorder()
		h.AnonymousSession(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "validation_error")

		// Dynamic disabled check
		_, _ = s.Create(context.Background(), "AppConfig", map[string]any{
			"config_key":   "disable_guest_signup",
			"config_value": "true",
		})
		body = `{"device_id": "test-device-disabled"}`
		req = httptest.NewRequest("POST", "/auth/anonymous", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.AnonymousSession(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "guest_signup_disabled")

		// Clean up for other tests
		cfgRecords, _ := s.Query(context.Background(), "AppConfig").Where("config_key", "=", "disable_guest_signup").Execute(context.Background())
		for _, rec := range cfgRecords {
			id, _ := rec["id"].(string)
			s.Delete(context.Background(), "AppConfig", id)
		}
	})

	t.Run("Signup_Validation", func(t *testing.T) {
		// 1. Missing fields
		body := `{"email": "", "password": ""}`
		req := httptest.NewRequest("POST", "/auth/signup", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Signup(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "validation_error")

		// 2. Weak password
		body = `{"email": "weak@example.com", "password": "short"}`
		req = httptest.NewRequest("POST", "/auth/signup", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.Signup(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "validation_error")
		assert.Contains(t, rr.Body.String(), "at least 8 characters")

		// 3. Duplicate email
		body = `{"email": "dup@example.com", "password": "Password123!"}`
		req = httptest.NewRequest("POST", "/auth/signup", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.Signup(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)

		req = httptest.NewRequest("POST", "/auth/signup", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.Signup(rr, req)
		assert.Equal(t, http.StatusConflict, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "validation_error")
	})

	t.Run("Login_EdgeCases", func(t *testing.T) {
		// 1. Wrong password
		body := `{"email": "dup@example.com", "password": "wrongpassword"}`
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "unauthorized")

		// 2. Non-existent user
		body = `{"email": "none@example.com", "password": "any"}`
		req = httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.Login(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "unauthorized")
	})

	t.Run("Handshake", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/handshake", nil)
		req.Header.Set("X-Device-ID", "test-device-1234")
		rr := httptest.NewRecorder()
		h.Handshake(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.NotEmpty(t, res["server_time"])
	})

	t.Run("Login_Success", func(t *testing.T) {
		body := `{"email": "dup@example.com", "password": "Password123!"}`
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.NotEmpty(t, res["token"])
		assert.NotEmpty(t, res["refresh_token"])
	})

	t.Run("Logout_EdgeCases", func(t *testing.T) {
		// 1. No claims (unauthenticated)
		req := httptest.NewRequest("POST", "/auth/logout", nil)
		rr := httptest.NewRecorder()
		h.Logout(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "unauthorized")

		// 2. Success path
		ctx := middleware.WithClaims(req.Context(), map[string]any{"sub": "user-1", "jti": "jti-123", "exp": float64(time.Now().Add(time.Hour).Unix())})
		req = req.WithContext(ctx)
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "logged out")

		// 3. Logout revokes refresh token family
		_, _ = s.Create(context.Background(), "RefreshToken", map[string]any{
			"user_id":    "user-logout",
			"device_id":  "device-logout",
			"token":      "logout-rt-1",
			"expires_at": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
			"used":       false,
			"family_id":  "logout-family-1",
		})
		_, _ = s.Create(context.Background(), "RefreshToken", map[string]any{
			"user_id":    "user-logout",
			"device_id":  "device-logout",
			"token":      "logout-rt-2",
			"expires_at": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
			"used":       false,
			"family_id":  "logout-family-1",
		})

		logoutBody := `{"refresh_token":"logout-rt-1"}`
		req = httptest.NewRequest("POST", "/auth/logout", strings.NewReader(logoutBody))
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{
			"sub": "user-logout",
			"dev": "device-logout",
			"jti": "jti-logout-1",
			"exp": float64(time.Now().Add(time.Hour).Unix()),
		}))
		rr = httptest.NewRecorder()
		h.Logout(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		rows, err := s.Query(context.Background(), "RefreshToken").Where("family_id", "=", "logout-family-1").Execute(context.Background())
		assert.NoError(t, err)
		assert.Len(t, rows, 2)
		for _, row := range rows {
			assert.True(t, coerceBool(row["used"]), "logout should revoke all tokens in family")
		}
	})

	t.Run("Refresh_EdgeCases", func(t *testing.T) {
		// 1. Expired refresh token
		expiry := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		s.Create(context.Background(), "RefreshToken", map[string]any{
			"user_id":    "user-1",
			"token":      "expired-rt",
			"expires_at": expiry,
			"used":       false,
			"family_id":  "f1",
		})

		body := `{"refresh_token": "expired-rt"}`
		req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.Refresh(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "token_expired")

		// 2. Revoked family (reuse)
		s.Create(context.Background(), "RefreshToken", map[string]any{
			"user_id":    "user-1",
			"token":      "reused-rt",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
			"used":       true,
			"family_id":  "f2",
		})

		body = `{"refresh_token": "reused-rt"}`
		req = httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(body))
		rr = httptest.NewRecorder()
		h.Refresh(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assertAPIError(t, rr.Body.Bytes(), "refresh_token_reused")
	})
}

func TestSignupAnonymousUpgrade(t *testing.T) {
	s := storage.NewMemoryStore()
	jwt := auth.NewJWTService("test-secret")
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}},
		},
	}
	h := NewAuthHandler(s, jwt, jwt, reg)

	// 1. Create anonymous session
	body := `{"device_id": "test-device-1234"}`
	req := httptest.NewRequest("POST", "/auth/anonymous", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.AnonymousSession(rr, req)
	var anonRes map[string]any
	json.Unmarshal(rr.Body.Bytes(), &anonRes)
	anonToken := anonRes["token"].(string)

	// 2. Upgrade to full account
	signupBody := map[string]any{
		"email":           "upgraded@example.com",
		"password":        "Password123!",
		"anonymous_token": anonToken,
	}
	b, _ := json.Marshal(signupBody)
	req = httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(b))
	rr = httptest.NewRecorder()
	h.Signup(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var signupRes map[string]any
	json.Unmarshal(rr.Body.Bytes(), &signupRes)
	assert.Equal(t, "upgraded@example.com", signupRes["user"].(map[string]any)["email"])
}

func TestAuthHandler_constructorsAndOptions(t *testing.T) {
	s := storage.NewMemoryStore()
	jwt := auth.NewJWTService("secret")
	reg := &manifest.Registry{}
	h := NewAuthHandlerWithRegistry(s, jwt, jwt, reg)
	assert.NotNil(t, h)
	h2 := NewAuthHandler(s, jwt, jwt, reg).
		WithHooks(func(string, string, *http.Request, map[string]any) error { return nil }).
		WithRevocation(nil)
	assert.NotNil(t, h2)
}

func TestAuthHandler_Link(t *testing.T) {
	s := storage.NewMemoryStore()
	jwtSvc := auth.NewJWTService("test-secret")
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "User"}},
			{Metadata: manifest.Metadata{Name: "Note"}},
		},
	}
	h := NewAuthHandler(s, jwtSvc, jwtSvc, reg)

	createGuest := func(deviceID string) (string, string) {
		body := `{"device_id": "` + deviceID + `"}`
		req := httptest.NewRequest("POST", "/auth/anonymous", strings.NewReader(body))
		rr := httptest.NewRecorder()
		h.AnonymousSession(rr, req)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		token := res["token"].(string)
		claims, _ := jwtSvc.ValidateToken(context.Background(), token)
		userID := claims["sub"].(string)
		return userID, token
	}

	t.Run("Link upgrades guest user (new identity)", func(t *testing.T) {
		guestID, token := createGuest("dev-link-1")

		linkBody := map[string]any{
			"provider":       "google",
			"identity_token": "MOCK_TOKEN_google_google-sub-1_Alice",
		}
		b, _ := json.Marshal(linkBody)
		req := httptest.NewRequest("POST", "/auth/link", bytes.NewBuffer(b))
		claims, _ := jwtSvc.ValidateToken(context.Background(), token)
		req = req.WithContext(middleware.WithClaims(req.Context(), claims))
		rr := httptest.NewRecorder()

		h.Link(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "linked", res["status"])
		assert.NotEmpty(t, res["token"])

		userRow, err := s.Get(context.Background(), "User", guestID)
		assert.NoError(t, err)
		assert.Equal(t, "user", userRow["role"])
		assert.Equal(t, "google-sub-1@example.com", userRow["email"])
		assert.Equal(t, "Alice", userRow["name"])
		assert.Equal(t, "google-sub-1", userRow["google_sub"])
	})

	t.Run("Link merges guest into existing account (existing identity)", func(t *testing.T) {
		regUser, _ := s.Create(context.Background(), "User", map[string]any{
			"email": "existing@example.com",
			"role":  "user",
			"name":  "Registered Bob",
		})
		regUserID := fmt.Sprintf("%v", regUser["id"])

		guestID, token := createGuest("dev-link-2")

		noteVal, _ := s.Create(context.Background(), "Note", map[string]any{
			"title":      "Guest Note",
			"created_by": guestID,
		})
		noteID := fmt.Sprintf("%v", noteVal["id"])

		linkBody := map[string]any{
			"provider":       "google",
			"identity_token": "MOCK_TOKEN_google_existing_Bob",
			"email":          "existing@example.com",
		}
		b, _ := json.Marshal(linkBody)
		req := httptest.NewRequest("POST", "/auth/link", bytes.NewBuffer(b))
		claims, _ := jwtSvc.ValidateToken(context.Background(), token)
		req = req.WithContext(middleware.WithClaims(req.Context(), claims))
		rr := httptest.NewRecorder()

		h.Link(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "linked", res["status"])
		assert.Equal(t, regUserID, res["user"].(map[string]any)["id"])

		updatedNote, err := s.Get(context.Background(), "Note", noteID)
		assert.NoError(t, err)
		assert.Equal(t, regUserID, updatedNote["created_by"])

		_, err = s.Get(context.Background(), "User", guestID)
		assert.Error(t, err)
	})
}

