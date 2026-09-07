package email

import (
	"context"
	"fmt"
)

type Manager struct {
	providers map[string]Provider
}

func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

func (m *Manager) RegisterProvider(key string, p Provider) {
	m.providers[key] = p
}

func (m *Manager) Send(ctx context.Context, providerKey, to, subject, body string) error {
	p, ok := m.providers[providerKey]
	if !ok {
		// Fallback to log if nothing registered
		if len(m.providers) == 0 {
			return (&LogProvider{}).Send(ctx, to, subject, body)
		}
		return fmt.Errorf("provider %s not found", providerKey)
	}
	return p.Send(ctx, to, subject, body)
}
