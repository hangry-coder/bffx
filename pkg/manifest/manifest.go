package manifest

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"strings"
)

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type Manifest struct {
	ApiVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Spec       yaml.Node `yaml:"spec"`
	Path       string    `yaml:"-"` // Internal path track
	Feature    string    `yaml:"-"` // Vertical-slice feature name (v2)
	Raw        []byte    `yaml:"-"` // Raw manifest bytes for hashing
}

func (m *Manifest) UnmarshalSpec(v any) error {
	return m.Spec.Decode(v)
}

func (m *Manifest) GetRoute(apiPrefix, bffxDir string) (method, path string) {
	if apiPrefix == "" {
		apiPrefix = "/api/v1"
	}
	// Ensure prefix has leading slash but no trailing slash for consistent joining
	if !strings.HasPrefix(apiPrefix, "/") {
		apiPrefix = "/" + apiPrefix
	}
	apiPrefix = strings.TrimSuffix(apiPrefix, "/")

	switch m.Kind {
	case "Action":
		var spec ActionSpec
		m.UnmarshalSpec(&spec)
		method = strings.ToUpper(spec.Route.Method)
		path = spec.Route.Path
		if method == "" {
			method = "POST"
		}
	case "Screen":
		var spec ScreenSpec
		m.UnmarshalSpec(&spec)
		method = strings.ToUpper(spec.Route.Method)
		path = spec.Route.Path
		if method == "" {
			method = "GET"
		}
	case "Builder":
		var spec BuilderSpec
		m.UnmarshalSpec(&spec)
		method = strings.ToUpper(spec.Route.Method)
		path = spec.Route.Path
		if method == "" {
			method = "GET"
		}
	case "Pipeline":
		var spec PipelineSpec
		m.UnmarshalSpec(&spec)
		method = strings.ToUpper(spec.Route.Method)
		path = spec.Route.Path
		if method == "" {
			method = "POST"
		}
	}

	if path == "" && bffxDir != "" && m.Path != "" {
		rel, err := filepath.Rel(bffxDir, m.Path)
		if err == nil {
			id := strings.TrimSuffix(rel, filepath.Ext(rel))
			// If in V2, we might have "auth/manifests/login".
			// We want to strip "manifests" if present.
			parts := strings.Split(id, string(filepath.Separator))
			var cleanParts []string
			for _, p := range parts {
				if p != "manifests" && p != "actions" && p != "screens" && p != "builders" {
					cleanParts = append(cleanParts, p)
				}
			}
			path = apiPrefix + "/" + strings.Join(cleanParts, "/")
		}
	}
	return method, path
}

type StoreConfig struct {
	Mode   string `yaml:"mode"`
	Url    string `yaml:"url,omitempty"`
	ApiKey string `yaml:"apiKey,omitempty"`
	Path   string `yaml:"path,omitempty"`
}

type FeatureConfig struct {
	Provider string            `yaml:"provider"`
	Config   map[string]string `yaml:"config,omitempty"`
	// RefreshTokens defaults to true when nil: LoadAll injects a built-in RefreshToken
	// resource when none is declared. Set to false to opt out (e.g. public API-only apps).
	RefreshTokens *bool `yaml:"refreshTokens,omitempty"`
	// Idempotency enables X-Idempotency-Key support for Actions/POSTs
	Idempotency bool `yaml:"idempotency,omitempty"`
}

type ProjectSpec struct {
	Runtime struct {
		Api struct {
			Language string `yaml:"language"`
			Port     int    `yaml:"port"`
		} `yaml:"api"`
		Worker struct {
			Language string `yaml:"language"`
			Enabled  bool   `yaml:"enabled"`
		} `yaml:"worker"`
		Redis struct {
			Enabled bool   `yaml:"enabled"`
			Url     string `yaml:"url,omitempty"`
		} `yaml:"redis"`
		Upstash struct {
			Enabled bool   `yaml:"enabled"`
			Url     string `yaml:"url,omitempty"`
			Token   string `yaml:"token,omitempty"`
		} `yaml:"upstash"`
		Streaming struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"streaming"`
		Wire struct {
			Enabled      bool   `yaml:"enabled"`
			GRPCPort     int    `yaml:"grpc_port,omitempty"`
			Package      string `yaml:"package,omitempty"`
			ProdJSON     bool   `yaml:"prod_json,omitempty"`
			ForceDevJSON bool   `yaml:"force_dev_json,omitempty"`
		} `yaml:"wire,omitempty"`
	} `yaml:"runtime"`
	Store          StoreConfig  `yaml:"store"`
	TelemetryStore *StoreConfig `yaml:"telemetryStore,omitempty"`

	Defaults struct {
		Auth      string   `yaml:"auth"`
		Providers []string `yaml:"providers,omitempty"`
		Files     bool     `yaml:"files"`
		Jobs      bool     `yaml:"jobs"`
		Builders  bool     `yaml:"builders"`
	} `yaml:"defaults"`
	Security struct {
		RateLimit struct {
			RequestsPerMinute int `yaml:"requestsPerMinute"`
			BurstSize         int `yaml:"burstSize"`
		} `yaml:"rateLimit"`
		AllowedOrigins []string `yaml:"allowedOrigins"`
	} `yaml:"security"`
	Jobs struct {
		RetentionDays int `yaml:"retentionDays"`
	} `yaml:"jobs"`
	App struct {
		Namespace        string            `yaml:"namespace"`
		ApiPrefix        string            `yaml:"apiPrefix"`
		AuthStrategy     string            `yaml:"authStrategy,omitempty"`
		Permissions      []AppPermission   `yaml:"permissions,omitempty"`
		UI               map[string]string `yaml:"ui,omitempty"`
		Menu             []AppMenuItem     `yaml:"menu,omitempty"`
		MinClientVersion string            `yaml:"minClientVersion,omitempty"`
		ForceSSL         bool              `yaml:"forceSSL,omitempty"`
	} `yaml:"app"`

	Admin struct {
		Enabled        bool     `yaml:"enabled"`
		AutoManifests  *bool    `yaml:"auto_manifests,omitempty" json:"auto_manifests,omitempty"`
		AllowedIPs     []string `yaml:"allowedIPs,omitempty"`
	} `yaml:"admin"`
	Monetization struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"monetization"`
	Packaging struct {
		// Mode controls build profile and vendoring: "full" (default) or "minimal".
		Mode string `yaml:"mode,omitempty"`
	} `yaml:"packaging,omitempty"`
	Batteries    BatteriesConfig `yaml:"batteries,omitempty"`
	FeatureFlags FeatureConfig   `yaml:"featureFlags,omitempty"`
	Lifecycle    LifecycleSpec   `yaml:"lifecycle,omitempty"`
	Layout       string          `yaml:"layout,omitempty"` // "legacy" or "v2"
}

type BatteriesConfig struct {
	Auth          string `yaml:"auth,omitempty"`          // builtin | clerk
	Store         string `yaml:"store,omitempty"`         // sqlite | postgres
	Cache         string `yaml:"cache,omitempty"`         // memory | redis | upstash
	Blob          string `yaml:"blob,omitempty"`          // local | minio | r2
	Analytics     string `yaml:"analytics,omitempty"`     // noop | posthog
	Observability string `yaml:"observability,omitempty"` // slog | axiom | axiom_sentry
	Flags         string `yaml:"flags,omitempty"`         // bffx | goff | launchdarkly | openfeature
	Vlm           string `yaml:"vlm,omitempty"`           // ollama | gemini | openrouter | noop
	Nutrition     string `yaml:"nutrition,omitempty"`     // openfoodfacts | noop
	I18n          string `yaml:"i18n,omitempty"`          // builtin | none
}

type LifecycleSpec struct {
	Semver struct {
		Enforce bool `yaml:"enforce"`
	} `yaml:"semver"`
	Platforms map[string]PlatformLifecycle `yaml:"platforms,omitempty"`
}

type PlatformLifecycle struct {
	MinVersion     string `yaml:"minVersion"`
	SuggestVersion string `yaml:"suggestVersion,omitempty"`
}

type ResourceField struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Bucket   string `yaml:"bucket,omitempty" json:"bucket,omitempty"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default  any    `yaml:"default,omitempty" json:"default,omitempty"`
	Target   string `yaml:"target,omitempty" json:"target,omitempty"`
	Unique   bool   `yaml:"unique,omitempty" json:"unique,omitempty"`
}

type HookConfig struct {
	Action string `yaml:"action"`
	Mode   string `yaml:"mode"` // "sync" or "async"
}

type ResourceSpec struct {
	Group       string `yaml:"group,omitempty"`
	DisplayName string `yaml:"displayName,omitempty"`
	// Tree accepts bool or legacy strings ("ownership", "none"). See TreeEnabled.
	Tree       any             `yaml:"tree,omitempty"`
	Stream     bool            `yaml:"stream,omitempty"`
	Telemetry  bool            `yaml:"telemetry,omitempty"`
	Fields     []ResourceField `yaml:"fields"`
	Transports []string        `yaml:"transports,omitempty"`
	Cache      *CacheConfig    `yaml:"cache,omitempty"`

	Routes struct {
		Crud           bool   `yaml:"crud"`
		CollectionPath string `yaml:"collectionPath,omitempty"` // e.g. "users" → /api/v1/users; empty → pluralized resource name
	} `yaml:"routes"`
	Policy struct {
		Read  any `yaml:"read"`
		Write any `yaml:"write"`
	} `yaml:"policy"`
	Hooks struct {
		BeforeCreate []HookConfig `yaml:"beforeCreate,omitempty"`
		AfterCreate  []HookConfig `yaml:"afterCreate,omitempty"`
		BeforeUpdate []HookConfig `yaml:"beforeUpdate,omitempty"`
		AfterUpdate  []HookConfig `yaml:"afterUpdate,omitempty"`
		BeforeDelete []HookConfig `yaml:"beforeDelete,omitempty"`
		AfterDelete  []HookConfig `yaml:"afterDelete,omitempty"`
	} `yaml:"hooks,omitempty"`
}

type BuilderSpec struct {
	DevOnly bool `yaml:"dev_only,omitempty"`
	Route   struct {
		Method string   `yaml:"method"`
		Path   string   `yaml:"path"`
		Auth   string   `yaml:"auth"`
		Roles  []string `yaml:"roles,omitempty"`
	} `yaml:"route"`
	Sources        []any          `yaml:"sources"` // Can be strings or structs
	Output         map[string]any `yaml:"output"`
	Cache          *CacheConfig   `yaml:"cache,omitempty"`
	PartialSuccess bool           `yaml:"partial_success,omitempty"`
	Transports     []string       `yaml:"transports,omitempty"`
}

type PolicySpec struct {
	Resource string `yaml:"resource"`
	Read     string `yaml:"read"`
	Write    string `yaml:"write"`
}

type ScreenSectionSpec struct {
	Key            string       `yaml:"key" json:"key"`
	DefaultVisible bool         `yaml:"default_visible" json:"default_visible"`
	Cache          *CacheConfig `yaml:"cache,omitempty" json:"-"`
	Route          *struct {
		Path string `yaml:"path"`
		Auth string `yaml:"auth"`
	} `yaml:"route,omitempty" json:"-"`
	UI     *SectionUI `yaml:"ui,omitempty" json:"ui,omitempty"`
	Layout string     `yaml:"layout,omitempty" json:"layout,omitempty"` // list, grid, carousel, etc.
}

type SectionUI struct {
	Type       string         `yaml:"type" json:"type"`
	Properties map[string]any `yaml:"properties,omitempty" json:"properties,omitempty"`
}

type CacheConfig struct {
	TTL            int      `yaml:"ttl" json:"-"`
	TagsFrom       []string `yaml:"tags_from,omitempty" json:"-"`
	Vary           []string `yaml:"vary,omitempty" json:"-"`
	AllowAnonymous bool     `yaml:"allow_anonymous,omitempty" json:"-"`
}

type ScreenSpec struct {
	DevOnly      bool   `yaml:"dev_only,omitempty"`
	Name         string `yaml:"name"`
	NavType      string `yaml:"nav_type"`                // "bottom" | "folded" | "auth_flow" | "onboarding"
	Icon         string `yaml:"icon,omitempty"`          // optional icon name for nav rendering
	RequiresAuth string `yaml:"requires_auth,omitempty"` // "" | "optional" | "required"
	Order        int    `yaml:"order,omitempty"`         // sort order in nav

	// Data Hydration (New)
	Route struct {
		Method string `yaml:"method"`
		Path   string `yaml:"path"`
		Auth   string `yaml:"auth"`
		// Rest, when set to false, disables the REST handler for this screen
		// (gRPC remains). Defaults to true. Honoured by registerScreenRoutes.
		Rest *bool `yaml:"rest,omitempty"`
	} `yaml:"route,omitempty"`
	Sources        []any               `yaml:"sources,omitempty"`
	Output         map[string]any      `yaml:"output,omitempty"`
	Sections       []ScreenSectionSpec `yaml:"sections,omitempty"`
	Cache          *CacheConfig        `yaml:"cache,omitempty"`
	Layout         string              `yaml:"layout,omitempty"` // default layout for the screen
	PartialSuccess bool                `yaml:"partial_success,omitempty"`

	// Stream, when true, makes the generated proto service expose a server-
	// streaming RPC `Watch` (proto: `stream <Screen>Response`). The runtime
	// can implement v1 as a polling shim that emits a snapshot every
	// `stream_interval_ms` (default 5000) when the screen hash changes; v2
	// uses the eventbus. Honoured by writeScreenServices.
	Stream           bool `yaml:"stream,omitempty"`
	StreamIntervalMs int  `yaml:"stream_interval_ms,omitempty"`

	// Transports gates which transports get codegen for this Screen
	// (e.g. ["rest", "grpc"]). Empty defaults to both.
	Transports []string `yaml:"transports,omitempty"`
}

type SkillSpec struct {
	Queue     string         `yaml:"queue"`
	Input     map[string]any `yaml:"input"`
	Output    map[string]any `yaml:"output"`
	OnSuccess map[string]any `yaml:"onSuccess"`
}

type CronJobSpec struct {
	Schedule string         `yaml:"schedule"` // e.g. "0 0 * * *"
	Action   string         `yaml:"action"`   // Action name to trigger
	Input    map[string]any `yaml:"input,omitempty"`
}

type ActionSpec struct {
	DevOnly bool `yaml:"dev_only,omitempty"`
	Route   struct {
		Method string   `yaml:"method"`
		Path   string   `yaml:"path"`
		Auth   string   `yaml:"auth"`
		Roles  []string `yaml:"roles,omitempty"`
		// IdempotencyMode controls endpoint-level replay guarantees:
		// ""/"cache" uses normal idempotency store behavior; "durable" requires
		// a durable backend and an X-Idempotency-Key header.
		IdempotencyMode string       `yaml:"idempotency_mode,omitempty"`
		CacheTTL        int          `yaml:"cache_ttl,omitempty"`
		Cache           *CacheConfig `yaml:"cache,omitempty"`
		// InvalidateCache: for GET handlers that write to the DB, bust action GET caches on success.
		// POST/PUT/PATCH/DELETE routes invalidate automatically via framework middleware.
		InvalidateCache bool `yaml:"invalidate_cache,omitempty"`
		// Rest, when explicitly set to false, disables the REST handler for
		// this Action (gRPC remains). Defaults to true. The global env var
		// BFFX_ACTIONS_REST_ENABLED=false disables REST for all Actions at
		// once (handy for production where mobile clients are gRPC-only).
		Rest *bool `yaml:"rest,omitempty"`
	} `yaml:"route"`
	Transports []string `yaml:"transports,omitempty"`
}

type StreamSpec struct {
	DevOnly bool `yaml:"dev_only,omitempty"`
	Route   struct {
		Path string `yaml:"path"`
		Auth string `yaml:"auth"`
	} `yaml:"route"`
	Channels []string `yaml:"channels"`
}

type FunctionSpec struct {
	Input  map[string]any `yaml:"input"`
	Output map[string]any `yaml:"output"`
}

type ServiceSpec struct {
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
	Timeout  string `yaml:"timeout"`
}

type FeatureFlagSpec struct {
	Key          string             `yaml:"key"`
	Enabled      bool               `yaml:"enabled"`
	Variations   []interface{}      `yaml:"variations"` // e.g., [true, false] or ["A", "B"]
	Rules        []TargetingRule    `yaml:"rules,omitempty"`
	Fallthrough  VariationSelection `yaml:"fallthrough"`
	OffVariation int                `yaml:"offVariation"` // Index of variation when Enabled is false
}

type TargetingRule struct {
	Clauses   []Clause `yaml:"clauses"`
	Variation *int     `yaml:"variation,omitempty"`
	Rollout   *Rollout `yaml:"rollout,omitempty"`
}

type Clause struct {
	Attribute string        `yaml:"attribute"`
	Operator  string        `yaml:"operator"` // "in", "notIn", "matches", "startsWith", "endsWith", "contains", "greaterThan", "lessThan"
	Values    []interface{} `yaml:"values"`
	Negate    bool          `yaml:"negate"`
}

type Rollout struct {
	Variations []WeightedVariation `yaml:"variations"`
	BucketBy   string              `yaml:"bucketBy,omitempty"` // defaults to "key" or "id"
}

type WeightedVariation struct {
	Variation int `yaml:"variation"`
	Weight    int `yaml:"weight"` // 0-100000 (representing 0.00% to 100.00%)
}

type VariationSelection struct {
	Variation *int     `yaml:"variation,omitempty"`
	Rollout   *Rollout `yaml:"rollout,omitempty"`
}

type SubscriptionSpec struct {
	Identifiers struct {
		Ios     string `yaml:"ios"`
		Android string `yaml:"android"`
	} `yaml:"identifiers"`
	Entitlements []string `yaml:"entitlements"`
}

type ProductSpec struct {
	Type        string `yaml:"type"` // "consumable", "non_consumable"
	Identifiers struct {
		Ios     string `yaml:"ios"`
		Android string `yaml:"android"`
	} `yaml:"identifiers"`
	Grants map[string]any `yaml:"grants"`
}

type AdProviderSpec struct {
	Type   string `yaml:"type"` // "admob", "applovin", "ironsource"
	ApiKey string `yaml:"apiKey,omitempty"`
}

type AdUnitSpec struct {
	Provider    string `yaml:"provider"`
	Type        string `yaml:"type"` // "banner", "interstitial", "rewarded"
	Identifiers struct {
		Ios     string `yaml:"ios"`
		Android string `yaml:"android"`
	} `yaml:"identifiers"`
}

type AppPermission struct {
	Type           string            `yaml:"type"`                      // "location", "notifications", "camera", "contacts", "health"
	ID             string            `yaml:"id,omitempty"`              // Optional unique ID
	Title          map[string]string `yaml:"title,omitempty"`           // Localized title
	Description    map[string]string `yaml:"description,omitempty"`     // Localized description
	Required       bool              `yaml:"required"`                  // If false, it's an optional perk
	Level          string            `yaml:"level,omitempty"`           // "always", "whenInUse", "coarse", "fine"
	Timing         string            `yaml:"timing,omitempty"`          // "immediate", "on-demand", "pre-feature"
	TriggerFeature string            `yaml:"trigger_feature,omitempty"` // Flag that triggers the request (e.g. "enable-health-sync")
	Rationales     map[string]string `yaml:"rationales,omitempty"`      // Localized rationales (key: locale, val: string)
	Justification  string            `yaml:"justification"`             // Default fallback justification
}

type AppMenuItem struct {
	Label string `yaml:"label"`
	Icon  string `yaml:"icon"`
	Path  string `yaml:"path"`
}

type TemplateSpec struct {
	Channels map[string]ChannelContent `yaml:"channels"`
}

type ChannelContent struct {
	Title   string `yaml:"title,omitempty"`
	Subject string `yaml:"subject,omitempty"`
	Body    string `yaml:"body"`
}

type BlueprintSpec struct {
	Resource string         `yaml:"resource"`
	Count    int            `yaml:"count,omitempty"`
	Fields   map[string]any `yaml:"fields,omitempty"`
	Defaults map[string]any `yaml:"defaults,omitempty"`
}

type SeedSpec struct {
	// Table is the BFFX resource name (e.g. FastingProtocol), not SQL table name.
	Table string `yaml:"table"`
	// Order controls apply sequence when multiple Seed manifests exist (lower first).
	Order int `yaml:"order,omitempty"`
	// Upsert updates existing rows (matched by code, id, email, or name) instead of skipping.
	Upsert bool `yaml:"upsert,omitempty"`
	Rows   []map[string]any `yaml:"rows"`
}

type PipelineRoute struct {
	Method string `yaml:"method" json:"method"`
	Path   string `yaml:"path" json:"path"`
	Auth   string `yaml:"auth" json:"auth"`
}

type PipelineCatalog struct {
	Adapter string `yaml:"adapter" json:"adapter"`
}

type PipelineSettings struct {
	CompressionMaxWidth int    `yaml:"compression_max_width,omitempty" json:"compression_max_width,omitempty"`
	CacheBypass         bool   `yaml:"cache_bypass,omitempty" json:"cache_bypass,omitempty"`
	MaxSlidingHistory   int    `yaml:"max_sliding_history,omitempty" json:"max_sliding_history,omitempty"`
	SystemPrompt        string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
}

type PipelineHooks struct {
	BeforePipeline []HookConfig `yaml:"beforePipeline,omitempty" json:"beforePipeline,omitempty"`
	AfterPipeline  []HookConfig `yaml:"afterPipeline,omitempty" json:"afterPipeline,omitempty"`
}

type PipelineSpec struct {
	DevOnly      bool             `yaml:"dev_only,omitempty" json:"dev_only,omitempty"`
	Type         string           `yaml:"type" json:"type"`                               // "ingestion" or "chatbot"
	Execution    string           `yaml:"execution,omitempty" json:"execution,omitempty"` // "sync" or "async"
	Route        PipelineRoute    `yaml:"route" json:"route"`
	ModelRouting []string         `yaml:"model_routing" json:"model_routing"`
	Catalog      *PipelineCatalog `yaml:"catalog,omitempty" json:"catalog,omitempty"`
	Settings     PipelineSettings `yaml:"settings,omitempty" json:"settings,omitempty"`
	QuotaProfile string           `yaml:"quota_profile,omitempty" json:"quota_profile,omitempty"`
	Hooks        PipelineHooks    `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Transports   []string         `yaml:"transports,omitempty" json:"transports,omitempty"`
}

func (m *Manifest) Validate(reg *Registry) error {
	if m.Kind == "Resource" {
		var spec ResourceSpec
		m.UnmarshalSpec(&spec)
		for _, f := range spec.Fields {
			// Check type
			validTypes := map[string]bool{"string": true, "int": true, "float": true, "bool": true, "date": true, "json": true}
			if !validTypes[f.Type] && f.Target == "" {
				return fmt.Errorf("invalid type %q for field %q", f.Type, f.Name)
			}
			// Check relation
			if f.Target != "" {
				if _, ok := reg.GetResource(f.Target); !ok {
					return fmt.Errorf("relation to unknown resource %s", f.Target)
				}
			}
		}
	}

	if m.Kind == "Action" {
		var spec ActionSpec
		m.UnmarshalSpec(&spec)
		// Relaxed: implicit paths are supported via GetRoute
	}

	if m.Kind == "Screen" {
		var spec ScreenSpec
		m.UnmarshalSpec(&spec)
		// Relaxed: implicit paths are supported via GetRoute
	}

	if m.Kind == "Builder" {
		var spec BuilderSpec
		m.UnmarshalSpec(&spec)
		// Relaxed: implicit paths are supported via GetRoute
	}

	if m.Kind == "CronJob" {
		var spec CronJobSpec
		m.UnmarshalSpec(&spec)
		if spec.Schedule == "" {
			return fmt.Errorf("missing 'schedule' field")
		}
		if spec.Action == "" {
			return fmt.Errorf("missing 'action' field")
		}
	}

	if m.Kind == "Pipeline" {
		var spec PipelineSpec
		m.UnmarshalSpec(&spec)
		if spec.Type != "ingestion" && spec.Type != "chatbot" {
			return fmt.Errorf("invalid pipeline type %q (must be 'ingestion' or 'chatbot')", spec.Type)
		}
		if len(spec.ModelRouting) == 0 {
			return fmt.Errorf("pipeline must define at least one entry in 'model_routing'")
		}
	}
	if m.Kind == "LiveOpsEvent" {
		var spec LiveOpsEventSpec
		m.UnmarshalSpec(&spec)
		if spec.Schedule.StartTime == "" {
			return fmt.Errorf("missing 'schedule.start_time' field")
		}
		if spec.Schedule.EndTime == "" {
			return fmt.Errorf("missing 'schedule.end_time' field")
		}
	}
	if m.Kind == "Leaderboard" {
		var spec LeaderboardSpec
		m.UnmarshalSpec(&spec)
		if spec.SortOrder != "asc" && spec.SortOrder != "desc" {
			return fmt.Errorf("invalid 'sort_order' %q (must be 'asc' or 'desc')", spec.SortOrder)
		}
		if spec.ScoreType != "int" && spec.ScoreType != "float" && spec.ScoreType != "millisecond" {
			return fmt.Errorf("invalid 'score_type' %q (must be 'int', 'float', or 'millisecond')", spec.ScoreType)
		}
		if spec.AggregateStrategy != "max" && spec.AggregateStrategy != "sum" && spec.AggregateStrategy != "latest" {
			return fmt.Errorf("invalid 'aggregate_strategy' %q (must be 'max', 'sum', or 'latest')", spec.AggregateStrategy)
		}
	}
	if m.Kind == "AdminResource" {
		var spec AdminResourceSpec
		m.UnmarshalSpec(&spec)
		if spec.Resource == "" {
			return fmt.Errorf("missing 'resource' field")
		}
		if _, ok := reg.GetResource(spec.Resource); !ok {
			return fmt.Errorf("admin resource links to unknown resource %s", spec.Resource)
		}
	}
	if m.Kind == "AdminScreen" {
		var spec AdminScreenSpec
		m.UnmarshalSpec(&spec)
		if spec.Screen == "" {
			return fmt.Errorf("missing 'screen' field")
		}
		found := false
		for _, s := range reg.Screens {
			if s.Metadata.Name == spec.Screen {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("admin screen links to unknown screen %s", spec.Screen)
		}
	}
	if m.Kind == "AdminAction" {
		var spec AdminActionSpec
		m.UnmarshalSpec(&spec)
		if spec.Action == "" {
			return fmt.Errorf("missing 'action' field")
		}
		found := false
		for _, a := range reg.Actions {
			if a.Metadata.Name == spec.Action {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("admin action links to unknown action %s", spec.Action)
		}
	}
	return nil
}

func (r *Registry) Validate() []string {
	var errors []string

	collections := [][]*Manifest{
		r.Resources, r.Actions, r.Screens, r.Builders, r.Streams,
		r.FeatureFlags, r.Products, r.AdProviders, r.AdUnits, r.CronJobs,
		r.Pipelines, r.LiveOpsEvents, r.Leaderboards,
		r.AdminSites, r.AdminResources, r.AdminDashboards, r.AdminPages,
		r.AdminScreens, r.AdminActions,
	}

	for _, col := range collections {
		for _, m := range col {
			if err := m.Validate(r); err != nil {
				errors = append(errors, fmt.Sprintf("[%s:%s] %v", m.Kind, m.Metadata.Name, err))
			}
		}
	}
	return errors
}

// LiveOpsEventSpec defines the manifest spec for LiveOps scheduled events.
type LiveOpsEventSpec struct {
	Title                string         `yaml:"title"`
	Description          string         `yaml:"description"`
	Priority             int            `yaml:"priority"`
	AudienceSegment      string         `yaml:"audience_segment"`
	Schedule             EventSchedule  `yaml:"schedule"`
	ConfigurationPayload map[string]any `yaml:"configuration_payload"`
}

// EventSchedule defines the start/end window for LiveOps events.
type EventSchedule struct {
	StartTime       string `yaml:"start_time"`
	EndTime         string `yaml:"end_time"`
	CooldownSeconds int    `yaml:"cooldown_seconds,omitempty"`
}

// LeaderboardSpec defines the manifest spec for authoritative leaderboards.
type LeaderboardSpec struct {
	Title             string           `yaml:"title"`
	SortOrder         string           `yaml:"sort_order"`         // "asc", "desc"
	ScoreType         string           `yaml:"score_type"`          // "int", "float", "millisecond"
	AggregateStrategy string           `yaml:"aggregate_strategy"` // "max", "sum", "latest"
	ResetSchedule     string           `yaml:"reset_schedule"`     // Cron expression
	RetentionSeasons  int              `yaml:"retention_seasons"`
	Cache             LeaderboardCache `yaml:"cache"`
	Rules             LeaderboardRules `yaml:"rules"`
}

type LeaderboardCache struct {
	TTL int `yaml:"ttl"`
}

type LeaderboardRules struct {
	MinScore          float64 `yaml:"min_score"`
	MaxScore          float64 `yaml:"max_score"`
	AntiSpamWindowSec int     `yaml:"anti_spam_window_sec"`
}

type AdminSiteSpec struct {
	Title          string              `yaml:"title" json:"title"`
	LogoURL        string              `yaml:"logo_url,omitempty" json:"logo_url,omitempty"`
	Theme          string              `yaml:"theme,omitempty" json:"theme,omitempty"`
	DefaultPerPage int                 `yaml:"default_per_page,omitempty" json:"default_per_page,omitempty"`
	Menu           []AdminSiteMenuItem `yaml:"menu,omitempty" json:"menu,omitempty"`
	Session          AdminSiteSession         `yaml:"session,omitempty" json:"session,omitempty"`
	SessionResolved  *AdminSiteSessionResolved `yaml:"-" json:"session_resolved,omitempty"`
	Features         AdminSiteFeatures          `yaml:"features,omitempty" json:"features,omitempty"`
	Export         AdminSiteExport     `yaml:"export,omitempty" json:"export,omitempty"`
}

// AdminSiteSession configures admin panel cookie lifetime and idle behavior.
type AdminSiteSession struct {
	MaxAge       string `yaml:"max_age,omitempty" json:"max_age,omitempty"`
	IdleTimeout  string `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	DevUnlimited *bool  `yaml:"dev_unlimited,omitempty" json:"dev_unlimited,omitempty"`
}

// AdminSiteSessionResolved is exposed to the admin SPA via admin-graph.json.
type AdminSiteSessionResolved struct {
	MaxAgeSeconds      int  `json:"max_age_seconds"`
	IdleTimeoutSeconds int  `json:"idle_timeout_seconds"`
	DevUnlimited       bool `json:"dev_unlimited"`
}

type AdminSiteMenuItem struct {
	Label    string              `yaml:"label" json:"label"`
	Page     string              `yaml:"page,omitempty" json:"page,omitempty"`
	Priority int                 `yaml:"priority,omitempty" json:"priority,omitempty"`
	Children []AdminSiteMenuItem `yaml:"children,omitempty" json:"children,omitempty"`
}

type AdminSiteFeatures struct {
	AppFeatures  bool `yaml:"app_features,omitempty" json:"app_features,omitempty"`
	FeatureFlags bool `yaml:"feature_flags,omitempty" json:"feature_flags,omitempty"`
	Jobs         bool `yaml:"jobs,omitempty" json:"jobs,omitempty"`
	Liveops      bool `yaml:"liveops,omitempty" json:"liveops,omitempty"`
	ApiDocs      bool `yaml:"api_docs,omitempty" json:"api_docs,omitempty"`
}

type AdminSiteExport struct {
	CSVEnabled bool `yaml:"csv_enabled,omitempty" json:"csv_enabled,omitempty"`
}

type AdminResourceSpec struct {
	Resource          string                `yaml:"resource" json:"resource"`
	Enabled           *bool                 `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Fields            []ResourceField       `yaml:"-" json:"fields,omitempty"`
	Menu              AdminResourceMenu     `yaml:"menu,omitempty" json:"menu,omitempty"`
	Index             AdminResourceIndex    `yaml:"index,omitempty" json:"index,omitempty"`
	Show              AdminResourceShow     `yaml:"show,omitempty" json:"show,omitempty"`
	Form              AdminResourceForm     `yaml:"form,omitempty" json:"form,omitempty"`
	Associations      []AdminAssociation    `yaml:"associations,omitempty" json:"associations,omitempty"`
	BelongsTo         []AdminBelongsTo      `yaml:"belongs_to,omitempty" json:"belongs_to,omitempty"`
	BatchActions      []AdminResourceAction `yaml:"batch_actions,omitempty" json:"batch_actions,omitempty"`
	MemberActions     []AdminResourceAction `yaml:"member_actions,omitempty" json:"member_actions,omitempty"`
	CollectionActions []AdminResourceAction `yaml:"collection_actions,omitempty" json:"collection_actions,omitempty"`
	Policy            AdminResourcePolicy   `yaml:"policy,omitempty" json:"policy,omitempty"`
}

// AdminAssociation declares a has-many child collection on a parent resource detail view.
type AdminAssociation struct {
	Name       string `yaml:"name" json:"name"`
	Resource   string `yaml:"resource" json:"resource"`
	ForeignKey string `yaml:"foreign_key" json:"foreign_key"`
	PerPage    int    `yaml:"per_page,omitempty" json:"per_page,omitempty"`
	Sort       string `yaml:"sort,omitempty" json:"sort,omitempty"`
}

// AdminBelongsTo declares a parent link for a foreign-key field in index/detail views.
type AdminBelongsTo struct {
	Field      string `yaml:"field" json:"field"`
	Resource   string `yaml:"resource" json:"resource"`
	LabelField string `yaml:"label_field,omitempty" json:"label_field,omitempty"`
}

type AdminResourceMenu struct {
	Label    string `yaml:"label" json:"label"`
	Parent   string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Priority int    `yaml:"priority,omitempty" json:"priority,omitempty"`
	Pin      bool   `yaml:"pin,omitempty" json:"pin,omitempty"`
}

type AdminResourceIndex struct {
	PerPage     int                   `yaml:"per_page,omitempty" json:"per_page,omitempty"`
	DefaultSort string                `yaml:"default_sort,omitempty" json:"default_sort,omitempty"`
	Selectable  bool                  `yaml:"selectable,omitempty" json:"selectable,omitempty"`
	Columns     []string              `yaml:"columns,omitempty" json:"columns,omitempty"`
	Scopes      []AdminResourceScope  `yaml:"scopes,omitempty" json:"scopes,omitempty"`
	Filters     []AdminResourceFilter `yaml:"filters,omitempty" json:"filters,omitempty"`
}

type AdminResourceScope struct {
	Name    string         `yaml:"name" json:"name"`
	Default bool           `yaml:"default,omitempty" json:"default,omitempty"`
	Where   map[string]any `yaml:"where,omitempty" json:"where,omitempty"`
}

type AdminResourceFilter struct {
	Field   string   `yaml:"field" json:"field"`
	As      string   `yaml:"as" json:"as"`
	Label   string   `yaml:"label,omitempty" json:"label,omitempty"`
	Options []string `yaml:"options,omitempty" json:"options,omitempty"`
}

type AdminResourceShow struct {
	Attributes []string `yaml:"attributes,omitempty" json:"attributes,omitempty"`
	Exclude    []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

type AdminResourceForm struct {
	Exclude []string             `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Inputs  []AdminResourceInput `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

type AdminResourceInput struct {
	Field   string   `yaml:"field" json:"field"`
	As      string   `yaml:"as" json:"as"`
	Options []string `yaml:"options,omitempty" json:"options,omitempty"`
}

type AdminResourceAction struct {
	Name    string   `yaml:"name" json:"name"`
	Label   string   `yaml:"label" json:"label"`
	Confirm string   `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	Hook    string   `yaml:"hook,omitempty" json:"hook,omitempty"`
	Builtin string   `yaml:"builtin,omitempty" json:"builtin,omitempty"`
	Only    []string `yaml:"only,omitempty" json:"only,omitempty"`
}

type AdminResourcePolicy struct {
	Read  string `yaml:"read,omitempty" json:"read,omitempty"`
	Write string `yaml:"write,omitempty" json:"write,omitempty"`
}

type AdminDashboardSpec struct {
	Widgets []AdminDashboardWidget `yaml:"widgets,omitempty" json:"widgets,omitempty"`
}

type AdminDashboardWidget struct {
	Type    string         `yaml:"type" json:"type"`
	Title   string         `yaml:"title" json:"title"`
	Query   map[string]any `yaml:"query,omitempty" json:"query,omitempty"`
	Source  string         `yaml:"source,omitempty" json:"source,omitempty"`
	Metric  string         `yaml:"metric,omitempty" json:"metric,omitempty"`
	Range   string         `yaml:"range,omitempty" json:"range,omitempty"`
	Limit   int            `yaml:"limit,omitempty" json:"limit,omitempty"`
	Columns []string       `yaml:"columns,omitempty" json:"columns,omitempty"`
}

type AdminPageSpec struct {
	Menu    AdminResourceMenu      `yaml:"menu,omitempty" json:"menu,omitempty"`
	Layout  string                 `yaml:"layout,omitempty" json:"layout,omitempty"`
	Widgets []AdminDashboardWidget `yaml:"widgets,omitempty" json:"widgets,omitempty"`
}

type AdminGraph struct {
	Site              AdminSiteSpec        `json:"site"`
	Dashboard         *AdminDashboardSpec  `json:"dashboard,omitempty"`
	Pages             []AdminPageSpec      `json:"pages"`
	Resources         []AdminResourceSpec  `json:"resources"`
	ReservedResources []string             `json:"reserved_resources"`
}

var ReservedAdminResources = []string{
	"feature_flag", "feature_flags", "featureflag", "featureflags",
	"audit_log", "audit_logs", "auditlog", "auditlogs",
	"incident", "incidents",
	"admin_user", "admin_users", "adminuser", "adminusers",
}

func IsReservedAdminResource(name string) bool {
	n := strings.ToLower(name)
	if strings.HasPrefix(n, "bffx_") {
		return true
	}
	for _, r := range ReservedAdminResources {
		if r == n {
			return true
		}
	}
	return false
}

type AdminScreenSpec struct {
	Screen          string `yaml:"screen" json:"screen"`
	KillSwitchPanel bool   `yaml:"kill_switch_panel,omitempty" json:"kill_switch_panel,omitempty"`
}

type AdminActionSpec struct {
	Action      string `yaml:"action" json:"action"`
	ObserveCard bool   `yaml:"observe_card,omitempty" json:"observe_card,omitempty"`
}


