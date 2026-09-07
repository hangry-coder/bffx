package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const LocaleKey contextKey = "locale"

// Locale middleware extracts the locale from the Accept-Language header.
func Locale(defaultLocale string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := r.Header.Get("Accept-Language")
			if locale == "" {
				locale = defaultLocale
			} else {
				// Extract primary language if multiple (e.g. "en-US,en;q=0.9" -> "en")
				parts := strings.Split(locale, ",")
				if len(parts) > 0 {
					subparts := strings.Split(parts[0], ";")
					if len(subparts) > 0 {
						locale = strings.Split(subparts[0], "-")[0]
					}
				}
			}

			if locale == "" {
				locale = defaultLocale
			}

			ctx := context.WithValue(r.Context(), LocaleKey, strings.ToLower(strings.TrimSpace(locale)))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithLocale returns a child context that has the locale set to the supplied
// value. It is used by the admin Inspect ("Run as user") flow which needs to
// synthesize a request context without going through the Locale middleware.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, LocaleKey, strings.ToLower(strings.TrimSpace(locale)))
}

// GetLocale retrieves the locale from the context.
func GetLocale(ctx context.Context) string {
	if ctx == nil {
		return "en"
	}
	if v, ok := ctx.Value(LocaleKey).(string); ok {
		return v
	}
	return "en"
}
