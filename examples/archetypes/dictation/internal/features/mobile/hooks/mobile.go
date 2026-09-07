package hooks

import (
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"net/http"
)

// HandleCompleteProfile handles complete profile onboarding actions.
func HandleCompleteProfile(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}