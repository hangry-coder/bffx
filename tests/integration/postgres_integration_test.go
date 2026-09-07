//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/hangry-coder/bffx/pkg/storage"
)

// TestPostgresIntegrationFromEnv verifies a real Postgres is reachable when DATABASE_URL
// is set (see docker-compose.test.yml and the CI integration job).
func TestPostgresIntegrationFromEnv(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set (run with docker compose -f docker-compose.test.yml up -d)")
	}
	st, err := storage.NewPostgresStore(url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.GetDB().Close()
	if err := st.GetDB().Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
