// Package batteries resolves infrastructure provider adapters from manifest
// batteries.* keys (auth, cache, blob, observability, analytics, vlm, flags).
//
// Domain capabilities belong in pkg/addons (see docs/core-concepts/extension_taxonomy.md).
// Feature flag evaluation contract lives in pkg/featureflags; provider adapters in
// pkg/featureflags/providers are wired here via batteries.flags.
package batteries

import (
	"context"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries/analytics"
	authbattery "github.com/hangry-coder/bffx/pkg/batteries/auth"
	cachebattery "github.com/hangry-coder/bffx/pkg/batteries/cache"
	"github.com/hangry-coder/bffx/pkg/batteries/observability"
	coreobs "github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	flagproviders "github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/storage/blob"
	"github.com/redis/go-redis/v9"
	"os"
)

type Registry struct {
	Auth          auth.Provider
	Store         string
	Cache         cache.Provider
	Blob          blob.Provider
	Analytics     analytics.Provider
	Observability observability.Provider
	Flags         featureflags.FlagProvider
	Vlm           vlm.Provider
	Nutrition     nutrition.Provider
	I18n          *i18n.Localizer
}

func resolveAuthProvider(spec *manifest.ProjectSpec, jwtSvc *auth.JWTService) (auth.Provider, error) {
	authType := spec.Batteries.Auth
	if authType == "" {
		authType = "builtin"
	}
	switch authType {
	case "clerk":
		return authbattery.NewClerkProvider()
	case "builtin":
		fallthrough
	default:
		return authbattery.NewBuiltinProvider(jwtSvc), nil
	}
}

func resolveCacheProvider(spec *manifest.ProjectSpec, redisClient *redis.Client) cache.Provider {
	cacheType := spec.Batteries.Cache
	if cacheType == "" {
		if spec.Runtime.Upstash.Enabled {
			cacheType = "upstash"
		} else if spec.Runtime.Redis.Enabled || redisClient != nil {
			cacheType = "redis"
		} else {
			cacheType = "memory"
		}
	}

	var provider cache.Provider
	switch cacheType {
	case "redis":
		if redisClient != nil {
			provider = cachebattery.NewRedisProvider(redisClient)
		} else {
			provider = cachebattery.NewMemoryProvider()
		}
	case "upstash":
		if spec.Runtime.Upstash.Url != "" {
			provider = cachebattery.NewUpstashProvider(spec.Runtime.Upstash.Url, spec.Runtime.Upstash.Token)
		} else {
			provider = cachebattery.NewMemoryProvider()
		}
	case "memory":
		fallthrough
	default:
		provider = cachebattery.NewMemoryProvider()
	}
	return cache.NewTracingProvider(provider)
}

func resolveBlobProvider(spec *manifest.ProjectSpec) blob.Provider {
	blobType := spec.Batteries.Blob
	if blobType == "" {
		blobType = "local"
	}
	switch blobType {
	case "s3", "r2", "minio":
		p, err := blob.NewS3ProviderFromEnv()
		if err == nil {
			return p
		}
		return blob.NewLocalProvider("default-secret-change-me", ".bffx/uploads")
	case "local":
		fallthrough
	default:
		return blob.NewLocalProvider("default-secret-change-me", ".bffx/uploads")
	}
}

func resolveObservabilityProvider(spec *manifest.ProjectSpec) observability.Provider {
	obsType := spec.Batteries.Observability
	if obsType == "" {
		obsType = coreobs.LogProviderSlog
	}
	switch obsType {
	case coreobs.LogProviderAxiom:
		token := os.Getenv("BFFX_AXIOM_TOKEN")
		if token == "" {
			token = os.Getenv("AXIOM_TOKEN")
		}
		dataset := os.Getenv("BFFX_AXIOM_DATASET")
		if dataset == "" {
			dataset = os.Getenv("AXIOM_DATASET")
		}
		if token != "" && dataset != "" {
			return observability.NewAxiomProvider(dataset, token)
		}
		return observability.NewSlogProvider()
	case coreobs.LogProviderSentry:
		dsn := os.Getenv("BFFX_SENTRY_DSN")
		if dsn == "" {
			dsn = os.Getenv("SENTRY_DSN")
		}
		if dsn != "" {
			return observability.NewSentryProvider(dsn)
		}
		return observability.NewSlogProvider()
	case coreobs.LogProviderSlog:
		fallthrough
	default:
		return observability.NewSlogProvider()
	}
}

func resolveAnalyticsProvider(spec *manifest.ProjectSpec) analytics.Provider {
	anaType := spec.Batteries.Analytics
	if anaType == "" {
		anaType = "noop"
	}
	switch anaType {
	case "posthog":
		apiKey := os.Getenv("BFFX_POSTHOG_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("POSTHOG_API_KEY")
		}
		host := os.Getenv("BFFX_POSTHOG_HOST")
		if host == "" {
			host = os.Getenv("POSTHOG_HOST")
		}
		if apiKey != "" {
			p := analytics.NewPostHog(apiKey, host, nil)
			return analytics.NewDispatcher(p, 5)
		}
		return analytics.NewNoopProvider()
	case "noop":
		fallthrough
	default:
		return analytics.NewNoopProvider()
	}
}

func resolveVLMProvider(spec *manifest.ProjectSpec) vlm.Provider {
	vlmType := spec.Batteries.Vlm
	if vlmType == "" {
		vlmType = "noop"
	}
	switch vlmType {
	case "gemini":
		key := os.Getenv("BFFX_GEMINI_API_KEY")
		if key != "" {
			p, err := vlm.NewGeminiProvider(context.Background(), key, "")
			if err == nil {
				return p
			}
		}
	case "ollama":
		return vlm.NewOllamaProvider(os.Getenv("BFFX_OLLAMA_HOST"), "")
	case "openrouter":
		key := os.Getenv("BFFX_OPENROUTER_API_KEY")
		if key != "" {
			return vlm.NewOpenRouterProvider(key, "")
		}
	}
	return vlm.NewNoopProvider()
}

func resolveNutritionProvider(spec *manifest.ProjectSpec) nutrition.Provider {
	nutType := spec.Batteries.Nutrition
	if nutType == "" {
		nutType = "noop"
	}
	switch nutType {
	case "openfoodfacts":
		return nutrition.NewOpenFoodFactsProvider()
	default:
		return nutrition.NewNoopProvider()
	}
}

func resolveStoreMode(spec *manifest.ProjectSpec) string {
	if spec.Batteries.Store != "" {
		return spec.Batteries.Store
	}
	if spec.Store.Mode != "" {
		return spec.Store.Mode
	}
	return "sqlite"
}

func Resolve(root string, spec *manifest.ProjectSpec, reg *manifest.Registry, jwtSvc *auth.JWTService, redisClient *redis.Client, store storage.Store) (*Registry, error) {
	if spec == nil {
		spec = &manifest.ProjectSpec{}
	}

	r := &Registry{
		Store:         resolveStoreMode(spec),
		Cache:         resolveCacheProvider(spec, redisClient),
		Blob:          resolveBlobProvider(spec),
		Observability: resolveObservabilityProvider(spec),
		Analytics:     resolveAnalyticsProvider(spec),
		Vlm:           resolveVLMProvider(spec),
		Nutrition:     resolveNutritionProvider(spec),
	}
	if spec.Batteries.I18n != "none" {
		r.I18n = i18n.NewLocalizer(r.Cache)
	}

	authProvider, err := resolveAuthProvider(spec, jwtSvc)
	if err != nil {
		return nil, err
	}
	r.Auth = authProvider

	flagProvider, err := flagproviders.NewProvider(root, spec, reg, store)
	if err != nil {
		return nil, err
	}
	r.Flags = flagProvider

	return r, nil
}
