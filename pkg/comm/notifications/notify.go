package notifications

import (
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
)

type Manager struct {
	store     storage.Store
	providers map[string]Provider
}

func NewManager(store storage.Store) *Manager {
	return &Manager{
		store:     store,
		providers: make(map[string]Provider),
	}
}

func (m *Manager) RegisterProvider(key string, p Provider) {
	m.providers[key] = p
}

func (m *Manager) NotifyUser(ctx context.Context, userId, title, body string, data map[string]string) error {
	// Query all devices for this user using the QueryBuilder
	devices, _ := m.store.Query(ctx, "Device").Where("user_id", "=", userId).Execute(ctx)
	if len(devices) == 0 {
		return nil // No devices is not an error, just nothing to do
	}


	for _, d := range devices {
		token, _ := d["push_token"].(string)
		platform, _ := d["platform"].(string)
		
		if token == "" {
			continue
		}

		// Try platform-specific provider first
		p, ok := m.providers[platform]
		if !ok {
			// Fallback to generic "fcm" if platform provider not found
			p, ok = m.providers["fcm"]
			if !ok {
				// Final fallback: use any registered provider
				for _, p1 := range m.providers {
					p = p1
					break
				}
			}
		}

		if p != nil {
			_ = p.Send(ctx, token, title, body, data)
		}
	}
	return nil
}

