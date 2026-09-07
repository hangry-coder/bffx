package doctor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

type Result struct {
	Name    string
	Status  string // "ok", "warn", "fail"
	Message string
}

// CheckBatteriesOnly runs a scoped check for battery configuration and connectivity.
func CheckBatteriesOnly(root string) ([]Result, error) {
	reg, err := manifest.LoadAll(root)
	if err != nil {
		return nil, fmt.Errorf("load manifests: %w", err)
	}
	results := checkBatteries(root, reg)
	results = append(results, archetypeDoctorChecks(root, reg)...)
	return results, nil
}

func Check(root string) ([]Result, error) {
	var results []Result

	// 1. Manifests valid
	reg, err := manifest.LoadAll(root)
	if err != nil {
		results = append(results, Result{
			Name:    "Manifest Stability",
			Status:  "fail",
			Message: fmt.Sprintf("Failed to load manifests: %v", err),
		})
	} else {
		results = append(results, Result{
			Name:    "Manifest Stability",
			Status:  "ok",
			Message: fmt.Sprintf("Loaded %d resources successfully", len(reg.Resources)),
		})

		// 2. No duplicate resource names
		names := make(map[string]bool)
		duplicates := false
		for _, res := range reg.Resources {
			if names[res.Metadata.Name] {
				results = append(results, Result{
					Name:    "Resource Uniqueness",
					Status:  "fail",
					Message: fmt.Sprintf("Duplicate resource name: %s", res.Metadata.Name),
				})
				duplicates = true
				break
			}
			names[res.Metadata.Name] = true
		}
		if !duplicates {
			results = append(results, Result{
				Name:    "Resource Uniqueness",
				Status:  "ok",
				Message: "All resource names are unique",
			})
		}

		// 2b. Registry Consistency (v2 patterns)
		regErrs := reg.Validate()
		if len(regErrs) > 0 {
			results = append(results, Result{
				Name:    "Registry Consistency",
				Status:  "fail",
				Message: fmt.Sprintf("Found %d consistency errors: %s", len(regErrs), strings.Join(regErrs, "; ")),
			})
		} else {
			results = append(results, Result{
				Name:    "Registry Consistency",
				Status:  "ok",
				Message: "Registry manifests are internally consistent",
			})
		}
	}

	// 2c. Route Prefix Linting
	if reg != nil {
		results = append(results, lintRoutePrefixes(root, reg)...)
	}

	// 2ca. Action Cache TTL Linting
	if reg != nil {
		results = append(results, lintCacheTTLOnActions(reg)...)
	}

	// 2cb. Pipelines & Deprecation Linting
	if reg != nil {
		results = append(results, lintPipelines(root, reg)...)
	}

	// 2cc1. Observability contract linting
	if reg != nil {
		results = append(results, lintObservabilityContract(reg.ProjectSpec())...)
	}

	// 2cc2. Extension taxonomy linting (batteries vs addons)
	if reg != nil {
		results = append(results, lintExtensionTaxonomy(reg)...)
	}

	// 2cc3. Build profile integrity (Phase 7)
	if reg != nil {
		results = append(results, lintBuildProfile(root, reg)...)
		results = append(results, lintPackagingMigration(root, reg)...)
		results = append(results, lintVersionedMigrations(root, reg)...)
	}

	// 2cc. Resource Policies Linting (WP-2)
	if reg != nil {
		results = append(results, lintResourcePolicies(reg)...)
	}

	// 2cd. Dev-Only Routes Linting (WP-4)
	if reg != nil {
		results = append(results, lintDevOnlyRoutes(reg)...)
	}

	// 2ce. Production Configuration Hardening (WP-3)
	if reg != nil {
		results = append(results, checkProductionPostures(root, reg)...)
	}
	results = append(results, checkUnresolvedPlaceholders(root)...)

	// 2cf. Blueprints & Seeds Security Linting (WP-8)
	if reg != nil {
		results = append(results, lintBlueprints(reg)...)
	}

	// 2cg. Admin Manifests Modernization Linting (Phase 1)
	if reg != nil {
		results = append(results, lintAdminManifests(reg)...)
	}

	// 2d. Batteries check
	if reg != nil {
		results = append(results, checkBatteries(root, reg)...)
	}

	// 2d. Config overlay (optional)
	envName := os.Getenv("BFFX_ENV")
	if envName == "" {
		envName = "development"
	}
	configPath := filepath.Join(root, "config", envName+".yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		results = append(results, Result{
			Name:    "Config Overlay",
			Status:  "warn",
			Message: fmt.Sprintf("config/%s.yaml not found — using .env and bffx/project.yaml only", envName),
		})
	} else if err != nil {
		results = append(results, Result{
			Name:    "Config Overlay",
			Status:  "warn",
			Message: fmt.Sprintf("config/%s.yaml: %v", envName, err),
		})
	} else {
		results = append(results, Result{
			Name:    "Config Overlay",
			Status:  "ok",
			Message: fmt.Sprintf("Loaded config overlay from config/%s.yaml", envName),
		})
	}

	// 3. BFFX_JWT_SECRET set and strong
	jwtSecret := os.Getenv("BFFX_JWT_SECRET")
	env := os.Getenv("BFFX_ENV")
	if jwtSecret == "" {
		jwtStatus := "warn"
		jwtMsg := "BFFX_JWT_SECRET is not set. Defaulting to insecure memory mode for local development."
		if env == "production" {
			jwtStatus = "fail"
			jwtMsg = "BFFX_JWT_SECRET is not set. This is required for secure authentication. Set it in your .env file."
		}
		results = append(results, Result{
			Name:    "Security: JWT",
			Status:  jwtStatus,
			Message: jwtMsg,
		})
	} else if len(jwtSecret) < 16 {
		status := "warn"
		if env == "production" {
			status = "fail"
		}
		results = append(results, Result{
			Name:    "Security: JWT",
			Status:  status,
			Message: "BFFX_JWT_SECRET is too short (< 16 chars). Risky for production.",
		})
	} else {
		results = append(results, Result{
			Name:    "Security: JWT",
			Status:  "ok",
			Message: "BFFX_JWT_SECRET is configured with adequate length",
		})
	}

	// 3b. BFFX_WORKER_SECRET set and strong
	workerSecret := os.Getenv("BFFX_WORKER_SECRET")
	if workerSecret == "" {
		results = append(results, Result{
			Name:   "Security: Worker",
			Status: "warn",
			Message: fmt.Sprintf("BFFX_WORKER_SECRET is not set. It secures PATCH %s/jobs/{id}/result (Python worker posts job results using Authorization: Bearer <secret>). ", reg.ApiPrefix) +
				"If unset, that endpoint does not require a Bearer token except in production. Add a random 32+ character value to the API env (usually project-root .env, same file as `docker compose` / `bffx dev` loads) and the identical key on the worker container or process (compose `worker` service `env_file`/environment). Restart the API and worker after editing.",
		})
	} else if len(workerSecret) < 32 {
		status := "warn"
		if env == "production" {
			status = "fail"
		}
		results = append(results, Result{
			Name:    "Security: Worker",
			Status:  status,
			Message: "BFFX_WORKER_SECRET is too short (< 32 chars). Use at least 32 random characters in .env (or your host env), ensure the worker process gets the same value, and restart API + worker.",
		})
	} else {
		results = append(results, Result{
			Name:    "Security: Worker",
			Status:  "ok",
			Message: "BFFX_WORKER_SECRET is configured with adequate length",
		})
	}

	// 3c. BFFX_APP_SECRET check
	appSecret := os.Getenv("BFFX_APP_SECRET")
	if appSecret != "" && len(appSecret) < 16 {
		status := "warn"
		if env == "production" {
			status = "fail"
		}
		results = append(results, Result{
			Name:    "Security: App Secret",
			Status:  status,
			Message: "BFFX_APP_SECRET is set but too short (< 16 chars).",
		})
	} else if appSecret != "" {
		results = append(results, Result{
			Name:    "Security: App Secret",
			Status:  "ok",
			Message: "BFFX_APP_SECRET is configured with adequate length",
		})
	}

	// 4. Redis Connectivity (if enabled)
	if reg != nil && reg.Project != nil {
		var projectSpec manifest.ProjectSpec
		if err := reg.Project.UnmarshalSpec(&projectSpec); err == nil {
			if projectSpec.Runtime.Redis.Enabled {
				if os.Getenv("BFFX_OFFLINE") == "true" {
					results = append(results, Result{
						Name:    "Redis Connectivity",
						Status:  "ok",
						Message: "Redis connectivity check skipped (offline mode)",
					})
				} else {
					addr := os.Getenv("BFFX_REDIS_ADDR")
					if addr == "" {
						addr = "localhost:6379"
					}
					// Simple check: see if we can reach the port
					timeout := 2 * time.Second
					conn, err := net.DialTimeout("tcp", addr, timeout)
					if err != nil {
						results = append(results, Result{
							Name:    "Redis Connectivity",
							Status:  "fail",
							Message: fmt.Sprintf("Redis enabled but unreachable at %s: %v", addr, err),
						})
					} else {
						conn.Close()
						results = append(results, Result{
							Name:    "Redis Connectivity",
							Status:  "ok",
							Message: fmt.Sprintf("Redis reachable at %s", addr),
						})
					}
				}
			}
		}
	}

	// 5. Config Overlays (Phase 4)
	if reg != nil {
		configDir := filepath.Join(root, "config")
		if st, err := os.Stat(configDir); err == nil && st.IsDir() {
			devConfig := filepath.Join(configDir, "development.yaml")
			if _, err := os.Stat(devConfig); os.IsNotExist(err) {
				results = append(results, Result{
					Name:    "Environment Config",
					Status:  "warn",
					Message: "config/ directory exists but development.yaml is missing. Recommended for local development overrides.",
				})
			} else {
				results = append(results, Result{
					Name:    "Environment Config",
					Status:  "ok",
					Message: "Environment config overlays (config/*.yaml) are present",
				})
			}
		}
	}

	// 6. Docker presence (for deploying)
	if _, err := exec.LookPath("docker"); err != nil {
		results = append(results, Result{
			Name:    "Docker Configuration",
			Status:  "warn",
			Message: "Docker is not installed or not in PATH. Required for 'bffx deploy'.",
		})
	} else {
		results = append(results, Result{
			Name:    "Docker Configuration",
			Status:  "ok",
			Message: "Docker is installed and available in PATH",
		})
	}

	// 7. Go compiler checks
	if _, err := exec.LookPath("go"); err != nil {
		results = append(results, Result{
			Name:    "Go Environment",
			Status:  "warn",
			Message: "Go compiler not found in PATH. Make sure Go 1.26 or newer is installed.",
		})
	} else {
		// Verify version
		cmd := exec.Command("go", "version")
		out, _ := cmd.Output()
		versionStr := string(out)

		isValid := false
		var parsedVersion string
		fields := strings.Fields(versionStr)
		if len(fields) >= 3 {
			parsedVersion = strings.TrimPrefix(fields[2], "go")
			parts := strings.Split(parsedVersion, ".")
			if len(parts) >= 2 {
				var major, minor int
				_, err1 := fmt.Sscan(parts[0], &major)
				_, err2 := fmt.Sscan(parts[1], &minor)
				if err1 == nil && err2 == nil {
					if major > 1 || (major == 1 && minor >= 26) {
						isValid = true
					}
				}
			}
		}

		if isValid {
			results = append(results, Result{
				Name:    "Go Environment",
				Status:  "ok",
				Message: fmt.Sprintf("Go %s is correctly installed (minimum 1.26 required)", parsedVersion),
			})
		} else {
			results = append(results, Result{
				Name:    "Go Environment",
				Status:  "warn",
				Message: fmt.Sprintf("Go version mismatch. Found %q, expected go1.26 or newer.", versionStr),
			})
		}
	}

	results = append(results, tlsSecurityChecks(root, reg)...)
	results = append(results, frameworkVendoringChecks(root, reg)...)
	results = append(results, archetypeDoctorChecks(root, reg)...)
	results = append(results, seedDoctorChecks(reg)...)
	results = append(results, gdprComplianceChecks(root, reg)...)

	return results, nil
}
