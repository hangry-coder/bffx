package email

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"fmt"
)

type Provider interface {
	Send(ctx context.Context, to, subject, body string) error
}

type LogProvider struct{}

func (m *LogProvider) Send(ctx context.Context, to, subject, body string) error {
	logger.LogMapCtx(ctx, "INFO", map[string]any{
		"event":   "email.log",
		"to":      to,
		"subject": subject,
		"body_snippet": func() string {
			if len(body) > 50 {
				return body[:50] + "..."
			}
			return body
		}(),
		"status": "logged_to_console",
	})
	fmt.Printf("\n📢  [DEV MAIL FALLBACK] Email Sent:\n  To:      %s\n  Subject: %s\n  Body:    %s\n\n", to, subject, body)
	return nil
}
