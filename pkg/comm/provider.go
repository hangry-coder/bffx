package comm

import "context"

type Channel string

const (
	Email     Channel = "email"
	Push      Channel = "push"
	WhatsApp  Channel = "whatsapp"
	Telegram  Channel = "telegram"
	Discord   Channel = "discord"
	Slack     Channel = "slack"
)

type Message struct {
	To      string
	Title   string
	Body    string
	Data    map[string]string
	Channel Channel
}

type Provider interface {
	Send(ctx context.Context, msg Message) error
}
