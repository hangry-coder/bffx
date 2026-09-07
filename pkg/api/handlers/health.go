package handlers

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	store storage.Store
	rdb   *redis.Client
}

func NewHealthHandler(store storage.Store, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{store: store, rdb: rdb}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()

	status := http.StatusOK
	checks := map[string]string{
		"db":    "ok",
		"redis": "skipped",
	}

	// 1. Check Database
	if err := h.pingDB(ctx); err != nil {
		status = http.StatusServiceUnavailable
		checks["db"] = "error: " + err.Error()
	}

	// 2. Check Redis (if enabled)
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			status = http.StatusServiceUnavailable
			checks["redis"] = "error: " + err.Error()
		} else {
			checks["redis"] = "ok"
		}
	}

	errors.WriteJSON(w, status, map[string]any{
		"status": func() string {
			if status == http.StatusOK {
				return "ok"
			}
			return "degraded"
		}(),
		"checks": checks,
	})
}

func (h *HealthHandler) pingDB(ctx context.Context) error {
	type sqlBacked interface{ GetDB() *sql.DB }

	var db *sql.DB
	unwrapped := storage.UnwrapStore(h.store)
	if rb, ok := unwrapped.(sqlBacked); ok {
		db = rb.GetDB()
	} else if rs, ok := unwrapped.(*storage.RouterStore); ok {
		primary := storage.UnwrapStore(rs.Primary)
		if rb, ok := primary.(sqlBacked); ok {
			db = rb.GetDB()
		}
	}

	if db == nil {
		// Non-SQL stores might need a different ping logic or just return nil
		return nil
	}

	return db.PingContext(ctx)
}
