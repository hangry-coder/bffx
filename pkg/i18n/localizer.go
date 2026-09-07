package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/hangry-coder/bffx/pkg/cache"
)

// Localizer provides entity-level localization helpers with cache-aside support.
type Localizer struct {
	cache cache.Provider
}

// NewLocalizer creates a new Localizer.
func NewLocalizer(c cache.Provider) *Localizer {
	return &Localizer{cache: c}
}

// GetLocalizedEntity fetches an entity from the cache or via the provided fetcher, 
// then caches the full object for sub-ms Go-side filtering.
func (l *Localizer) GetLocalizedEntity(ctx context.Context, entityType, id string, target any, fetcher func() (any, error)) error {
	key := fmt.Sprintf("i18n:%s:%s", entityType, id)
	
	// Cache-aside: check cache first
	if l.cache != nil {
		data, err := l.cache.Get(ctx, key)
		if err == nil && data != nil {
			return json.Unmarshal(data, target)
		}
	}

	// Fetch from DB or other source
	entity, err := fetcher()
	if err != nil {
		return err
	}

	// Serialize and store in cache
	if l.cache != nil {
		data, err := json.Marshal(entity)
		if err == nil {
			_ = l.cache.Set(ctx, key, data, 10*time.Minute)
		}
	}

	// Load into target
	data, _ := json.Marshal(entity)
	return json.Unmarshal(data, target)
}

// InvalidateEntity removes an entity from the i18n cache.
func (l *Localizer) InvalidateEntity(ctx context.Context, entityType, id string) error {
	if l.cache == nil {
		return nil
	}
	key := fmt.Sprintf("i18n:%s:%s", entityType, id)
	return l.cache.Delete(ctx, key)
}
