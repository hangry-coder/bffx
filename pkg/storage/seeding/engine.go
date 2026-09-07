package seeding

import (
	"context"
	"fmt"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/brianvoe/gofakeit/v6"
)

// Engine handles versioned data seeding
type Engine struct {
	store storage.Store
}

func NewEngine(store storage.Store) *Engine {
	return &Engine{store: store}
}

// Seed executes a seed payload for a given resource
func (e *Engine) Seed(ctx context.Context, resource string, count int, template map[string]any) error {
	for i := 0; i < count; i++ {
		payload := make(map[string]any)
		for k, v := range template {
			payload[k] = e.resolveValue(v)
		}

		if _, err := e.store.Create(ctx, resource, payload); err != nil {
			return fmt.Errorf("failed to seed %s at index %d: %w", resource, i, err)
		}
	}
	return nil
}

func (e *Engine) resolveValue(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}

	// Dynamic generators using gofakeit
	switch s {
	case "faker:name":
		return gofakeit.Name()
	case "faker:email":
		return gofakeit.Email()
	case "faker:phone":
		return gofakeit.Phone()
	case "faker:sentence":
		return gofakeit.Sentence(5)
	case "faker:date_future":
		return gofakeit.DateRange(gofakeit.Date(), gofakeit.Date().AddDate(1, 0, 0))
	case "faker:date_past":
		return gofakeit.DateRange(gofakeit.Date().AddDate(-1, 0, 0), gofakeit.Date())
	default:
		return v
	}
}
