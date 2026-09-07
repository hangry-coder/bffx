package comm
 
import (
	"github.com/hangry-coder/bffx/pkg/errors"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DiscordProvider struct {
	WebhookURL string
}

func (p *DiscordProvider) Send(ctx context.Context, msg Message) error {
	if p.WebhookURL == "" {
		return fmt.Errorf("%w: discord webhook URL not configured", errors.ErrNotImplemented)
	}

	payload := map[string]any{
		"content": fmt.Sprintf("**%s**\n%s", msg.Title, msg.Body),
	}

	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", p.WebhookURL, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord error: status %d", resp.StatusCode)
	}
	return nil
}

type TelegramProvider struct {
	BotToken string
	ChatID   string
}

func (p *TelegramProvider) Send(ctx context.Context, msg Message) error {
	if p.BotToken == "" || p.ChatID == "" {
		return fmt.Errorf("%w: telegram bot token or chat ID not configured", errors.ErrNotImplemented)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.BotToken)
	payload := map[string]any{
		"chat_id": p.ChatID,
		"text":    fmt.Sprintf("*%s*\n%s", msg.Title, msg.Body),
		"parse_mode": "Markdown",
	}

	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram error: status %d", resp.StatusCode)
	}
	return nil
}

type WhatsAppProvider struct {
	TwilioSID   string
	TwilioAuth  string
	FromNumber  string
}

func (p *WhatsAppProvider) Send(ctx context.Context, msg Message) error {
	// TODO: Implement Twilio WhatsApp API (Week 8)
	return fmt.Errorf("%w: Twilio WhatsApp integration pending", errors.ErrNotImplemented)
}
