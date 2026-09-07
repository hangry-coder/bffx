package comm

import (
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
)

type Hub struct {
	Store     storage.Store
	Notify    *notifications.Manager
	EmailMgr  *email.Manager
	Templates *TemplateManager
	Providers map[Channel]Provider
}

func NewHub(store storage.Store, n *notifications.Manager, e *email.Manager, tm *TemplateManager) *Hub {
	return &Hub{
		Store:     store,
		Notify:    n,
		EmailMgr:  e,
		Templates: tm,
		Providers: make(map[Channel]Provider),
	}
}

func (h *Hub) RegisterProvider(ch Channel, p Provider) {
	h.Providers[ch] = p
}

// Send sends a message to specific channels for a user
func (h *Hub) Send(ctx context.Context, userId, title, body string, channels ...Channel) {
	user, err := h.Store.Get(ctx, "User", userId)
	if err != nil {
		return
	}

	for _, ch := range channels {
		switch ch {
		case Email:
			emailAddr, _ := user["email"].(string)
			if emailAddr != "" {
				h.EmailMgr.Send(ctx, "default", emailAddr, title, body)
			}
		case Push:
			h.Notify.NotifyUser(ctx, userId, title, body, nil)
		default:
			if p, ok := h.Providers[ch]; ok {
				// Resolve target address from user profile based on channel
				to := ""
				switch ch {
				case Telegram:
					to, _ = user["telegram_id"].(string)
				case WhatsApp:
					to, _ = user["phone"].(string)
				case Discord:
					to, _ = user["discord_id"].(string)
				}

				if to != "" {
					p.Send(ctx, Message{
						To:      to,
						Title:   title,
						Body:    body,
						Channel: ch,
					})
				}
			}
		}
	}
}

// Alert is the high-level "send everywhere" command
func (h *Hub) Alert(ctx context.Context, userId, title, body string) {
	// By default, try Push and Email
	h.Send(ctx, userId, title, body, Push, Email)
	
	// Also try social channels if registered
	for ch := range h.Providers {
		h.Send(ctx, userId, title, body, ch)
	}
}

// SendTemplate renders and sends a template across all supported channels for a user
func (h *Hub) SendTemplate(ctx context.Context, userId, templateName string, data any) {
	if _, err := h.Store.Get(ctx, "User", userId); err != nil {
		return
	}

	// Determine channels to send to
	channels := []Channel{Push, Email}
	for ch := range h.Providers {
		channels = append(channels, ch)
	}

	for _, ch := range channels {
		title, body, err := h.Templates.Render(templateName, ch, data)
		if err != nil {
			continue
		}

		h.Send(ctx, userId, title, body, ch)
	}
}

