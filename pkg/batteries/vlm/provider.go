package vlm

import (
	"context"
)

// Provider defines the interface for Vision Language Models.
type Provider interface {
	// Analyze processes an image with a text prompt and returns the model's response.
	Analyze(ctx context.Context, image []byte, prompt string) (string, error)
	
	// Type returns the provider type (e.g., "gemini", "ollama").
	Type() string
}
