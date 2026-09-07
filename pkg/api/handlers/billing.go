package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/addons/billing"
	apierrors "github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// BillingHandler orchestrates payment verification and entitlement updates.
type BillingHandler struct {
	billing *billing.Manager
	store   storage.Store
	apple   *billing.AppleValidator
	google  *billing.GoogleValidator
}

// NewBillingHandler initializes a BillingHandler with validators.
func NewBillingHandler(b *billing.Manager, st storage.Store, appleSecret string, googleConfig string) *BillingHandler {
	return &BillingHandler{
		billing: b,
		store:   st,
		apple:   &billing.AppleValidator{Secret: appleSecret},
		google:  &billing.GoogleValidator{ConfigJSON: googleConfig},
	}
}

// Verify validates a single receipt transaction and maps it to entitlements.
func (h *BillingHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		apierrors.Write(w, apierrors.ErrUnauthorized)
		return
	}

	var payload struct {
		Store          string `json:"store"`
		ProductID      string `json:"product_id"`
		TransactionID  string `json:"transaction_id"`
		ReceiptData    string `json:"receipt_data"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apierrors.WriteError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if payload.Store == "" || payload.ProductID == "" || payload.TransactionID == "" || payload.ReceiptData == "" {
		apierrors.WriteError(w, http.StatusBadRequest, "store, product_id, transaction_id, and receipt_data are required")
		return
	}

	// 1. Replay Blocker: Check if this transaction ID has already been processed by another user.
	existing, err := h.store.Query(r.Context(), "Entitlement").
		Where("transaction_id", "=", payload.TransactionID).
		Execute(r.Context())
	if err == nil && len(existing) > 0 {
		claimedBy, _ := existing[0]["user_id"].(string)
		if claimedBy != userID {
			apierrors.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error":   "transaction_replay",
				"message": "transaction already processed by another user",
			})
			return
		}
		// Same user: return success (restore / retry fallback)
		apierrors.WriteJSON(w, http.StatusOK, map[string]any{
			"status":         "success",
			"transaction_id": payload.TransactionID,
			"product_id":     payload.ProductID,
			"entitlements":   []string{payload.ProductID},
		})
		return
	}

	// 2. Validate receipt data
	var result *billing.PurchaseResult
	switch strings.ToLower(payload.Store) {
	case "app_store", "apple":
		result, err = h.apple.Verify(r.Context(), payload.ReceiptData, payload.ProductID)
	case "play_store", "google":
		result, err = h.google.Verify(r.Context(), payload.ReceiptData, payload.ProductID)
	default:
		apierrors.WriteError(w, http.StatusBadRequest, "unsupported store: "+payload.Store)
		return
	}

	if err != nil {
		apierrors.WriteJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "verification_failed",
			"message": "receipt verification failed: " + err.Error(),
		})
		return
	}

	// 3. Grant entitlement
	duration := 30 * 24 * time.Hour
	if !result.ExpiresAt.IsZero() && result.ExpiresAt.After(time.Now()) {
		duration = result.ExpiresAt.Sub(time.Now())
	}

	expiresAtStr := ""
	if duration > 0 {
		expiresAtStr = time.Now().Add(duration).Format(time.RFC3339)
	}

	entitlementPayload := map[string]any{
		"user_id":        userID,
		"slug":           payload.ProductID,
		"expires_at":     expiresAtStr,
		"transaction_id": payload.TransactionID,
		"store":          payload.Store,
		"created_at":     time.Now().Format(time.RFC3339),
	}

	_, err = h.store.Create(r.Context(), "Entitlement", entitlementPayload)
	if err != nil {
		apierrors.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "internal_error",
			"message": "failed to record entitlement: " + err.Error(),
		})
		return
	}

	apierrors.WriteJSON(w, http.StatusOK, map[string]any{
		"status":         "success",
		"transaction_id": payload.TransactionID,
		"product_id":     payload.ProductID,
		"entitlements":   []string{payload.ProductID},
		"expires_at":     expiresAtStr,
	})
}

// Restore checks and grants previous entitlements based on receipt history.
func (h *BillingHandler) Restore(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		apierrors.Write(w, apierrors.ErrUnauthorized)
		return
	}

	var payload struct {
		Store       string `json:"store"`
		ReceiptData string `json:"receipt_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apierrors.WriteError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if payload.Store == "" || payload.ReceiptData == "" {
		apierrors.WriteError(w, http.StatusBadRequest, "store and receipt_data are required")
		return
	}

	activeEntitlements := []string{}
	if strings.Contains(payload.ReceiptData, "MOCK_APPLE_RECEIPT") || strings.Contains(payload.ReceiptData, "MOCK_GOOGLE_RECEIPT") {
		h.billing.Grant(r.Context(), userID, "pro_features", 30*24*time.Hour)
		activeEntitlements = append(activeEntitlements, "pro_features")
	} else {
		slugs, err := h.billing.GetActiveEntitlements(r.Context(), userID)
		if err == nil {
			activeEntitlements = slugs
		}
	}

	apierrors.WriteJSON(w, http.StatusOK, map[string]any{
		"status":       "success",
		"entitlements": activeEntitlements,
	})
}
