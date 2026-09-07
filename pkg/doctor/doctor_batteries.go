package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"

	"github.com/redis/go-redis/v9"
)

func checkBatteries(root string, reg *manifest.Registry) []Result {
	var results []Result

	jwtSecret := os.Getenv("BFFX_JWT_SECRET")
	jwtSvc := auth.NewJWTService(jwtSecret)

	// Initialize Redis for doctor if possible
	var redisClient *redis.Client
	spec := reg.ProjectSpec()
	if spec != nil {
		if spec.Runtime.Redis.Enabled && spec.Runtime.Redis.Url != "" {
			if opts, err := redis.ParseURL(spec.Runtime.Redis.Url); err == nil {
				redisClient = redis.NewClient(opts)
			}
		}
	}

	s, _ := storage.NewStore(root, spec, reg)
	bat, err := batteries.Resolve(root, spec, reg, jwtSvc, redisClient, s)
	if err != nil {
		results = append(results, Result{
			Name:    "Batteries Resolution",
			Status:  "fail",
			Message: fmt.Sprintf("Failed to resolve batteries: %v", err),
		})
		return results
	}

	// Check each battery
	batteryList := []struct {
		name, value string
	}{
		{"Auth", bat.Auth.Type()},
		{"Store", bat.Store},
		{"Cache", bat.Cache.Type()},
		{"Blob", func() string {
			if bat.Blob == nil {
				return "disabled"
			}
			return bat.Blob.Type()
		}()},
		{"Analytics", bat.Analytics.Type()},
		{"Observability", bat.Observability.Type()},
		{"Flags", func() string {
			if bat.Flags == nil {
				return "disabled"
			}
			return bat.Flags.Name()
		}()},
		{"VLM", func() string {
			if bat.Vlm == nil {
				return "disabled"
			}
			return bat.Vlm.Type()
		}()},
		{"Nutrition", func() string {
			if bat.Nutrition == nil {
				return "disabled"
			}
			return bat.Nutrition.Type()
		}()},
	}

	for _, b := range batteryList {
		status := "ok"
		msg := fmt.Sprintf("Using %s", b.value)

		switch b.name {
		case "Auth":
			if b.value == "clerk" && os.Getenv("CLERK_SECRET_KEY") == "" {
				status = "warn"
				msg = "clerk selected but CLERK_SECRET_KEY is missing"
			}
		case "Cache":
			if os.Getenv("BFFX_OFFLINE") == "true" {
				msg = fmt.Sprintf("%s cache connection check skipped (offline mode)", b.value)
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := bat.Cache.Ping(ctx); err != nil {
				status = "fail"
				msg = fmt.Sprintf("%s cache ping failed: %v", b.value, err)
			} else {
				msg = fmt.Sprintf("%s cache is reachable", b.value)
			}
		case "Store":
			if os.Getenv("BFFX_OFFLINE") == "true" {
				msg = fmt.Sprintf("%s store connection check skipped (offline mode)", b.value)
				break
			}
			if reg.Project != nil {
				var storeSpec manifest.ProjectSpec
				if err := reg.Project.UnmarshalSpec(&storeSpec); err == nil {
					storeSpec.Store.Mode = b.value
					_, err := storage.NewStore(root, &storeSpec, reg)
					if err != nil {
						status = "fail"
						msg = fmt.Sprintf("Failed to connect to %s store: %v", b.value, err)
					} else {
						msg = fmt.Sprintf("%s store is reachable", b.value)
					}
				}
			} else {
				status = "warn"
				msg = fmt.Sprintf("No Project manifest found to check %s store connectivity", b.value)
			}
		case "Flags":
			if spec == nil {
				break
			}
			provider := strings.ToLower(spec.FeatureFlags.Provider)
			if provider == "" {
				provider = "bffx" // Default
			}

			if provider == "launchdarkly" {
				apiKey := spec.FeatureFlags.Config["apiKey"]
				envKey := os.Getenv("LAUNCHDARKLY_SDK_KEY")

				if apiKey == "" && envKey == "" {
					status = "fail"
					msg = "LaunchDarkly provider selected but no SDK key found (checked features.config.apiKey and LAUNCHDARKLY_SDK_KEY)"
				} else if apiKey == "" && envKey != "" {
					msg = "LaunchDarkly provider: using SDK key from LAUNCHDARKLY_SDK_KEY"
				} else {
					msg = "LaunchDarkly provider: configured via project.yaml apiKey"
				}
			} else if provider == "goff" {
				url := spec.FeatureFlags.Config["url"]
				if url != "" {
					msg = fmt.Sprintf("Goff provider: using remote URL %s", url)
				} else {
					flagFile := spec.FeatureFlags.Config["flagsFile"]
					if flagFile == "" {
						flagFile = "config/flags.yaml"
					}
					if info, err := os.Stat(flagFile); err != nil {
						status = "warn"
						msg = fmt.Sprintf("Goff provider: url missing and local file %q not found", flagFile)
					} else if info.Size() == 0 {
						status = "warn"
						msg = fmt.Sprintf("Goff provider: local file %q is empty", flagFile)
					} else {
						msg = fmt.Sprintf("Goff provider: using local file %s", flagFile)
					}
				}
			} else if provider == "bffx" {
				msg = "Using builtin Bffx provider (manifest-backed)"
			}
		}

		results = append(results, Result{
			Name:    fmt.Sprintf("Battery: %s", b.name),
			Status:  status,
			Message: msg,
		})
	}

	return results
}
