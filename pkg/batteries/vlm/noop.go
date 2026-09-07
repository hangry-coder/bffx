package vlm

import (
	"context"
	"os"
)

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider {
	return &NoopProvider{}
}

func (p *NoopProvider) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	if val := os.Getenv("BFFX_VLM_MOCK_RESPONSE"); val != "" {
		return val, nil
	}
	return "Noop VLM: Analysis disabled or not configured.", nil
}

func (p *NoopProvider) Type() string {
	return "noop"
}
