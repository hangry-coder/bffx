package manifest

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const adminDevUnlimitedMaxAge = 365 * 24 * time.Hour

// AdminSessionPolicy holds effective session policy for login, middleware, and the SPA.
type AdminSessionPolicy struct {
	DevUnlimited       bool
	AbsoluteTTL        time.Duration
	IdleTimeout        time.Duration
	CookieMaxAge       time.Duration
	MaxAgeSeconds      int
	IdleTimeoutSeconds int
}

// ResolveAdminSession merges AdminSite session spec, environment, and BFFX_ENV defaults.
func ResolveAdminSession(site AdminSiteSession) AdminSessionPolicy {
	r := AdminSessionPolicy{
		DevUnlimited: true,
		AbsoluteTTL:  0,
		IdleTimeout:  0,
		CookieMaxAge: adminDevUnlimitedMaxAge,
	}

	if site.DevUnlimited != nil {
		r.DevUnlimited = *site.DevUnlimited
	}
	if v := strings.TrimSpace(os.Getenv("BFFX_ADMIN_SESSION_DEV_UNLIMITED")); v != "" {
		r.DevUnlimited = v == "1" || strings.EqualFold(v, "true")
	}

	isProd := os.Getenv("BFFX_ENV") == "production"
	if isProd {
		r.DevUnlimited = false
	}

	if d := parseAdminDuration(site.MaxAge); d > 0 {
		r.AbsoluteTTL = d
		r.CookieMaxAge = d
	}
	if d := parseAdminDuration(site.IdleTimeout); d > 0 {
		r.IdleTimeout = d
	}
	if d := parseAdminDuration(os.Getenv("BFFX_ADMIN_SESSION_MAX_AGE")); d > 0 {
		r.AbsoluteTTL = d
		r.CookieMaxAge = d
	}
	if d := parseAdminDuration(os.Getenv("BFFX_ADMIN_SESSION_IDLE")); d > 0 {
		r.IdleTimeout = d
	}

	if r.DevUnlimited && !isProd {
		r.AbsoluteTTL = 0
		r.CookieMaxAge = adminDevUnlimitedMaxAge
	} else {
		if r.AbsoluteTTL == 0 {
			r.AbsoluteTTL = 24 * time.Hour
			r.CookieMaxAge = 24 * time.Hour
		}
		if r.IdleTimeout == 0 && isProd {
			r.IdleTimeout = 30 * time.Minute
		}
	}

	r.MaxAgeSeconds = int(r.CookieMaxAge.Seconds())
	if r.DevUnlimited && !isProd && r.AbsoluteTTL == 0 {
		r.MaxAgeSeconds = 0
	}
	if r.IdleTimeout > 0 {
		r.IdleTimeoutSeconds = int(r.IdleTimeout.Seconds())
	}
	return r
}

func parseAdminDuration(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}
