package storage

import (
	"os"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// ApplyEnvStoreOverrides maps DATABASE_URL / BFFX_DB_URL onto the project store
// before opening a Store. When the manifest requests postgres but only a sqlite
// BFFX_DB_URL is set (common local dev), the store mode switches to sqlite.
func ApplyEnvStoreOverrides(project *manifest.ProjectSpec) {
	if project == nil {
		return
	}
	ApplyEnvStoreConfigOverrides(&project.Store)
	if project.Store.Mode != "" {
		project.Batteries.Store = project.Store.Mode
	}
}

// ApplyEnvStoreConfigOverrides applies env-based overrides to a single store config.
func ApplyEnvStoreConfigOverrides(conf *manifest.StoreConfig) {
	if conf == nil || conf.Mode != "postgres" {
		return
	}
	if url := strings.TrimSpace(conf.Url); url != "" {
		return
	}
	// Local dev: prefer BFFX_DB_URL sqlite over a stray DATABASE_URL in the shell.
	if os.Getenv("BFFX_ENV") != "production" {
		if raw := strings.TrimSpace(os.Getenv("BFFX_DB_URL")); strings.HasPrefix(raw, "sqlite://") {
			conf.Mode = "sqlite"
			conf.Path = strings.TrimPrefix(raw, "sqlite://")
			conf.Url = ""
			return
		}
	}
	if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" {
		conf.Url = url
		return
	}
	raw := strings.TrimSpace(os.Getenv("BFFX_DB_URL"))
	if raw == "" {
		return
	}
	if strings.HasPrefix(raw, "sqlite://") {
		conf.Mode = "sqlite"
		conf.Path = strings.TrimPrefix(raw, "sqlite://")
		conf.Url = ""
		return
	}
	conf.Url = raw
}
