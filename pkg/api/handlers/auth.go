package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/auth/revocation"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// AnonymousSessionTTL is the JWT lifetime for tokens from POST /api/v1/auth/anonymous.
const AnonymousSessionTTL = 720 * time.Hour // 30 days

type AuthHandler struct {
	store   storage.Store
	auth    auth.Provider
	jwt     *auth.JWTService   // optional; used for GenerateToken in Builtin mode
	reg     *manifest.Registry // optional; used to merge anonymous-owned rows on login
	hooks   func(resource string, event string, req *http.Request, data map[string]any) error
	checker *revocation.RedisChecker
}

func (h *AuthHandler) WithRevocation(c *revocation.RedisChecker) *AuthHandler {
	h.checker = c
	return h
}

func NewAuthHandler(store storage.Store, authProvider auth.Provider, jwt *auth.JWTService, reg *manifest.Registry) *AuthHandler {
	return &AuthHandler{store: store, auth: authProvider, jwt: jwt, reg: reg}
}

func (h *AuthHandler) WithHooks(executor func(resource string, event string, req *http.Request, data map[string]any) error) *AuthHandler {
	h.hooks = executor
	return h
}

// NewAuthHandlerWithRegistry attaches the manifest registry for anonymous→account merges on login.
func NewAuthHandlerWithRegistry(store storage.Store, authProvider auth.Provider, jwt *auth.JWTService, reg *manifest.Registry) *AuthHandler {
	return &AuthHandler{store: store, auth: authProvider, jwt: jwt, reg: reg}
}

type PublicUser struct {
	ID               string   `json:"id"`
	Email            string   `json:"email,omitempty"`
	Name             string   `json:"name,omitempty"`
	DisplayName      string   `json:"display_name,omitempty"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	Role             string   `json:"role,omitempty"`
	Roles            []string `json:"roles,omitempty"`
	Status           string   `json:"status,omitempty"`
	DeviceID         string   `json:"device_id,omitempty"`
	SubscriptionTier string   `json:"subscription_tier,omitempty"`
	CreatedAt        string   `json:"created_at,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

// PublicUserFields is the explicit allowlist of User-row keys safe to expose
// over the API. Any new sensitive field added to the User manifest is
// **not** exposed by default — that's the whole point of an allowlist.
//
// Also exposed via SanitizeUser (map form) for back-compat with hooks/plugins
// that historically called the older helper.
var PublicUserFields = []string{
	"id",
	"email",
	"name",
	"display_name",
	"avatar_url",
	"role",
	"roles",
	"status",
	"device_id",
	"subscription_tier",
	"created_at",
	"updated_at",
}

// SanitizeUser projects a User row down to PublicUserFields. Returns a map
// (rather than the PublicUser struct) so existing call-sites that JSON-encode
// arbitrary maps keep working. New code should prefer ToPublicUser.
func SanitizeUser(user map[string]any) map[string]any {
	if user == nil {
		return nil
	}
	out := make(map[string]any, len(PublicUserFields))
	for _, k := range PublicUserFields {
		if v, ok := user[k]; ok {
			out[k] = v
		}
	}
	return out
}

func ToPublicUser(m map[string]any) PublicUser {
	if m == nil {
		return PublicUser{}
	}
	u := PublicUser{
		ID:               fmt.Sprintf("%v", m["id"]),
		Email:            getString(m, "email"),
		Name:             getString(m, "name"),
		DisplayName:      getString(m, "display_name"),
		AvatarURL:        getString(m, "avatar_url"),
		Role:             getString(m, "role"),
		Status:           getString(m, "status"),
		DeviceID:         getString(m, "device_id"),
		SubscriptionTier: getString(m, "subscription_tier"),
		CreatedAt:        getString(m, "created_at"),
		UpdatedAt:        getString(m, "updated_at"),
	}
	if roles, ok := m["roles"].([]any); ok {
		for _, r := range roles {
			u.Roles = append(u.Roles, fmt.Sprintf("%v", r))
		}
	} else if roles, ok := m["roles"].([]string); ok {
		u.Roles = roles
	}
	return u
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// coerceBool reads a "boolean" field from a row map. SQLite scans INTEGER
// columns as int64, postgres scans booleans as bool, JSON round-trips can
// produce float64. Treat any non-zero numeric, the bool true, or the string
// "1"/"true" as truthy. Anything else (nil, missing, false-equivalent) is
// false.
func coerceBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int32:
		return x != 0
	case int64:
		return x != 0
	case float32:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x == "1" || strings.EqualFold(x, "true")
	}
	return false
}

func coerceUnixSeconds(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float32:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func ensureGuestUser(ctx context.Context, st storage.Store, deviceID string) (map[string]any, error) {
	user, err := st.GetByField(ctx, "User", "device_id", deviceID)
	if err == nil {
		return user, nil
	}
	newID := uuid.New().String()
	return st.Create(ctx, "User", map[string]any{
		"id":         newID,
		"created_by": newID,
		"device_id":  deviceID,
		"name":       "Guest " + middleware.SafePrefixRunes(deviceID, 4),
		"role":       "guest",
		"status":     "active",
	})
}

func claimAnonymous(c map[string]any) bool {
	v, ok := c["anon"]
	if !ok || v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func (h *AuthHandler) AnonymousSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	var body struct {
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	deviceID := strings.TrimSpace(body.DeviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(r.Header.Get("X-Device-ID"))
	}
	if deviceID == "" {
		errors.Write(w, errors.New(http.StatusBadRequest, "device_id required (body or X-Device-ID)", "validation_error"))
		return
	}
	if !middleware.ValidDeviceID(deviceID) {
		errors.Write(w, errors.New(http.StatusBadRequest, "invalid device_id", "validation_error"))
		return
	}

	// Dynamic check for guest signups from configuration store
	if cfg, err := h.store.Query(r.Context(), "AppConfig").Where("config_key", "=", "disable_guest_signup").Execute(r.Context()); err == nil && len(cfg) > 0 {
		if val, _ := cfg[0]["config_value"].(string); val == "true" {
			errors.Write(w, errors.New(http.StatusForbidden, "Guest sign-up is currently disabled.", "guest_signup_disabled"))
			return
		}
	}

	user, err := ensureGuestUser(r.Context(), h.store, deviceID)
	if err != nil {
		observability.AuthAttemptsTotal.WithLabelValues("anonymous", "error").Inc()
		errors.Write(w, errors.New(http.StatusInternalServerError, "failed to provision guest", "internal_error"))
		return
	}

	userID := fmt.Sprintf("%v", user["id"])
	fingerprint := r.Header.Get("User-Agent")

	if h.jwt == nil {
		errors.Write(w, errors.New(http.StatusNotImplemented, "anonymous sessions requires builtin auth battery", "not_implemented"))
		return
	}
	token, err := h.jwt.GenerateToken(userID, "guest", deviceID, fingerprint, true, AnonymousSessionTTL)
	if err != nil {
		observability.AuthAttemptsTotal.WithLabelValues("anonymous", "error").Inc()
		errors.Write(w, errors.New(http.StatusInternalServerError, "failed to issue token", "internal_error"))
		return
	}

	observability.AuthAttemptsTotal.WithLabelValues("anonymous", "success").Inc()

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(AnonymousSessionTTL.Seconds()),
		"user":       ToPublicUser(user),
	})
}

// Handshake reports whether this device_id is already known and whether an optional Bearer token is still valid.
// GET /api/v1/auth/handshake (legacy: /api/v1/app/handshake) — Auth middleware skips anonymous auto-provision on these paths.
func (h *AuthHandler) Handshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	deviceID := strings.TrimSpace(r.Header.Get("X-Device-ID"))
	if deviceID == "" || !middleware.ValidDeviceID(deviceID) {
		errors.Write(w, errors.New(http.StatusBadRequest, "valid X-Device-ID header required", "validation_error"))
		return
	}
	_, err := h.store.GetByField(r.Context(), "User", "device_id", deviceID)
	deviceKnown := (err == nil)

	out := map[string]any{
		"device_known": deviceKnown,
		"server_time":  time.Now().UTC().Format(time.RFC3339),
	}
	authHeader := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if authHeader == "" {
		errors.WriteJSON(w, http.StatusOK, out)
		return
	}
	claims, err := h.auth.ValidateToken(r.Context(), authHeader)
	if err != nil {
		out["token_valid"] = false
		errors.WriteJSON(w, http.StatusOK, out)
		return
	}
	sub := h.auth.UserIDFromClaims(claims)
	user, err := h.store.Get(r.Context(), "User", sub)
	tokenValid := (err == nil && user != nil)
	if tokenValid {
		if dev, ok := claims["dev"].(string); ok && dev != "" && dev != deviceID {
			tokenValid = false
		}
	}
	out["token_valid"] = tokenValid
	if tokenValid {
		out["user"] = ToPublicUser(user)
	}
	errors.WriteJSON(w, http.StatusOK, out)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	var payload struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Name           string `json:"name"`
		AnonymousToken string `json:"anonymous_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	if payload.Email == "" || payload.Password == "" {
		errors.Write(w, errors.New(http.StatusBadRequest, "email and password required", "validation_error"))
		return
	}

	if len(payload.Password) < 8 {
		errors.Write(w, errors.New(http.StatusBadRequest, "password must be at least 8 characters", "validation_error"))
		return
	}

	// Upgrade existing guest row using anonymous JWT (same user id, full account).
	if payload.AnonymousToken != "" {
		claims, err := h.auth.ValidateToken(r.Context(), payload.AnonymousToken)
		if err != nil || !claimAnonymous(claims) {
			observability.AuthAttemptsTotal.WithLabelValues("signup_upgrade", "unauthorized").Inc()
			errors.Write(w, errors.New(http.StatusUnauthorized, "invalid anonymous_token", "validation_error"))
			return
		}
		guestID := h.auth.UserIDFromClaims(claims)
		guest, err := h.store.Get(r.Context(), "User", guestID)
		if err != nil {
			errors.Write(w, errors.ErrUnauthorized)
			return
		}
		if role, _ := guest["role"].(string); role != "guest" {
			errors.Write(w, errors.New(http.StatusBadRequest, "anonymous_token does not refer to a guest session", "validation_error"))
			return
		}
		if existing, err := h.store.GetByField(r.Context(), "User", "email", payload.Email); err == nil {
			if fmt.Sprintf("%v", existing["id"]) != guestID {
				observability.AuthAttemptsTotal.WithLabelValues("signup_upgrade", "conflict").Inc()
				errors.Write(w, errors.New(http.StatusConflict, "email already registered", "validation_error"))
				return
			}
		}

		hash, err := auth.HashPassword(payload.Password)
		if err != nil {
			errors.Write(w, errors.New(http.StatusInternalServerError, "could not hash password", "internal_error"))
			return
		}

		updated, err := h.store.Update(r.Context(), "User", guestID, map[string]any{
			"email":    payload.Email,
			"password": hash,
			"name":     payload.Name,
			"role":     "user",
		})
		if err != nil || updated == nil {
			errors.Write(w, errors.New(http.StatusInternalServerError, "failed to upgrade account", "internal_error"))
			return
		}

		deviceID, _ := guest["device_id"].(string)
		if deviceID == "" {
			deviceID = r.Header.Get("X-Device-ID")
		}
		fp := r.Header.Get("User-Agent")
		tokens, err := h.issueTokens(r.Context(), guestID, "user", deviceID, fp)
		if err != nil {
			observability.AuthAttemptsTotal.WithLabelValues("signup_upgrade", "error").Inc()
			errors.Write(w, errors.New(http.StatusInternalServerError, "could not issue tokens", "internal_error"))
			return
		}

		observability.AuthAttemptsTotal.WithLabelValues("signup_upgrade", "success").Inc()

		errors.WriteJSON(w, http.StatusOK, map[string]any{
			"token":         tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"user":          ToPublicUser(updated),
		})
		return
	}

	if existing, err := h.store.GetByField(r.Context(), "User", "email", payload.Email); err == nil && existing != nil {
		observability.AuthAttemptsTotal.WithLabelValues("signup", "conflict").Inc()
		errors.Write(w, errors.New(http.StatusConflict, "email already registered", "validation_error"))
		return
	}

	hash, err := auth.HashPassword(payload.Password)
	if err != nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "could not hash password", "internal_error"))
		return
	}
	user := map[string]any{
		"email":    payload.Email,
		"password": hash,
		"name":     payload.Name,
	}

	created, err := h.store.Create(r.Context(), "User", user)
	if err != nil || created == nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "failed to create user", "internal_error"))
		return
	}

	if h.hooks != nil {
		h.hooks("user", "afterCreate", r, created)
	}

	observability.AuthAttemptsTotal.WithLabelValues("signup", "success").Inc()

	deviceID := r.Header.Get("X-Device-ID")
	fp := r.Header.Get("User-Agent")
	userID := fmt.Sprintf("%v", created["id"])
	tokens, err := h.issueTokens(r.Context(), userID, "user", deviceID, fp)
	if err != nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "could not issue tokens", "internal_error"))
		return
	}

	errors.WriteJSON(w, http.StatusCreated, map[string]any{
		"token":         tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"user":          ToPublicUser(created),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		DeviceID       string `json:"device_id"`
		AnonymousToken string `json:"anonymous_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	found, err := h.store.GetByField(r.Context(), "User", "email", payload.Email)
	if err != nil {
		observability.AuthAttemptsTotal.WithLabelValues("login", "unauthorized").Inc()
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	hash, _ := found["password"].(string)
	if !auth.CheckPasswordHash(payload.Password, hash) {
		observability.AuthAttemptsTotal.WithLabelValues("login", "unauthorized").Inc()
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	userID := fmt.Sprintf("%v", found["id"])
	role, _ := found["role"].(string)
	if role == "" {
		role = "user"
	}

	deviceID := payload.DeviceID
	if deviceID == "" {
		deviceID = r.Header.Get("X-Device-ID")
	}
	fingerprint := r.Header.Get("User-Agent")

	// Merge anonymous guest data into this account (existing registered user flow).
	if payload.AnonymousToken != "" && h.reg != nil {
		if claims, err := h.auth.ValidateToken(r.Context(), payload.AnonymousToken); err == nil && claimAnonymous(claims) {
			guestID := h.auth.UserIDFromClaims(claims)
			if guestID != "" && guestID != userID {
				storage.MergeGuestIntoAccount(r.Context(), h.store, h.reg, guestID, userID)
			}
		}
	}
	found, _ = h.store.Get(r.Context(), "User", userID)

	tokens, err := h.issueTokens(r.Context(), userID, role, deviceID, fingerprint)
	if err != nil {
		observability.AuthAttemptsTotal.WithLabelValues("login", "error").Inc()
		errors.Write(w, errors.New(http.StatusInternalServerError, "could not issue tokens", "internal_error"))
		return
	}

	observability.AuthAttemptsTotal.WithLabelValues("login", "success").Inc()

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"token":         tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"user":          ToPublicUser(found),
	})
}

// Link links an anonymous guest account with a third-party provider (Apple/Google).
func (h *AuthHandler) Link(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	guestID := h.auth.UserIDFromClaims(claims)
	if guestID == "" {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	guest, err := h.store.Get(r.Context(), "User", guestID)
	if err != nil {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}
	if role, _ := guest["role"].(string); role != "guest" {
		errors.Write(w, errors.New(http.StatusBadRequest, "session is not an anonymous guest session", "validation_error"))
		return
	}

	var payload struct {
		Provider      string `json:"provider"`
		IdentityToken string `json:"identity_token"`
		Email         string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}

	if payload.Provider == "" || payload.IdentityToken == "" {
		errors.Write(w, errors.New(http.StatusBadRequest, "provider and identity_token are required", "validation_error"))
		return
	}

	ext := auth.GetExternalProvider()
	identity, err := ext.ValidateIdentityToken(r.Context(), payload.Provider, payload.IdentityToken)
	if err != nil {
		errors.Write(w, errors.New(http.StatusUnauthorized, "invalid identity token: "+err.Error(), "validation_error"))
		return
	}

	email := identity.Email
	if email == "" {
		email = payload.Email
	}
	email = strings.TrimSpace(strings.ToLower(email))

	// Look up if a registered user already exists with this email
	var existing map[string]any
	if email != "" {
		existing, _ = h.store.GetByField(r.Context(), "User", "email", email)
	}

	if existing != nil {
		existingID := fmt.Sprintf("%v", existing["id"])
		if existingID != guestID {
			if h.reg != nil {
				storage.MergeGuestIntoAccount(r.Context(), h.store, h.reg, guestID, existingID)
			}
		}

		deviceID, _ := guest["device_id"].(string)
		if deviceID == "" {
			deviceID = r.Header.Get("X-Device-ID")
		}
		fp := r.Header.Get("User-Agent")

		tokens, err := h.issueTokens(r.Context(), existingID, "user", deviceID, fp)
		if err != nil {
			errors.Write(w, errors.New(http.StatusInternalServerError, "failed to issue tokens", "internal_error"))
			return
		}

		updatedUser, _ := h.store.Get(r.Context(), "User", existingID)

		errors.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "linked",
			"user":   ToPublicUser(updatedUser),
			"token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
		})
		return
	}

	// Upgrade the guest user account to a full user
	updateFields := map[string]any{
		"role": "user",
	}
	if email != "" {
		updateFields["email"] = email
	}
	if identity.Name != "" {
		updateFields["name"] = identity.Name
	}
	providerField := payload.Provider + "_sub"
	updateFields[providerField] = identity.Sub

	updated, err := h.store.Update(r.Context(), "User", guestID, updateFields)
	if err != nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "failed to upgrade guest account", "internal_error"))
		return
	}

	deviceID, _ := guest["device_id"].(string)
	if deviceID == "" {
		deviceID = r.Header.Get("X-Device-ID")
	}
	fp := r.Header.Get("User-Agent")

	tokens, err := h.issueTokens(r.Context(), guestID, "user", deviceID, fp)
	if err != nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "failed to issue tokens", "internal_error"))
		return
	}

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "linked",
		"user":   ToPublicUser(updated),
		"token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	})
}


type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// hasRefreshTokenResource reports whether the registry knows a RefreshToken
// resource. Refresh-token persistence is opt-in per project (declare a
// RefreshToken manifest with at least: user_id, device_id, token, expires_at,
// used (bool), family_id (string)). When absent, issueTokens still issues
// access tokens and just returns an empty refresh token.
func (h *AuthHandler) hasRefreshTokenResource() bool {
	if h.reg == nil {
		return false
	}
	_, ok := h.reg.GetResource("RefreshToken")
	return ok
}

func (h *AuthHandler) issueTokens(ctx context.Context, userID, role, deviceID, fingerprint string) (*TokenPair, error) {
	return h.issueTokensInFamily(ctx, userID, role, deviceID, fingerprint, "")
}

// issueTokensInFamily mints a new access+refresh token pair. If familyID is
// non-empty, the new refresh token is added to that family (used by the
// rotation path); otherwise a fresh family is created and any prior
// per-(user,device) tokens are revoked.
func (h *AuthHandler) issueTokensInFamily(ctx context.Context, userID, role, deviceID, fingerprint, familyID string) (*TokenPair, error) {
	if h.jwt == nil {
		return nil, errors.New(http.StatusNotImplemented, "account token issuance requires builtin auth battery", "not_implemented")
	}
	accessToken, err := h.jwt.GenerateToken(userID, role, deviceID, fingerprint, false, 24*time.Hour)
	if err != nil {
		return nil, err
	}

	if !h.hasRefreshTokenResource() {
		return &TokenPair{AccessToken: accessToken}, nil
	}

	rowID, wireToken, tokenHash, err := auth.MintRefreshTokenCredential()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)

	if familyID == "" {
		// Fresh login → revoke any prior refresh tokens for this user+device.
		existing, err := h.store.Query(ctx, "RefreshToken").
			Where("user_id", "=", userID).
			Where("device_id", "=", deviceID).
			Execute(ctx)
		if err == nil {
			for _, rt := range existing {
				if id, ok := rt["id"].(string); ok {
					h.store.Delete(ctx, "RefreshToken", id)
				}
			}
		}
		familyID = uuid.New().String()
	}

	if _, err := h.store.Create(ctx, "RefreshToken", map[string]any{
		"id":         rowID,
		"user_id":    userID,
		"device_id":  deviceID,
		"token":      tokenHash,
		"expires_at": expiresAt,
		"used":       false,
		"family_id":  familyID,
	}); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: wireToken,
	}, nil
}

// revokeFamily marks every RefreshToken row in the given family as used so any
// further refresh attempt with any token from that family fails as reuse.
func (h *AuthHandler) revokeFamily(ctx context.Context, familyID string) {
	if familyID == "" {
		return
	}
	family, err := h.store.Query(ctx, "RefreshToken").Where("family_id", "=", familyID).Execute(ctx)
	if err != nil {
		return
	}
	for _, t := range family {
		id, _ := t["id"].(string)
		if id == "" {
			continue
		}
		h.store.Update(ctx, "RefreshToken", id, map[string]any{"used": true})
	}
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if !h.hasRefreshTokenResource() {
		errors.Write(w, errors.New(http.StatusNotImplemented, "refresh tokens not configured for this project", "not_implemented"))
		return
	}

	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		errors.Write(w, errors.ErrBadRequest)
		return
	}
	if payload.RefreshToken == "" {
		errors.Write(w, errors.New(http.StatusBadRequest, "refresh_token required", "validation_error"))
		return
	}

	rt, err := resolveRefreshTokenRow(r.Context(), h.store, payload.RefreshToken)
	if err != nil {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	// REUSE DETECTION: if the supplied token has already been rotated, the
	// family is compromised. Mark every member of the family as used and
	// reject this request with a distinct error code so clients (and SOC
	// dashboards) can react.
	if coerceBool(rt["used"]) {
		familyID, _ := rt["family_id"].(string)
		h.revokeFamily(r.Context(), familyID)
		errors.Write(w, errors.New(http.StatusUnauthorized, "refresh token reused; family revoked", "refresh_token_reused"))
		return
	}

	expiresStr, _ := rt["expires_at"].(string)
	expiresAt, _ := time.Parse(time.RFC3339, expiresStr)
	if time.Now().After(expiresAt) {
		if id, ok := rt["id"].(string); ok {
			h.store.Update(r.Context(), "RefreshToken", id, map[string]any{"used": true})
		}
		errors.Write(w, errors.New(http.StatusUnauthorized, "refresh token expired", "token_expired"))
		return
	}

	userID := fmt.Sprintf("%v", rt["user_id"])
	deviceID := fmt.Sprintf("%v", rt["device_id"])
	fingerprint := r.Header.Get("User-Agent")

	user, err := h.store.Get(r.Context(), "User", userID)
	if err != nil {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}
	role, _ := user["role"].(string)

	// Mark the supplied token as used (preserve the row so a replay is
	// detectable) before issuing the rotated pair within the same family.
	if id, ok := rt["id"].(string); ok {
		h.store.Update(r.Context(), "RefreshToken", id, map[string]any{"used": true})
	}
	familyID, _ := rt["family_id"].(string)

	tokens, err := h.issueTokensInFamily(r.Context(), userID, role, deviceID, fingerprint, familyID)
	if err != nil {
		errors.Write(w, errors.New(http.StatusInternalServerError, "could not issue tokens", "internal_error"))
		return
	}

	errors.WriteJSON(w, http.StatusOK, map[string]any{
		"token":         tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		errors.Write(w, errors.ErrUnauthorized)
		return
	}

	var payload struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			errors.Write(w, errors.ErrBadRequest)
			return
		}
	}

	userID := h.auth.UserIDFromClaims(claims)
	deviceID, _ := claims["dev"].(string)
	h.revokeRefreshSession(r.Context(), userID, deviceID, payload.RefreshToken)

	jti, _ := claims["jti"].(string)
	exp := coerceUnixSeconds(claims["exp"])

	if jti != "" && exp > 0 && h.checker != nil {
		ttl := time.Unix(exp, 0).Sub(time.Now())
		if ttl > 0 {
			if err := h.checker.Revoke(r.Context(), jti, ttl); err != nil && os.Getenv("BFFX_ENV") == "production" {
				errors.Write(w, errors.New(http.StatusServiceUnavailable, "revocation backend unavailable", "revocation_unavailable"))
				return
			}
		}
	}

	errors.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) revokeRefreshSession(ctx context.Context, userID, deviceID, refreshToken string) {
	if !h.hasRefreshTokenResource() || userID == "" {
		return
	}
	revokedFamilies := make(map[string]struct{})
	revokeRow := func(row map[string]any) {
		if row == nil {
			return
		}
		if owner := fmt.Sprintf("%v", row["user_id"]); owner != "" && owner != userID {
			return
		}
		if familyID := fmt.Sprintf("%v", row["family_id"]); familyID != "" {
			if _, seen := revokedFamilies[familyID]; !seen {
				h.revokeFamily(ctx, familyID)
				revokedFamilies[familyID] = struct{}{}
			}
			return
		}
		if id := fmt.Sprintf("%v", row["id"]); id != "" {
			_, _ = h.store.Update(ctx, "RefreshToken", id, map[string]any{"used": true})
		}
	}

	if refreshToken != "" {
		if row, err := resolveRefreshTokenRow(ctx, h.store, refreshToken); err == nil {
			revokeRow(row)
		}
		return
	}

	if deviceID == "" {
		return
	}
	rows, err := h.store.Query(ctx, "RefreshToken").Where("user_id", "=", userID).Where("device_id", "=", deviceID).Execute(ctx)
	if err != nil {
		return
	}
	for _, row := range rows {
		revokeRow(row)
	}
}
