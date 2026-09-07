//go:build sentry

package observability

import (
	"net/http"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/hangry-coder/bffx/pkg/logger"
)

var sentryInitialized = false

func initSentryHook() {
	dsn := strings.TrimSpace(os.Getenv("BFFX_SENTRY_DSN"))
	if dsn == "" {
		return
	}
	opts := sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      os.Getenv("BFFX_ENV"),
		Release:          os.Getenv("BFFX_RELEASE"),
		AttachStacktrace: true,
		TracesSampleRate: 0,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Drop common PII fields if they appear on extra contexts.
			if event.Request != nil {
				event.Request.Cookies = ""
			}
			return event
		},
	}
	if err := sentry.Init(opts); err != nil {
		logger.Warn("Failed to initialize Sentry: %v. Continuing without Sentry crash reporting.", err)
		return
	}
	sentryInitialized = true
}

func httpRecoverHook(next http.Handler) http.Handler {
	if !sentryInitialized {
		return next
	}
	sh := sentryhttp.New(sentryhttp.Options{Repanic: true})
	return sh.Handle(next)
}
