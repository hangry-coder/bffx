// Package router provides the core HTTP routing pipeline, CORS enforcement, JWT middleware,
// and request lifecycle hooks mapping manifests to Go controllers.
package router

import (
	"context"
	"fmt"
	"github.com/hangry-coder/bffx/pkg/addons/ads"
	"github.com/hangry-coder/bffx/pkg/addons/billing"
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/api/validation"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/auth/revocation"
	"github.com/hangry-coder/bffx/pkg/batteries/analytics"
	"github.com/hangry-coder/bffx/pkg/batteries/audit"
	"github.com/hangry-coder/bffx/pkg/batteries/nutrition"
	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/hangry-coder/bffx/pkg/cache"
	"github.com/hangry-coder/bffx/pkg/cache/idempotency"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/game/leaderboard"
	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
	"github.com/hangry-coder/bffx/pkg/storage"
	blobstore "github.com/hangry-coder/bffx/pkg/storage/blob"
	"github.com/hangry-coder/bffx/pkg/worker"

	"github.com/redis/go-redis/v9"
	"net/http"
	"os"
	"strings"
	"time"
)

// Router is the central request orchestrator for a BFFX project. It manages
// HTTP multiplexing, dependency injection for handlers, middleware chaining,
// and dynamic route registration based on the project registry.
type Router struct {
	mux                 *http.ServeMux
	store               storage.Store
	reg                 *manifest.Registry
	validator           *validation.Validator
	auth                auth.Provider
	jwt                 *auth.JWTService
	jobStore            worker.JobStore
	queue               worker.Queue // may be nil
	workerSecret        string
	actionHandlers      map[string]handlers.ActionHandler
	notify              *notifications.Manager
	bus                 events.Bus
	policyEngine        *auth.PolicyEngine
	featureFlags        *featureflags.FlagService
	billing             *billing.Manager
	ads                 *ads.AdService
	email               *email.Manager
	comm                *comm.Hub
	hooks               map[string]HookFunc
	telemetry           storage.Store
	i18n                *i18n.Bundle
	otp                 *auth.OTPService
	redis               *redis.Client
	blob                blobstore.Provider
	analytics           analytics.Provider
	tagCache            cache.Provider
	idempotency         idempotency.Store
	stripeWebhookSecret string
	localizer           *i18n.Localizer
	vlm                 vlm.Provider
	nutrition           nutrition.Provider
	audit               audit.Provider
	handler             http.Handler
	killSwitch          KillSwitchEvaluator
	liveopsScheduler    *liveops.Scheduler
	LeaderboardService  *leaderboard.LeaderboardService
}

// KillSwitchEvaluator is the minimal contract the screens evaluator needs
// from the kill switch store. It mirrors the relevant slice of
// runtimecontracts.KillSwitchStore so the api/router package does not depend
// on admin-specific types for screen runtime evaluation.
type KillSwitchEvaluator interface {
	Get(ctx context.Context, screen, section string) (*runtimecontracts.KillSwitch, error)
}

// SetKillSwitchEvaluator wires a kill switch lookup function. nil is allowed
// and disables the check.
func (r *Router) SetKillSwitchEvaluator(ev KillSwitchEvaluator) {
	r.killSwitch = ev
}

// HookFunc defines the signature for custom business logic hooks that can be
// triggered before or after resource mutations.
type HookFunc func(*handlers.ActionContext, map[string]any) error

// RouterCoreDeps groups core runtime dependencies used by Router.
type RouterCoreDeps struct {
	Store          storage.Store
	Telemetry      storage.Store
	Registry       *manifest.Registry
	AuthProvider   auth.Provider
	JWTService     *auth.JWTService
	WorkerSecret   string
	EventBus       events.Bus
	Hooks          map[string]HookFunc
	ActionHandlers map[string]handlers.ActionHandler
}

// RouterInfraDeps groups infrastructure/runtime dependencies.
type RouterInfraDeps struct {
	JobStore     worker.JobStore
	Queue        worker.Queue
	Redis        *redis.Client
	BlobProvider blobstore.Provider
	TagCache     cache.Provider
	Idempotency  idempotency.Store
}

// RouterCommsDeps groups communication and localization dependencies.
type RouterCommsDeps struct {
	Notifications *notifications.Manager
	Email         *email.Manager
	CommHub       *comm.Hub
	I18n          *i18n.Bundle
	Localizer     *i18n.Localizer
}

// RouterFeatureDeps groups optional feature batteries and services.
type RouterFeatureDeps struct {
	FlagProvider        featureflags.FlagProvider
	Analytics           analytics.Provider
	VlmProvider         vlm.Provider
	NutritionProvider   nutrition.Provider
	AuditProvider       audit.Provider
	StripeWebhookSecret string
	LiveOpsScheduler    *liveops.Scheduler
	LeaderboardService  *leaderboard.LeaderboardService
}

// RouterConfig holds all dependencies and settings required to initialize a
// Router. It is typically populated in the orchestrator's main.go.
type RouterConfig struct {
	Core     *RouterCoreDeps
	Infra    *RouterInfraDeps
	Comms    *RouterCommsDeps
	Features *RouterFeatureDeps

	Store          storage.Store
	Telemetry      storage.Store
	JobStore       worker.JobStore
	Queue          worker.Queue
	Registry       *manifest.Registry
	AuthProvider   auth.Provider
	JWTService     *auth.JWTService
	WorkerSecret   string
	EventBus       events.Bus
	Hooks          map[string]HookFunc
	Notifications  *notifications.Manager
	Email          *email.Manager
	CommHub        *comm.Hub
	I18n           *i18n.Bundle
	Redis          *redis.Client
	FlagProvider   featureflags.FlagProvider
	ActionHandlers map[string]handlers.ActionHandler
	// BlobProvider issues presigned object URLs and handles deletions (S3 or local dev).
	BlobProvider blobstore.Provider
	// Analytics emits product events (e.g. PostHog). Nil uses a no-op implementation.
	Analytics analytics.Provider
	// TagCache is the enterprise tagged cache engine.
	TagCache cache.Provider
	// Idempotency is the store for mutation replay.
	Idempotency idempotency.Store
	// StripeWebhookSecret enables POST /api/v1/billing/stripe/webhook when non-empty.
	StripeWebhookSecret string
	Localizer           *i18n.Localizer
	VlmProvider         vlm.Provider
	NutritionProvider   nutrition.Provider
	AuditProvider       audit.Provider
	LiveOpsScheduler    *liveops.Scheduler
	LeaderboardService  *leaderboard.LeaderboardService
}

func normalizeRouterConfig(cfg RouterConfig) RouterConfig {
	resolved := cfg
	if cfg.Core != nil {
		if cfg.Core.Store != nil {
			resolved.Store = cfg.Core.Store
		}
		if cfg.Core.Telemetry != nil {
			resolved.Telemetry = cfg.Core.Telemetry
		}
		if cfg.Core.Registry != nil {
			resolved.Registry = cfg.Core.Registry
		}
		if cfg.Core.AuthProvider != nil {
			resolved.AuthProvider = cfg.Core.AuthProvider
		}
		if cfg.Core.JWTService != nil {
			resolved.JWTService = cfg.Core.JWTService
		}
		if cfg.Core.WorkerSecret != "" {
			resolved.WorkerSecret = cfg.Core.WorkerSecret
		}
		if cfg.Core.EventBus != nil {
			resolved.EventBus = cfg.Core.EventBus
		}
		if cfg.Core.Hooks != nil {
			resolved.Hooks = cfg.Core.Hooks
		}
		if cfg.Core.ActionHandlers != nil {
			resolved.ActionHandlers = cfg.Core.ActionHandlers
		}
	}
	if cfg.Infra != nil {
		if cfg.Infra.JobStore != nil {
			resolved.JobStore = cfg.Infra.JobStore
		}
		if cfg.Infra.Queue != nil {
			resolved.Queue = cfg.Infra.Queue
		}
		if cfg.Infra.Redis != nil {
			resolved.Redis = cfg.Infra.Redis
		}
		if cfg.Infra.BlobProvider != nil {
			resolved.BlobProvider = cfg.Infra.BlobProvider
		}
		if cfg.Infra.TagCache != nil {
			resolved.TagCache = cfg.Infra.TagCache
		}
		if cfg.Infra.Idempotency != nil {
			resolved.Idempotency = cfg.Infra.Idempotency
		}
	}
	if cfg.Comms != nil {
		if cfg.Comms.Notifications != nil {
			resolved.Notifications = cfg.Comms.Notifications
		}
		if cfg.Comms.Email != nil {
			resolved.Email = cfg.Comms.Email
		}
		if cfg.Comms.CommHub != nil {
			resolved.CommHub = cfg.Comms.CommHub
		}
		if cfg.Comms.I18n != nil {
			resolved.I18n = cfg.Comms.I18n
		}
		if cfg.Comms.Localizer != nil {
			resolved.Localizer = cfg.Comms.Localizer
		}
	}
	if cfg.Features != nil {
		if cfg.Features.FlagProvider != nil {
			resolved.FlagProvider = cfg.Features.FlagProvider
		}
		if cfg.Features.Analytics != nil {
			resolved.Analytics = cfg.Features.Analytics
		}
		if cfg.Features.VlmProvider != nil {
			resolved.VlmProvider = cfg.Features.VlmProvider
		}
		if cfg.Features.NutritionProvider != nil {
			resolved.NutritionProvider = cfg.Features.NutritionProvider
		}
		if cfg.Features.AuditProvider != nil {
			resolved.AuditProvider = cfg.Features.AuditProvider
		}
		if cfg.Features.StripeWebhookSecret != "" {
			resolved.StripeWebhookSecret = cfg.Features.StripeWebhookSecret
		}
		if cfg.Features.LiveOpsScheduler != nil {
			resolved.LiveOpsScheduler = cfg.Features.LiveOpsScheduler
		}
		if cfg.Features.LeaderboardService != nil {
			resolved.LeaderboardService = cfg.Features.LeaderboardService
		}
	}
	return resolved
}

// Authed wraps a handler with the standard authentication middleware.
func (r *Router) Authed(h http.HandlerFunc) http.Handler {
	checker := revocation.NewRedisChecker(r.redis)
	return middleware.Auth(r.reg.ApiPrefix, r.auth, r.billing, r.store, checker)(h)
}

// NewRouter initializes a Router with the provided configuration and
// sets up its internal services (PolicyEngine, FeatureService, etc.).
func NewRouter(cfg RouterConfig) *Router {
	cfg = normalizeRouterConfig(cfg)
	an := cfg.Analytics
	if an == nil {
		an = analytics.NewNoopProvider()
	}
	au := cfg.AuditProvider
	if au == nil {
		au = audit.NewMemoryProvider()
	}
	r := &Router{
		mux:       http.NewServeMux(),
		store:     cfg.Store,
		reg:       cfg.Registry,
		validator: validation.NewValidator(),
		auth:      cfg.AuthProvider,
		jwt:       cfg.JWTService,
		jobStore:  cfg.JobStore,
		telemetry: cfg.Telemetry,
		redis:     cfg.Redis,
		queue:     cfg.Queue,

		workerSecret:        cfg.WorkerSecret,
		bus:                 cfg.EventBus,
		policyEngine:        auth.NewPolicyEngine(),
		featureFlags:        featureflags.NewFlagService(cfg.Registry, cfg.FlagProvider),
		billing:             billing.NewManager(cfg.Store, cfg.Registry),
		ads:                 ads.NewAdService(cfg.Registry),
		notify:              cfg.Notifications,
		email:               cfg.Email,
		comm:                cfg.CommHub,
		hooks:               cfg.Hooks,
		i18n:                cfg.I18n,
		otp:                 auth.NewOTPService(),
		actionHandlers:      cfg.ActionHandlers,
		blob:                cfg.BlobProvider,
		analytics:           an,
		tagCache:            cfg.TagCache,
		idempotency:         cfg.Idempotency,
		stripeWebhookSecret: strings.TrimSpace(cfg.StripeWebhookSecret),
		localizer:           cfg.Localizer,
		vlm:                 cfg.VlmProvider,
		nutrition:           cfg.NutritionProvider,
		audit:               au,
		liveopsScheduler:    cfg.LiveOpsScheduler,
		LeaderboardService:  cfg.LeaderboardService,
	}

	if r.actionHandlers == nil {
		r.actionHandlers = make(map[string]handlers.ActionHandler)
	}

	if r.reg != nil && r.reg.ApiPrefix == "" {
		r.reg.ApiPrefix = "/api/v1"
		if r.reg.Project != nil {
			var ps manifest.ProjectSpec
			if err := r.reg.Project.UnmarshalSpec(&ps); err == nil && ps.App.ApiPrefix != "" {
				r.reg.ApiPrefix = ps.App.ApiPrefix
			}
		}
	}

	if cfg.Redis != nil && r.jwt != nil {
		checker := revocation.NewRedisChecker(cfg.Redis)
		r.jwt.SetRevocationChecker(checker.IsRevoked)
	}

	observability.RegistryResourcesTotal.Set(float64(len(r.reg.Resources)))

	return r
}

func (r *Router) SetActionHandlers(handlers map[string]handlers.ActionHandler) {
	r.actionHandlers = handlers
}

func (r *Router) SetHooks(hooks map[string]HookFunc) {
	r.hooks = hooks
}

func (r *Router) SetTagCache(t cache.Provider) {
	r.tagCache = t
}

// WithQueue injects an optional Redis queue into the router.
func (r *Router) WithQueue(q worker.Queue) {
	r.queue = q
}

// Audit logs a security or mutation event using the configured audit provider.
func (r *Router) Audit(ctx context.Context, action, resource, resourceID string, metadata map[string]any) {
	if r.audit == nil {
		return
	}
	entry := observability.AuditEntry{
		Timestamp:  time.Now(),
		UserID:     middleware.GetUserID(ctx),
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Metadata:   metadata,
		Level:      observability.AuditLevelInfo,
		IP:         middleware.GetIP(ctx),
		UserAgent:  middleware.GetUserAgent(ctx),
	}
	_ = r.audit.Log(ctx, entry)
}

// Mux returns the underlying http.ServeMux.
func (r *Router) Mux() *http.ServeMux {
	return r.mux
}

// Handler returns the fully initialized middleware chain. Call Setup() first.
func (r *Router) Handler() http.Handler {
	return r.handler
}

type setupHandlers struct {
	authH    *handlers.AuthHandler
	jobsH    *handlers.JobsHandler
	metricsH *handlers.MetricsHandler
	healthH  *handlers.HealthHandler
	commonH  *handlers.CommonHandler
}

type featureHandlers struct {
	vlmH         *handlers.VLMHandler
	nutH         *handlers.NutritionHandler
	leaderboardH *handlers.LeaderboardHandler
	billingH     *handlers.BillingHandler
}

func (r *Router) initSetupHandlers() setupHandlers {
	authH := handlers.NewAuthHandlerWithRegistry(r.store, r.auth, r.jwt, r.reg).WithHooks(func(resource string, event string, req *http.Request, data map[string]any) error {
		m, ok := r.reg.GetResource(resource)
		if !ok {
			return nil
		}
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)

		var hooks []manifest.HookConfig
		if event == "afterCreate" {
			hooks = spec.Hooks.AfterCreate
		} else if event == "afterUpdate" {
			hooks = spec.Hooks.AfterUpdate
		}

		if len(hooks) > 0 {
			ctx := r.newActionContext(req)
			return r.executeHooks(hooks, ctx, data)
		}
		return nil
	})
	if r.redis != nil {
		authH.WithRevocation(revocation.NewRedisChecker(r.redis))
	}
	return setupHandlers{
		authH:    authH,
		jobsH:    handlers.NewJobsHandler(r.jobStore, r.queue),
		metricsH: handlers.NewMetricsHandler(),
		healthH:  handlers.NewHealthHandler(r.store, r.redis),
		commonH:  handlers.NewCommonHandler(r.reg, r.featureFlags, r.ads, r.i18n, r.store, r.liveopsScheduler),
	}
}

func (r *Router) initFeatureHandlers() featureHandlers {
	appleSecret := os.Getenv("BFFX_APPLE_SHARED_SECRET")
	googleConfig := os.Getenv("BFFX_GOOGLE_PLAY_CONFIG_JSON")
	return featureHandlers{
		vlmH:         handlers.NewVLMHandler(r.vlm),
		nutH:         handlers.NewNutritionHandler(r.nutrition),
		leaderboardH: handlers.NewLeaderboardHandler(r.LeaderboardService),
		billingH:     handlers.NewBillingHandler(r.billing, r.store, appleSecret, googleConfig),
	}
}

func (r *Router) registerCoreAndAuthRoutes(h setupHandlers) {
	r.mux.HandleFunc("GET /health", h.healthH.Health)
	r.mux.HandleFunc("GET /metrics", h.metricsH.GetMetrics)

	// Check if a builder overrides the bootstrap route
	hasBootstrapBuilder := false
	for _, b := range r.reg.Builders {
		var spec manifest.BuilderSpec
		if err := b.UnmarshalSpec(&spec); err == nil {
			if spec.Route.Path == fmt.Sprintf("%s/app/bootstrap", r.reg.ApiPrefix) && spec.Route.Method == "GET" {
				hasBootstrapBuilder = true
				break
			}
		}
	}
	if !hasBootstrapBuilder {
		r.mux.HandleFunc(fmt.Sprintf("GET %s/app/bootstrap", r.reg.ApiPrefix), h.commonH.Bootstrap)
	}
	r.mux.HandleFunc(fmt.Sprintf("GET %s/ui/registry", r.reg.ApiPrefix), h.commonH.UIRegistry)

	// Hello Welcome Route
	r.mux.HandleFunc(fmt.Sprintf("GET %s/hello", r.reg.ApiPrefix), func(w http.ResponseWriter, req *http.Request) {
		errors.WriteJSON(w, http.StatusOK, map[string]any{
			"name":             r.reg.Project.Metadata.Name,
			"version":          "0.1.0",
			"apiPrefix":        r.reg.ApiPrefix,
			"minClientVersion": "0.1.0",
		})
	})

	r.mux.HandleFunc(fmt.Sprintf("GET %s/auth/handshake", r.reg.ApiPrefix), h.authH.Handshake)
	credLimit := middleware.AuthCredentialRateLimit(middleware.TrustForwardedHeaders())
	r.mux.Handle(fmt.Sprintf("POST %s/auth/signup", r.reg.ApiPrefix), credLimit(http.HandlerFunc(h.authH.Signup)))
	r.mux.Handle(fmt.Sprintf("POST %s/auth/register", r.reg.ApiPrefix), credLimit(http.HandlerFunc(h.authH.Signup)))
	r.mux.Handle(fmt.Sprintf("POST %s/auth/login", r.reg.ApiPrefix), credLimit(http.HandlerFunc(h.authH.Login)))
	r.mux.HandleFunc(fmt.Sprintf("POST %s/auth/refresh", r.reg.ApiPrefix), h.authH.Refresh)
	r.mux.Handle(fmt.Sprintf("POST %s/auth/logout", r.reg.ApiPrefix), r.Authed(h.authH.Logout))
	r.mux.Handle(fmt.Sprintf("POST %s/auth/link", r.reg.ApiPrefix), r.Authed(h.authH.Link))
	anonProtect := middleware.AnonymousSessionProtect(r.redis)
	r.mux.Handle(fmt.Sprintf("POST %s/auth/anonymous", r.reg.ApiPrefix), anonProtect(http.HandlerFunc(h.authH.AnonymousSession)))
}

func (r *Router) registerFeatureReadRoutes(fh featureHandlers) {
	r.mux.HandleFunc(fmt.Sprintf("POST %s/vlm/analyze", r.reg.ApiPrefix), r.wrapWithAuth(fh.vlmH.Analyze, "optional"))
	r.mux.HandleFunc(fmt.Sprintf("POST %s/nutrition", r.reg.ApiPrefix), r.wrapWithAuth(fh.nutH.GetNutrition, "optional"))
	r.mux.HandleFunc(fmt.Sprintf("POST %s/leaderboards/{name}/submit", r.reg.ApiPrefix), r.wrapWithAuth(fh.leaderboardH.Submit, "required"))
	r.mux.HandleFunc(fmt.Sprintf("GET %s/leaderboards/{name}/rankings", r.reg.ApiPrefix), r.wrapWithAuth(fh.leaderboardH.Rankings, "optional"))
}

func (r *Router) registerOptionalDocsAndMeRoute() {
	// OAuth2 (mock) — register only when BFFX_ENABLE_OAUTH=true (not a real ID-token flow).
	if os.Getenv("BFFX_ENABLE_OAUTH") == "true" {
		r.mux.HandleFunc(fmt.Sprintf("GET %s/auth/google", r.reg.ApiPrefix), func(w http.ResponseWriter, r *http.Request) {
			errors.WriteJSON(w, http.StatusOK, map[string]string{"url": "https://accounts.google.com/o/oauth2/auth?..."})
		})
		r.mux.HandleFunc(fmt.Sprintf("GET %s/auth/google/callback", r.reg.ApiPrefix), func(w http.ResponseWriter, req *http.Request) {
			errors.WriteJSON(w, http.StatusOK, map[string]string{"token": "mock-oauth-token"})
		})
	}

	r.mux.HandleFunc(fmt.Sprintf("GET %s/me", r.reg.ApiPrefix), func(w http.ResponseWriter, req *http.Request) {
		userID := middleware.GetUserID(req.Context())
		if userID == "" {
			errors.Write(w, errors.ErrUnauthorized)
			return
		}
		user, err := r.store.Get(req.Context(), "User", userID)
		if err != nil {
			errors.WriteError(w, http.StatusNotFound, "user not found", "user_not_found")
			return
		}
		delete(user, "password")
		errors.WriteJSON(w, http.StatusOK, user)
	})
}

func (r *Router) registerJobsAndUploadsRoutes(h setupHandlers) {
	r.mux.HandleFunc(fmt.Sprintf("POST %s/jobs/dispatch", r.reg.ApiPrefix), r.wrapWithAuth(h.jobsH.Dispatch, "required"))
	r.mux.HandleFunc(fmt.Sprintf("GET %s/jobs/{id}", r.reg.ApiPrefix), r.wrapWithAuth(h.jobsH.GetJob, "required"))
	r.mux.HandleFunc(fmt.Sprintf("GET %s/jobs", r.reg.ApiPrefix), r.wrapWithAuth(h.jobsH.ListJobs, "required"))

	if r.blob != nil {
		uh := handlers.NewUploadsHandler(r.blob, r.reg)
		r.mux.HandleFunc(fmt.Sprintf("POST %s/uploads/presign", r.reg.ApiPrefix), r.wrapWithAuth(uh.Presign, "required"))
		uh.MountLocal(r.mux)
	}

	// Protected write-back
	writeBackHandler := http.HandlerFunc(h.jobsH.WriteBack)
	r.mux.Handle(fmt.Sprintf("PATCH %s/jobs/{id}/result", r.reg.ApiPrefix), middleware.WorkerAuth(r.workerSecret)(writeBackHandler))
}

func (r *Router) manifestActionRouteOccupied(method, path string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, m := range r.reg.Actions {
		mth, p := r.reg.GetManifestRoute(m)
		if strings.EqualFold(mth, method) && p == path {
			return true
		}
	}
	return false
}

func (r *Router) registerBillingRoutes(fh featureHandlers) {
	if r.stripeWebhookSecret != "" {
		sh := billing.NewStripeWebhook(r.stripeWebhookSecret, r.redis, billing.StripeEventHooks{})
		r.mux.Handle(fmt.Sprintf("POST %s/billing/stripe/webhook", r.reg.ApiPrefix), sh)
	}
	verifyPath := fmt.Sprintf("%s/billing/verify", r.reg.ApiPrefix)
	if !r.manifestActionRouteOccupied("POST", verifyPath) {
		r.mux.Handle(fmt.Sprintf("POST %s", verifyPath), r.Authed(fh.billingH.Verify))
	}
	restorePath := fmt.Sprintf("%s/billing/restore", r.reg.ApiPrefix)
	if !r.manifestActionRouteOccupied("POST", restorePath) {
		r.mux.Handle(fmt.Sprintf("POST %s", restorePath), r.Authed(fh.billingH.Restore))
	}
}

// Setup configures all routes and middleware, returning the final http.Handler.
func (r *Router) Setup() http.Handler {
	sh := r.initSetupHandlers()
	fh := r.initFeatureHandlers()
	r.registerCoreAndAuthRoutes(sh)
	r.registerFeatureReadRoutes(fh)
	r.registerOptionalDocsAndMeRoute()
	r.registerJobsAndUploadsRoutes(sh)
	r.registerBillingRoutes(fh)

	r.registerCRUDRoutes()
	r.registerBuilderRoutes()
	r.registerScreenRoutes()
	r.registerActionRoutes()
	r.registerStreamRoutes()
	r.registerPipelineRoutes()

	// Resolve security settings
	var projectSpec manifest.ProjectSpec
	r.reg.Project.UnmarshalSpec(&projectSpec)

	rps := float64(projectSpec.Security.RateLimit.RequestsPerMinute) / 60.0
	if rps <= 0 {
		rps = 2.0 // 120 RPM default
	}
	burst := projectSpec.Security.RateLimit.BurstSize
	if burst <= 0 {
		burst = 20
	}
	allowedOrigins := projectSpec.Security.AllowedOrigins

	handler := http.Handler(r.mux)
	if r.tagCache != nil {
		handler = middleware.InvalidateActionCacheOnMutation(r.tagCache)(handler)
	}
	if projectSpec.App.AuthStrategy == "mandatory" {
		handler = middleware.EnforceAuth(r.reg.ApiPrefix)(handler)
	}
	checker := revocation.NewRedisChecker(r.redis)
	handler = middleware.Auth(r.reg.ApiPrefix, r.auth, r.billing, r.store, checker)(handler)
	if projectSpec.FeatureFlags.Idempotency && r.idempotency != nil {
		handler = middleware.IdempotencyWithStrictDurable(r.idempotency, 24*time.Hour, r.strictDurableIdempotencyRoutes())(handler)
	}
	handler = middleware.Locale("en")(handler)

	lifecycle := projectSpec.Lifecycle
	if len(lifecycle.Platforms) == 0 && projectSpec.App.MinClientVersion != "" {
		lifecycle.Platforms = map[string]manifest.PlatformLifecycle{
			"default": {MinVersion: projectSpec.App.MinClientVersion},
		}
	}
	handler = middleware.ClientVersion(lifecycle)(handler)

	handler = middleware.RateLimit(r.redis, rps, burst)(handler)
	handler = middleware.AppSecret(r.reg.ApiPrefix, os.Getenv("BFFX_APP_SECRET"))(handler)
	isProd := os.Getenv("BFFX_ENV") == "production"
	handler = middleware.ForceSSL(projectSpec.App.ForceSSL, isProd)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Timeout(30 * time.Second)(handler)
	handler = middleware.CORS(allowedOrigins)(handler)
	handler = middleware.Logger(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Tracing("bffx-framework")(handler)
	handler = middleware.MetricsAccess(handler)
	handler = middleware.Metrics()(handler)
	handler = middleware.MaxBytes(1 << 20)(handler)
	handler = middleware.RecoveryWithStore(r.store)(handler)

	r.handler = handler
	return handler
}

func (r *Router) strictDurableIdempotencyRoutes() map[string]bool {
	strict := map[string]bool{}
	for _, m := range r.reg.Actions {
		var spec manifest.ActionSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(spec.Route.IdempotencyMode), "durable") {
			continue
		}
		method, path := r.reg.GetManifestRoute(m)
		if method == "" || path == "" {
			continue
		}
		strict[strings.ToUpper(method)+" "+path] = true
	}
	return strict
}

// Store returns the underlying storage.Store.
func (r *Router) Store() storage.Store {
	return r.store
}

// Registry returns the underlying manifest.Registry.
func (r *Router) Registry() *manifest.Registry {
	return r.reg
}
