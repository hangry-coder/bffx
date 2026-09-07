package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Registry represents the compiled project graph, containing all validated
// manifests discovered in the project's bffx/ directory.
type Registry struct {
	Project       *Manifest
	Resources     []*Manifest
	Builders      []*Manifest
	Policies      []*Manifest
	Skills        []*Manifest
	Actions       []*Manifest
	Functions     []*Manifest
	Services      []*Manifest
	Streams       []*Manifest
	FeatureFlags  []*Manifest
	Subscriptions []*Manifest
	Products      []*Manifest
	AdProviders   []*Manifest
	AdUnits       []*Manifest
	Templates     []*Manifest
	Blueprints    []*Manifest
	Seeds         []*Manifest
	Screens       []*Manifest
	CronJobs      []*Manifest
	Pipelines     []*Manifest
	LiveOpsEvents   []*Manifest
	Leaderboards    []*Manifest
	AdminSites      []*Manifest
	AdminResources  []*Manifest
	AdminDashboards []*Manifest
	AdminPages      []*Manifest
	AdminScreens    []*Manifest
	AdminActions    []*Manifest
	Warnings        []string
	Root            string // Project root directory
	ApiPrefix       string // Project apiPrefix (e.g. /api/v1)
}

// ResourceHash calculates a stable SHA256 checksum of all resources in the
// registry. This is used to detect when the orchestrator needs to be
// re-synchronized with the database schema.
func (r *Registry) ResourceHash() string {
	var parts []string
	for _, m := range r.Resources {
		// Include resource name and its raw content to detect any field changes
		parts = append(parts, m.Metadata.Name+":"+string(m.Raw))
	}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}

// GetResource retrieves a resource manifest by its metadata name.
func (r *Registry) GetResource(name string) (*Manifest, bool) {
	for _, m := range r.Resources {
		if m.Metadata.Name == name {
			return m, true
		}
	}
	return nil, false
}

var (
	cacheByRoot = make(map[string]*Registry)
	cacheMu     sync.RWMutex
)

func loadAllCacheKey(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return abs
}

// InvalidateLoadAllCache drops the cached registry for root after project manifest edits.
func InvalidateLoadAllCache(root string) {
	key := loadAllCacheKey(root)
	cacheMu.Lock()
	delete(cacheByRoot, key)
	cacheMu.Unlock()
}

func LoadAll(root string) (*Registry, error) {
	key := loadAllCacheKey(root)

	cacheMu.RLock()
	if reg, ok := cacheByRoot[key]; ok {
		cacheMu.RUnlock()
		return reg, nil
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if reg, ok := cacheByRoot[key]; ok {
		return reg, nil
	}

	if _, err := os.Stat(root); err != nil {
		return nil, err
	}

	reg := &Registry{Root: root}
	seen := make(map[string]string) // "Kind:Name" -> FilePath

	legacyDir := filepath.Join(root, "bffx")
	featuresDir := filepath.Join(root, "internal", "features")

	_, errLegacy := os.Stat(legacyDir)
	_, errFeatures := os.Stat(featuresDir)

	if os.IsNotExist(errLegacy) && os.IsNotExist(errFeatures) {
		return nil, fmt.Errorf("no manifest directory found in %q (looked for 'bffx/' or 'internal/features/')", root)
	}

	// 1. Walk legacy bffx/ directory
	if err := reg.walkManifests(root, legacyDir, seen); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// 2. Walk v2 internal/features/ directory
	if err := reg.walkManifests(root, featuresDir, seen); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Inject internal framework resources
	internalResources := []struct {
		Name string
		Yaml string
	}{
		{"User", `
fields:
  - name: email
    type: string
    required: true
  - name: name
    type: string
  - name: password
    type: string
  - name: role
    type: string
  - name: status
    type: string
  - name: device_id
    type: string
`},
		{"Subscription", `
fields:
  - name: user_id
    type: string
    required: true
  - name: plan_id
    type: string
    required: true
  - name: status
    type: string
  - name: expires_at
    type: string
`},
		{"Entitlement", `
fields:
  - name: user_id
    type: string
    required: true
  - name: slug
    type: string
    required: true
  - name: expires_at
    type: string
`},
		{"TelemetryEvent", `
fields:
  - name: user_id
    type: string
  - name: event_type
    type: string
  - name: screen_name
    type: string
  - name: action_name
    type: string
  - name: properties
    type: string
`},
		{"Device", `
fields:
  - name: user_id
    type: string
  - name: fingerprint
    type: string
  - name: push_token
    type: string
  - name: platform
    type: string
  - name: model
    type: string
  - name: os_version
    type: string
  - name: app_version
    type: string
  - name: is_trusted
    type: bool
  - name: last_seen_at
    type: string
`},
		{"AdminUser", `
fields:
  - name: email
    type: string
    required: true
  - name: password
    type: string
  - name: role
    type: string
routes:
  crud: false
`},
		{"Role", `

fields:
  - name: name
    type: string
  - name: slug
    type: string
`},
		{"Permission", `
fields:
  - name: name
    type: string
  - name: slug
    type: string
`},
		{"UserRole", `
fields:
  - name: user_id
    type: string
  - name: role_id
    type: string
`},
		{"AppConfig", `
fields:
  - name: config_key
    type: string
  - name: config_value
    type: string
  - name: description
    type: string
group: system
routes:
  crud: false
`},
		{"Incident", `
fields:
  - name: title
    type: string
    required: true
  - name: severity
    type: string
  - name: stack_trace
    type: string
  - name: resolved
    type: bool
  - name: resolved_by
    type: string
group: system
routes:
  crud: false
`},
		{"MetricSample", `
fields:
  - name: timestamp
    type: string
  - name: goroutines
    type: int
  - name: mem_alloc
    type: int
  - name: mem_sys
    type: int
  - name: cpu_percent
    type: float
  - name: http_total
    type: int
  - name: http_errors
    type: int
  - name: http_p95_ms
    type: int
group: system
routes:
  crud: false
`},
	}

	for _, ir := range internalResources {
		var existingManifest *Manifest
		for _, r := range reg.Resources {
			// Case-insensitive and underscore-agnostic: project manifests often use 
			// lowercase snake_case (e.g. telemetry_event) while internal defaults 
			// use PascalCase (TelemetryEvent).
			normalizedExisting := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(r.Metadata.Name)), "_", "")
			normalizedInternal := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(ir.Name)), "_", "")
			if normalizedExisting == normalizedInternal {
				existingManifest = r
				break
			}
		}

		defaultFields := getInternalDefaultFields(ir.Name)

		if existingManifest == nil {
			// Create a proper spec node
			mapping := &yaml.Node{Kind: yaml.MappingNode}
			fieldsKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "fields"}
			fieldsVal := &yaml.Node{Kind: yaml.SequenceNode}
			fieldsVal.Content = defaultFields

			groupKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "group"}
			groupVal := &yaml.Node{Kind: yaml.ScalarNode, Value: "system"}

			routesKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "routes"}
			routesVal := &yaml.Node{Kind: yaml.MappingNode}
			crudKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "crud"}

			crudValue := "true"
			if ir.Name == "AdminUser" || ir.Name == "AppConfig" || ir.Name == "Incident" || ir.Name == "MetricSample" {
				crudValue = "false"
			}
			crudVal := &yaml.Node{Kind: yaml.ScalarNode, Value: crudValue}

			routesVal.Content = append(routesVal.Content, crudKey, crudVal)

			policyKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "policy"}
			policyVal := &yaml.Node{Kind: yaml.MappingNode}

			readKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "read"}
			writeValue := "public"
			readValue := "public"
			if ir.Name == "User" || ir.Name == "Device" || ir.Name == "Subscription" || ir.Name == "Entitlement" {
				writeValue = "owner"
				readValue = "owner"
			} else if ir.Name == "AdminUser" || ir.Name == "Role" || ir.Name == "Permission" || ir.Name == "UserRole" || ir.Name == "Incident" || ir.Name == "MetricSample" {
				writeValue = "admin"
				readValue = "admin"
			}
			readVal := &yaml.Node{Kind: yaml.ScalarNode, Value: readValue}

			writeKey := &yaml.Node{Kind: yaml.ScalarNode, Value: "write"}
			writeVal := &yaml.Node{Kind: yaml.ScalarNode, Value: writeValue}

			policyVal.Content = append(policyVal.Content, readKey, readVal, writeKey, writeVal)

			mapping.Content = append(mapping.Content, fieldsKey, fieldsVal, groupKey, groupVal, routesKey, routesVal, policyKey, policyVal)

			reg.Resources = append(reg.Resources, &Manifest{
				Kind:     "Resource",
				Metadata: Metadata{Name: ir.Name},
				Spec:     *mapping,
			})
		} else {
			// Merge missing fields into existing resource manifest
			mergeInternalFields(existingManifest, defaultFields)
		}
	}
	
	injectBuiltinRefreshTokenResource(reg)

	// Sort for determinism
	sort.Slice(reg.Resources, func(i, j int) bool {
		return reg.Resources[i].Metadata.Name < reg.Resources[j].Metadata.Name
	})
	sort.Slice(reg.Builders, func(i, j int) bool {
		return reg.Builders[i].Metadata.Name < reg.Builders[j].Metadata.Name
	})
	sort.Slice(reg.Policies, func(i, j int) bool {
		return reg.Policies[i].Metadata.Name < reg.Policies[j].Metadata.Name
	})
	sort.Slice(reg.Skills, func(i, j int) bool {
		return reg.Skills[i].Metadata.Name < reg.Skills[j].Metadata.Name
	})
	sort.Slice(reg.Actions, func(i, j int) bool {
		return reg.Actions[i].Metadata.Name < reg.Actions[j].Metadata.Name
	})
	sort.Slice(reg.Functions, func(i, j int) bool {
		return reg.Functions[i].Metadata.Name < reg.Functions[j].Metadata.Name
	})
	sort.Slice(reg.Services, func(i, j int) bool {
		return reg.Services[i].Metadata.Name < reg.Services[j].Metadata.Name
	})
	sort.Slice(reg.Streams, func(i, j int) bool {
		return reg.Streams[i].Metadata.Name < reg.Streams[j].Metadata.Name
	})
	sort.Slice(reg.FeatureFlags, func(i, j int) bool {
		return reg.FeatureFlags[i].Metadata.Name < reg.FeatureFlags[j].Metadata.Name
	})
	sort.Slice(reg.Subscriptions, func(i, j int) bool {
		return reg.Subscriptions[i].Metadata.Name < reg.Subscriptions[j].Metadata.Name
	})
	sort.Slice(reg.Products, func(i, j int) bool {
		return reg.Products[i].Metadata.Name < reg.Products[j].Metadata.Name
	})
	sort.Slice(reg.AdProviders, func(i, j int) bool {
		return reg.AdProviders[i].Metadata.Name < reg.AdProviders[j].Metadata.Name
	})
	sort.Slice(reg.AdUnits, func(i, j int) bool {
		return reg.AdUnits[i].Metadata.Name < reg.AdUnits[j].Metadata.Name
	})
	sort.Slice(reg.Templates, func(i, j int) bool {
		return reg.Templates[i].Metadata.Name < reg.Templates[j].Metadata.Name
	})
	sort.Slice(reg.Blueprints, func(i, j int) bool {
		return reg.Blueprints[i].Metadata.Name < reg.Blueprints[j].Metadata.Name
	})
	sort.Slice(reg.Seeds, func(i, j int) bool {
		oi, oj := seedManifestOrder(reg.Seeds[i]), seedManifestOrder(reg.Seeds[j])
		if oi != oj {
			return oi < oj
		}
		return reg.Seeds[i].Metadata.Name < reg.Seeds[j].Metadata.Name
	})
	sort.Slice(reg.CronJobs, func(i, j int) bool {
		return reg.CronJobs[i].Metadata.Name < reg.CronJobs[j].Metadata.Name
	})
	sort.Slice(reg.Pipelines, func(i, j int) bool {
		return reg.Pipelines[i].Metadata.Name < reg.Pipelines[j].Metadata.Name
	})
	sort.Slice(reg.LiveOpsEvents, func(i, j int) bool {
		return reg.LiveOpsEvents[i].Metadata.Name < reg.LiveOpsEvents[j].Metadata.Name
	})
	sort.Slice(reg.Leaderboards, func(i, j int) bool {
		return reg.Leaderboards[i].Metadata.Name < reg.Leaderboards[j].Metadata.Name
	})
	sort.Slice(reg.AdminSites, func(i, j int) bool {
		return reg.AdminSites[i].Metadata.Name < reg.AdminSites[j].Metadata.Name
	})
	sort.Slice(reg.AdminResources, func(i, j int) bool {
		return reg.AdminResources[i].Metadata.Name < reg.AdminResources[j].Metadata.Name
	})
	sort.Slice(reg.AdminDashboards, func(i, j int) bool {
		return reg.AdminDashboards[i].Metadata.Name < reg.AdminDashboards[j].Metadata.Name
	})
	sort.Slice(reg.AdminPages, func(i, j int) bool {
		return reg.AdminPages[i].Metadata.Name < reg.AdminPages[j].Metadata.Name
	})
	sort.Slice(reg.AdminScreens, func(i, j int) bool {
		return reg.AdminScreens[i].Metadata.Name < reg.AdminScreens[j].Metadata.Name
	})
	sort.Slice(reg.AdminActions, func(i, j int) bool {
		return reg.AdminActions[i].Metadata.Name < reg.AdminActions[j].Metadata.Name
	})

	if reg.Project != nil {
		spec := reg.ProjectSpec()
		reg.ApiPrefix = spec.App.ApiPrefix
		if reg.ApiPrefix == "" {
			reg.ApiPrefix = "/api/v1"
		}
	} else {
		reg.ApiPrefix = "/api/v1"
	}

	cacheByRoot[key] = reg
	return reg, nil
}

func (r *Registry) GetManifestRoute(m *Manifest) (method, path string) {
	bffxDir := filepath.Join(r.Root, "bffx")
	if m.Feature != "" {
		bffxDir = filepath.Join(r.Root, "internal", "features")
	}
	return m.GetRoute(r.ApiPrefix, bffxDir)
}

func seedManifestOrder(m *Manifest) int {
	if m == nil {
		return 0
	}
	var spec SeedSpec
	if err := m.UnmarshalSpec(&spec); err != nil {
		return 0
	}
	return spec.Order
}

func (reg *Registry) walkManifests(projectRoot, searchRoot string, seen map[string]string) error {
	return filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// v2 Filter: internal/features/ must be inside one of the allowed subdirectories
		if strings.Contains(path, filepath.Join("internal", "features")) {
			allowed := false
			for _, dir := range []string{"manifests", "actions", "screens", "builders", "blueprints", "templates", "cronjobs", "pipelines", "liveops", "leaderboards", "seeds"} {
				if strings.Contains(path, string(filepath.Separator)+dir+string(filepath.Separator)) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil
			}
		}

		m, err := Load(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		// Inject implicit name if missing
		if m.Metadata.Name == "" {
			rel, _ := filepath.Rel(searchRoot, path)
			id := strings.TrimSuffix(rel, ext)
			// Strip "manifests/" from v2 paths for cleaner naming
			if strings.Contains(path, filepath.Join("internal", "features")) {
				parts := strings.Split(id, "manifests"+string(filepath.Separator))
				if len(parts) == 2 {
					id = parts[0] + parts[1]
				}
			}
			// features/auth/login -> features_auth_login
			m.Metadata.Name = strings.ReplaceAll(id, string(filepath.Separator), "_")
		}

		// Extract Feature name for V2
		if strings.Contains(path, filepath.Join("internal", "features")) {
			rel, _ := filepath.Rel(filepath.Join(projectRoot, "internal", "features"), path)
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) > 0 {
				m.Feature = parts[0]
			}
		}


		// Collision Detection
		key := fmt.Sprintf("%s:%s", m.Kind, m.Metadata.Name)
		if prev, ok := seen[key]; ok {
			return fmt.Errorf("duplicate manifest found for %s: %s and %s", key, prev, path)
		}
		seen[key] = path

		return reg.addManifest(m)
	})
}

func (reg *Registry) addManifest(m *Manifest) error {
	switch m.Kind {
	case "Project":
		if reg.Project != nil {
			return fmt.Errorf("multiple Project manifests found: %s and %s", reg.Project.Path, m.Path)
		}
		reg.Project = m
	case "Resource":
		reg.Resources = append(reg.Resources, m)
	case "Builder":
		reg.Builders = append(reg.Builders, m)
	case "Policy":
		reg.Policies = append(reg.Policies, m)
	case "Skill":
		reg.Skills = append(reg.Skills, m)
	case "Action":
		reg.Actions = append(reg.Actions, m)
	case "Function":
		reg.Functions = append(reg.Functions, m)
	case "Service":
		reg.Services = append(reg.Services, m)
	case "Stream":
		reg.Streams = append(reg.Streams, m)
	case "FeatureFlag":
		reg.FeatureFlags = append(reg.FeatureFlags, m)
	case "Subscription":
		reg.Subscriptions = append(reg.Subscriptions, m)
	case "Product":
		reg.Products = append(reg.Products, m)
	case "AdProvider":
		reg.AdProviders = append(reg.AdProviders, m)
	case "AdUnit":
		reg.AdUnits = append(reg.AdUnits, m)
	case "Template":
		reg.Templates = append(reg.Templates, m)
	case "Blueprint":
		reg.Blueprints = append(reg.Blueprints, m)
	case "Seed":
		reg.Seeds = append(reg.Seeds, m)
	case "Screen":
		reg.Screens = append(reg.Screens, m)
	case "CronJob":
		reg.CronJobs = append(reg.CronJobs, m)
	case "Pipeline":
		reg.Pipelines = append(reg.Pipelines, m)
	case "LiveOpsEvent":
		reg.LiveOpsEvents = append(reg.LiveOpsEvents, m)
	case "Leaderboard":
		reg.Leaderboards = append(reg.Leaderboards, m)
	case "AdminSite":
		reg.AdminSites = append(reg.AdminSites, m)
	case "AdminResource":
		reg.AdminResources = append(reg.AdminResources, m)
	case "AdminDashboard":
		reg.AdminDashboards = append(reg.AdminDashboards, m)
	case "AdminPage":
		reg.AdminPages = append(reg.AdminPages, m)
	case "AdminScreen":
		reg.AdminScreens = append(reg.AdminScreens, m)
	case "AdminAction":
		reg.AdminActions = append(reg.AdminActions, m)
	default:
		reg.Warnings = append(reg.Warnings, fmt.Sprintf("Unknown kind %q in %s", m.Kind, m.Path))
	}
	return nil
}

func (reg *Registry) ProjectSpec() *ProjectSpec {
	var spec ProjectSpec
	if reg.Project != nil {
		reg.Project.UnmarshalSpec(&spec)
	}

	// Alias batteries.flags <-> featureFlags.provider
	if spec.Batteries.Flags != "" && spec.FeatureFlags.Provider == "" {
		spec.FeatureFlags.Provider = spec.Batteries.Flags
	} else if spec.FeatureFlags.Provider != "" && spec.Batteries.Flags == "" {
		spec.Batteries.Flags = spec.FeatureFlags.Provider
	}

	// Alias batteries.store <-> store.mode
	if spec.Batteries.Store != "" && spec.Store.Mode == "" {
		spec.Store.Mode = spec.Batteries.Store
	} else if spec.Store.Mode != "" && spec.Batteries.Store == "" {
		spec.Batteries.Store = spec.Store.Mode
	}

	return &spec
}

func (reg *Registry) ResourceSpec(name string) *ResourceSpec {
	for _, m := range reg.Resources {
		if m.Metadata.Name == name {
			var spec ResourceSpec
			m.UnmarshalSpec(&spec)
			return &spec
		}
	}
	return nil
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("error in %s: %w", path, err)
	}

	if m.Kind == "" {
		return nil, fmt.Errorf("error in %s: missing 'kind' field", path)
	}

	m.Path = path
	m.Raw = data
	return &m, nil
}

func createFieldNode(name, typ string, required bool) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	n.Content = append(n.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "name"}, &yaml.Node{Kind: yaml.ScalarNode, Value: name},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "type"}, &yaml.Node{Kind: yaml.ScalarNode, Value: typ},
	)
	if required {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "required"}, &yaml.Node{Kind: yaml.ScalarNode, Value: "true"})
	}
	return n
}

func getInternalDefaultFields(name string) []*yaml.Node {
	switch name {
	case "User":
		return []*yaml.Node{
			createFieldNode("email", "string", true),
			createFieldNode("name", "string", false),
			createFieldNode("password", "string", false),
			createFieldNode("role", "string", false),
			createFieldNode("status", "string", false),
			createFieldNode("device_id", "string", false),
			createFieldNode("phone", "string", false),
			createFieldNode("whatsapp", "string", false),
			createFieldNode("is_verified", "bool", false),
			createFieldNode("otp_code", "string", false),
		}
	case "AdminUser":
		return []*yaml.Node{
			createFieldNode("email", "string", true),
			createFieldNode("password", "string", false),
			createFieldNode("role", "string", false),
		}
	case "AppConfig":
		return []*yaml.Node{
			createFieldNode("config_key", "string", false),
			createFieldNode("config_value", "string", false),
			createFieldNode("description", "string", false),
		}
	case "Incident":
		return []*yaml.Node{
			createFieldNode("title", "string", true),
			createFieldNode("severity", "string", false),
			createFieldNode("stack_trace", "string", false),
			createFieldNode("resolved", "bool", false),
			createFieldNode("resolved_by", "string", false),
		}
	case "Entitlement":
		return []*yaml.Node{
			createFieldNode("user_id", "string", true),
			createFieldNode("slug", "string", true),
			createFieldNode("expires_at", "string", false),
		}
	case "Subscription":
		return []*yaml.Node{
			createFieldNode("user_id", "string", true),
			createFieldNode("plan_id", "string", true),
			createFieldNode("status", "string", false),
			createFieldNode("expires_at", "string", false),
		}
	case "TelemetryEvent":
		return []*yaml.Node{
			createFieldNode("user_id", "string", false),
			createFieldNode("event_type", "string", true),
			createFieldNode("screen_name", "string", false),
			createFieldNode("action_name", "string", false),
			createFieldNode("properties", "string", false),
		}
	case "Device":
		return []*yaml.Node{
			createFieldNode("user_id", "string", true),
			createFieldNode("fingerprint", "string", true),
			createFieldNode("push_token", "string", false),
			createFieldNode("platform", "string", false),
			createFieldNode("model", "string", false),
			createFieldNode("os_version", "string", false),
			createFieldNode("app_version", "string", false),
			createFieldNode("is_trusted", "bool", false),
			createFieldNode("last_seen_at", "string", false),
		}
	case "Role", "Permission":
		return []*yaml.Node{
			createFieldNode("name", "string", true),
			createFieldNode("slug", "string", true),
		}
	case "MetricSample":
		return []*yaml.Node{
			createFieldNode("timestamp", "string", false),
			createFieldNode("goroutines", "int", false),
			createFieldNode("mem_alloc", "int", false),
			createFieldNode("mem_sys", "int", false),
			createFieldNode("cpu_percent", "float", false),
			createFieldNode("http_total", "int", false),
			createFieldNode("http_errors", "int", false),
			createFieldNode("http_p95_ms", "int", false),
		}
	case "UserRole":
		return []*yaml.Node{
			createFieldNode("user_id", "string", true),
			createFieldNode("role_id", "string", true),
		}
	}
	return nil
}

func mergeInternalFields(r *Manifest, defaultFields []*yaml.Node) {
	if len(defaultFields) == 0 {
		return
	}
	// Decode existing fields
	var spec ResourceSpec
	if err := r.UnmarshalSpec(&spec); err != nil {
		return
	}
	existingFields := make(map[string]bool)
	for _, f := range spec.Fields {
		existingFields[strings.ToLower(strings.TrimSpace(f.Name))] = true
	}

	// Find the "fields" key in the MappingNode r.Spec
	if r.Spec.Kind != yaml.MappingNode {
		return
	}

	var fieldsValNode *yaml.Node
	for i := 0; i < len(r.Spec.Content); i += 2 {
		if r.Spec.Content[i].Value == "fields" {
			fieldsValNode = r.Spec.Content[i+1]
			break
		}
	}

	// If fields key doesn't exist, create it
	if fieldsValNode == nil {
		fieldsKeyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "fields"}
		fieldsValNode = &yaml.Node{Kind: yaml.SequenceNode}
		r.Spec.Content = append(r.Spec.Content, fieldsKeyNode, fieldsValNode)
	}

	hasChanges := false
	// Append missing fields
	for _, df := range defaultFields {
		if len(df.Content) >= 2 && df.Content[0].Value == "name" {
			fieldName := df.Content[1].Value
			if !existingFields[strings.ToLower(strings.TrimSpace(fieldName))] {
				fieldsValNode.Content = append(fieldsValNode.Content, df)
				hasChanges = true
			}
		}
	}

	if hasChanges {
		if updatedBytes, err := yaml.Marshal(r); err == nil {
			r.Raw = updatedBytes
		}
	}
}
