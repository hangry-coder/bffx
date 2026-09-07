package generator

import (
	"fmt"
	"strings"
)

// normalizeStoreMode validates and canonicalizes the primary store mode for scaffolding.
func normalizeStoreMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "sqlite", nil
	}
	switch mode {
	case "sqlite", "postgres", "mongo", "memory", "pocketbase":
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported store mode %q (use sqlite, postgres, mongo, memory, or pocketbase)", mode)
	}
}

// storeSpecYAMLLines returns project.yaml lines for spec.store (mode-specific).
func storeSpecYAMLLines(mode string) []string {
	switch mode {
	case "postgres":
		return []string{
			"  store:",
			"    mode: postgres",
			"    url: postgres://bffx:bffx@127.0.0.1:5432/bffx?sslmode=disable",
		}
	case "mongo":
		return []string{
			"  store:",
			"    mode: mongo",
		}
	case "memory":
		return []string{
			"  store:",
			"    mode: memory",
		}
	case "pocketbase":
		return []string{
			"  store:",
			"    mode: pocketbase",
		}
	default:
		return []string{
			"  store:",
			"    mode: sqlite",
			"    path: .bffx/data/app.db",
		}
	}
}

// defaultEnvLinesForStore returns scaffolded .env lines for the chosen store mode.
func defaultEnvLinesForStore(mode string) []string {
	lines := []string{
		"BFFX_JWT_SECRET=bffx-dev-jwt-secret-key-must-be-at-least-32-characters",
		"BFFX_WORKER_SECRET=bffx-dev-worker-secret-key-must-be-at-least-32-characters",
		"BFFX_APP_SECRET=bffx-dev-app-secret-key-must-be-at-least-16-characters",
		"BFFX_ADMIN_SESSION_KEY=bffx-dev-admin-session-key-must-be-at-least-32-characters",
		"REDIS_URL=redis://localhost:6379",
	}
	switch mode {
	case "postgres":
		lines = append(lines, "DATABASE_URL=postgres://bffx:bffx@127.0.0.1:5432/bffx?sslmode=disable")
	case "mongo", "pocketbase":
		lines = append(lines, "BFFX_MONGODB_URL=mongodb://localhost:27017")
	default:
		// sqlite: optional mongo line omitted to reduce confusion
	}
	return lines
}
