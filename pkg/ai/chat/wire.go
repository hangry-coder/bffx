package chat

import (
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/sse"
)

// NewWriter opens an SSE stream for chatbot pipeline responses.
func NewWriter(w http.ResponseWriter) (*sse.Writer, error) {
	return sse.NewWriter(w, sse.WithAllowOrigin("*"))
}
