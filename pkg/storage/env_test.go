package storage

import (
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestApplyEnvStoreConfigOverrides_sqliteFromBFFXDBURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFFX_DB_URL", "sqlite://.bffx/data/app.db")

	conf := manifest.StoreConfig{Mode: "postgres"}
	ApplyEnvStoreConfigOverrides(&conf)

	if conf.Mode != "sqlite" {
		t.Fatalf("mode=%q want sqlite", conf.Mode)
	}
	if conf.Path != ".bffx/data/app.db" {
		t.Fatalf("path=%q", conf.Path)
	}
}

func TestApplyEnvStoreConfigOverrides_databaseURL(t *testing.T) {
	t.Setenv("BFFX_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/testdb")
	t.Setenv("BFFX_DB_URL", "sqlite://ignored.db")

	conf := manifest.StoreConfig{Mode: "postgres"}
	ApplyEnvStoreConfigOverrides(&conf)

	if conf.Mode != "postgres" {
		t.Fatalf("mode=%q want postgres", conf.Mode)
	}
	if conf.Url != "postgres://localhost:5432/testdb" {
		t.Fatalf("url=%q", conf.Url)
	}
}

func TestApplyEnvStoreConfigOverrides_devPrefersSQLiteOverDATABASE_URL(t *testing.T) {
	t.Setenv("BFFX_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/testdb")
	t.Setenv("BFFX_DB_URL", "sqlite://.bffx/data/app.db")

	conf := manifest.StoreConfig{Mode: "postgres"}
	ApplyEnvStoreConfigOverrides(&conf)

	if conf.Mode != "sqlite" {
		t.Fatalf("mode=%q want sqlite in dev", conf.Mode)
	}
	if conf.Path != ".bffx/data/app.db" {
		t.Fatalf("path=%q", conf.Path)
	}
}

func TestApplyEnvStoreOverrides_alignsBatteriesStore(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BFFX_DB_URL", "sqlite://.bffx/data/app.db")

	spec := &manifest.ProjectSpec{
		Store:     manifest.StoreConfig{Mode: "postgres"},
		Batteries: manifest.BatteriesConfig{Store: "postgres"},
	}
	ApplyEnvStoreOverrides(spec)

	if spec.Batteries.Store != "sqlite" {
		t.Fatalf("batteries.store=%q", spec.Batteries.Store)
	}
}
