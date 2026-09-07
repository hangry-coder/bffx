package chat

import (
	"encoding/json"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/sse"
)

// TokenChunk is the wire format for a single streamed LLM token in chatbot pipelines.
type TokenChunk struct {
	Token        string `json:"token"`
	SessionID    string `json:"session_id,omitempty"`
	ProviderUsed string `json:"provider_used,omitempty"`
}

// WriteToken emits one chatbot token chunk over SSE.
func WriteToken(w *sse.Writer, token, sessionID, provider string) error {
	return w.WriteJSON(TokenChunk{
		Token:        token,
		SessionID:    sessionID,
		ProviderUsed: provider,
	})
}

// WriteError emits a terminal SSE error event for chatbot streams.
func WriteError(w *sse.Writer, err error) {
	apiErr := errors.New(http.StatusInternalServerError, err.Error(), "STREAM_ERROR")
	data, _ := json.Marshal(apiErr)
	_ = w.WriteEvent("error", data)
}
