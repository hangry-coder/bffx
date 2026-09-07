//go:build !sentry

package observability

import "net/http"

func initSentryHook() {}

func httpRecoverHook(next http.Handler) http.Handler {
	return next
}
