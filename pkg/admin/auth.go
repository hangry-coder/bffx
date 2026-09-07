package admin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"golang.org/x/crypto/bcrypt"
)

const SessionCookieName = "bffx_admin_session"

type loginAttempt struct {
	Count       int
	LockedUntil time.Time
}

type AuthHandler struct {
	store    storage.Store
	reg      *manifest.Registry
	attempts sync.Map
}

func NewAuthHandler(store storage.Store, reg *manifest.Registry) *AuthHandler {
	return &AuthHandler{
		store: store,
		reg:   reg,
	}
}

func (h *AuthHandler) sessionPolicy() manifest.AdminSessionPolicy {
	if h.reg != nil && len(h.reg.AdminSites) > 0 {
		var spec manifest.AdminSiteSpec
		if err := h.reg.AdminSites[0].UnmarshalSpec(&spec); err == nil {
			return manifest.ResolveAdminSession(spec.Session)
		}
	}
	return manifest.ResolveAdminSession(manifest.AdminSiteSession{})
}

func getSessionSecret() []byte {
	secret := os.Getenv("BFFX_ADMIN_SESSION_KEY")
	if secret == "" {
		if os.Getenv("BFFX_ENV") == "production" {
			logger.Fatal("CRITICAL: BFFX_ADMIN_SESSION_KEY must be set in production")
		}
		secret = "default-admin-session-secret-change-me-in-production"
	}
	return []byte(secret)
}

func signValue(value string) string {
	mac := hmac.New(sha256.New, getSessionSecret())
	mac.Write([]byte(value))
	signature := hex.EncodeToString(mac.Sum(nil))
	return value + "." + signature
}

func verifySignature(signedValue string) (string, bool) {
	lastDot := strings.LastIndex(signedValue, ".")
	if lastDot == -1 {
		return "", false
	}
	value := signedValue[:lastDot]
	expectedSignature := signedValue[lastDot+1:]

	mac := hmac.New(sha256.New, getSessionSecret())
	mac.Write([]byte(value))
	actualSignature := hex.EncodeToString(mac.Sum(nil))

	if hmac.Equal([]byte(actualSignature), []byte(expectedSignature)) {
		return value, true
	}
	return "", false
}

const (
	adminEmailKey = "admin_email"
	adminIdKey    = "admin_id"
	adminRoleKey  = "admin_role"
)

func adminRoleAllowed(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "superadmin", "superuser", "operator":
		return true
	default:
		return false
	}
}

func (h *AuthHandler) authenticateCookie(cookieValue string) (email string, ok bool) {
	email, _, expires, idleDeadline, ok := parseSessionPayload(cookieValue)
	if !ok {
		return "", false
	}
	if sessionPayloadExpired(expires, idleDeadline) {
		return "", false
	}
	return email, true
}

func AuthMiddleware(store storage.Store, reg *manifest.Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h := &AuthHandler{store: store, reg: reg}
		email, ok := h.authenticateCookie(cookie.Value)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		admin, err := store.GetByField(r.Context(), "AdminUser", "email", email)
		if err != nil {
			admin, err = store.GetByField(r.Context(), "AdminUser", "email", strings.ToLower(email))
		}
		if err != nil {
			logger.Error("[AUTH] Admin account not found for: %s", email)
			http.Error(w, "Unauthorized: Admin account revoked", http.StatusUnauthorized)
			return
		}

		role, _ := admin["role"].(string)
		if !adminRoleAllowed(role) {
			http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), adminEmailKey, email)
		if id, ok := admin["id"].(string); ok {
			ctx = context.WithValue(ctx, adminIdKey, id)
		}
		ctx = context.WithValue(ctx, adminRoleKey, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	email, _ := r.Context().Value(adminEmailKey).(string)
	role, _ := r.Context().Value(adminRoleKey).(string)
	pol := h.sessionPolicy()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"authenticated":        true,
		"email":                  email,
		"role":                   role,
		"max_age_seconds":        pol.MaxAgeSeconds,
		"idle_timeout_seconds":   pol.IdleTimeoutSeconds,
		"dev_unlimited":          pol.DevUnlimited,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	isSecure := r.TLS != nil || (os.Getenv("BFFX_ENV") == "production" && !strings.HasPrefix(r.Host, "localhost") && !strings.HasPrefix(r.Host, "127.0.0.1"))
	// #nosec G124
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// adminLoginRateLimitEnabled is on in production unless BFFX_ADMIN_LOGIN_RATE_LIMIT overrides.
func adminLoginRateLimitEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BFFX_ADMIN_LOGIN_RATE_LIMIT"))) {
	case "0", "off", "false", "disabled":
		return false
	case "1", "on", "true", "enabled":
		return true
	}
	return os.Getenv("BFFX_ENV") == "production"
}

func (h *AuthHandler) checkRateLimit(ip string, email string) (time.Time, bool) {
	now := time.Now()
	for _, key := range []string{ip, email} {
		if val, ok := h.attempts.Load(key); ok {
			attempt := val.(*loginAttempt)
			if now.Before(attempt.LockedUntil) {
				return attempt.LockedUntil, false
			}
		}
	}
	return time.Time{}, true
}

func (h *AuthHandler) recordFailure(ip string, email string) {
	now := time.Now()
	for _, key := range []string{ip, email} {
		var attempt *loginAttempt
		if val, ok := h.attempts.Load(key); ok {
			attempt = val.(*loginAttempt)
		} else {
			attempt = &loginAttempt{}
		}

		attempt.Count++
		if attempt.Count >= 5 {
			attempt.LockedUntil = now.Add(15 * time.Minute)
		} else {
			attempt.LockedUntil = now
		}
		h.attempts.Store(key, attempt)
	}
}

func (h *AuthHandler) recordSuccess(ip string, email string) {
	for _, key := range []string{ip, email} {
		h.attempts.Delete(key)
	}
}

func clientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	if strings.Contains(ip, ":") {
		if host, _, err := net.SplitHostPort(ip); err == nil {
			return host
		}
	}
	return ip
}

func (h *AuthHandler) issueSessionCookie(w http.ResponseWriter, r *http.Request, email string) {
	pol := h.sessionPolicy()
	now := time.Now()
	var expiresUnix int64
	var idleUnix int64
	if pol.AbsoluteTTL > 0 {
		expiresUnix = now.Add(pol.AbsoluteTTL).Unix()
	}
	if pol.IdleTimeout > 0 {
		idleUnix = now.Add(pol.IdleTimeout).Unix()
	}
	payload := formatSessionPayload(email, now.Unix(), expiresUnix, idleUnix)
	signedValue := signValue(payload)

	isSecure := r.TLS != nil || (os.Getenv("BFFX_ENV") == "production" && !strings.HasPrefix(r.Host, "localhost") && !strings.HasPrefix(r.Host, "127.0.0.1"))

	maxAge := int(pol.CookieMaxAge.Seconds())
	if maxAge <= 0 {
		maxAge = -1
	}
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    signedValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
	if expiresUnix > 0 {
		cookie.Expires = time.Unix(expiresUnix, 0)
	}
	// #nosec G124
	http.SetCookie(w, cookie)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ip := clientIP(r)
	rateLimit := adminLoginRateLimitEnabled()
	if rateLimit {
		if lockedUntil, ok := h.checkRateLimit(ip, payload.Email); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":        "Too many login attempts. Account temporarily locked.",
				"locked_until": lockedUntil.Format(time.RFC3339),
			})
			return
		}
	}

	admin, err := h.store.GetByField(r.Context(), "AdminUser", "email", payload.Email)
	if err != nil {
		admin, err = h.store.GetByField(r.Context(), "AdminUser", "email", strings.ToLower(payload.Email))
	}

	if err != nil {
		logger.Error("[AUTH] AdminUser not found: %s", payload.Email)
		if rateLimit {
			h.recordFailure(ip, payload.Email)
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	storedPassword, _ := admin["password"].(string)

	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(payload.Password))
	if err != nil {
		logger.Error("[AUTH] Password mismatch for %s", payload.Email)
		if rateLimit {
			h.recordFailure(ip, payload.Email)
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	role, _ := admin["role"].(string)
	if !adminRoleAllowed(role) {
		if rateLimit {
			h.recordFailure(ip, payload.Email)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "This account does not have admin panel access (role: " + role + "). Use a superadmin, admin, or operator account.",
		})
		return
	}

	if rateLimit {
		h.recordSuccess(ip, payload.Email)
	}

	email, _ := admin["email"].(string)
	if email == "" {
		email = payload.Email
	}
	h.issueSessionCookie(w, r, email)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"user": map[string]any{
			"email": admin["email"],
			"role":  admin["role"],
		},
	})
}
