package receiptscan

import (
	"context"
	"fmt"
)

// GenericCatalogAdapter implements the orchestrator.Catalog interface.
type GenericCatalogAdapter struct{}

func NewGenericCatalogAdapter() *GenericCatalogAdapter {
	return &GenericCatalogAdapter{}
}

func (a *GenericCatalogAdapter) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	return map[string]any{
		"resolved": true,
		"query":    query,
		"source":   "",
	}, nil
}

func (a *GenericCatalogAdapter) Source() string {
	if "" != "" {
		return ""
	}
	return "generic"
}
