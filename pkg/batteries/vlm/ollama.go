package vlm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

type OllamaProvider struct {
	Host  string
	Model string
}

func NewOllamaProvider(host, model string) *OllamaProvider {
	if host == "" {
		host = "http://localhost:11434"
	}
	if model == "" {
		model = "llava" // Default multimodal model
	}
	return &OllamaProvider{Host: host, Model: model}
}

func (p *OllamaProvider) Analyze(ctx context.Context, image []byte, prompt string) (string, error) {
	imgBase64 := base64.StdEncoding.EncodeToString(image)

	payload := map[string]any{
		"model":  p.Model,
		"prompt": prompt,
		"images": []string{imgBase64},
		"stream": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/generate", p.Host), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var res struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Response, nil
}

func (p *OllamaProvider) Type() string {
	return "ollama"
}
