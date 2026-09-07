package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/hangry-coder/bffx/pkg/admin/features"
	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/hangry-coder/bffx/pkg/worker"
)

//go:embed all:ui-v2/dist
var uiV2Assets embed.FS

type Router struct {
	mux        http.Handler
	features   *handlers.FeaturesHandler
	killSwitch features.KillSwitchStore
}

// SetScreenRunner wires the in-process Screen runner used by the
// `POST /api/admin/features/screens/{name}/run-as` endpoint. Wiring is
// optional; when unset, the endpoint returns 503.
func (r *Router) SetScreenRunner(runner features.ScreenRunner) {
	if r.features != nil {
		r.features.SetRunner(runner)
	}
}

// SetCacheInvalidator wires the tag cache flusher used by the per-Screen and
// per-Section cache invalidate endpoints. nil disables those endpoints.
func (r *Router) SetCacheInvalidator(c features.CacheInvalidator) {
	if r.features != nil {
		r.features.SetCacheInvalidator(c)
	}
}

// SetTelemetryProvider wires the provider used by App Features → Observe.
func (r *Router) SetTelemetryProvider(p observability.TelemetryProvider) {
	if r.features != nil {
		r.features.SetTelemetryProvider(p)
	}
}

// SetKillSwitchStore replaces the kill switch persistence backend. By default
// the admin router uses an in-process memory store; production wiring should
// pass a SQL-backed implementation here. Returns the previous store so wiring
// code can chain replacement.
func (r *Router) SetKillSwitchStore(store features.KillSwitchStore) features.KillSwitchStore {
	prev := r.killSwitch
	if store == nil {
		store = features.NewMemoryKillSwitchStore()
	}
	r.killSwitch = store
	if r.features != nil {
		r.features.SetKillSwitchStore(store)
	}
	return prev
}

// KillSwitchStore returns the currently wired kill switch store so external
// consumers (notably the API screens evaluator) can read it.
func (r *Router) KillSwitchStore() features.KillSwitchStore {
	return r.killSwitch
}

func NewRouter(store, telemetry storage.Store, jobStore worker.JobStore, q worker.Queue, reg *manifest.Registry, bus events.Bus, auditor *audit.Auditor, flagProvider featureflags.FlagProvider, liveopsScheduler *liveops.Scheduler) *Router {

	mux := http.NewServeMux()

	resH := handlers.NewResourceHandler(store, telemetry, reg, auditor)
	adminUsersH := handlers.NewAdminUserHandler(store, auditor)
	jobsH := handlers.NewJobsHandler(jobStore, q)
	telProv := observability.GetTelemetryProvider(telemetry)
	metricsH := handlers.NewMetricsHandler(telProv, store, reg)
	flagsH := handlers.NewFlagHandlerV2(flagProvider, reg)
	liveopsH := handlers.NewLiveOpsHandler(liveopsScheduler, reg)
	authH := NewAuthHandler(store, reg)
	usersH := handlers.NewUserHandler(store, reg)
	settingsH := handlers.NewSettingsHandler(store, reg)
	incidentsH := handlers.NewIncidentsHandler(store)
	providersH := handlers.NewProvidersHandler(store)
	navPrefsH := handlers.NewNavPreferencesHandler(reg)
	featuresH := handlers.NewFeaturesHandler(reg)
	featuresH.SetAuditor(auditor)
	defaultKillSwitch := features.NewMemoryKillSwitchStore()
	featuresH.SetKillSwitchStore(defaultKillSwitch)
	docsH := handlers.NewDocsHandler(reg)

	// Admin Auth
	mux.HandleFunc("POST /api/admin/login", authH.Login)
	mux.HandleFunc("POST /api/admin/logout", authH.Logout)


	// Admin API (Protected)
	adminAPI := http.NewServeMux()
	adminAPI.HandleFunc("GET /session", authH.Session)
	adminAPI.HandleFunc("GET /nav-preferences", navPrefsH.Get)
	adminAPI.HandleFunc("PUT /nav-preferences", navPrefsH.Put)
	adminAPI.HandleFunc("GET /config", resH.GetConfig)
	adminAPI.HandleFunc("GET /docs/openapi.json", docsH.GetOpenAPISpec)
	adminAPI.HandleFunc("GET /docs/guide/index.json", docsH.ListGuideDocs)
	adminAPI.HandleFunc("GET /docs/guide/{path...}", docsH.GetGuideDoc)
	adminAPI.HandleFunc("GET /docs", docsH.GetDocsHTML)
	adminAPI.HandleFunc("GET /resources", resH.ListResources)
	adminAPI.HandleFunc("GET /resources/{name}/export.csv", resH.ExportCSV)
	adminAPI.HandleFunc("GET /resources/{name}", resH.GetRecords)
	adminAPI.HandleFunc("GET /resources/{name}/{id}", resH.GetRecord)
	adminAPI.HandleFunc("POST /resources/{name}", resH.CreateRecord)
	adminAPI.HandleFunc("PATCH /resources/{name}/{id}", resH.UpdateRecord)
	adminAPI.HandleFunc("DELETE /resources/AdminUser/{id}", adminUsersH.DeleteAdminUser)
	adminAPI.HandleFunc("DELETE /resources/{name}/{id}", resH.DeleteRecord)
	adminAPI.HandleFunc("POST /resources/{name}/batch/{action}", resH.HandleBatchAction)
	adminAPI.HandleFunc("POST /resources/{name}/{id}/actions/{action}", resH.HandleMemberAction)
	adminAPI.HandleFunc("POST /resources/{name}/collection/{action}", resH.HandleCollectionAction)
	adminAPI.HandleFunc("GET /resources/{name}/{id}/associations/{association}", resH.GetAssociations)

	adminAPI.HandleFunc("GET /flags", flagsH.ListFlags)
	adminAPI.HandleFunc("POST /flags", flagsH.CreateFlag)
	adminAPI.HandleFunc("GET /flags/{key}", flagsH.GetFlag)
	adminAPI.HandleFunc("PUT /flags/{key}", flagsH.UpdateFlag)
	adminAPI.HandleFunc("PATCH /flags/{key}/toggle", flagsH.ToggleFlag)
	adminAPI.HandleFunc("DELETE /flags/{key}", flagsH.DeleteFlag)
	adminAPI.HandleFunc("POST /flags/{key}/bindings", flagsH.AddBinding)
	adminAPI.HandleFunc("GET /screens/{name}/sections", flagsH.ListScreenSections)

	adminAPI.HandleFunc("GET /liveops", liveopsH.ListEvents)
	adminAPI.HandleFunc("POST /liveops", liveopsH.CreateEvent)
	adminAPI.HandleFunc("GET /liveops/{name}", liveopsH.GetEvent)
	adminAPI.HandleFunc("PUT /liveops/{name}", liveopsH.UpdateEvent)
	adminAPI.HandleFunc("PATCH /liveops/{name}/toggle", liveopsH.ToggleEvent)
	adminAPI.HandleFunc("DELETE /liveops/{name}", liveopsH.DeleteEvent)

	adminAPI.HandleFunc("GET /jobs", jobsH.ListJobs)
	adminAPI.HandleFunc("GET /jobs/{id}", jobsH.GetJob)
	adminAPI.HandleFunc("POST /jobs/{id}/retry", jobsH.RetryJob)
	adminAPI.HandleFunc("POST /jobs/{id}/cancel", jobsH.CancelJob)
	
	adminAPI.HandleFunc("GET /metrics", metricsH.GetSystemMetrics)
	adminAPI.HandleFunc("GET /metrics/summary", metricsH.GetMetricsSummary)
	adminAPI.HandleFunc("GET /metrics/stream", metricsH.StreamMetrics)
	adminAPI.HandleFunc("GET /dashboard/data", metricsH.GetDashboardData)

	adminAPI.HandleFunc("GET /users", usersH.ListUsers)
	adminAPI.HandleFunc("GET /users/{id}", usersH.GetUserDetail)
	
	adminAPI.HandleFunc("GET /settings", settingsH.GetSettings)
	adminAPI.HandleFunc("PUT /settings", settingsH.UpdateSettings)

	adminAPI.HandleFunc("GET /incidents", incidentsH.GetIncidents)
	adminAPI.HandleFunc("POST /incidents/{id}/resolve", incidentsH.ResolveIncident)
	adminAPI.HandleFunc("GET /providers", providersH.GetProviders)

	adminAPI.HandleFunc("GET /features", featuresH.ListFeatures)
	adminAPI.HandleFunc("POST /features/screens/{name}/run-as", featuresH.RunAs)
	adminAPI.HandleFunc("PATCH /features/screens/{name}", featuresH.ToggleScreenKillSwitch)
	adminAPI.HandleFunc("PATCH /features/screens/{name}/sections/{key}", featuresH.ToggleSectionKillSwitch)
	adminAPI.HandleFunc("GET /features/kill-switches", featuresH.ListKillSwitches)
	adminAPI.HandleFunc("POST /features/screens/{name}/cache/invalidate", featuresH.InvalidateScreenCache)
	adminAPI.HandleFunc("POST /features/screens/{name}/sections/{key}/cache/invalidate", featuresH.InvalidateSectionCache)
	adminAPI.HandleFunc("GET /features/screens/{name}/metrics", featuresH.ScreenMetrics)

	// Mount protected API
	mux.Handle("/api/admin/", http.StripPrefix("/api/admin", AuthMiddleware(store, reg, adminAPI)))

	// Static UI Assets
	uiSub, _ := fs.Sub(uiV2Assets, "ui-v2/dist")
	fsSys := http.FS(uiSub)
	
	fileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If path has a dot (like .js, .css, .png), serve as static file
		if strings.Contains(r.URL.Path, ".") {
			http.FileServer(fsSys).ServeHTTP(w, r)
			return
		}
		// Otherwise serve index.html (SPA routing fallback)
		f, err := fsSys.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f)
	})
	
	mux.Handle("/", fileServer)

	// Apply IP Whitelisting to everything in Admin
	finalHandler := IPMiddleware(reg, mux)

	return &Router{mux: finalHandler, features: featuresH, killSwitch: defaultKillSwitch}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func IPMiddleware(reg *manifest.Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spec := reg.ProjectSpec()

		if len(spec.Admin.AllowedIPs) > 0 {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
			if os.Getenv("BFFX_TRUST_PROXY_HEADERS") == "true" {
				if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					ip = strings.TrimSpace(strings.Split(xff, ",")[0])
				}
			}
			
			allowed := false
			for _, allowedIP := range spec.Admin.AllowedIPs {
				if ip == allowedIP || allowedIP == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "Forbidden: IP not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
