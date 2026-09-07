package notifications

import "context"

type Provider interface {
	Send(ctx context.Context, token string, title, body string, data map[string]string) error
}

type Notification struct {
	Title string
	Body  string
	Data  map[string]string
}
