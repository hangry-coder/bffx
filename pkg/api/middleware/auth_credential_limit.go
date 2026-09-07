package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type credentialAttempt struct {
	Count       int
	LockedUntil time.Time
}

var credentialAttempts sync.Map

func authCredentialLimitsFromEnv() (maxAttempts int, lockout time.Duration) {
	maxAttempts = 5
	lockout = 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("BFFX_AUTH_LOGIN_MAX_ATTEMPTS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			maxAttempts = v
		}
	}
	if raw := strings.TrimSpace(os.Getenv("BFFX_AUTH_LOGIN_LOCKOUT_MINUTES")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			lockout = time.Duration(v) * time.Minute
		}
	}
	return maxAttempts, lockout
}

func checkCredentialRateLimit(ip, email string) (lockedUntil time.Time, ok bool) {
	now := time.Now()
	maxAttempts, _ := authCredentialLimitsFromEnv()
	keys := []string{"ip:" + ip}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		keys = append(keys, "email:"+e)
	}
	for _, key := range keys {
		if val, loaded := credentialAttempts.Load(key); loaded {
			attempt := val.(*credentialAttempt)
			if attempt.Count >= maxAttempts && now.Before(attempt.LockedUntil) {
				return attempt.LockedUntil, false
			}
		}
	}
	return time.Time{}, true
}

func recordCredentialFailure(ip, email string) {
	maxAttempts, lockout := authCredentialLimitsFromEnv()
	now := time.Now()
	keys := []string{"ip:" + ip}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		keys = append(keys, "email:"+e)
	}
	for _, key := range keys {
		var attempt *credentialAttempt
		if val, ok := credentialAttempts.Load(key); ok {
			attempt = val.(*credentialAttempt)
			if !attempt.LockedUntil.IsZero() && now.After(attempt.LockedUntil) {
				attempt.Count = 0
				attempt.LockedUntil = time.Time{}
			}
		} else {
			attempt = &credentialAttempt{}
		}
		attempt.Count++
		if attempt.Count >= maxAttempts {
			attempt.LockedUntil = now.Add(lockout)
		}
		credentialAttempts.Store(key, attempt)
	}
}

func recordCredentialSuccess(ip, email string) {
	for _, key := range []string{"ip:" + ip, "email:" + strings.ToLower(strings.TrimSpace(email))} {
		if key == "email:" {
			continue
		}
		credentialAttempts.Delete(key)
	}
	if e := strings.ToLower(strings.TrimSpace(email)); e != "" {
		credentialAttempts.Delete("email:" + e)
	}
}

func writeCredentialLocked(w http.ResponseWriter, lockedUntil time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":        "Too many login attempts. Try again later.",
		"code":         "rate_limit_exceeded",
		"locked_until": lockedUntil.Format(time.RFC3339),
	})
}

// AuthCredentialRateLimit applies per-IP and per-email lockout to login and signup POST handlers.
func AuthCredentialRateLimit(trustForwarded bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var peek struct {
				Email string `json:"email"`
			}
			bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			if len(bodyBytes) > 0 {
				_ = json.Unmarshal(bodyBytes, &peek)
			}

			ip := ClientIP(r, trustForwarded)
			if lockedUntil, ok := checkCredentialRateLimit(ip, peek.Email); !ok {
				writeCredentialLocked(w, lockedUntil)
				return
			}

			rec := &credentialRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.failedAuth {
				recordCredentialFailure(ip, peek.Email)
			} else if rec.succeededAuth {
				recordCredentialSuccess(ip, peek.Email)
			}
		})
	}
}

type credentialRecorder struct {
	http.ResponseWriter
	failedAuth    bool
	succeededAuth bool
}

func (c *credentialRecorder) WriteHeader(code int) {
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		c.failedAuth = true
	}
	if code == http.StatusOK || code == http.StatusCreated {
		c.succeededAuth = true
	}
	c.ResponseWriter.WriteHeader(code)
}
