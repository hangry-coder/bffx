package hooks

import (
	"fmt"
	"net/http"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// HandleMockTranscribe simulates transcription engine calls in local/test modes.
func HandleMockTranscribe(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
	fmt.Println("🎙️ Running mock transcription pipeline step...")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"completed","transcript":"Meeting notes: ant gravity is incredibly fast and elegant.","confidence":0.99}`))
}
