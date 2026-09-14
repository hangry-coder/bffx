package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestAppConfigSeedingSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_appconfig.db")

	var appConfigSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`
fields:
  - { name: key, type: string }
  - { name: value, type: string }
  - { name: type, type: string }
  - { name: config_key, type: string }
  - { name: config_value, type: string }
  - { name: description, type: string }
group: system
routes:
  crud: false
`), &appConfigSpec); err != nil {
		t.Fatal(err)
	}

	var bpSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`
resource: AppConfig
fields:
  key: theme_color
  value: "#1a73e8"
  type: string
  description: Primary brand color
`), &bpSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "AppConfig"},
				Spec:     appConfigSpec,
			},
		},
		Blueprints: []*manifest.Manifest{
			{
				Metadata: manifest.Metadata{Name: "AppConfigDefault"},
				Spec:     bpSpec,
			},
		},
	}

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.db.Close()

	ctx := context.Background()
	if _, err := store.Reconcile(ctx, reg); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Run SeedRegistry: must succeed and not fail with "table bffx_app_config has no column named key"
	if err := SeedRegistry(reg, store); err != nil {
		t.Fatalf("SeedRegistry failed: %v", err)
	}

	// Verify record exists and can be retrieved by key or config_key
	rec, err := store.GetByField(ctx, "AppConfig", "key", "theme_color")
	if err != nil {
		t.Fatalf("failed to get AppConfig by key: %v", err)
	}
	if rec == nil {
		t.Fatalf("expected AppConfig record to exist")
	}
	if rec["value"] != "#1a73e8" {
		t.Errorf("expected value '#1a73e8', got %v", rec["value"])
	}
	if rec["config_key"] != "theme_color" {
		t.Errorf("expected config_key 'theme_color', got %v", rec["config_key"])
	}
}
