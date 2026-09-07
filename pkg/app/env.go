package app

import (
	"bufio"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnv reads .env file from the project root and sets environment variables.
// It does not override existing environment variables.
func LoadEnv(root string) error {
	path := filepath.Join(root, ".env")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env is optional
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // invalid line
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func ValidateEnv() error {
	env := os.Getenv("BFFX_ENV")
	isProd := env == "production"

	// Critical secrets
	secrets := []struct {
		key    string
		minLen int
		prod   bool // required in prod
	}{
		{"BFFX_JWT_SECRET", 32, true},
		{"BFFX_ADMIN_SESSION_KEY", 32, true},
		{"BFFX_APP_SECRET", 16, false},   // Optional but should be strong if set
		{"BFFX_WORKER_SECRET", 32, false}, // Required if worker is enabled (checked in server.go)
	}

	for _, s := range secrets {
		val := os.Getenv(s.key)
		if val == "" {
			if isProd && s.prod {
				return fmt.Errorf("MISSING CRITICAL SECRET: %s must be set in production", s.key)
			}
			continue
		}

		if len(val) < s.minLen {
			return fmt.Errorf("INSECURE SECRET: %s is too short (min %d chars)", s.key, s.minLen)
		}
	}

	return nil
}

// ValidateEnvProject enforces production secrets that depend on manifest flags (after registry load).
func ValidateEnvProject(spec *manifest.ProjectSpec) error {
	if os.Getenv("BFFX_ENV") != "production" || spec == nil {
		return nil
	}
	appSecret := os.Getenv("BFFX_APP_SECRET")
	if appSecret == "" || len(appSecret) < 16 {
		return fmt.Errorf("production requires BFFX_APP_SECRET (min 16 chars)")
	}
	jobsOn := spec.Defaults.Jobs || spec.Runtime.Worker.Enabled
	if jobsOn {
		w := os.Getenv("BFFX_WORKER_SECRET")
		if w == "" || len(w) < 32 {
			return fmt.Errorf("production requires BFFX_WORKER_SECRET (min 32 chars) when jobs or worker runtime is enabled")
		}
	}
	return nil
}
