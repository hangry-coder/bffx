package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/sse"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/logger"
)

type StreamHandler struct {
	bus events.Bus
}

func NewStreamHandler(bus events.Bus) *StreamHandler {
	return &StreamHandler{bus: bus}
}

func (h *StreamHandler) HandleStream(channels []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			errors.WriteError(w, http.StatusInternalServerError, "Streaming unsupported", "STREAMING_UNSUPPORTED")
			return
		}

		sseWriter, err := sse.NewWriter(w, sse.WithAllowOrigin("*"))
		if err != nil {
			errors.WriteError(w, http.StatusInternalServerError, "Streaming unsupported", "STREAMING_UNSUPPORTED")
			return
		}

		logger.InfoCtx(r.Context(), "Client connected to stream on channels: %v", channels)

		eventChan := make(chan events.Event, 100)
		ctx := r.Context()

		for _, ch := range channels {
			go func(c string) {
				defer func() {
					if r := recover(); r != nil {
						logger.ErrorCtx(ctx, "Stream subscriber panicked: %v\n%s", r, logger.Stack())
					}
				}()
				sub := h.bus.Subscribe(ctx, c)
				for ev := range sub {
					select {
					case <-ctx.Done():
						return
					case eventChan <- ev:
					}
				}
			}(ch)
		}

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.InfoCtx(ctx, "Client disconnected from stream")
				return
			case ev := <-eventChan:
				data, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				_ = sseWriter.WriteRaw(data)
			case <-ticker.C:
				_ = sseWriter.WriteComment("ping")
			}
		}
	}
}

// HandleUniversalRealtime dynamically subscribes to channels specified via query parameters.
func (h *StreamHandler) HandleUniversalRealtime() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channels := r.URL.Query()["channel"]
		if len(channels) == 0 {
			if ch := r.URL.Query().Get("channels"); ch != "" {
				channels = strings.Split(ch, ",")
			}
		}
		var resolvedChannels []string
		for _, ch := range channels {
			ch = strings.TrimSpace(ch)
			if ch == "" {
				continue
			}
			resolvedChannels = append(resolvedChannels, ch)
			if !strings.HasPrefix(ch, "bffx:events:") {
				resolvedChannels = append(resolvedChannels, "bffx:events:"+ch)
			}
		}
		if len(resolvedChannels) == 0 {
			resolvedChannels = []string{"bffx:events:*", "bffx:events"}
		}
		h.HandleStream(resolvedChannels)(w, r)
	}
}

