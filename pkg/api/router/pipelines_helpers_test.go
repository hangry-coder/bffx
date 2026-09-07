package router

import (
	"testing"

	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestResolvePipelineCatalogUsesAddonWhenNoLegacyBattery(t *testing.T) {
	spec := manifest.PipelineSpec{
		Catalog: &manifest.PipelineCatalog{Adapter: "openfoodfacts"},
	}
	cat := resolvePipelineCatalog(spec, nutrition.NewNoopProvider())
	if cat == nil {
		t.Fatal("expected addon catalog")
	}
	if cat.Source() != "openfoodfacts" {
		t.Fatalf("source: got %q", cat.Source())
	}
}

func TestResolvePipelineCatalogNilWithoutAdapter(t *testing.T) {
	cat := resolvePipelineCatalog(manifest.PipelineSpec{}, nil)
	if cat != nil {
		t.Fatalf("expected nil catalog, got %#v", cat)
	}
}
