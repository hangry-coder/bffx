package vlm

import (
	"context"
	"google.golang.org/genai"
)

type GeminiProvider struct {
	client *genai.Client
	model  string
}

func NewGeminiProvider(ctx context.Context, apiKey string, model string) (*GeminiProvider, error) {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	
	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

func (p *GeminiProvider) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{
			Data:     image,
			MIMEType: "image/jpeg", // Default to jpeg, should ideally detect
		}},
	}

	contents := []*genai.Content{{Parts: parts, Role: "user"}}

	result, err := p.client.Models.GenerateContent(ctx, p.model, contents, nil)
	if err != nil {
		return "", err
	}

	return result.Text(), nil
}

func (p *GeminiProvider) Type() string {
	return "gemini"
}
