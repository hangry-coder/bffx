package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage/blob"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// UploadsHandler exposes presigned upload/download URLs when a blob.Provider is configured.
type UploadsHandler struct {
	blob  blob.Provider
	local *blob.LocalProvider
	reg   *manifest.Registry
}

func NewUploadsHandler(p blob.Provider, reg *manifest.Registry) *UploadsHandler {
	h := &UploadsHandler{blob: p, reg: reg}
	if lp, ok := p.(*blob.LocalProvider); ok {
		h.local = lp
	}
	return h
}

// MountLocal registers PUT/GET handlers for LocalPresigner URLs (no-op for S3-only configs).
func (h *UploadsHandler) MountLocal(mux *http.ServeMux) {
	if h.local == nil {
		return
	}
	mux.HandleFunc("PUT /api/v1/uploads/local", h.local.ServePut)
	mux.HandleFunc("GET /api/v1/uploads/local", h.local.ServeGet)
}

func requestBaseURL(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = "127.0.0.1"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return strings.TrimSpace(p) + "://" + host
	}
	if r.TLS != nil {
		return "https://" + host
	}
	return "http://" + host
}

// Presign issues a time-limited URL for direct client upload (op=put) or download (op=get).
func (h *UploadsHandler) Presign(w http.ResponseWriter, r *http.Request) {
	if h.blob == nil {
		errors.WriteError(w, http.StatusNotImplemented, "uploads_disabled", "feature_disabled")
		return
	}
	var body struct {
		Key        string `json:"key"`
		Resource   string `json:"resource"`
		Field      string `json:"field"`
		Op         string `json:"op"`
		TTLSeconds int    `json:"ttl_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errors.WriteError(w, http.StatusBadRequest, "invalid_json", "bad_request")
		return
	}
	if err := blob.ValidateObjectKey(body.Key); err != nil {
		errors.WriteError(w, http.StatusBadRequest, err.Error(), "invalid_key")
		return
	}
	if h.reg != nil {
		if err := blob.ValidateUploadManifestField(h.reg, body.Resource, body.Field); err != nil {
			errors.WriteError(w, http.StatusBadRequest, err.Error(), "invalid_field")
			return
		}
	}

	ttl := 15 * time.Minute
	if body.TTLSeconds > 0 && body.TTLSeconds <= 86400 {
		ttl = time.Duration(body.TTLSeconds) * time.Second
	}
	base := requestBaseURL(r)
	op := strings.ToLower(strings.TrimSpace(body.Op))
	if op == "" {
		op = "put"
	}

	ctx := r.Context()
	var res blob.PresignResult
	var err error
	switch op {
	case "put":
		res, err = h.blob.PresignPut(ctx, base, body.Key, ttl)
	case "get":
		res, err = h.blob.PresignGet(ctx, base, body.Key, ttl)
	default:
		errors.WriteError(w, http.StatusBadRequest, "op must be put or get", "invalid_operation")
		return
	}
	if err != nil {
		errors.WriteError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	out := map[string]any{
		"url":        res.URL,
		"method":     res.Method,
		"expires_at": res.ExpiresAt.Format(time.RFC3339),
	}
	if len(res.Headers) > 0 {
		out["headers"] = res.Headers
	}
	if len(res.Fields) > 0 {
		out["fields"] = res.Fields
	}
	errors.WriteJSON(w, http.StatusOK, out)
}
