package middleware

import (
	"fmt"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// Recovery catches panics and returns a 500 Internal Server Error, logging the stack trace.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("PANIC RECOVERED: %v\n%s", err, logger.Stack())
				errors.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RecoveryWithStore catches panics, logs stack trace, writes an Incident record, and returns 500.
func RecoveryWithStore(store storage.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := logger.Stack()
					logger.Error("PANIC RECOVERED: %v\n%s", err, stack)

					if store != nil {
						_, dbErr := store.Create(r.Context(), "Incident", map[string]any{
							"title":       fmt.Sprintf("Panic: %v", err),
							"severity":    "Critical",
							"stack_trace": stack,
							"resolved":    false,
							"resolved_by": "",
						})
						if dbErr != nil {
							logger.Error("Failed to persist panic incident: %v", dbErr)
						}
					}

					errors.WriteError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
