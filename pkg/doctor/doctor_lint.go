package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/observability"
)

func lintObservabilityContract(spec *manifest.ProjectSpec) []Result {
	var results []Result
	if spec == nil {
		return results
	}

	obsType := spec.Batteries.Observability
	if obsType == "" {
		obsType = observability.LogProviderSlog
	}
	if observability.IsCanonicalLogProvider(obsType) {
		results = append(results, Result{
			Name:    "Observability: Canonical Log Provider",
			Status:  "ok",
			Message: fmt.Sprintf("batteries.observability=%q uses canonical naming", obsType),
		})
	} else {
		results = append(results, Result{
			Name:    "Observability: Canonical Log Provider",
			Status:  "fail",
			Message: fmt.Sprintf("batteries.observability=%q is non-canonical; use slog, axiom, or sentry", obsType),
		})
	}

	for _, legacy := range []string{
		observability.DeprecatedEnvIncidentProvider,
		observability.DeprecatedEnvTelemetryProvider,
		observability.DeprecatedEnvAnalyticsProvider,
	} {
		if os.Getenv(legacy) != "" {
			results = append(results, Result{
				Name:    "Observability: Legacy Env Override",
				Status:  "warn",
				Message: fmt.Sprintf("%s is set; prefer manifest batteries.observability and canonical provider adapters", legacy),
			})
		}
	}

	switch obsType {
	case observability.LogProviderAxiom:
		if os.Getenv("BFFX_AXIOM_TOKEN") == "" && os.Getenv("AXIOM_TOKEN") == "" {
			results = append(results, Result{
				Name:    "Observability: Axiom Credentials",
				Status:  "warn",
				Message: "batteries.observability=axiom but no Axiom token configured; runtime falls back to local slog",
			})
		}
	case observability.LogProviderSentry:
		if os.Getenv("BFFX_SENTRY_DSN") == "" && os.Getenv("SENTRY_DSN") == "" {
			results = append(results, Result{
				Name:    "Observability: Sentry Credentials",
				Status:  "warn",
				Message: "batteries.observability=sentry but no Sentry DSN configured; runtime falls back to local slog",
			})
		}
	}

	return results
}
func lintCacheTTLOnActions(reg *manifest.Registry) []Result {
	var results []Result
	for _, m := range reg.Actions {
		var spec manifest.ActionSpec
		m.UnmarshalSpec(&spec)
		if spec.Route.CacheTTL > 0 {
			method := strings.ToUpper(spec.Route.Method)
			if method == "" {
				method = "GET" // default
			}
			if method != "GET" {
				results = append(results, Result{
					Name:    "Action Cache Safety",
					Status:  "fail",
					Message: fmt.Sprintf("Action %q has cache_ttl=%d configured on mutable method %s. Mutable routes must never be cached.", m.Metadata.Name, spec.Route.CacheTTL, method),
				})
			} else {
				results = append(results, Result{
					Name:    "Action Cache Safety",
					Status:  "warn",
					Message: fmt.Sprintf("Action %q has cache_ttl=%d on GET. Ensure this custom action returns static data and that state mutation invalidation is properly handled.", m.Metadata.Name, spec.Route.CacheTTL),
				})
			}
		}
	}
	return results
}
func lintRoutePrefixes(root string, reg *manifest.Registry) []Result {
	var results []Result
	var violations []string
	var duplicates []string
	var implicitScreens []string

	prefix := strings.TrimSuffix(strings.TrimSpace(reg.ApiPrefix), "/")
	if prefix == "" {
		prefix = "/api/v1"
		if reg.Project != nil {
			var ps manifest.ProjectSpec
			if err := reg.Project.UnmarshalSpec(&ps); err == nil && ps.App.ApiPrefix != "" {
				prefix = strings.TrimSuffix(strings.TrimSpace(ps.App.ApiPrefix), "/")
			}
		}
	}
	prefix += "/"

	// Track (method, path) -> manifest name
	routes := make(map[string]string)

	check := func(m *manifest.Manifest, method, path string, isExplicit bool) {
		if path == "" {
			return
		}

		// 1. Prefix check
		// Skip for certain kinds if needed, but Phase 12 says "all Action/Builder"
		// Screen is also typically under the prefix.
		if !strings.HasPrefix(path, prefix) {
			// Legacy exceptions? Phase 12 says "document exception list for legacy /app/... only routes"
			// For now let's just warn.
			violations = append(violations, fmt.Sprintf("[%s] %s (expected prefix %s)", m.Kind, path, prefix))
		}

		// 2. Duplicate check
		key := fmt.Sprintf("%s:%s", strings.ToUpper(method), path)
		if existing, ok := routes[key]; ok {
			duplicates = append(duplicates, fmt.Sprintf("%s and %s share %s", existing, m.Metadata.Name, key))
		} else {
			routes[key] = m.Metadata.Name
		}

		// 3. Implicit screen check
		if m.Kind == "Screen" && !isExplicit {
			implicitScreens = append(implicitScreens, m.Metadata.Name)
		}
	}

	// Actions
	for _, m := range reg.Actions {
		var spec manifest.ActionSpec
		m.UnmarshalSpec(&spec)
		method, path := reg.GetManifestRoute(m)
		check(m, method, path, spec.Route.Path != "")
	}

	// Screens
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		m.UnmarshalSpec(&spec)
		method, path := reg.GetManifestRoute(m)
		check(m, method, path, spec.Route.Path != "")
	}

	// Builders
	for _, m := range reg.Builders {
		var spec manifest.BuilderSpec
		m.UnmarshalSpec(&spec)
		method, path := reg.GetManifestRoute(m)
		check(m, method, path, spec.Route.Path != "")
	}

	// Streams
	for _, m := range reg.Streams {
		var spec manifest.StreamSpec
		m.UnmarshalSpec(&spec)
		// Streams are typically GET/Upgrade, path is spec.Route.Path
		check(m, "GET", spec.Route.Path, true)
	}

	// Pipelines
	for _, m := range reg.Pipelines {
		var spec manifest.PipelineSpec
		m.UnmarshalSpec(&spec)
		method, path := reg.GetManifestRoute(m)
		check(m, method, path, spec.Route.Path != "")
	}

	// Resources (Crud routes)
	for _, m := range reg.Resources {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)
		if spec.Routes.Crud {
			colPath := spec.Routes.CollectionPath
			if colPath == "" {
				// Internal logic in router usually pluralizes,
				// but let's assume /api/v1/{resource} for linting if not specified.
				// Actually, the framework handles this.
				continue
			}

			path := colPath
			if !strings.HasPrefix(path, "/") {
				// framework prepends /api/v1/ (or prefix)
				path = prefix + path
			}

			// Check standard CRUD paths
			check(m, "GET", path, true)
			check(m, "POST", path, true)
			check(m, "GET", path+"/:id", true)
			check(m, "PATCH", path+"/:id", true)
			check(m, "DELETE", path+"/:id", true)
		}
	}

	if len(violations) > 0 {
		results = append(results, Result{
			Name:    "Route Prefix Lint",
			Status:  "warn",
			Message: fmt.Sprintf("Found %d routes not following %s prefix: %s", len(violations), prefix, strings.Join(violations, ", ")),
		})
	} else {
		results = append(results, Result{
			Name:    "Route Prefix Lint",
			Status:  "ok",
			Message: fmt.Sprintf("All routes follow standard %s prefixing", prefix),
		})
	}

	if len(duplicates) > 0 {
		results = append(results, Result{
			Name:    "Route Duplicate Lint",
			Status:  "fail",
			Message: fmt.Sprintf("Found %d duplicate routes: %s", len(duplicates), strings.Join(duplicates, "; ")),
		})
	}

	if len(implicitScreens) > 0 {
		results = append(results, Result{
			Name:    "Screen Path Hint",
			Status:  "warn", // Informational warning
			Message: fmt.Sprintf("%d screens use implicit paths: %s", len(implicitScreens), strings.Join(implicitScreens, ", ")),
		})
	}

	return results
}
func lintPipelines(root string, reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil {
		return results
	}

	// Check deprecation warning for batteries.nutrition
	spec := reg.ProjectSpec()
	if spec != nil && spec.Batteries.Nutrition != "" && spec.Batteries.Nutrition != "noop" {
		results = append(results, Result{
			Name:    "Pipeline Deprecation",
			Status:  "warn",
			Message: fmt.Sprintf("batteries.nutrition (%q) is deprecated and will be removed in v0.2.0. Please migrate to Ingestion Pipelines with the %q catalog adapter.", spec.Batteries.Nutrition, spec.Batteries.Nutrition),
		})
	}

	for _, m := range reg.Pipelines {
		var ps manifest.PipelineSpec
		if err := m.UnmarshalSpec(&ps); err != nil {
			continue
		}

		// 1. Ingestion type checks
		if ps.Type == "ingestion" {
			if ps.Catalog == nil || ps.Catalog.Adapter == "" {
				results = append(results, Result{
					Name:    fmt.Sprintf("Pipeline %q Ingestion Check", m.Metadata.Name),
					Status:  "fail",
					Message: "Ingestion pipeline must define a catalog adapter under spec.catalog.adapter.",
				})
			}
		}

		// 2. VLM alignment checks
		vlmBattery := "noop"
		if spec != nil && spec.Batteries.Vlm != "" {
			vlmBattery = spec.Batteries.Vlm
		}

		if vlmBattery == "noop" {
			hasRealModel := false
			for _, model := range ps.ModelRouting {
				if model != "noop" {
					hasRealModel = true
					break
				}
			}
			if hasRealModel {
				results = append(results, Result{
					Name:    fmt.Sprintf("Pipeline %q VLM Safety", m.Metadata.Name),
					Status:  "warn",
					Message: fmt.Sprintf("Pipeline uses model routing %v but batteries.vlm is set to %q.", ps.ModelRouting, vlmBattery),
				})
			}
		}

		// 3. Planned or Beta pipeline types (condenser, audio, rag, agent)
		switch ps.Type {
		case "condenser", "audio", "rag", "agent":
			results = append(results, Result{
				Name:    fmt.Sprintf("Pipeline %q Status", m.Metadata.Name),
				Status:  "warn",
				Message: fmt.Sprintf("Pipeline type %q is in preview / beta. Run verification tests to validate active capabilities.", ps.Type),
			})
		}
	}

	return results
}
func lintResourcePolicies(reg *manifest.Registry) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")

	for _, m := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}

		read := spec.Policy.Read
		write := spec.Policy.Write

		hasPolicy := read != nil && write != nil && read != "" && write != ""

		if !hasPolicy {
			status := "warn"
			if env == "production" {
				status = "fail"
			}
			results = append(results, Result{
				Name:    "Security: Resource Policy Omission",
				Status:  status,
				Message: fmt.Sprintf("Resource %q is missing a policy block. Add 'policy: { read: owner, write: owner }' to %s to secure it.", m.Metadata.Name, m.Path),
			})
			continue
		}

		readStr := fmt.Sprintf("%v", read)
		writeStr := fmt.Sprintf("%v", write)

		// Check User write policy (both dev and prod)
		if writeStr == "public" && strings.EqualFold(m.Metadata.Name, "User") {
			status := "warn"
			if env == "production" {
				status = "fail"
			}
			results = append(results, Result{
				Name:    "Security: Insecure User Public Write Policy",
				Status:  status,
				Message: fmt.Sprintf("Resource %q has 'write: public' (%s). This allows unauthenticated anonymous users to modify user credentials. Use 'authenticated' or 'owner' write policy instead.", m.Metadata.Name, m.Path),
			})
		}

		if env == "production" {
			if writeStr == "public" && !strings.EqualFold(m.Metadata.Name, "User") {
				results = append(results, Result{
					Name:    "Security: Insecure Public Write Policy",
					Status:  "fail",
					Message: fmt.Sprintf("Resource %q has 'write: public' in production profile (%s). This allows unauthenticated users to create/update records.", m.Metadata.Name, m.Path),
				})
			}

			sensitiveNames := []string{"user", "device", "config", "log", "billing", "setting", "session", "admin", "audit"}
			isSensitive := false
			lowerName := strings.ToLower(m.Metadata.Name)
			for _, s := range sensitiveNames {
				if strings.Contains(lowerName, s) {
					isSensitive = true
					break
				}
			}
			if isSensitive && readStr == "public" {
				results = append(results, Result{
					Name:    "Security: Sensitive Resource Public Read",
					Status:  "warn",
					Message: fmt.Sprintf("Resource %q (sensitive) has 'read: public' in production profile (%s). Confirm this is intended to be visible to all anonymous users.", m.Metadata.Name, m.Path),
				})
			}
		}
	}
	return results
}
func lintDevOnlyRoutes(reg *manifest.Registry) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")

	hasDevOnly := false
	var devOnlyList []string

	checkDevOnly := func(kind, name string, devOnly bool) {
		if devOnly {
			hasDevOnly = true
			devOnlyList = append(devOnlyList, fmt.Sprintf("%s:%s", kind, name))
		}
	}

	for _, m := range reg.Actions {
		var spec manifest.ActionSpec
		m.UnmarshalSpec(&spec)
		checkDevOnly(m.Kind, m.Metadata.Name, spec.DevOnly)
	}
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		m.UnmarshalSpec(&spec)
		checkDevOnly(m.Kind, m.Metadata.Name, spec.DevOnly)
	}
	for _, m := range reg.Builders {
		var spec manifest.BuilderSpec
		m.UnmarshalSpec(&spec)
		checkDevOnly(m.Kind, m.Metadata.Name, spec.DevOnly)
	}
	for _, m := range reg.Pipelines {
		var spec manifest.PipelineSpec
		m.UnmarshalSpec(&spec)
		checkDevOnly(m.Kind, m.Metadata.Name, spec.DevOnly)
	}
	for _, m := range reg.Streams {
		var spec manifest.StreamSpec
		m.UnmarshalSpec(&spec)
		checkDevOnly(m.Kind, m.Metadata.Name, spec.DevOnly)
	}

	if hasDevOnly {
		if env == "production" {
			results = append(results, Result{
				Name:    "Security: Dev-Only Routes Disabled",
				Status:  "ok",
				Message: fmt.Sprintf("Dev-only routes are disabled in production: %s", strings.Join(devOnlyList, ", ")),
			})
		} else {
			results = append(results, Result{
				Name:    "Security: Dev-Only Routes Active",
				Status:  "warn",
				Message: fmt.Sprintf("Found dev-only routes: %s. They will be disabled in production.", strings.Join(devOnlyList, ", ")),
			})
		}
	}
	return results
}
func lintBlueprints(reg *manifest.Registry) []Result {
	var results []Result
	env := os.Getenv("BFFX_ENV")

	for _, m := range reg.Blueprints {
		var spec manifest.BlueprintSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}

		checkMap := func(mName string, data map[string]any) {
			for k, v := range data {
				kLower := strings.ToLower(k)
				if strings.Contains(kLower, "password") || strings.Contains(kLower, "secret") || strings.Contains(kLower, "otp") || strings.Contains(kLower, "token") {
					if strVal, ok := v.(string); ok && strVal != "" {
						// Check if it's a plaintext literal (i.e. does not start with "$" or "${")
						if !strings.HasPrefix(strVal, "$") {
							status := "warn"
							if env == "production" {
								status = "fail"
							}
							results = append(results, Result{
								Name:    "Security: Insecure Seed Credential",
								Status:  status,
								Message: fmt.Sprintf("Blueprint %q has a suspicious plaintext credential for field %q in %s. Use environment variables (e.g. ${VAR_NAME}) instead of hardcoded literals in production.", m.Metadata.Name, k, filepath.Base(m.Path)),
							})
						}
					}
				}
			}
		}

		checkMap(m.Metadata.Name, spec.Fields)
		checkMap(m.Metadata.Name, spec.Defaults)
	}

	return results
}

var canonicalFlagProviders = map[string]bool{
	"bffx":         true,
	"goff":         true,
	"launchdarkly": true,
}

// lintExtensionTaxonomy enforces batteries vs addons boundaries from Phase 6 taxonomy.
func lintExtensionTaxonomy(reg *manifest.Registry) []Result {
	var results []Result
	spec := reg.ProjectSpec()
	if spec == nil {
		return results
	}

	if spec.Batteries.Nutrition != "" && spec.Batteries.Nutrition != "noop" {
		results = append(results, Result{
			Name:    "Taxonomy: batteries.nutrition",
			Status:  "warn",
			Message: fmt.Sprintf("batteries.nutrition=%q is deprecated; use an Ingestion Pipeline with spec.catalog.adapter (addon: pkg/addons/catalog/nutrition). See docs/core-concepts/extension_taxonomy.md", spec.Batteries.Nutrition),
		})
	} else {
		results = append(results, Result{
			Name:    "Taxonomy: batteries.nutrition",
			Status:  "ok",
			Message: "No deprecated batteries.nutrition override (use pipeline catalog adapters)",
		})
	}

	flagsProvider := strings.ToLower(strings.TrimSpace(spec.FeatureFlags.Provider))
	if flagsProvider == "" {
		flagsProvider = strings.ToLower(strings.TrimSpace(spec.Batteries.Flags))
	}
	if flagsProvider == "" {
		flagsProvider = "bffx"
	}
	if canonicalFlagProviders[flagsProvider] {
		results = append(results, Result{
			Name:    "Taxonomy: featureflags provider",
			Status:  "ok",
			Message: fmt.Sprintf("Feature flag provider %q uses canonical adapter wiring (core: pkg/featureflags, adapter: pkg/featureflags/providers)", flagsProvider),
		})
	} else {
		results = append(results, Result{
			Name:    "Taxonomy: featureflags provider",
			Status:  "fail",
			Message: fmt.Sprintf("Feature flag provider %q is non-canonical; use bffx, goff, or launchdarkly", flagsProvider),
		})
	}

	return results
}

func lintAdminManifests(reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil {
		return results
	}

	// 1. Build map of resource name -> fields
	resourceFields := make(map[string]map[string]bool)
	for _, res := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := res.UnmarshalSpec(&spec); err == nil {
			fields := make(map[string]bool)
			for _, f := range spec.Fields {
				fields[strings.ToLower(f.Name)] = true
			}
			fields["id"] = true
			fields["created_at"] = true
			fields["updated_at"] = true
			resourceFields[strings.ToLower(res.Metadata.Name)] = fields
		}
	}

	// 2. Iterate through AdminResources
	for _, ar := range reg.AdminResources {
		var spec manifest.AdminResourceSpec
		if err := ar.UnmarshalSpec(&spec); err != nil {
			continue
		}

		resNameLower := strings.ToLower(spec.Resource)
		fieldsMap, exists := resourceFields[resNameLower]
		if !exists {
			results = append(results, Result{
				Name:    fmt.Sprintf("Admin Resource %q backing check", spec.Resource),
				Status:  "fail",
				Message: fmt.Sprintf("Admin resource %q references non-existent backing resource %q", ar.Metadata.Name, spec.Resource),
			})
			continue
		}

		// Check index columns
		for _, col := range spec.Index.Columns {
			if !fieldsMap[strings.ToLower(col)] {
				results = append(results, Result{
					Name:    fmt.Sprintf("Admin Resource %q columns check", spec.Resource),
					Status:  "fail",
					Message: fmt.Sprintf("Admin resource %q index column %q does not exist on backing resource %q", ar.Metadata.Name, col, spec.Resource),
				})
			}
		}

		// Check filters
		for _, filter := range spec.Index.Filters {
			if !fieldsMap[strings.ToLower(filter.Field)] {
				results = append(results, Result{
					Name:    fmt.Sprintf("Admin Resource %q filter check", spec.Resource),
					Status:  "fail",
					Message: fmt.Sprintf("Admin resource %q filter field %q does not exist on backing resource %q", ar.Metadata.Name, filter.Field, spec.Resource),
				})
			}
		}

		// Check scopes
		for _, scope := range spec.Index.Scopes {
			for key := range scope.Where {
				if !fieldsMap[strings.ToLower(key)] {
					results = append(results, Result{
						Name:    fmt.Sprintf("Admin Resource %q scope check", spec.Resource),
						Status:  "fail",
						Message: fmt.Sprintf("Admin resource %q scope %q where-clause field %q does not exist on backing resource %q", ar.Metadata.Name, scope.Name, key, spec.Resource),
					})
				}
			}
		}
	}

	return results
}
