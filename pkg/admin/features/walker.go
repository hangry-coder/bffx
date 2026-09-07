// Package features implements the App Features admin surface:
// discovery walker that turns Screen/Action/Resource manifests into a
// hierarchical tree, the private bffx_screen_kill_switch store, and
// the helpers used by the admin handlers.
package features

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// SourceRef describes one resolved entry from Screen.spec.sources.
type SourceRef struct {
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`              // "builtin" | "action" | "resource" | "unknown"
	Action           string   `json:"action,omitempty"`  // action manifest name when Kind=="action"
	Resource         string   `json:"resource,omitempty"`// resource name when Kind=="resource"
	RoutePath        string   `json:"route_path,omitempty"`
	RouteMethod      string   `json:"route_method,omitempty"`
	TouchesResources []string `json:"touches_resources,omitempty"`
}

// SectionRef describes one entry from Screen.spec.sections.
type SectionRef struct {
	Key             string         `json:"key"`
	DefaultVisible  bool           `json:"default_visible"`
	UIType          string         `json:"ui_type,omitempty"`
	UIProperties    map[string]any `json:"ui_properties,omitempty"`
	Layout          string         `json:"layout,omitempty"`
}

// ScreenNode is the per-Screen entry in the feature tree.
type ScreenNode struct {
	Name             string       `json:"name"`
	NavType          string       `json:"nav_type,omitempty"`
	Icon             string       `json:"icon,omitempty"`
	Order            int          `json:"order"`
	RequiresAuth     string       `json:"requires_auth,omitempty"`
	RouteMethod      string       `json:"route_method,omitempty"`
	RoutePath        string       `json:"route_path,omitempty"`
	ProtoService     string       `json:"proto_service,omitempty"` // populated post Phase 5
	Stream           bool         `json:"stream"`
	Sources          []SourceRef  `json:"sources"`
	Sections         []SectionRef `json:"sections"`
	TouchesResources []string     `json:"touches_resources"`
}

// GroupNode is a top-level group bucket (e.g. "mobile").
type GroupNode struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"display_name"`
	Screens     []ScreenNode `json:"screens"`
}

// OrphanAction is an Action manifest not referenced by any Screen
// (nor by any other Action's sources nor a hook's static calls).
type OrphanAction struct {
	Name        string `json:"name"`
	Group       string `json:"group,omitempty"`
	RoutePath   string `json:"route_path,omitempty"`
	RouteMethod string `json:"route_method,omitempty"`
	File        string `json:"file,omitempty"`
}

// ActionRef describes a single Action surfaced inside a FeatureCluster.
type ActionRef struct {
	Name        string `json:"name"`
	RouteMethod string `json:"route_method,omitempty"`
	RoutePath   string `json:"route_path,omitempty"`
	File        string `json:"file,omitempty"`
	IsOrphan    bool   `json:"is_orphan"`
}

// FeatureCluster represents a logical mobile/web capability derived from
// one or more Actions (and any Resources they touch) that share a common
// name root. Clusters give operators a "Feature"-shaped view of work that
// has no dedicated Screen yet (e.g. "meal_log", "fasting", "activity").
//
// Derivation order:
//  1. Explicit `spec.feature` on the Action (forward-compatible)
//  2. Heuristic on the Action's metadata name (verb prefix stripped,
//     `_action` suffix stripped, first significant token wins)
//  3. The folder-derived `Feature` field as a last resort
type FeatureCluster struct {
	Name             string      `json:"name"`
	DisplayName      string      `json:"display_name"`
	Group            string      `json:"group,omitempty"`
	Actions          []ActionRef `json:"actions"`
	TouchesResources []string    `json:"touches_resources,omitempty"`
	ScreensUsing     []string    `json:"screens_using,omitempty"`
}

// FeatureTree is the response payload for GET /api/admin/features.
type FeatureTree struct {
	Groups          []GroupNode      `json:"groups"`
	OrphanActions   []OrphanAction   `json:"orphan_actions"`
	FeatureClusters []FeatureCluster `json:"feature_clusters"`
}

// Walk produces a FeatureTree from a manifest.Registry. The optional
// groupFilter, when non-empty, restricts groups to that single value
// (case-insensitive). An empty groupFilter returns every group that has
// at least one Screen.
//
// The walker is read-only and side-effect-free; it can be called from
// HTTP handlers without locking.
func Walk(reg *manifest.Registry, groupFilter string) FeatureTree {
	if reg == nil {
		return FeatureTree{
			Groups:          []GroupNode{},
			OrphanActions:   []OrphanAction{},
			FeatureClusters: []FeatureCluster{},
		}
	}

	// Pre-index Actions by manifest name for O(1) lookup, and by lower-cased
	// name to tolerate older `action: get_subscription` style which
	// often omits the `_action` suffix.
	actionByName := make(map[string]*manifest.Manifest, len(reg.Actions))
	actionByShort := make(map[string]*manifest.Manifest, len(reg.Actions))
	for _, a := range reg.Actions {
		actionByName[a.Metadata.Name] = a
		short := strings.TrimSuffix(a.Metadata.Name, "_action")
		actionByShort[short] = a
	}

	// Pre-index Resources for lookup.
	resourceByName := make(map[string]*manifest.Manifest, len(reg.Resources))
	for _, r := range reg.Resources {
		resourceByName[r.Metadata.Name] = r
	}

	referencedActions := make(map[string]struct{})
	groupBuckets := make(map[string][]ScreenNode)

	// First pass: turn Screens into ScreenNodes.
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}

		group := strings.ToLower(strings.TrimSpace(m.Feature))
		if group == "" {
			// `Feature` is populated from folder discovery; fall back to a
			// declared spec.group key on the raw map for legacy manifests.
			group = "mobile"
		}
		if groupFilter != "" && !strings.EqualFold(group, groupFilter) {
			continue
		}

		node := ScreenNode{
			Name:         m.Metadata.Name,
			NavType:      spec.NavType,
			Icon:         spec.Icon,
			Order:        spec.Order,
			RequiresAuth: spec.RequiresAuth,
			RouteMethod:  strings.ToUpper(spec.Route.Method),
			RoutePath:    spec.Route.Path,
			Sections:     make([]SectionRef, 0, len(spec.Sections)),
			Sources:      make([]SourceRef, 0, len(spec.Sources)),
			TouchesResources: []string{},
		}
		if node.RouteMethod == "" {
			node.RouteMethod = "GET"
		}
		if node.RoutePath == "" {
			node.RoutePath = reg.ApiPrefix + "/screens/" + strings.ToLower(m.Metadata.Name)
		}

		touchedSet := make(map[string]struct{})
		for _, src := range spec.Sources {
			ref := classifySource(src, actionByName, actionByShort, resourceByName)
			node.Sources = append(node.Sources, ref)
			if ref.Kind == "action" && ref.Action != "" {
				referencedActions[ref.Action] = struct{}{}
			}
			for _, t := range ref.TouchesResources {
				if t == "" {
					continue
				}
				touchedSet[t] = struct{}{}
			}
		}

		for _, sec := range spec.Sections {
			sr := SectionRef{
				Key:            sec.Key,
				DefaultVisible: sec.DefaultVisible,
				Layout:         sec.Layout,
			}
			if sec.UI != nil {
				sr.UIType = sec.UI.Type
				sr.UIProperties = sec.UI.Properties
			}
			node.Sections = append(node.Sections, sr)
		}

		// Stable sort of touched resources.
		for t := range touchedSet {
			node.TouchesResources = append(node.TouchesResources, t)
		}
		sort.Strings(node.TouchesResources)

		groupBuckets[group] = append(groupBuckets[group], node)
	}

	// Second pass: Action-to-Action references (rare, but the plan calls
	// for it to avoid false-positive orphans).
	for _, a := range reg.Actions {
		var aSpec map[string]any
		_ = a.UnmarshalSpec(&aSpec)
		if srcs, ok := aSpec["sources"].([]any); ok {
			for _, s := range srcs {
				ref := classifySource(s, actionByName, actionByShort, nil)
				if ref.Kind == "action" && ref.Action != "" && ref.Action != a.Metadata.Name {
					referencedActions[ref.Action] = struct{}{}
				}
			}
		}
	}

	// Order screens inside each group by spec.Order, then name.
	for _, screens := range groupBuckets {
		sort.SliceStable(screens, func(i, j int) bool {
			if screens[i].Order != screens[j].Order {
				return screens[i].Order < screens[j].Order
			}
			return screens[i].Name < screens[j].Name
		})
	}

	groupNames := make([]string, 0, len(groupBuckets))
	for g := range groupBuckets {
		groupNames = append(groupNames, g)
	}
	sort.Strings(groupNames)

	groups := make([]GroupNode, 0, len(groupNames))
	for _, g := range groupNames {
		groups = append(groups, GroupNode{
			Name:        g,
			DisplayName: humanize(g),
			Screens:     groupBuckets[g],
		})
	}

	// Orphan Actions: not referenced by any Screen and not referenced by
	// any other Action.
	orphans := make([]OrphanAction, 0)
	for _, a := range reg.Actions {
		if _, used := referencedActions[a.Metadata.Name]; used {
			continue
		}
		// If the caller filtered by group, only surface orphans in that group.
		if groupFilter != "" && !strings.EqualFold(a.Feature, groupFilter) {
			continue
		}
		var aSpec manifest.ActionSpec
		_ = a.UnmarshalSpec(&aSpec)
		orphans = append(orphans, OrphanAction{
			Name:        a.Metadata.Name,
			Group:       a.Feature,
			RoutePath:   aSpec.Route.Path,
			RouteMethod: strings.ToUpper(aSpec.Route.Method),
			File:        a.Path,
		})
	}
	sort.SliceStable(orphans, func(i, j int) bool {
		if orphans[i].Group != orphans[j].Group {
			return orphans[i].Group < orphans[j].Group
		}
		return orphans[i].Name < orphans[j].Name
	})

	clusters := buildFeatureClusters(reg, groupFilter)

	return FeatureTree{
		Groups:          groups,
		OrphanActions:   orphans,
		FeatureClusters: clusters,
	}
}

// buildFeatureClusters groups every Action manifest into logical "Feature"
// buckets and surfaces each bucket as a FeatureCluster. Every Action is
// included exactly once; clusters are sorted by name. A cluster also
// records which Screens already reference its Actions, so operators can
// see at a glance whether a feature has a UI yet.
func buildFeatureClusters(reg *manifest.Registry, groupFilter string) []FeatureCluster {
	if len(reg.Actions) == 0 {
		return []FeatureCluster{}
	}

	// Reverse-index: action name -> screens that reference it.
	actionToScreens := make(map[string][]string)
	for _, m := range reg.Screens {
		var spec manifest.ScreenSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			continue
		}
		for _, src := range spec.Sources {
			if srcMap, ok := src.(map[string]any); ok {
				if act, _ := srcMap["action"].(string); act != "" {
					actionToScreens[act] = append(actionToScreens[act], m.Metadata.Name)
					// Also map by canonical "<name>_action" suffix in case
					// the manifest used the short form.
					actionToScreens[act+"_action"] = append(actionToScreens[act+"_action"], m.Metadata.Name)
				}
				if strings.EqualFold(fmt.Sprint(srcMap["kind"]), "action") {
					if n, _ := srcMap["name"].(string); n != "" {
						actionToScreens[n] = append(actionToScreens[n], m.Metadata.Name)
					}
				}
			}
		}
	}

	clusters := make(map[string]*FeatureCluster)
	for _, a := range reg.Actions {
		if groupFilter != "" && !strings.EqualFold(a.Feature, groupFilter) {
			continue
		}

		featureName := deriveFeatureName(a)
		if featureName == "" {
			continue
		}

		var spec manifest.ActionSpec
		_ = a.UnmarshalSpec(&spec)
		method := strings.ToUpper(spec.Route.Method)
		if method == "" {
			method = "GET"
		}

		// Resources the action touches (best-effort from raw spec).
		var raw map[string]any
		_ = a.UnmarshalSpec(&raw)
		var touches []string
		if t, ok := raw["touches"].([]any); ok {
			for _, item := range t {
				if s, ok := item.(string); ok && s != "" {
					touches = append(touches, s)
				}
			}
		}

		_, hasScreenRef := actionToScreens[a.Metadata.Name]
		ref := ActionRef{
			Name:        a.Metadata.Name,
			RouteMethod: method,
			RoutePath:   spec.Route.Path,
			File:        a.Path,
			IsOrphan:    !hasScreenRef,
		}

		c, ok := clusters[featureName]
		if !ok {
			c = &FeatureCluster{
				Name:        featureName,
				DisplayName: humanize(featureName),
				Group:       a.Feature,
			}
			clusters[featureName] = c
		}
		c.Actions = append(c.Actions, ref)
		if len(touches) > 0 {
			c.TouchesResources = mergeUnique(c.TouchesResources, touches)
		}
		if screens, ok := actionToScreens[a.Metadata.Name]; ok {
			c.ScreensUsing = mergeUnique(c.ScreensUsing, screens)
		}
	}

	names := make([]string, 0, len(clusters))
	for n := range clusters {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]FeatureCluster, 0, len(names))
	for _, n := range names {
		c := clusters[n]
		sort.SliceStable(c.Actions, func(i, j int) bool { return c.Actions[i].Name < c.Actions[j].Name })
		sort.Strings(c.TouchesResources)
		sort.Strings(c.ScreensUsing)
		out = append(out, *c)
	}
	return out
}

// deriveFeatureName turns an Action manifest into the cluster key.
// Heuristic (precedence order):
//  1. Read the raw `spec.feature` string when set (forward-compatible).
//  2. Strip the canonical `_action` suffix, drop a leading verb
//     ("log", "get", "list", "create", "update", "delete", "enforce",
//     "validate", "convert", "recalculate", "estimate"), take the
//     remaining tokens.
//  3. Fall back to the folder-derived Feature field.
func deriveFeatureName(a *manifest.Manifest) string {
	if a == nil {
		return ""
	}
	var raw map[string]any
	_ = a.UnmarshalSpec(&raw)
	if v, ok := raw["feature"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}

	name := strings.ToLower(strings.TrimSpace(a.Metadata.Name))
	if name == "" {
		return strings.ToLower(a.Feature)
	}

	// Strip a leading `<feature>_actions_` namespace if present (e.g.
	// `mobile_actions_log_meal_action` -> `log_meal_action`).
	for _, prefix := range []string{"mobile_actions_", "admin_actions_", "app_actions_", "user_actions_"} {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	name = strings.TrimSuffix(name, "_action")

	verbs := []string{
		"log", "get", "list", "fetch", "load",
		"create", "update", "patch", "delete", "remove",
		"enforce", "validate", "convert", "recalculate",
		"estimate", "compute", "calculate",
	}
	for _, v := range verbs {
		if strings.HasPrefix(name, v+"_") {
			name = strings.TrimPrefix(name, v+"_")
			break
		}
	}

	if name == "" {
		return strings.ToLower(a.Metadata.Name)
	}

	// Collapse compound action names down to their root noun so multiple
	// related actions cluster together. Examples:
	//   fasting_overview          -> fasting
	//   fasting_weekly_stats      -> fasting
	//   activity_overview         -> activity
	//   ai_credits                -> ai
	//   guest_to_user             -> guest
	// Operators who want a sub-feature granularity can override via
	// `spec.feature` on the manifest.
	if idx := strings.Index(name, "_"); idx > 0 {
		root := name[:idx]
		// Sanity guard: a 1-char root is rarely meaningful, fall back to
		// the full name.
		if len(root) >= 2 {
			return root
		}
	}
	return name
}

func mergeUnique(dst, src []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, v := range dst {
		seen[v] = struct{}{}
	}
	for _, v := range src {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		dst = append(dst, v)
	}
	return dst
}

// classifySource maps one Screen.spec.sources entry into a SourceRef.
//
// Accepted shapes (matching what real manifests use):
//   - "currentUser"                  → builtin
//   - "navigation" / "flags" / etc.  → builtin
//   - { kind: Resource, name: X }    → resource ref
//   - { name: <local>, action: X }   → action ref (shorthand)
//   - { kind: Action, name: X }      → action ref (canonical)
//   - any other map                  → kind="unknown" (still rendered in admin)
func classifySource(
	src any,
	actionByName, actionByShort, resourceByName map[string]*manifest.Manifest,
) SourceRef {
	switch v := src.(type) {
	case string:
		return SourceRef{Name: v, Kind: "builtin"}
	case map[string]any:
		name, _ := v["name"].(string)
		kind, _ := v["kind"].(string)
		actName, _ := v["action"].(string)
		resName, _ := v["resource"].(string)

		// Canonical { kind: Action, name: X } form.
		if strings.EqualFold(kind, "Action") {
			ref := SourceRef{Name: name, Kind: "action"}
			ref.Action, ref.RouteMethod, ref.RoutePath, ref.TouchesResources =
				resolveAction(name, actionByName, actionByShort)
			return ref
		}
		// Canonical { kind: Resource, name: X } form.
		if strings.EqualFold(kind, "Resource") {
			rname := name
			if rname == "" {
				rname = resName
			}
			ref := SourceRef{Name: rname, Kind: "resource", Resource: rname}
			if resourceByName != nil {
				if _, ok := resourceByName[rname]; !ok {
					ref.Kind = "unknown"
				}
			}
			if rname != "" {
				ref.TouchesResources = []string{rname}
			}
			return ref
		}
		// Shorthand: { name: subscription, action: get_subscription }
		if actName != "" {
			ref := SourceRef{Name: nonEmpty(name, actName), Kind: "action"}
			ref.Action, ref.RouteMethod, ref.RoutePath, ref.TouchesResources =
				resolveAction(actName, actionByName, actionByShort)
			return ref
		}
		// Last-chance bare-resource map: { resource: X }
		if resName != "" {
			ref := SourceRef{Name: resName, Kind: "resource", Resource: resName}
			if resourceByName != nil {
				if _, ok := resourceByName[resName]; !ok {
					ref.Kind = "unknown"
				}
			}
			ref.TouchesResources = []string{resName}
			return ref
		}
		return SourceRef{Name: nonEmpty(name, "unknown"), Kind: "unknown"}
	default:
		return SourceRef{Name: "unknown", Kind: "unknown"}
	}
}

func resolveAction(
	name string,
	actionByName, actionByShort map[string]*manifest.Manifest,
) (resolvedName, method, path string, touches []string) {
	if name == "" {
		return "", "", "", nil
	}
	a, ok := actionByName[name]
	if !ok {
		a, ok = actionByShort[name]
	}
	if !ok {
		return name, "", "", nil
	}
	var spec manifest.ActionSpec
	_ = a.UnmarshalSpec(&spec)

	// Best-effort touch detection: if the Action manifest declares a
	// `touches` slice (forward-compatible field), surface it. Otherwise
	// leave the slice empty — the admin UI accepts that.
	var raw map[string]any
	_ = a.UnmarshalSpec(&raw)
	if t, ok := raw["touches"].([]any); ok {
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				touches = append(touches, s)
			}
		}
	}
	method = strings.ToUpper(spec.Route.Method)
	if method == "" {
		method = "GET"
	}
	return a.Metadata.Name, method, spec.Route.Path, touches
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func humanize(s string) string {
	if s == "" {
		return ""
	}
	// Title-case the first letter; "mobile" → "Mobile".
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}
