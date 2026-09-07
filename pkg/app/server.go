package app

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hangry-coder/bffx/pkg/admin"
	"github.com/hangry-coder/bffx/pkg/admin/features"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/api/router"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/batteries"
	"github.com/hangry-coder/bffx/pkg/buildprofile"
	wiregrpc "github.com/hangry-coder/bffx/pkg/batteries/wire/grpc"
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
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/version"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/storage/migrator"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
	"github.com/hangry-coder/bffx/pkg/worker"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

// CustomRouteRegistrar is an optional user-defined callback that allows developers
// to register custom HTTP routes directly onto the underlying *http.ServeMux before the server starts.
var CustomRouteRegistrar func(mux *http.ServeMux)

// GRPCRegister is an optional user-defined or generated callback that allows registering
// gRPC services directly onto the gRPC server before it starts serving.
var GRPCRegister func(s *grpc.Server, r *router.Router)

var traceIDExtractorOnce sync.Once

func initRedisClient(projectSpec *manifest.ProjectSpec) *redis.Client {
	redisURL := projectSpec.Runtime.Redis.Url
	if !projectSpec.Runtime.Redis.Enabled || redisURL == "" {
		return nil
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Warn("Failed to parse Redis URL: %v", err)
		return nil
	}
	redisClient := redis.NewClient(opts)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis ping failed: %v", err)
		return nil
	}
	return redisClient
}

func initJobStore(root string, projectSpec *manifest.ProjectSpec) (worker.JobStore, error) {
	if projectSpec.Store.Mode == "sqlite" {
		sqliteJobStore, err := worker.NewSQLiteJobStore(root + "/bffx_jobs.db")
		if err != nil {
			return nil, fmt.Errorf("init sqlite job store: %w", err)
		}
		logger.Info("Using SQLiteJobStore at ./bffx_jobs.db")
		return sqliteJobStore, nil
	}
	logger.Info("Using MemoryJobStore")
	return worker.NewMemoryJobStore(), nil
}

func initEventBus(redisClient *redis.Client) events.Bus {
	if redisClient != nil {
		logger.Info("Redis event bus connected")
		return events.NewRedisBus(redisClient)
	}
	logger.Info("Using Memory event bus")
	return events.NewMemoryBus()
}

func initCacheAndStore(store storage.Store, cacheProvider cache.Provider) (storage.Store, cache.Provider, idempotency.Store) {
	tCache := cache.NewIndexedProvider(cacheProvider)
	logger.Info("Cache battery initialized: %s", tCache.Type())
	idempStore := idempotency.NewCacheStore(tCache)
	logger.Info("Idempotency store initialized via Cache battery")

	if tCache != nil {
		store = &storage.InvalidatingStore{
			Store: store,
			OnWrite: func(ctx context.Context, resource string, id string) {
				middleware.InvalidateActionCache(ctx, tCache)
				if os.Getenv("BFFX_TAG_CACHE_INVALIDATE") == "false" {
					return
				}
				tags := []string{"resource:" + resource}
				if id != "" {
					tags = append(tags, "resource:"+resource+":"+id)
				}
				_, _ = tCache.InvalidateTags(ctx, tags)
			},
		}
	}
	return store, tCache, idempStore
}

func initCommsAndI18n(root string, store storage.Store, reg *manifest.Registry) (*email.Manager, *notifications.Manager, *comm.Hub, *i18n.Bundle, error) {
	emailMgr := email.NewManager()
	switch {
	case strings.TrimSpace(os.Getenv("BFFX_SMTP_HOST")) != "":
		smtpPort := strings.TrimSpace(os.Getenv("BFFX_SMTP_PORT"))
		if smtpPort == "" {
			smtpPort = "587"
		}
		smtpFrom := strings.TrimSpace(os.Getenv("BFFX_SMTP_FROM"))
		if smtpFrom == "" {
			smtpFrom = "noreply@localhost"
		}
		emailMgr.RegisterProvider("default", &email.SMTPProvider{
			Host:     strings.TrimSpace(os.Getenv("BFFX_SMTP_HOST")),
			Port:     smtpPort,
			User:     strings.TrimSpace(os.Getenv("BFFX_SMTP_USER")),
			Password: strings.TrimSpace(os.Getenv("BFFX_SMTP_PASS")),
			From:     smtpFrom,
		})
		logger.Info("SMTP email provider active (%s port %s)", os.Getenv("BFFX_SMTP_HOST"), smtpPort)
	case strings.TrimSpace(os.Getenv("RESEND_API_KEY")) != "":
		emailMgr.RegisterProvider("default", &email.ResendProvider{
			APIKey: strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
			From:   strings.TrimSpace(os.Getenv("BFFX_RESEND_FROM")),
		})
		logger.Info("Resend email provider active")
	default:
		emailMgr.RegisterProvider("default", &email.LogProvider{})
		logger.Info("Email: set BFFX_SMTP_HOST (+ BFFX_SMTP_FROM) or RESEND_API_KEY for outbound mail")
	}

	notifyMgr := notifications.NewManager(store)
	switch {
	case strings.TrimSpace(os.Getenv("BFFX_FCM_SERVICE_ACCOUNT_JSON")) != "":
		raw := []byte(strings.TrimSpace(os.Getenv("BFFX_FCM_SERVICE_ACCOUNT_JSON")))
		pid := strings.TrimSpace(os.Getenv("BFFX_FCM_PROJECT_ID"))
		fcm := notifications.NewFirebaseV1Provider(pid, raw)
		notifyMgr.RegisterProvider("fcm", fcm)
		notifyMgr.RegisterProvider("ios", fcm)
		notifyMgr.RegisterProvider("android", fcm)
		logger.Info("FCM HTTP v1 provider registered (BFFX_FCM_SERVICE_ACCOUNT_JSON)")
	case strings.TrimSpace(os.Getenv("BFFX_FCM_SERVICE_ACCOUNT_PATH")) != "":
		p := strings.TrimSpace(os.Getenv("BFFX_FCM_SERVICE_ACCOUNT_PATH"))
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("read BFFX_FCM_SERVICE_ACCOUNT_PATH: %w", err)
		}
		pid := strings.TrimSpace(os.Getenv("BFFX_FCM_PROJECT_ID"))
		fcm := notifications.NewFirebaseV1Provider(pid, raw)
		notifyMgr.RegisterProvider("fcm", fcm)
		notifyMgr.RegisterProvider("ios", fcm)
		notifyMgr.RegisterProvider("android", fcm)
		logger.Info("FCM HTTP v1 provider registered (%s)", p)
	default:
		logger.Info("Push: FCM not configured (set BFFX_FCM_SERVICE_ACCOUNT_JSON or BFFX_FCM_SERVICE_ACCOUNT_PATH)")
	}

	tmplMgr := comm.NewTemplateManager(reg)
	commHub := comm.NewHub(store, notifyMgr, emailMgr, tmplMgr)
	if webhook := os.Getenv("BFFX_DISCORD_WEBHOOK"); webhook != "" {
		commHub.RegisterProvider(comm.Discord, &comm.DiscordProvider{WebhookURL: webhook})
	}
	if token := os.Getenv("BFFX_TELEGRAM_TOKEN"); token != "" {
		commHub.RegisterProvider(comm.Telegram, &comm.TelegramProvider{
			BotToken: token,
			ChatID:   os.Getenv("BFFX_TELEGRAM_CHAT_ID"),
		})
	}

	bundle := i18n.NewBundle("en")
	if err := bundle.DiscoverAndLoad(root); err != nil {
		logger.Warn("Failed to load translations (legacy or V2): %v", err)
	}
	return emailMgr, notifyMgr, commHub, bundle, nil
}

func initQueue(projectSpec *manifest.ProjectSpec, redisURL string) worker.Queue {
	if projectSpec.Runtime.Redis.Enabled && redisURL != "" {
		rq, err := worker.NewRedisQueue(context.Background(), redisURL)
		if err != nil {
			logger.Warn("Redis queue unavailable (%v), falling back to memory", err)
			return worker.NewMemoryQueue()
		}
		logger.Info("Redis queue connected at %s", redisURL)
		return rq
	}
	if projectSpec.Runtime.Worker.Enabled {
		logger.Info("Worker enabled, using Memory queue")
		return worker.NewMemoryQueue()
	}
	return nil
}

func initLiveopsScheduler(store storage.Store, reg *manifest.Registry) *liveops.Scheduler {
	liveopsScheduler := liveops.NewScheduler(store, reg)
	if err := liveopsScheduler.Init(); err != nil {
		logger.Error("Failed to initialize LiveOps scheduler: %v", err)
	}
	return liveopsScheduler
}

func initLeaderboardService(store storage.Store, reg *manifest.Registry, redisClient *redis.Client) *leaderboard.LeaderboardService {
	leaderboardService := leaderboard.NewService(store, reg, redisClient)
	if err := leaderboardService.Init(); err != nil {
		logger.Error("Failed to initialize leaderboard service: %v", err)
	}
	return leaderboardService
}

func mountAdminDashboardIfEnabled(projectSpec *manifest.ProjectSpec, store storage.Store, jobStore worker.JobStore, q worker.Queue, reg *manifest.Registry, eventBus events.Bus, auditor *audit.Auditor, flags featureflags.FlagProvider, liveopsScheduler *liveops.Scheduler, r *router.Router, sqlDB *sql.DB, driverName string) {
	if !projectSpec.Admin.Enabled {
		return
	}

	adminRouter := admin.NewRouter(store, store, jobStore, q, reg, eventBus, auditor, flags, liveopsScheduler)
	// Wire the in-process Screen runner so the admin "Run as user" inspect
	// endpoint can execute Screens against this Router instance.
	adminRouter.SetScreenRunner(r)
	// Wire the tag cache invalidator so admin "Reset Cache" buttons work
	// even when the project uses Redis-backed tag cache.
	adminRouter.SetCacheInvalidator(r)

	// If we have a SQL database available, swap the in-memory kill switch
	// store for a SQL-backed one that persists across restarts.
	if sqlDB != nil {
		adminRouter.SetKillSwitchStore(features.NewSQLKillSwitchStore(sqlDB, driverName))
	}
	// Share the kill switch store with the screens evaluator so live
	// requests honour admin toggles.
	r.SetKillSwitchEvaluator(adminRouter.KillSwitchStore())
	// Wire the telemetry provider so App Features -> Observe tiles can
	// render project health (and deep-link to vendor dashboards).
	adminRouter.SetTelemetryProvider(observability.GetTelemetryProvider(store))

	r.Mux().Handle("/admin/", http.StripPrefix("/admin", adminRouter))
	logger.Info("Admin dashboard enabled at /admin")
}

func startCronScheduler(ctx context.Context, reg *manifest.Registry, r *router.Router, leaderboardService *leaderboard.LeaderboardService) {
	cronSched := worker.NewCronScheduler()
	if err := cronSched.LoadFromRegistry(reg); err != nil {
		logger.Warn("failed to load cron jobs from registry: %v", err)
		return
	}
	if len(cronSched.Jobs()) == 0 {
		return
	}

	logger.Info("Starting Cron Scheduler with %d jobs", len(cronSched.Jobs()))
	err := cronSched.Start(ctx, func(job worker.CronJob) error {
		logger.Info("Executing cron job: %s (action: %s)", job.Name, job.Action)
		var err error
		if strings.HasPrefix(job.Action, "leaderboard_reset:") {
			leaderboardName := strings.TrimPrefix(job.Action, "leaderboard_reset:")
			seasonName := fmt.Sprintf("season_%d", time.Now().Unix())
			err = leaderboardService.ResetSeason(ctx, leaderboardName, seasonName)
		} else {
			err = r.ExecuteAction(ctx, job.Action, nil, nil)
		}
		if err != nil {
			logger.Error("Cron job %s failed: %v", job.Name, err)
			return err
		}
		logger.Info("Cron job %s completed successfully", job.Name)
		return nil
	})
	if err != nil {
		logger.Warn("Failed to start cron scheduler: %v", err)
	}
}

func startGRPCServerIfEnabled(projectSpec *manifest.ProjectSpec, reg *manifest.Registry, authProvider auth.Provider, store storage.Store, tCache cache.Provider, r *router.Router) (*grpc.Server, error) {
	if !projectSpec.Runtime.Wire.Enabled {
		return nil, nil
	}

	grpcPort := projectSpec.Runtime.Wire.GRPCPort
	if grpcPort == 0 {
		grpcPort = 9090
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on gRPC port %d: %w", grpcPort, err)
	}

	wirePackage := projectSpec.Runtime.Wire.Package
	if wirePackage == "" {
		wirePackage = "bffx.v1"
	}
	ttls := wiregrpc.BuildCacheTTLMap(reg, wirePackage)

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			wiregrpc.TraceIDInterceptor(),
			wiregrpc.RecoveryInterceptor(),
			wiregrpc.AuthInterceptor(authProvider, store),
			wiregrpc.UnaryCacheInterceptor(tCache, ttls),
		),
	)

	if GRPCRegister != nil {
		GRPCRegister(grpcSrv, r)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("gRPC server listener panicked: %v\n%s", r, logger.Stack())
			}
		}()
		logger.Info("gRPC server listening on :%d", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Warn("gRPC server exited with error: %v", err)
		}
	}()

	return grpcSrv, nil
}

func startHTTPServer(addr string, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Server listener panicked: %v\n%s", r, logger.Stack())
			}
		}()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Warn("dev server exited with error: %v", err)
		}
	}()

	return srv
}

func waitForShutdown(ctx context.Context, srv *http.Server, grpcSrv *grpc.Server) error {
	// Wait for interrupt signal or context cancellation
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("Shutting down server (signal received)...")
	case <-ctx.Done():
		logger.Info("Shutting down server (context cancelled)...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if grpcSrv != nil {
		logger.Info("Shutting down gRPC server...")
		grpcSrv.GracefulStop()
		logger.Info("gRPC server stopped")
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logger.Info("Server stopped")
	return nil
}

func seedBlueprints(store storage.Store, reg *manifest.Registry) {
	if err := storage.SeedRegistry(reg, store); err != nil {
		logger.Warn("Blueprint seeding incomplete: %v", err)
	}
}

func logPrimaryStorePath(root string, spec *manifest.ProjectSpec) {
	if spec == nil {
		return
	}
	switch spec.Store.Mode {
	case "sqlite":
		p := spec.Store.Path
		if p == "" {
			p = filepath.Join(".bffx", "data", "app.db")
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		logger.Info("Primary SQLite store: %s", p)
	case "postgres":
		if spec.Store.Url != "" {
			logger.Info("Primary Postgres store configured (DATABASE_URL or manifest url)")
		}
	case "memory":
		logger.Info("Primary store: in-memory (data resets each process)")
	}
}

func resolveSQLPrimaryDB(store storage.Store) (*sql.DB, string) {
	unwrappedStore := storage.UnwrapStore(store)
	routerStore, ok := unwrappedStore.(*storage.RouterStore)
	if !ok {
		return nil, ""
	}
	primary := storage.UnwrapStore(routerStore.Primary)
	if sqlite, ok := primary.(*storage.SQLiteStore); ok {
		return sqlite.GetDB(), "sqlite"
	}
	if pg, ok := primary.(*storage.PostgresStore); ok {
		return pg.GetDB(), "postgres"
	}
	return nil, ""
}

func enforceMigrationsOrReconcile(ctx context.Context, root string, projectSpec *manifest.ProjectSpec, reg *manifest.Registry, store storage.Store) (*sql.DB, string, error) {
	sqlDB, driverName := resolveSQLPrimaryDB(store)

	migrationsDir := filepath.Join(root, "migrations")
	if projectSpec.Layout == "v2" {
		migrationsDir = filepath.Join(root, "db", "migrations")
	} else if _, err := os.Stat(filepath.Join(root, "db", "migrations")); err == nil {
		// Fallback for hybrid projects
		migrationsDir = filepath.Join(root, "db", "migrations")
	}
	useMigrator := sqlDB != nil && hasMigrationFiles(migrationsDir)

	if useMigrator {
		runner, err := migrator.NewRunner(sqlDB, driverName, migrationsDir)
		if err != nil {
			return nil, "", fmt.Errorf("init migrator: %w", err)
		}
		// Do not call runner.Close(): golang-migrate closes the shared *sql.DB,
		// which would leave the primary Store unusable for the rest of the process.

		isProduction := os.Getenv("BFFX_ENV") == "production"
		if isProduction {
			pending, perr := migratorPending(sqlDB, driverName, migrationsDir, runner)
			if perr != nil {
				return nil, "", fmt.Errorf("migration drift check: %w", perr)
			}
			if pending {
				return nil, "", fmt.Errorf("refusing to start: pending migrations detected. Run `bffx migrate apply` before starting in production")
			}
			logger.Info("Production mode: schema is up to date.")
		} else {
			logger.Info("Development mode: auto-applying migrations...")
			if err := runner.Apply(); err != nil {
				logger.Warn("Auto-migration failed: %v", err)
			} else if snap, _ := schema.LoadSnapshot(filepath.Join(migrationsDir, "schema.json")); snap != nil {
				if !migrator.DatabaseHasAnyProjectTable(sqlDB, driverName, snap) {
					v, _, _ := runner.Status()
					if v > 0 {
						logger.Warn("%s", migrator.MissingTablesMessage(v))
					}
				}
			}
		}
	} else {
		// Legacy / fallback path: declarative reconcile. Used for:
		//   * non-SQL stores (Mongo/PocketBase)
		//   * SQL projects that have not yet adopted `bffx migrate init`
		if sqlDB != nil {
			logger.Info("No migrations/ directory present; using legacy declarative reconcile. Run `bffx migrate init` to adopt the versioned migration system.")
		}
		if _, err := store.Reconcile(ctx, reg); err != nil {
			logger.Warn("Schema reconciliation failed: %v", err)
		}
	}
	return sqlDB, driverName, nil
}

type runtimeRouterInputs struct {
	store              storage.Store
	reg                *manifest.Registry
	batteryReg         *batteries.Registry
	jwtSvc             *auth.JWTService
	workerSecret       string
	eventBus           events.Bus
	hooks              map[string]router.HookFunc
	actionHandlers     map[string]handlers.ActionHandler
	jobStore           worker.JobStore
	queue              worker.Queue
	redisClient        *redis.Client
	tagCache           cache.Provider
	idempotencyStore   idempotency.Store
	notifications      *notifications.Manager
	email              *email.Manager
	commHub            *comm.Hub
	bundle             *i18n.Bundle
	stripeWebhookSecret string
	liveopsScheduler   *liveops.Scheduler
	leaderboardService *leaderboard.LeaderboardService
}

func newRuntimeRouter(in runtimeRouterInputs) *router.Router {
	return router.NewRouter(router.RouterConfig{
		Core: &router.RouterCoreDeps{
			Store:          in.store,
			Telemetry:      in.store,
			Registry:       in.reg,
			AuthProvider:   in.batteryReg.Auth,
			JWTService:     in.jwtSvc,
			WorkerSecret:   in.workerSecret,
			EventBus:       in.eventBus,
			Hooks:          in.hooks,
			ActionHandlers: in.actionHandlers,
		},
		Infra: &router.RouterInfraDeps{
			JobStore:     in.jobStore,
			Queue:        in.queue,
			Redis:        in.redisClient,
			BlobProvider: in.batteryReg.Blob,
			TagCache:     in.tagCache,
			Idempotency:  in.idempotencyStore,
		},
		Comms: &router.RouterCommsDeps{
			Notifications: in.notifications,
			Email:         in.email,
			CommHub:       in.commHub,
			I18n:          in.bundle,
			Localizer:     in.batteryReg.I18n,
		},
		Features: &router.RouterFeatureDeps{
			FlagProvider:        in.batteryReg.Flags,
			Analytics:           in.batteryReg.Analytics,
			VlmProvider:         in.batteryReg.Vlm,
			NutritionProvider:   in.batteryReg.Nutrition,
			StripeWebhookSecret: in.stripeWebhookSecret,
			LiveOpsScheduler:    in.liveopsScheduler,
			LeaderboardService:  in.leaderboardService,
		},
	})
}

func RunServer(ctx context.Context, root string, port int, actionHandlers map[string]handlers.ActionHandler, hooks map[string]router.HookFunc) error {
	if err := LoadEnv(root); err != nil {
		logger.Warn("failed to load .env: %v", err)
	}

	// Connect logger to middleware once to avoid global write races
	// when tests spin multiple servers concurrently.
	traceIDExtractorOnce.Do(func() {
		logger.TraceIDExtractor = middleware.GetTraceID
	})

	overlay, err := LoadConfigOverlay(root)
	if err != nil {
		logger.Warn("failed to load config overlay: %v", err)
	}

	if err := ValidateEnv(); err != nil {
		return fmt.Errorf("env validation: %w", err)
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		return fmt.Errorf("load manifests: %w", err)
	}

	if stored, err := buildprofile.Load(root); err == nil && stored != nil {
		derived, derr := buildprofile.Derive(reg, buildprofile.DeriveOptions{GraphHash: stored.GraphHash})
		if derr == nil {
			for _, issue := range buildprofile.DriftIssues(stored, derived) {
				logger.Warn("build profile drift: %s", issue.Message)
			}
			for _, issue := range buildprofile.ValidateConsistency(stored, reg) {
				logger.Warn("build profile consistency: %s", issue.Message)
			}
		}
		logger.Info("Build profile: mode=%s ai=%t game=%t admin=%t", stored.Mode, stored.Capabilities.AI, stored.Capabilities.Game, stored.Capabilities.Admin)
	}

	// Load Project Spec to get store info and layout
	projectSpec := reg.ProjectSpec()

	// Apply Overlay
	overlay.ApplyTo(projectSpec)

	storage.ApplyEnvStoreOverrides(projectSpec)

	// CLI port override
	if port > 0 {
		projectSpec.Runtime.Api.Port = port
	}

	if err := ValidateEnvProject(projectSpec); err != nil {
		return fmt.Errorf("env validation (project): %w", err)
	}

	// Resolve JWT secret
	jwtSecret := os.Getenv("BFFX_JWT_SECRET")
	if jwtSecret == "" {
		if projectSpec.Store.Mode == "memory" {
			b := make([]byte, 16)
			crand.Read(b)

			jwtSecret = hex.EncodeToString(b)
			logger.Warn("Using auto-generated JWT secret. Set BFFX_JWT_SECRET for persistent auth.")
		} else {
			return fmt.Errorf("BFFX_JWT_SECRET must be set for %s store mode", projectSpec.Store.Mode)
		}
	}

	jwtSvc := auth.NewJWTService(jwtSecret)

	// Initialize Redis (needed for some batteries)
	redisClient := initRedisClient(projectSpec)
	redisURL := projectSpec.Runtime.Redis.Url

	// Resolve worker secret
	workerSecret := os.Getenv("BFFX_WORKER_SECRET")

	store, err := storage.NewStore(root, projectSpec, reg)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	logPrimaryStorePath(root, projectSpec)

	// Resolve batteries (now we have store for Flags)
	batteryReg, err := batteries.Resolve(root, projectSpec, reg, jwtSvc, redisClient, store)
	if err != nil {
		return fmt.Errorf("resolve batteries: %w", err)
	}

	// Connect global logger to observability battery
	logger.SetSlogLogger(batteryReg.Observability.Logger())
	logger.Info("Observability battery: %s", batteryReg.Observability.Type())

	sqlDB, driverName, err := enforceMigrationsOrReconcile(ctx, root, projectSpec, reg, store)
	if err != nil {
		return err
	}
	seedBlueprints(store, reg)

	jobStore, err := initJobStore(root, projectSpec)
	if err != nil {
		return err
	}

	// Start retention worker
	worker.StartRetentionWorker(context.Background(), jobStore, projectSpec.Jobs.RetentionDays)

	eventBus := initEventBus(redisClient)
	store, tCache, idempStore := initCacheAndStore(store, batteryReg.Cache)

	// Initialize Auditor
	auditor := audit.NewAuditor(store)

	emailMgr, notifyMgr, commHub, bundle, err := initCommsAndI18n(root, store, reg)
	if err != nil {
		return err
	}

	q := initQueue(projectSpec, redisURL)

	logger.Info("Blob battery: %s (ping: %v)", batteryReg.Blob.Type(), batteryReg.Blob.Ping(context.Background()) == nil)

	analyticsBattery := batteryReg.Analytics
	logger.Info("Analytics battery: %s", analyticsBattery.Type())
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = analyticsBattery.Close(shCtx)
	}()

	stripeWebhookSecret := strings.TrimSpace(os.Getenv("BFFX_STRIPE_WEBHOOK_SECRET"))
	if stripeWebhookSecret != "" {
		logger.Info("Stripe webhook enabled at POST /api/v1/billing/stripe/webhook")
	}

	liveopsScheduler := initLiveopsScheduler(store, reg)

	leaderboardService := initLeaderboardService(store, reg, redisClient)

	r := newRuntimeRouter(runtimeRouterInputs{
		store:               store,
		reg:                 reg,
		batteryReg:          batteryReg,
		jwtSvc:              jwtSvc,
		workerSecret:        workerSecret,
		eventBus:            eventBus,
		hooks:               hooks,
		actionHandlers:      actionHandlers,
		jobStore:            jobStore,
		queue:               q,
		redisClient:         redisClient,
		tagCache:            tCache,
		idempotencyStore:    idempStore,
		notifications:       notifyMgr,
		email:               emailMgr,
		commHub:             commHub,
		bundle:              bundle,
		stripeWebhookSecret: stripeWebhookSecret,
		liveopsScheduler:    liveopsScheduler,
		leaderboardService:  leaderboardService,
	})

	mountAdminDashboardIfEnabled(projectSpec, store, jobStore, q, reg, eventBus, auditor, batteryReg.Flags, liveopsScheduler, r, sqlDB, driverName)
	startCronScheduler(ctx, reg, r, leaderboardService)

	if CustomRouteRegistrar != nil {
		CustomRouteRegistrar(r.Mux())
	}

	r.Setup()
	handler := observability.HTTPRecoverMiddleware(r.Handler())

	addr := fmt.Sprintf(":%d", port)
	fmt.Print(buildStartupHUD(port, projectSpec, reg))

	grpcSrv, err := startGRPCServerIfEnabled(projectSpec, reg, batteryReg.Auth, store, tCache, r)
	if err != nil {
		return err
	}

	srv := startHTTPServer(addr, handler)
	return waitForShutdown(ctx, srv, grpcSrv)
}

func buildStartupHUD(port int, spec *manifest.ProjectSpec, reg *manifest.Registry) string {
	var out string
	out += "┌─────────────────────────────────────┐\n"
	out += fmt.Sprintf("│  bffx dev server  %-17s │\n", "v"+version.FrameworkVersion)
	out += fmt.Sprintf("│  http://localhost:%-18d │\n", port)
	out += "├─────────────────────────────────────┤\n"
	out += fmt.Sprintf("│  Storage  : %-23s │\n", spec.Store.Mode)

	teleStatus := "primary"
	if spec.TelemetryStore != nil {
		teleStatus = spec.TelemetryStore.Mode
	}
	out += fmt.Sprintf("│  Telemetry: %-23s │\n", teleStatus)

	redisStatus := "disabled"
	if spec.Runtime.Redis.Enabled {
		redisStatus = "enabled"
	}
	out += fmt.Sprintf("│  Redis    : %-23s │\n", redisStatus)

	workerStatus := "disabled"
	if spec.Runtime.Worker.Enabled {
		workerStatus = "enabled"
	}
	out += fmt.Sprintf("│  Worker   : %-23s │\n", workerStatus)

	jwtStatus := "auto-generated"
	if os.Getenv("BFFX_JWT_SECRET") != "" {
		jwtStatus = "env var (BFFX_JWT_...)"
	}
	out += fmt.Sprintf("│  JWT      : %-23s │\n", jwtStatus)
	out += "├─────────────────────────────────────┤\n"
	out += fmt.Sprintf("│  Resources: %-23d │\n", len(reg.Resources))
	out += fmt.Sprintf("│  Actions  : %-23d │\n", len(reg.Actions))
	out += fmt.Sprintf("│  Builders : %-23d │\n", len(reg.Builders))
	out += "└─────────────────────────────────────┘\n"
	if wStr := warnWeakSecrets(); wStr != "" {
		out += "\n" + wStr
	}
	return out
}

func warnWeakSecrets() string {
	var weak []string
	jwtVal := os.Getenv("BFFX_JWT_SECRET")
	if jwtVal == "bffx-dev-secret-keep-it-secret" || jwtVal == "dev-secret-change-me" || jwtVal == "bffx-dev-jwt-secret-key-must-be-at-least-32-characters" {
		weak = append(weak, "BFFX_JWT_SECRET")
	}
	workerVal := os.Getenv("BFFX_WORKER_SECRET")
	if workerVal == "bffx-worker-secret" || workerVal == "bffx-dev-worker-secret-key-must-be-at-least-32-characters" {
		weak = append(weak, "BFFX_WORKER_SECRET")
	}
	appVal := os.Getenv("BFFX_APP_SECRET")
	if appVal == "bffx-app-secret" || appVal == "bffx-dev-app-secret-key-must-be-at-least-16-characters" {
		weak = append(weak, "BFFX_APP_SECRET")
	}
	adminVal := os.Getenv("BFFX_ADMIN_SESSION_KEY")
	if adminVal == "admin-session-secret" || adminVal == "bffx-dev-admin-session-key-must-be-at-least-32-characters" {
		weak = append(weak, "BFFX_ADMIN_SESSION_KEY")
	}

	if len(weak) == 0 {
		return ""
	}
	var out string
	out += "⚠️  SECURITY WARNING: Weak default secrets detected in environment:\n"
	for _, w := range weak {
		out += fmt.Sprintf("   - %s\n", w)
	}
	out += "   Never use these default values in production. Set strong, random secrets in your environment/production configuration.\n"
	return out
}

// hasMigrationFiles reports whether the migrations directory exists and contains
// at least one `.up.sql` file. Used to decide between the migrator and the
// legacy declarative reconcile path.
func hasMigrationFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			return true
		}
	}
	return false
}

// migratorPending returns true when there is at least one `.up.sql` file on disk
// whose timestamp version is greater than the version currently recorded in the
// migrations table. It also flags a "dirty" state (a migration partially
// applied) as pending so production refuses to start until an operator
// resolves it.
func migratorPending(_ *sql.DB, _ string, dir string, runner *migrator.Runner) (bool, error) {
	current, dirty, err := runner.Status()
	if err != nil {
		return false, err
	}
	if dirty {
		return true, fmt.Errorf("dirty migration state at version %d", current)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	var latest uint64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// Filename is `<timestamp>_<name>.up.sql`. Extract the leading version.
		idx := strings.Index(name, "_")
		if idx <= 0 {
			continue
		}
		v, perr := parseUint(name[:idx])
		if perr != nil {
			continue
		}
		if v > latest {
			latest = v
		}
	}
	return latest > uint64(current), nil
}

func parseUint(s string) (uint64, error) {
	var n uint64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not numeric: %q", s)
		}
		n = n*10 + uint64(ch-'0')
	}
	return n, nil
}
