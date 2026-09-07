package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultResendAPIURL = "https://api.resend.com/emails"

// ResendProvider sends transactional email via the Resend HTTP API.
type ResendProvider struct {
	APIKey string
	// From overrides the default sender (e.g. verified domain in Resend).
	From string
	// APIURL is the full emails endpoint; empty means defaultResendAPIURL (tests may override).
	APIURL string
	// HTTPClient is optional; production uses a 10s-timeout client when nil.
	HTTPClient *http.Client
}

func (p *ResendProvider) apiURL() string {
	if strings.TrimSpace(p.APIURL) != "" {
		return strings.TrimSpace(p.APIURL)
	}
	return defaultResendAPIURL
}

func (p *ResendProvider) httpClient() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *ResendProvider) fromAddress() string {
	if strings.TrimSpace(p.From) != "" {
		return strings.TrimSpace(p.From)
	}
	return "onboarding@resend.dev"
}

func (p *ResendProvider) Send(ctx context.Context, to, subject, body string) error {
	payload := map[string]any{
		"from":    p.fromAddress(),
		"to":      []string{to},
		"subject": subject,
		"html":    body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = "(empty body)"
		}
		return fmt.Errorf("%w: status=%d body=%s", ErrResendNonOK, resp.StatusCode, msg)
	}

	return nil
}
