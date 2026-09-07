package observability

import "net/http"

// InitSentryFromEnv initializes Sentry when the binary is built with `-tags sentry`
// and BFFX_SENTRY_DSN is set. Default builds are no-ops.
func InitSentryFromEnv() {
	initSentryHook()
}

// HTTPRecoverMiddleware wraps the root HTTP handler with panic reporting when
// built with `-tags sentry`; otherwise it returns next unchanged.
func HTTPRecoverMiddleware(next http.Handler) http.Handler {
	return httpRecoverHook(next)
}
