package router

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/sse"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
)

func (r *Router) streamScreenHandler(m *manifest.Manifest) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var spec manifest.ScreenSpec
		_ = m.UnmarshalSpec(&spec)

		// Honour the kill switch even on streams.
		if r.killSwitch != nil {
			if ks, err := r.killSwitch.Get(req.Context(), m.Metadata.Name, ""); err == nil && ks != nil && ks.IsActiveNow() {
				writeKilledResponse(w, ks)
				return
			}
		}

		sseWriter, err := sse.NewWriter(w, sse.WithHeader("X-Accel-Buffering", "no"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		intervalMs := spec.StreamIntervalMs
		if intervalMs <= 0 {
			intervalMs = 5000
		}
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()

		var lastHash string

		emitOnce := func() bool {
			sourceData, errs := r.resolveSources(req, spec.Sources, spec.PartialSuccess)
			if len(errs) > 0 && !spec.PartialSuccess {
				body, _ := json.Marshal(map[string]any{"error": errs[0].Error()})
				_ = sseWriter.WriteEvent("error", body)
				return req.Context().Err() == nil
			}
			output := r.resolveOutput(req.Context(), sourceData, spec.Output)
			payload, err := json.Marshal(output)
			if err != nil {
				return req.Context().Err() == nil
			}
			h := sha256.Sum256(payload)
			hash := hex.EncodeToString(h[:])
			if hash == lastHash {
				_ = sseWriter.WriteComment("heartbeat " + hash)
				return req.Context().Err() == nil
			}
			lastHash = hash
			_ = sseWriter.WriteEventID(hash, "screen", payload)
			return req.Context().Err() == nil
		}

		if !emitOnce() {
			return
		}

		for {
			select {
			case <-req.Context().Done():
				return
			case <-ticker.C:
				if !emitOnce() {
					return
				}
			}
		}
	}
}

// writeKilledResponse emits the canonical 503 envelope when a screen-
// or section-level kill switch is active.
func writeKilledResponse(w http.ResponseWriter, ks *runtimecontracts.KillSwitch) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	body := map[string]any{
		"killed": true,
		"reason": ks.Reason,
	}
	if ks.ExpiresAt != nil {
		body["expires_at"] = ks.ExpiresAt
	}
	if ks.Section != "" {
		body["section"] = ks.Section
	}
	_ = json.NewEncoder(w).Encode(body)
}
