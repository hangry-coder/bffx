package nutrition

import (
	"context"
	"testing"
)

func TestAddonCatalog_Source(t *testing.T) {
	c := NewAddonCatalog()
	if got := c.Source(); got != "openfoodfacts" {
		t.Fatalf("Source() = %q, want openfoodfacts", got)
	}
}

func TestAddonCatalog_Resolve_EmptyQuery(t *testing.T) {
	c := NewAddonCatalog()
	_, err := c.Resolve(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

