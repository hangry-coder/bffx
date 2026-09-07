package handlers

import (
	"context"
	"strings"

	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func resolveRefreshTokenRow(ctx context.Context, store storage.Store, presented string) (map[string]any, error) {
	presented = strings.TrimSpace(presented)
	if presented == "" {
		return nil, bffx_errors.ErrNotFound
	}

	if rowID, secret, ok := auth.ParseRefreshWire(presented); ok {
		row, err := store.Get(ctx, "RefreshToken", rowID)
		if err != nil {
			return nil, err
		}
		stored, _ := row["token"].(string)
		if stored == "" || !auth.VerifyRefreshSecret(stored, secret) {
			return nil, bffx_errors.ErrNotFound
		}
		return row, nil
	}

	rows, err := store.Query(ctx, "RefreshToken").Where("token", "=", presented).Execute(ctx)
	if err != nil || len(rows) == 0 {
		return nil, bffx_errors.ErrNotFound
	}
	return rows[0], nil
}
