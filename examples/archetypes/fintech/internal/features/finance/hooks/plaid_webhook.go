package hooks

import (
	"fmt"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// HandlePlaidWebhook processes incoming mock webhooks from Plaid.
func HandlePlaidWebhook(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	fmt.Println("💰 Received mock Plaid transaction webhook!")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"processed","transactions_synced":12}`))
}
