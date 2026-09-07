package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/hangry-coder/bffx/pkg/auth"
	bffxerrors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// SeedOptions configures SeedRegistry behavior.
type SeedOptions struct {
	// Upsert updates existing rows (by code, id, email, or name) instead of skipping.
	Upsert bool
}

// SeedRegistry applies Blueprint manifests (spec.defaults) and Seed manifests to the store.
// Idempotent by default: existing records are skipped unless Upsert is enabled globally or per Seed spec.
func SeedRegistry(reg *manifest.Registry, store Store, opts ...SeedOptions) error {
	if reg == nil {
		return fmt.Errorf("registry is nil")
	}
	var opt SeedOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	ctx := context.Background()

	// 1. Process Blueprints
	for _, bp := range reg.Blueprints {
		var spec manifest.BlueprintSpec
		if err := bp.UnmarshalSpec(&spec); err != nil {
			return fmt.Errorf("blueprint %s: %w", bp.Metadata.Name, err)
		}
		if spec.Resource == "" {
			continue
		}

		count := spec.Count
		if count <= 0 {
			count = 1
		}

		for i := 0; i < count; i++ {
			payload := make(map[string]any)
			for k, v := range spec.Defaults {
				payload[k] = v
			}
			for k, v := range spec.Fields {
				if _, exists := payload[k]; !exists {
					payload[k] = v
				}
			}
			if err := normalizeSeedPayload(payload); err != nil {
				return fmt.Errorf("blueprint %s: %w", bp.Metadata.Name, err)
			}
			if skip, err := seedRecordExists(ctx, store, spec.Resource, payload); err != nil {
				return err
			} else if skip {
				continue
			}
			if _, err := store.Create(ctx, spec.Resource, payload); err != nil {
				return fmt.Errorf("seed %s[%d]: %w", spec.Resource, i, err)
			}
		}
	}

	// 2. Process Seeds (kind: Seed) — registry order is spec.order then metadata.name
	for _, seed := range reg.Seeds {
		var spec manifest.SeedSpec
		if err := seed.UnmarshalSpec(&spec); err != nil {
			return fmt.Errorf("seed %s: %w", seed.Metadata.Name, err)
		}
		if spec.Table == "" {
			continue
		}
		upsert := opt.Upsert || spec.Upsert

		for i, row := range spec.Rows {
			if err := normalizeSeedPayload(row); err != nil {
				return fmt.Errorf("seed %s row %d: %w", seed.Metadata.Name, i, err)
			}
			existingID, found, err := findSeedRow(ctx, store, spec.Table, row)
			if err != nil {
				return err
			}
			if found {
				if !upsert {
					continue
				}
				if existingID == "" {
					return fmt.Errorf("seed %s row %d: upsert matched row but no id field", seed.Metadata.Name, i)
				}
				if _, err := store.Update(ctx, spec.Table, existingID, row); err != nil {
					return fmt.Errorf("seed %s row %d upsert: %w", seed.Metadata.Name, i, err)
				}
				continue
			}
			if _, err := store.Create(ctx, spec.Table, row); err != nil {
				return fmt.Errorf("seed table %s row %d: %w", spec.Table, i, err)
			}
		}
	}

	return nil
}

func normalizeSeedPayload(payload map[string]any) error {
	if raw, ok := payload["password"].(string); ok && raw != "" && !strings.HasPrefix(raw, "$2") {
		hash, err := auth.HashPassword(raw)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		payload["password"] = hash
	}
	return nil
}

func seedRecordExists(ctx context.Context, store Store, resource string, payload map[string]any) (bool, error) {
	email, _ := payload["email"].(string)
	if email == "" {
		return false, nil
	}
	_, err := store.GetByField(ctx, resource, "email", email)
	if err == nil {
		return true, nil
	}
	if err == bffxerrors.ErrNotFound {
		return false, nil
	}
	return false, err
}

func findSeedRow(ctx context.Context, store Store, table string, row map[string]any) (id string, found bool, err error) {
	for _, field := range []string{"code", "id", "email", "name"} {
		val, ok := row[field]
		if !ok {
			continue
		}
		strVal := fmt.Sprintf("%v", val)
		if strVal == "" {
			continue
		}
		existing, err := store.GetByField(ctx, table, field, strVal)
		if err == nil {
			if idVal, ok := existing["id"].(string); ok && idVal != "" {
				return idVal, true, nil
			}
			return "", true, nil
		}
		if err != bffxerrors.ErrNotFound {
			return "", false, err
		}
	}
	return "", false, nil
}
