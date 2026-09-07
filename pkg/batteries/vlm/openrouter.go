package vlm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenRouterProvider struct {
	APIKey string
	Model  string
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	if model == "" {
		model = "google/gemini-2.0-flash-001" // Good default for OpenRouter
	}
	return &OpenRouterProvider{APIKey: apiKey, Model: model}
}

func (p *OpenRouterProvider) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	imgBase64 := base64.StdEncoding.EncodeToString(image)
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", imgBase64)

	payload := map[string]any{
		"model": p.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{
						"type": "image_url",
						"image_url": map[string]string{"url": dataURL},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.APIKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter returned status %d", resp.StatusCode)
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned no choices")
	}

	return res.Choices[0].Message.Content, nil
}

func (p *OpenRouterProvider) Type() string {
	return "openrouter"
}
