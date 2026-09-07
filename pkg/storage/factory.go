package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"os"
	"path/filepath"
)

func NewStore(root string, project *manifest.ProjectSpec, reg *manifest.Registry) (Store, error) {
	ApplyEnvStoreOverrides(project)
	primary, err := createBaseStore(root, &project.Store)
	if err != nil {
		return nil, err
	}

	if project.TelemetryStore == nil {
		return NewTracingStore(&RouterStore{
			Primary:   primary,
			Registry:  reg,
		}), nil
	}

	telemetry, err := createBaseStore(root, project.TelemetryStore)
	if err != nil {
		fmt.Printf("[STORAGE] Warning: failed to init telemetry store: %v. Falling back to primary.\n", err)
		return NewTracingStore(primary), nil
	}

	return NewTracingStore(&RouterStore{
		Primary:   primary,
		Telemetry: telemetry,
		Registry:  reg,
	}), nil
}

func createBaseStore(root string, conf *manifest.StoreConfig) (Store, error) {
	mode := conf.Mode
	switch mode {
	case "memory":
		return NewMemoryStore(), nil
	case "sqlite":
		path := conf.Path
		if path == "" {
			path = filepath.Join(root, ".bffx", "data", "app.db")
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		return NewSQLiteStore(path)
	case "mongo":
		return newMongoStoreFromConfig(conf)
	case "pocketbase":
		return newPocketBaseStoreFromConfig(conf)
	case "postgres":
		url := conf.Url
		if url == "" {
			url = os.Getenv("DATABASE_URL")
		}
		if url == "" {
			return nil, fmt.Errorf("DATABASE_URL required for postgres")
		}
		return NewPostgresStore(url)
	default:
		return NewMemoryStore(), nil
	}
}

func UnwrapStore(s Store) Store {
	if ts, ok := s.(*tracingStore); ok {
		return UnwrapStore(ts.underlying)
	}
	if is, ok := s.(*InvalidatingStore); ok {
		return UnwrapStore(is.Store)
	}
	return s
}

