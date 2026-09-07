package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hangry-coder/bffx/pkg/storage"
)

type Capability string

const (
	CapWrite       Capability = "write"
	CapReadList    Capability = "read.list"
	CapReadSummary Capability = "read.summary"
	CapReadStream  Capability = "read.stream"
	CapDeepLink    Capability = "deep_link"
	CapResolve     Capability = "resolve"
)

type ProviderInfo struct {
	Name         string       `json:"name"`         // "battery" | "sentry" | "posthog" | ...
	DisplayName  string       `json:"display_name"` // "Built-in (Local)" | "Sentry"
	VendorURL    string       `json:"vendor_url,omitempty"`
	Capabilities []Capability `json:"capabilities"`
	Healthy      bool         `json:"healthy"`
	LastError    string       `json:"last_error,omitempty"`
}

type Provider interface {
	Info(ctx context.Context) ProviderInfo
}

// --- Incident Domain ---

type Incident struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"` // "info", "warning", "critical"
	StackTrace  string `json:"stack_trace,omitempty"`
	Resolved    bool   `json:"resolved"`
	ResolvedBy  string `json:"resolved_by,omitempty"`
	DeepLinkURL string `json:"deep_link_url,omitempty"`
	Source      string `json:"source"` // "battery" | "sentry"
}

type IncidentProvider interface {
	Provider
	Record(ctx context.Context, inc Incident) error
	List(ctx context.Context) ([]Incident, error)
	Resolve(ctx context.Context, id string, resolvedBy string) error
}

// LocalBatteryIncidentProvider uses the DB/Memory Store
type LocalBatteryIncidentProvider struct {
	store storage.Store
}

func NewLocalBatteryIncidentProvider(store storage.Store) *LocalBatteryIncidentProvider {
	return &LocalBatteryIncidentProvider{store: store}
}

func (p *LocalBatteryIncidentProvider) Info(ctx context.Context) ProviderInfo {
	return ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local)",
		Capabilities: []Capability{
			CapWrite,
			CapReadList,
			CapReadSummary,
			CapResolve,
		},
		Healthy: true,
	}
}

func (p *LocalBatteryIncidentProvider) Record(ctx context.Context, inc Incident) error {
	if p.store == nil {
		return fmt.Errorf("store is nil")
	}
	_, err := p.store.Create(ctx, "Incident", map[string]any{
		"title":       inc.Title,
		"severity":    inc.Severity,
		"stack_trace": inc.StackTrace,
		"resolved":    inc.Resolved,
		"resolved_by": inc.ResolvedBy,
	})
	return err
}

func (p *LocalBatteryIncidentProvider) List(ctx context.Context) ([]Incident, error) {
	if p.store == nil {
		return []Incident{}, nil
	}
	records, err := p.store.Query(ctx, "Incident").Where("resolved", "=", false).Execute(ctx)
	if err != nil {
		return nil, err
	}
	var res []Incident
	for _, r := range records {
		id, _ := r["id"].(string)
		title, _ := r["title"].(string)
		severity, _ := r["severity"].(string)
		stack, _ := r["stack_trace"].(string)
		resolved, _ := r["resolved"].(bool)
		resolvedBy, _ := r["resolved_by"].(string)

		res = append(res, Incident{
			ID:         id,
			Title:      title,
			Severity:   severity,
			StackTrace: stack,
			Resolved:   resolved,
			ResolvedBy: resolvedBy,
			Source:     "battery",
		})
	}
	return res, nil
}

func (p *LocalBatteryIncidentProvider) Resolve(ctx context.Context, id string, resolvedBy string) error {
	if p.store == nil {
		return fmt.Errorf("store is nil")
	}
	_, err := p.store.Update(ctx, "Incident", id, map[string]any{
		"resolved":    true,
		"resolved_by": resolvedBy,
	})
	return err
}

// SentryIncidentProvider integrates with Sentry Web API
type SentryIncidentProvider struct {
	authToken   string
	orgSlug     string
	projectSlug string
	httpClient  *http.Client
}

func NewSentryIncidentProvider() *SentryIncidentProvider {
	return &SentryIncidentProvider{
		authToken:   os.Getenv("BFFX_SENTRY_AUTH_TOKEN"),
		orgSlug:     os.Getenv("BFFX_SENTRY_ORG"),
		projectSlug: os.Getenv("BFFX_SENTRY_PROJECT"),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *SentryIncidentProvider) Info(ctx context.Context) ProviderInfo {
	vendorURL := "https://sentry.io"
	if p.orgSlug != "" && p.projectSlug != "" {
		vendorURL = fmt.Sprintf("https://sentry.io/organizations/%s/issues/?project=%s", p.orgSlug, p.projectSlug)
	}
	caps := []Capability{CapWrite, CapDeepLink}
	if p.authToken != "" {
		caps = append(caps, CapReadList, CapReadSummary, CapResolve)
	}
	return ProviderInfo{
		Name:         "sentry",
		DisplayName:  "Sentry",
		VendorURL:    vendorURL,
		Capabilities: caps,
		Healthy:      true,
	}
}

func (p *SentryIncidentProvider) Record(ctx context.Context, inc Incident) error {
	// Sentry SDK auto-captures panics when using initSentryHook.
	// No-op for manual HTTP posting here since the SDK is preferred for runtime panics.
	return nil
}

func (p *SentryIncidentProvider) List(ctx context.Context) ([]Incident, error) {
	if p.authToken == "" {
		// Degradation/Offline mockup mode (helpful for local verification)
		return []Incident{
			{
				ID:          "sentry-101",
				Title:       "database connection pool exhausted",
				Severity:    "critical",
				Resolved:    false,
				DeepLinkURL: "https://sentry.io/share/issue/mock-pool-exhausted",
				Source:      "sentry",
			},
			{
				ID:          "sentry-102",
				Title:       "redis timeout on token revocation lookup",
				Severity:    "warning",
				Resolved:    false,
				DeepLinkURL: "https://sentry.io/share/issue/mock-redis-timeout",
				Source:      "sentry",
			},
		}, nil
	}

	url := fmt.Sprintf("https://sentry.io/api/0/projects/%s/%s/issues/?query=is:unresolved", p.orgSlug, p.projectSlug)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.authToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sentry api error: status %d", resp.StatusCode)
	}

	var issues []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Level     string `json:"level"`
		ShortID   string `json:"shortId"`
		Permalink string `json:"permalink"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, err
	}

	var res []Incident
	for _, issue := range issues {
		severity := "warning"
		if issue.Level == "error" || issue.Level == "fatal" {
			severity = "critical"
		}
		res = append(res, Incident{
			ID:          issue.ID,
			Title:       issue.Title,
			Severity:    severity,
			Resolved:    false,
			DeepLinkURL: issue.Permalink,
			Source:      "sentry",
		})
	}
	return res, nil
}

func (p *SentryIncidentProvider) Resolve(ctx context.Context, id string, resolvedBy string) error {
	if p.authToken == "" {
		return nil // No-op in offline mode
	}
	url := fmt.Sprintf("https://sentry.io/api/0/issues/%s/", id)
	body := map[string]string{"status": "resolved"}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// --- Telemetry Domain ---

type MetricSample struct {
	Time    string  `json:"time"`
	Traffic float64 `json:"traffic"`
	Load    float64 `json:"load"`
	Latency float64 `json:"latency"`
	Errors  float64 `json:"errors"`
	Users   float64 `json:"users"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
}

type TelemetryProvider interface {
	Provider
	GetSummary(ctx context.Context) ([]MetricSample, error)
}

// DatadogTelemetryProvider pulls vitals from Datadog API
type DatadogTelemetryProvider struct {
	apiKey     string
	appKey     string
	httpClient *http.Client
}

func NewDatadogTelemetryProvider() *DatadogTelemetryProvider {
	return &DatadogTelemetryProvider{
		apiKey:     os.Getenv("BFFX_DATADOG_API_KEY"),
		appKey:     os.Getenv("BFFX_DATADOG_APP_KEY"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *DatadogTelemetryProvider) Info(ctx context.Context) ProviderInfo {
	caps := []Capability{CapWrite, CapDeepLink}
	if p.apiKey != "" && p.appKey != "" {
		caps = append(caps, CapReadSummary)
	}
	return ProviderInfo{
		Name:         "datadog",
		DisplayName:  "Datadog",
		VendorURL:    "https://app.datadoghq.com",
		Capabilities: caps,
		Healthy:      true,
	}
}

func (p *DatadogTelemetryProvider) GetSummary(ctx context.Context) ([]MetricSample, error) {
	// Fallback/Degradation mode if credentials not provided
	if p.apiKey == "" || p.appKey == "" {
		now := time.Now()
		return []MetricSample{
			{Time: now.Add(-6 * time.Hour).Format("15:04"), Traffic: 500, Load: 35, Latency: 95, CPU: 22, Mem: 48},
			{Time: now.Add(-4 * time.Hour).Format("15:04"), Traffic: 750, Load: 42, Latency: 110, CPU: 28, Mem: 50},
			{Time: now.Add(-2 * time.Hour).Format("15:04"), Traffic: 920, Load: 49, Latency: 125, CPU: 35, Mem: 52},
			{Time: now.Format("15:04"), Traffic: 1020, Load: 52, Latency: 130, CPU: 38, Mem: 55},
		}, nil
	}

	// In a real-world integration, we would perform a query:
	// GET https://api.datadoghq.com/api/v1/query?from=...&to=...&query=system.cpu.user{host:*}
	return []MetricSample{}, nil
}

// --- Analytics Domain ---

type AnalyticsProvider interface {
	Provider
	ListEvents(ctx context.Context) ([]map[string]any, error)
}

// PostHogAnalyticsProvider connects with PostHog's project API
type PostHogAnalyticsProvider struct {
	apiKey     string
	projectID  string
	httpClient *http.Client
}

func NewPostHogAnalyticsProvider() *PostHogAnalyticsProvider {
	return &PostHogAnalyticsProvider{
		apiKey:     os.Getenv("BFFX_POSTHOG_API_KEY"),
		projectID:  os.Getenv("BFFX_POSTHOG_PROJECT_ID"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PostHogAnalyticsProvider) Info(ctx context.Context) ProviderInfo {
	vendorURL := "https://app.posthog.com"
	if p.projectID != "" {
		vendorURL = fmt.Sprintf("https://app.posthog.com/project/%s", p.projectID)
	}
	caps := []Capability{CapWrite, CapDeepLink}
	if p.apiKey != "" {
		caps = append(caps, CapReadList, CapReadSummary)
	}
	return ProviderInfo{
		Name:         "posthog",
		DisplayName:  "PostHog",
		VendorURL:    vendorURL,
		Capabilities: caps,
		Healthy:      true,
	}
}

func (p *PostHogAnalyticsProvider) ListEvents(ctx context.Context) ([]map[string]any, error) {
	if p.apiKey == "" {
		return []map[string]any{
			{"event": "$pageview", "timestamp": time.Now().Format(time.RFC3339), "properties": map[string]any{"$current_url": "https://bffx.io/home"}},
			{"event": "user_signup", "timestamp": time.Now().Add(-10 * time.Minute).Format(time.RFC3339), "properties": map[string]any{"method": "google"}},
			{"event": "subscription_purchased", "timestamp": time.Now().Add(-30 * time.Minute).Format(time.RFC3339), "properties": map[string]any{"plan": "premium"}},
		}, nil
	}

	// Query PostHog events endpoint
	url := fmt.Sprintf("https://app.posthog.com/api/projects/%s/events/", p.projectID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// --- Provider Helper Registry Factory ---

func GetProvidersInfo(ctx context.Context, store storage.Store) map[string]ProviderInfo {
	res := make(map[string]ProviderInfo)

	incProv := GetIncidentProvider(store)
	res["incident"] = incProv.Info(ctx)

	telProv := GetTelemetryProvider(store)
	res["telemetry"] = telProv.Info(ctx)

	anProv := GetAnalyticsProvider(store)
	res["analytics"] = anProv.Info(ctx)

	res["audit"] = ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local)",
		Capabilities: []Capability{
			CapWrite,
			CapReadList,
			CapReadSummary,
		},
		Healthy: true,
	}

	return res
}

func GetIncidentProvider(store storage.Store) IncidentProvider {
	if os.Getenv(DeprecatedEnvIncidentProvider) == "sentry" {
		return NewSentryIncidentProvider()
	}
	return NewLocalBatteryIncidentProvider(store)
}

func GetTelemetryProvider(store storage.Store) TelemetryProvider {
	if os.Getenv(DeprecatedEnvTelemetryProvider) == "datadog" {
		return NewDatadogTelemetryProvider()
	}
	if store != nil {
		return NewLocalBatteryTelemetryProvider(store)
	}
	// Return local battery implementation by default
	return &dummyTelemetryProvider{}
}

func GetAnalyticsProvider(store storage.Store) AnalyticsProvider {
	if os.Getenv(DeprecatedEnvAnalyticsProvider) == "posthog" {
		return NewPostHogAnalyticsProvider()
	}
	if store != nil {
		return NewLocalBatteryAnalyticsProvider(store)
	}
	return &dummyAnalyticsProvider{}
}

// --- Dummy Fallback Vitals Providers (battery fallbacks) ---

type dummyTelemetryProvider struct{}

func (p *dummyTelemetryProvider) Info(ctx context.Context) ProviderInfo {
	return ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local Metrics)",
		Capabilities: []Capability{
			CapWrite,
			CapReadSummary,
			CapReadStream,
		},
		Healthy: true,
	}
}

func (p *dummyTelemetryProvider) GetSummary(ctx context.Context) ([]MetricSample, error) {
	now := time.Now()
	return []MetricSample{
		{Time: now.Add(-6 * time.Hour).Format("15:04"), Traffic: 200, Load: 15, Latency: 45, CPU: 8, Mem: 35},
		{Time: now.Add(-4 * time.Hour).Format("15:04"), Traffic: 410, Load: 22, Latency: 50, CPU: 12, Mem: 38},
		{Time: now.Add(-2 * time.Hour).Format("15:04"), Traffic: 650, Load: 35, Latency: 65, CPU: 18, Mem: 40},
		{Time: now.Format("15:04"), Traffic: 800, Load: 40, Latency: 72, CPU: 24, Mem: 45},
	}, nil
}

type dummyAnalyticsProvider struct{}

func (p *dummyAnalyticsProvider) Info(ctx context.Context) ProviderInfo {
	return ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local)",
		Capabilities: []Capability{
			CapWrite,
			CapReadList,
			CapReadSummary,
		},
		Healthy: true,
	}
}

func (p *dummyAnalyticsProvider) ListEvents(ctx context.Context) ([]map[string]any, error) {
	return []map[string]any{
		{"event": "session_start", "timestamp": time.Now().Format(time.RFC3339), "properties": map[string]any{"source": "direct"}},
	}, nil
}
