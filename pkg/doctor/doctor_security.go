package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func tlsSecurityChecks(root string, reg *manifest.Registry) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")
	if env != "production" {
		return results
	}

	if reg == nil || reg.Project == nil {
		return results
	}

	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		return results
	}

	// 1. Database TLS Check
	if projectSpec.Store.Mode == "postgres" {
		dbURL := projectSpec.Store.Url
		if dbURL == "" {
			dbURL = os.Getenv("DATABASE_URL")
		}
		if dbURL != "" {
			if strings.Contains(dbURL, "sslmode=disable") {
				results = append(results, Result{
					Name:    "Security: DB TLS",
					Status:  "fail",
					Message: "Database connection uses sslmode=disable in production, which is insecure.",
				})
			} else if strings.Contains(dbURL, "sslmode=allow") || strings.Contains(dbURL, "sslmode=prefer") {
				results = append(results, Result{
					Name:    "Security: DB TLS",
					Status:  "fail",
					Message: "Database connection uses weak sslmode (allow/prefer) in production.",
				})
			} else if !strings.Contains(dbURL, "sslmode=require") && !strings.Contains(dbURL, "sslmode=verify-ca") && !strings.Contains(dbURL, "sslmode=verify-full") {
				results = append(results, Result{
					Name:    "Security: DB TLS",
					Status:  "warn",
					Message: "Database connection does not explicitly specify a secure sslmode (e.g., sslmode=require) in production.",
				})
			} else {
				results = append(results, Result{
					Name:    "Security: DB TLS",
					Status:  "ok",
					Message: "Database connection uses secure TLS configuration.",
				})
			}
		} else {
			results = append(results, Result{
				Name:    "Security: DB TLS",
				Status:  "fail",
				Message: "Postgres store mode is active in production but no database URL is configured.",
			})
		}
	}

	// 2. Redis TLS Check
	if projectSpec.Runtime.Redis.Enabled {
		redisURL := projectSpec.Runtime.Redis.Url
		if redisURL == "" {
			redisURL = os.Getenv("BFFX_REDIS_ADDR")
		}
		if redisURL == "" {
			redisURL = os.Getenv("BFFX_REDIS_URL")
		}

		if redisURL != "" {
			if strings.HasPrefix(redisURL, "redis://") {
				results = append(results, Result{
					Name:    "Security: Redis TLS",
					Status:  "fail",
					Message: "Redis connection uses unencrypted redis:// protocol in production. Use rediss:// for TLS.",
				})
			} else if !strings.HasPrefix(redisURL, "rediss://") {
				results = append(results, Result{
					Name:    "Security: Redis TLS",
					Status:  "fail",
					Message: "Redis connection does not use TLS (rediss://) in production.",
				})
			} else {
				results = append(results, Result{
					Name:    "Security: Redis TLS",
					Status:  "ok",
					Message: "Redis connection uses secure TLS (rediss://) configuration.",
				})
			}
		} else {
			results = append(results, Result{
				Name:    "Security: Redis TLS",
				Status:  "fail",
				Message: "Redis is enabled in production but no URL is configured.",
			})
		}
	}

	return results
}
func gdprComplianceChecks(root string, reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil || reg.Project == nil {
		return results
	}

	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		return results
	}

	env := os.Getenv("BFFX_ENV")
	if env == "production" {
		results = append(results, Result{
			Name:    "Compliance: GDPR",
			Status:  "warn",
			Message: "Right to be Forgotten (GDPR Article 17): Ensure a user erasure playbook is active. Refer to docs/compliance/gdpr_erasure.md for transactional Go hook templates.",
		})
	}
	return results
}
func checkProductionPostures(root string, reg *manifest.Registry) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")
	if env != "production" {
		return results
	}

	if reg == nil || reg.Project == nil {
		return results
	}

	var projectSpec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&projectSpec); err != nil {
		return results
	}

	if projectSpec.Store.Mode == "sqlite" {
		results = append(results, Result{
			Name:    "Production: SQLite Store Mode",
			Status:  "fail",
			Message: "SQLite is configured as the active datastore in production. Use 'postgres' for a production-grade database to allow horizontal scaling.",
		})
	}

	if projectSpec.Batteries.Blob == "local" || projectSpec.Batteries.Blob == "" {
		results = append(results, Result{
			Name:    "Production: Local Blob Storage",
			Status:  "fail",
			Message: "Local filesystem is configured for blob storage in production. Switch batteries.blob to 's3' or 'minio' to prevent data loss on container restarts.",
		})
	}

	if _, ok := reg.GetResource("RefreshToken"); ok {
		redisConfigured := projectSpec.Runtime.Redis.Enabled || strings.TrimSpace(projectSpec.Runtime.Redis.Url) != "" || strings.TrimSpace(os.Getenv("BFFX_REDIS_ADDR")) != ""
		if !redisConfigured {
			results = append(results, Result{
				Name:    "Production: JWT Revocation Backend",
				Status:  "fail",
				Message: "Refresh-token rotation is enabled but Redis revocation backend is not configured. Enable runtime.redis and set BFFX_REDIS_ADDR so logout/JTI denylisting remains durable.",
			})
		} else {
			results = append(results, Result{
				Name:    "Production: JWT Revocation Backend",
				Status:  "ok",
				Message: "Redis revocation backend is configured for refresh-token/JTI denylisting.",
			})
		}

		override := strings.ToLower(strings.TrimSpace(os.Getenv("BFFX_JWT_REVOCATION_FAIL_CLOSED")))
		if override == "0" || override == "false" || override == "no" || override == "off" {
			results = append(results, Result{
				Name:    "Production: JWT Revocation Fail-Closed",
				Status:  "fail",
				Message: "BFFX_JWT_REVOCATION_FAIL_CLOSED disables fail-closed behavior in production. Remove this override or set it to true to avoid accepting revoked tokens during Redis outages.",
			})
		} else {
			results = append(results, Result{
				Name:    "Production: JWT Revocation Fail-Closed",
				Status:  "ok",
				Message: "JWT revocation is configured fail-closed for production.",
			})
		}
	}

	return results
}
func checkUnresolvedPlaceholders(root string) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")
	if env != "production" {
		return results
	}

	filesToCheck := []string{
		filepath.Join(root, "bffx", "project.yaml"),
		filepath.Join(root, "config", "production.yaml"),
	}

	placeholderRegex := regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

	for _, p := range filesToCheck {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		matches := placeholderRegex.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := match[1]
				if os.Getenv(varName) == "" {
					results = append(results, Result{
						Name:    "Security: Unresolved Placeholder",
						Status:  "fail",
						Message: fmt.Sprintf("Environment variable %q is referenced as a placeholder in %s but is not set in the host environment.", varName, filepath.Base(p)),
					})
				}
			}
		}
	}
	return results
}
