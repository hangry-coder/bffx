package orchestrator

import (
	"context"
)

// Catalog decouples domain-specific operations (e.g. nutrition, invoicing) from the core execution loop.
type Catalog interface {
	// Resolve queries the domain catalog to lookup or hydrate items.
	Resolve(ctx context.Context, query string, hints map[string]string) (any, error)
	// Source returns the identifier of this catalog (e.g., "openfoodfacts", "sqlite").
	Source() string
}
