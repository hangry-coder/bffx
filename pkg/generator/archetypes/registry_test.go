package archetypes

import (
	"testing"
)

func TestRegistryLoads(t *testing.T) {
	reg := GetRegistry()
	if reg == nil {
		t.Fatal("expected registry to be loaded, got nil")
	}
	if len(reg.Archetypes) == 0 {
		t.Error("expected archetypes in registry, got 0")
	}
}
