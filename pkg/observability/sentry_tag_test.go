//go:build sentry

package observability

import "testing"

func TestSentryBuildTag_Compiles(t *testing.T) {
	// Integration with sentry-go is compile-tested via `go test -tags=sentry ./...`.
}

func TestSentryInit_FailSoft(t *testing.T) {
	t.Setenv("BFFX_SENTRY_DSN", ":")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("InitSentryFromEnv panicked on invalid Sentry configuration: %v", r)
		}
	}()
	InitSentryFromEnv()
	if sentryInitialized {
		t.Errorf("expected Sentry to not be initialized successfully with invalid DSN configuration")
	}
}
