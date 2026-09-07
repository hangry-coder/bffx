package manifest

import (
	"strings"
)

func isSensitiveField(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "password") ||
		strings.Contains(n, "otp") ||
		strings.Contains(n, "secret") ||
		strings.Contains(n, "token") ||
		strings.Contains(n, "hash")
}

func BuildAdminGraph(reg *Registry) (*AdminGraph, error) {
	graph := &AdminGraph{
		ReservedResources: ReservedAdminResources,
	}

	// 1. Compile AdminSite
	if len(reg.AdminSites) > 0 {
		var spec AdminSiteSpec
		if err := reg.AdminSites[0].UnmarshalSpec(&spec); err == nil {
			graph.Site = spec
		}
	}
	// Apply Site defaults if empty
	if graph.Site.Title == "" {
		if reg.Project != nil {
			graph.Site.Title = reg.Project.Metadata.Name + " Console"
		} else {
			graph.Site.Title = "BFFX Console"
		}
	}
	if graph.Site.Theme == "" {
		graph.Site.Theme = "system"
	}
	if graph.Site.DefaultPerPage == 0 {
		graph.Site.DefaultPerPage = 25
	}
	// Default features if not specified
	if !graph.Site.Features.AppFeatures && !graph.Site.Features.FeatureFlags && !graph.Site.Features.Jobs && !graph.Site.Features.Liveops && !graph.Site.Features.ApiDocs {
		graph.Site.Features = AdminSiteFeatures{
			AppFeatures:  true,
			FeatureFlags: true,
			Jobs:         true,
			Liveops:      true,
			ApiDocs:      true,
		}
	}
	pol := ResolveAdminSession(graph.Site.Session)
	graph.Site.SessionResolved = &AdminSiteSessionResolved{
		MaxAgeSeconds:      pol.MaxAgeSeconds,
		IdleTimeoutSeconds: pol.IdleTimeoutSeconds,
		DevUnlimited:       pol.DevUnlimited,
	}

	// 2. Compile AdminDashboard
	if len(reg.AdminDashboards) > 0 {
		var spec AdminDashboardSpec
		if err := reg.AdminDashboards[0].UnmarshalSpec(&spec); err == nil {
			graph.Dashboard = &spec
		}
	}

	// 3. Compile AdminPages
	for _, p := range reg.AdminPages {
		var spec AdminPageSpec
		if err := p.UnmarshalSpec(&spec); err == nil {
			// Apply menu label default if empty
			if spec.Menu.Label == "" {
				spec.Menu.Label = p.Metadata.Name
			}
			graph.Pages = append(graph.Pages, spec)
		}
	}

	// 4. Compile AdminResources (with defaults fallback)
	adminResMap := make(map[string]*Manifest)
	for _, ar := range reg.AdminResources {
		var spec AdminResourceSpec
		if err := ar.UnmarshalSpec(&spec); err == nil {
			adminResMap[spec.Resource] = ar
		}
	}

	for _, res := range reg.Resources {
		var resSpec ResourceSpec
		_ = res.UnmarshalSpec(&resSpec)

		var spec AdminResourceSpec
		ar, hasManifest := adminResMap[res.Metadata.Name]
		if hasManifest {
			_ = ar.UnmarshalSpec(&spec)
		} else {
			spec.Resource = res.Metadata.Name
		}
		spec.Fields = resSpec.Fields

		// Apply defaults for fields that are empty
		trueVal := true
		if spec.Enabled == nil {
			// By default, enable all resources except reserved ones
			enabled := !IsReservedAdminResource(res.Metadata.Name)
			spec.Enabled = &enabled
		}

		if spec.Menu.Label == "" {
			if resSpec.DisplayName != "" {
				spec.Menu.Label = resSpec.DisplayName
			} else {
				// Simple pluralization fallback
				name := res.Metadata.Name
				if strings.HasSuffix(strings.ToLower(name), "y") && !strings.HasSuffix(strings.ToLower(name), "ay") && !strings.HasSuffix(strings.ToLower(name), "ey") && !strings.HasSuffix(strings.ToLower(name), "oy") && !strings.HasSuffix(strings.ToLower(name), "uy") {
					spec.Menu.Label = name[:len(name)-1] + "ies"
				} else if !strings.HasSuffix(strings.ToLower(name), "s") {
					spec.Menu.Label = name + "s"
				} else {
					spec.Menu.Label = name
				}
			}
		}

		if spec.Menu.Parent == "" {
			if resSpec.Group != "" {
				spec.Menu.Parent = resSpec.Group
			} else if res.Feature != "" {
				spec.Menu.Parent = "feature:" + res.Feature
			} else {
				spec.Menu.Parent = "app"
			}
		}

		if spec.Menu.Priority == 0 {
			spec.Menu.Priority = 100
		}

		// Backward-compatibility menu pins
		nameLower := strings.ToLower(res.Metadata.Name)
		if !hasManifest && (nameLower == "user" || nameLower == "device" || nameLower == "fasting_plan" || nameLower == "activity") {
			spec.Menu.Pin = true
		}

		// Index defaults
		if spec.Index.PerPage == 0 {
			spec.Index.PerPage = 25
		}
		if !hasManifest {
			spec.Index.Selectable = true
		}
		if len(spec.Index.Columns) == 0 {
			var cols []string
			for _, f := range resSpec.Fields {
				if !isSensitiveField(f.Name) {
					cols = append(cols, f.Name)
					if len(cols) == 6 {
						break
					}
				}
			}
			spec.Index.Columns = cols
		}
		if len(spec.Index.Scopes) == 0 {
			spec.Index.Scopes = []AdminResourceScope{
				{Name: "all", Default: trueVal},
			}
		}
		if len(spec.Index.Filters) == 0 {
			var filters []AdminResourceFilter
			for _, f := range resSpec.Fields {
				if isSensitiveField(f.Name) {
					continue
				}
				var filterType string
				switch f.Type {
				case "string":
					filterType = "string"
				case "bool", "boolean":
					filterType = "bool"
				case "date", "datetime", "time":
					filterType = "date_range"
				default:
					continue
				}
				filters = append(filters, AdminResourceFilter{
					Field: f.Name,
					As:    filterType,
					Label: strings.Title(f.Name),
				})
				if len(filters) == 3 {
					break
				}
			}
			spec.Index.Filters = filters
		}

		// Show defaults
		if len(spec.Show.Attributes) == 0 {
			var attrs []string
			for _, f := range resSpec.Fields {
				if !isSensitiveField(f.Name) {
					attrs = append(attrs, f.Name)
				}
			}
			spec.Show.Attributes = attrs
		}
		if len(spec.Show.Exclude) == 0 {
			var excl []string
			for _, f := range resSpec.Fields {
				if isSensitiveField(f.Name) {
					excl = append(excl, f.Name)
				}
			}
			spec.Show.Exclude = excl
		}

		// Form defaults
		if len(spec.Form.Exclude) == 0 {
			var excl []string
			for _, f := range resSpec.Fields {
				if isSensitiveField(f.Name) {
					excl = append(excl, f.Name)
				}
			}
			spec.Form.Exclude = excl
		}
		if len(spec.Form.Inputs) == 0 {
			var inputs []AdminResourceInput
			for _, f := range resSpec.Fields {
				if isSensitiveField(f.Name) {
					continue
				}
				var asType string
				if f.Target != "" {
					asType = "relation"
				} else {
					switch f.Type {
					case "string":
						if strings.Contains(strings.ToLower(f.Name), "email") {
							asType = "email"
						} else {
							asType = "string"
						}
					case "bool", "boolean":
						asType = "bool"
					case "date", "datetime", "time":
						asType = "date"
					default:
						asType = "string"
					}
				}
				inputs = append(inputs, AdminResourceInput{
					Field: f.Name,
					As:    asType,
				})
			}
			spec.Form.Inputs = inputs
		}

		spec.BelongsTo = mergeBelongsTo(spec.BelongsTo, inferBelongsTo(resSpec.Fields))

		graph.Resources = append(graph.Resources, spec)
	}

	ReconcileAdminMenu(graph, reg, NavPreferences{})

	return graph, nil
}
