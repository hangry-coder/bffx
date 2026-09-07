package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hangry-coder/bffx/pkg/admin/features"
	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
)

// FeaturesHandler serves the App Features admin section.
type FeaturesHandler struct {
	reg        *manifest.Registry
	auditor    *audit.Auditor
	runner     atomic.Value // features.ScreenRunner; nil-safe; loaded with .Load()
	cache      atomic.Value // features.CacheInvalidator; nil-safe
	killSwitch features.KillSwitchStore
	telemetry  observability.TelemetryProvider
}

// SetTelemetryProvider wires the provider used by GET /features/screens/{name}/metrics.
// Pass nil to disable the metrics endpoint.
func (h *FeaturesHandler) SetTelemetryProvider(p observability.TelemetryProvider) {
	h.telemetry = p
}

func NewFeaturesHandler(reg *manifest.Registry) *FeaturesHandler {
	h := &FeaturesHandler{reg: reg}
	return h
}

// SetKillSwitchStore wires the persistence layer used by the kill switch
// endpoints. Passing nil disables the endpoints.
func (h *FeaturesHandler) SetKillSwitchStore(store features.KillSwitchStore) {
	h.killSwitch = store
}

// SetAuditor sets the auditor used by Inspect to record impersonation calls.
func (h *FeaturesHandler) SetAuditor(a *audit.Auditor) { h.auditor = a }

// SetRunner wires the in-process Screen runner. Calling SetRunner(nil) disables
// the run-as endpoint and falls back to a 503 response.
func (h *FeaturesHandler) SetRunner(r features.ScreenRunner) {
	if r == nil {
		h.runner.Store((features.ScreenRunner)(nil))
		return
	}
	h.runner.Store(r)
}

func (h *FeaturesHandler) currentRunner() features.ScreenRunner {
	if v := h.runner.Load(); v != nil {
		if r, ok := v.(features.ScreenRunner); ok {
			return r
		}
	}
	return nil
}

// SetCacheInvalidator wires the tag cache flusher used by the per-Screen
// and per-Section cache invalidate endpoints. Pass nil to disable them.
func (h *FeaturesHandler) SetCacheInvalidator(c features.CacheInvalidator) {
	if c == nil {
		h.cache.Store((features.CacheInvalidator)(nil))
		return
	}
	h.cache.Store(c)
}

func (h *FeaturesHandler) currentInvalidator() features.CacheInvalidator {
	if v := h.cache.Load(); v != nil {
		if c, ok := v.(features.CacheInvalidator); ok {
			return c
		}
	}
	return nil
}

// ListFeatures returns the feature tree for the admin UI.
//
// Query parameters:
//
//	group=<name>  optional, case-insensitive; restricts to one group
func (h *FeaturesHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	tree := features.Walk(h.reg, group)

	type respScreen struct {
		features.ScreenNode
		KillSwitch          *features.KillSwitch           `json:"kill_switch,omitempty"`
		SectionKillSwitches map[string]features.KillSwitch `json:"section_kill_switches,omitempty"`
	}
	type respGroup struct {
		Name        string       `json:"name"`
		DisplayName string       `json:"display_name"`
		Screens     []respScreen `json:"screens"`
	}
	type resp struct {
		Groups          []respGroup               `json:"groups"`
		OrphanActions   []features.OrphanAction   `json:"orphan_actions"`
		FeatureClusters []features.FeatureCluster `json:"feature_clusters"`
	}

	// The UI expects every slice to be non-nil so it can map() safely;
	// guarantee that here regardless of what Walk returned.
	out := resp{
		OrphanActions:   tree.OrphanActions,
		FeatureClusters: tree.FeatureClusters,
	}
	if out.OrphanActions == nil {
		out.OrphanActions = []features.OrphanAction{}
	}
	if out.FeatureClusters == nil {
		out.FeatureClusters = []features.FeatureCluster{}
	}
	for _, g := range tree.Groups {
		grp := respGroup{Name: g.Name, DisplayName: g.DisplayName, Screens: make([]respScreen, 0, len(g.Screens))}
		for _, s := range g.Screens {
			rs := respScreen{ScreenNode: s}
			if h.killSwitch != nil {
				if ks, err := h.killSwitch.Get(r.Context(), s.Name, ""); err == nil && ks != nil {
					rs.KillSwitch = ks
				}
				if len(s.Sections) > 0 {
					rs.SectionKillSwitches = make(map[string]features.KillSwitch, len(s.Sections))
					for _, sec := range s.Sections {
						if ks, err := h.killSwitch.Get(r.Context(), s.Name, sec.Key); err == nil && ks != nil {
							rs.SectionKillSwitches[sec.Key] = *ks
						}
					}
					if len(rs.SectionKillSwitches) == 0 {
						rs.SectionKillSwitches = nil
					}
				}
			}
			grp.Screens = append(grp.Screens, rs)
		}
		out.Groups = append(out.Groups, grp)
	}
	if out.Groups == nil {
		out.Groups = []respGroup{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// RunAsRequest is the body of POST /features/screens/{name}/run-as.
type RunAsRequest struct {
	UserID      string            `json:"user_id,omitempty"`
	Locale      string            `json:"locale,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	ExtraClaims map[string]any    `json:"extra_claims,omitempty"`
}

// RunAs executes a Screen with admin impersonation.
//
// Routes:  POST /features/screens/{name}/run-as
func (h *FeaturesHandler) RunAs(w http.ResponseWriter, r *http.Request) {
	screen := r.PathValue("name")
	if strings.TrimSpace(screen) == "" {
		http.Error(w, "missing screen name", http.StatusBadRequest)
		return
	}

	runner := h.currentRunner()
	if runner == nil {
		http.Error(w, "screen runner not wired (feature unavailable on this build)", http.StatusServiceUnavailable)
		return
	}

	// Make sure the screen exists in the registry so we can surface a clean
	// 404 instead of a runner-side error.
	found := false
	for _, m := range h.reg.Screens {
		if m.Metadata.Name == screen {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "screen not found", http.StatusNotFound)
		return
	}

	var body RunAsRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	opts := features.RunOpts{
		UserID:      body.UserID,
		Locale:      body.Locale,
		ExtraClaims: body.ExtraClaims,
	}
	if len(body.Query) > 0 {
		opts.Query = make(url.Values, len(body.Query))
		for k, v := range body.Query {
			opts.Query.Set(k, v)
		}
	}
	if len(body.Headers) > 0 {
		opts.Headers = make(http.Header, len(body.Headers))
		for k, v := range body.Headers {
			opts.Headers.Set(k, v)
		}
	}

	result, err := runner.RunScreen(r.Context(), screen, opts)
	if err != nil {
		http.Error(w, "run-as failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit log the impersonation so it is traceable.
	if h.auditor != nil {
		actorID, _ := r.Context().Value(adminIDKey).(string)
		payload, _ := json.Marshal(map[string]any{
			"screen":  screen,
			"user_id": body.UserID,
			"locale":  body.Locale,
		})
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "feature.run_as",
			ResourceKind: "Screen",
			ResourceID:   screen,
			Payload:      string(payload),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// adminIDKey mirrors the string used by auth.AuthMiddleware. We cannot
// import that here without a cycle, so we use the same literal value.
const adminIDKey = "admin_id"

// KillSwitchRequest is the JSON body for kill-switch toggle endpoints.
//
// ExpiresIn is a human-friendly shortcut: "15m", "1h", "" (forever).
// When ExpiresIn is set, it overrides ExpiresAt. Otherwise ExpiresAt
// (RFC3339) is used as-is. Both nil means "no auto-revert".
type KillSwitchRequest struct {
	Enabled   bool       `json:"enabled"`
	Reason    string     `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	ExpiresIn string     `json:"expires_in,omitempty"`
}

// ToggleScreenKillSwitch handles PATCH /features/screens/{name}.
func (h *FeaturesHandler) ToggleScreenKillSwitch(w http.ResponseWriter, r *http.Request) {
	h.toggleKillSwitch(w, r, r.PathValue("name"), "")
}

// ToggleSectionKillSwitch handles PATCH /features/screens/{name}/sections/{key}.
func (h *FeaturesHandler) ToggleSectionKillSwitch(w http.ResponseWriter, r *http.Request) {
	h.toggleKillSwitch(w, r, r.PathValue("name"), r.PathValue("key"))
}

// ListKillSwitches handles GET /features/kill-switches.
func (h *FeaturesHandler) ListKillSwitches(w http.ResponseWriter, r *http.Request) {
	if h.killSwitch == nil {
		http.Error(w, "kill switch store not wired", http.StatusServiceUnavailable)
		return
	}
	rows, err := h.killSwitch.List(r.Context())
	if err != nil {
		http.Error(w, "list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"kill_switches": rows})
}

func (h *FeaturesHandler) toggleKillSwitch(w http.ResponseWriter, r *http.Request, screen, section string) {
	if strings.TrimSpace(screen) == "" {
		http.Error(w, "missing screen name", http.StatusBadRequest)
		return
	}
	if h.killSwitch == nil {
		http.Error(w, "kill switch store not wired", http.StatusServiceUnavailable)
		return
	}

	// Validate that the screen (and section, if supplied) is known.
	var screenFound bool
	var sectionFound bool
	for _, m := range h.reg.Screens {
		if m.Metadata.Name != screen {
			continue
		}
		screenFound = true
		if section == "" {
			break
		}
		var spec manifest.ScreenSpec
		_ = m.UnmarshalSpec(&spec)
		for _, sec := range spec.Sections {
			if sec.Key == section {
				sectionFound = true
				break
			}
		}
		break
	}
	if !screenFound {
		http.Error(w, "screen not found", http.StatusNotFound)
		return
	}
	if section != "" && !sectionFound {
		http.Error(w, "section not found on screen", http.StatusNotFound)
		return
	}

	var body KillSwitchRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	expiresAt := body.ExpiresAt
	if body.ExpiresIn != "" {
		d, err := time.ParseDuration(body.ExpiresIn)
		if err != nil {
			http.Error(w, "invalid expires_in (use e.g. 15m, 1h): "+err.Error(), http.StatusBadRequest)
			return
		}
		t := time.Now().Add(d).UTC()
		expiresAt = &t
	}

	upd := features.KillSwitchUpdate{
		Enabled:   body.Enabled,
		Reason:    body.Reason,
		ExpiresAt: expiresAt,
	}
	if actorID, ok := r.Context().Value(adminIDKey).(string); ok {
		upd.UpdatedBy = actorID
	}
	if actorEmail, ok := r.Context().Value("admin_email").(string); ok && upd.UpdatedBy == "" {
		upd.UpdatedBy = actorEmail
	}

	ks, err := h.killSwitch.Set(r.Context(), screen, section, upd)
	if err != nil {
		http.Error(w, "set kill switch failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditor != nil {
		actorID, _ := r.Context().Value(adminIDKey).(string)
		payload, _ := json.Marshal(map[string]any{
			"screen":     screen,
			"section":    section,
			"enabled":    body.Enabled,
			"reason":     body.Reason,
			"expires_at": expiresAt,
		})
		action := "feature.kill_switch.screen"
		if section != "" {
			action = "feature.kill_switch.section"
		}
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       action,
			ResourceKind: "Screen",
			ResourceID:   screen,
			Payload:      string(payload),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ks)
}

// InvalidateScreenCache handles POST /features/screens/{name}/cache/invalidate.
func (h *FeaturesHandler) InvalidateScreenCache(w http.ResponseWriter, r *http.Request) {
	h.invalidate(w, r, r.PathValue("name"), "")
}

// InvalidateSectionCache handles POST /features/screens/{name}/sections/{key}/cache/invalidate.
func (h *FeaturesHandler) InvalidateSectionCache(w http.ResponseWriter, r *http.Request) {
	h.invalidate(w, r, r.PathValue("name"), r.PathValue("key"))
}

func (h *FeaturesHandler) invalidate(w http.ResponseWriter, r *http.Request, screen, section string) {
	if strings.TrimSpace(screen) == "" {
		http.Error(w, "missing screen name", http.StatusBadRequest)
		return
	}
	inv := h.currentInvalidator()
	if inv == nil {
		http.Error(w, "cache invalidator not wired", http.StatusServiceUnavailable)
		return
	}

	found := false
	for _, m := range h.reg.Screens {
		if m.Metadata.Name == screen {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "screen not found", http.StatusNotFound)
		return
	}

	var (
		n   int
		err error
	)
	if section == "" {
		n, err = inv.InvalidateScreenCache(r.Context(), screen)
	} else {
		n, err = inv.InvalidateSectionCache(r.Context(), screen, section)
	}
	if err != nil {
		http.Error(w, "invalidate failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditor != nil {
		actorID, _ := r.Context().Value(adminIDKey).(string)
		payload, _ := json.Marshal(map[string]any{
			"screen":  screen,
			"section": section,
			"flushed": n,
		})
		action := "feature.cache.invalidate.screen"
		if section != "" {
			action = "feature.cache.invalidate.section"
		}
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       action,
			ResourceKind: "Screen",
			ResourceID:   screen,
			Payload:      string(payload),
			IPAddress:    r.RemoteAddr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"flushed": n})
}

// ScreenMetrics handles GET /features/screens/{name}/metrics.
//
// The current implementation returns the project-wide telemetry summary
// together with the provider's capability descriptor. Per-screen instrumentation
// is a follow-up; until then the admin UI is expected to render a single
// "Observe" tile with the project-wide vitals and, when the provider exposes
// a deep_link, a "View in <Provider>" button.
func (h *FeaturesHandler) ScreenMetrics(w http.ResponseWriter, r *http.Request) {
	screen := r.PathValue("name")
	if strings.TrimSpace(screen) == "" {
		http.Error(w, "missing screen name", http.StatusBadRequest)
		return
	}
	// Validate the screen exists; we do not return 503 when telemetry is
	// nil because the UI still wants to display the catalog metadata
	// without metrics tiles.
	found := false
	for _, m := range h.reg.Screens {
		if m.Metadata.Name == screen {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "screen not found", http.StatusNotFound)
		return
	}

	resp := map[string]any{
		"screen":   screen,
		"provider": nil,
		"samples":  []observability.MetricSample{},
	}

	if h.telemetry != nil {
		resp["provider"] = h.telemetry.Info(r.Context())
		samples, err := h.telemetry.GetSummary(r.Context())
		if err == nil {
			resp["samples"] = samples
		} else {
			resp["error"] = err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
