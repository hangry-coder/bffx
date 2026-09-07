package billing

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"time"
)

// Entitlement represents a permission granted to a user via purchase or subscription.
type Entitlement struct {
	UserID    string    `json:"user_id"`
	Slug      string    `json:"slug"`       // e.g. "pro_features"
	ExpiresAt time.Time `json:"expires_at"` // Zero time means never expires
}

// Manager orchestrates monetization logic across platforms.
type Manager struct {
	store storage.Store
	reg   *manifest.Registry
}

// NewManager initializes a new billing manager with the provided store and registry.
func NewManager(store storage.Store, reg *manifest.Registry) *Manager {
	return &Manager{store: store, reg: reg}
}

// GetActiveEntitlements returns all valid entitlements for a user.
func (m *Manager) GetActiveEntitlements(ctx context.Context, userID string) ([]string, error) {
	// Query storage for active entitlements
	// This uses the new QueryBuilder (ActiveRecord power!)
	results, err := m.store.Query(ctx, "Entitlement").
		Where("user_id", "=", userID).
		Execute(ctx)
	if err != nil {
		return nil, err
	}

	var active []string
	for _, res := range results {
		expiresAtStr, _ := res["expires_at"].(string)
		if expiresAtStr != "" {
			expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
			if err == nil && expiresAt.Before(time.Now()) {
				continue // Expired
			}
		}
		slug, _ := res["slug"].(string)
		active = append(active, slug)
	}

	return active, nil
}

// Grant grants an entitlement to a user manually or via purchase hook.
func (m *Manager) Grant(ctx context.Context, userID, slug string, duration time.Duration) error {
	var expiresAt string
	if duration > 0 {
		expiresAt = time.Now().Add(duration).Format(time.RFC3339)
	}

	payload := map[string]any{
		"user_id":    userID,
		"slug":       slug,
		"expires_at": expiresAt,
		"created_at": time.Now().Format(time.RFC3339),
	}

	_, err := m.store.Create(ctx, "Entitlement", payload)
	return err
}

// ResolveStoreIdentifiers returns the mapping of internal product names to iOS/Android IDs.
func (m *Manager) ResolveStoreIdentifiers() map[string]map[string]string {
	ids := make(map[string]map[string]string)

	for _, s := range m.reg.Subscriptions {
		var spec manifest.SubscriptionSpec
		if err := s.UnmarshalSpec(&spec); err == nil {
			ids[s.Metadata.Name] = map[string]string{
				"ios":     spec.Identifiers.Ios,
				"android": spec.Identifiers.Android,
			}
		}
	}

	for _, p := range m.reg.Products {
		var spec manifest.ProductSpec
		if err := p.UnmarshalSpec(&spec); err == nil {
			ids[p.Metadata.Name] = map[string]string{
				"ios":     spec.Identifiers.Ios,
				"android": spec.Identifiers.Android,
			}
		}
	}

	return ids
}
