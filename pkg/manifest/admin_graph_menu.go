package manifest

import (
	"sort"
	"strings"
)

// ReconcileAdminMenu rebuilds sidebar children (resource links, pins, feature links).
// Call after loading .bffx/admin-graph.json so runtime picks up manifest/pref changes without a full sync.
func ReconcileAdminMenu(graph *AdminGraph, reg *Registry, prefs NavPreferences) {
	ApplyNavPreferences(graph, prefs)
	finalizeAdminMenu(graph, reg, prefs)
}

func finalizeAdminMenu(graph *AdminGraph, reg *Registry, prefs NavPreferences) {
	pruneMenuByFeatures(graph)
	if len(graph.Site.Menu) == 0 {
		graph.Site.Menu = buildDefaultAdminMenu(graph, reg, prefs)
	} else {
		ensureResourcesMenuSection(graph, prefs)
		attachResourcesToMenuParents(graph)
		injectPinnedNavItems(graph)
		ensureAppFeaturesMenuSection(graph, reg, prefs)
		injectPinnedFeatureItems(graph, reg, prefs)
		ensureSettingsMenuSection(graph)
		ensureDocsMenuSection(graph)
	}
	sortMenuItems(graph.Site.Menu)
}

func pruneMenuByFeatures(graph *AdminGraph) {
	if !graph.Site.Features.FeatureFlags {
		removeMenuPage(&graph.Site.Menu, "feature-flags")
	}
	if !graph.Site.Features.Liveops {
		removeMenuItemByLabel(&graph.Site.Menu, "Monitoring")
	}
}

func removeMenuItemByLabel(items *[]AdminSiteMenuItem, label string) {
	out := (*items)[:0]
	for _, it := range *items {
		if strings.EqualFold(it.Label, label) {
			continue
		}
		if len(it.Children) > 0 {
			removeMenuItemByLabel(&it.Children, label)
		}
		out = append(out, it)
	}
	*items = out
}

func ensureResourcesMenuSection(graph *AdminGraph, prefs NavPreferences) {
	var pinned []AdminSiteMenuItem
	unpinned := 0
	for _, r := range graph.Resources {
		if !adminResourceNavVisible(r) {
			continue
		}
		if strings.EqualFold(r.Resource, "User") {
			continue
		}
		item := AdminSiteMenuItem{
			Label:    r.Menu.Label,
			Page:     "resource:" + r.Resource,
			Priority: r.Menu.Priority,
		}
		if r.Menu.Pin || prefs.ResourcePinned(r.Resource, false) {
			pinned = append(pinned, item)
		} else {
			unpinned++
		}
	}
	if len(pinned) == 0 && unpinned == 0 {
		return
	}
	sortMenuItems(pinned)
	children := pinned
	if unpinned > 0 {
		children = append(children, AdminSiteMenuItem{Label: "Other Resources", Page: "resources", Priority: 999})
	}
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label != "Resources" {
			continue
		}
		graph.Site.Menu[i].Children = mergeMenuChildrenByPage(graph.Site.Menu[i].Children, children)
		return
	}
}

func mergeMenuChildrenByPage(existing, injected []AdminSiteMenuItem) []AdminSiteMenuItem {
	if len(existing) == 0 {
		return injected
	}
	seen := make(map[string]struct{}, len(existing)+len(injected))
	out := make([]AdminSiteMenuItem, 0, len(existing)+len(injected))
	for _, it := range existing {
		if it.Page != "" {
			seen[it.Page] = struct{}{}
		}
		out = append(out, it)
	}
	for _, it := range injected {
		if it.Page != "" {
			if _, ok := seen[it.Page]; ok {
				continue
			}
			seen[it.Page] = struct{}{}
		}
		out = append(out, it)
	}
	sortMenuItems(out)
	return out
}

func ensureSettingsMenuSection(graph *AdminGraph) {
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label != "Settings" {
			continue
		}
		if !menuContainsPage(graph.Site.Menu[i].Children, "app-settings") {
			graph.Site.Menu[i].Children = append([]AdminSiteMenuItem{
				{Label: "App Settings", Page: "app-settings", Priority: 10},
			}, graph.Site.Menu[i].Children...)
		}
		sortMenuItems(graph.Site.Menu[i].Children)
		return
	}
}

func menuParentMatches(resourceParent, menuLabel string) bool {
	if strings.EqualFold(resourceParent, menuLabel) {
		return true
	}
	// Scaffold mismatch: AdminResource defaults use parent "app"; generated AdminSite uses label "Operations".
	if strings.EqualFold(resourceParent, "app") && strings.EqualFold(menuLabel, "Operations") {
		return true
	}
	return false
}

func buildDefaultAdminMenu(graph *AdminGraph, reg *Registry, prefs NavPreferences) []AdminSiteMenuItem {
	menu := []AdminSiteMenuItem{
		{Label: "Dashboard", Page: "dashboard", Priority: 0},
	}

	if graph.Site.Features.FeatureFlags {
		menu = append(menu, AdminSiteMenuItem{Label: "Feature Flags", Page: "feature-flags", Priority: 5})
	}

	for _, r := range graph.Resources {
		if !adminResourceNavVisible(r) {
			continue
		}
		if strings.EqualFold(r.Resource, "User") {
			menu = append(menu, AdminSiteMenuItem{Label: r.Menu.Label, Page: "users", Priority: 8})
			break
		}
	}

	if graph.Site.Features.Liveops {
		monitoring := []AdminSiteMenuItem{
			{Label: "Performance", Page: "performance", Priority: 0},
		}
		sortMenuItems(monitoring)
		menu = append(menu, AdminSiteMenuItem{Label: "Monitoring", Priority: 50, Children: monitoring})
	}

	var pinned []AdminSiteMenuItem
	unpinned := 0
	for _, r := range graph.Resources {
		if !adminResourceNavVisible(r) {
			continue
		}
		if strings.EqualFold(r.Resource, "User") {
			continue
		}
		item := AdminSiteMenuItem{
			Label:    r.Menu.Label,
			Page:     "resource:" + r.Resource,
			Priority: r.Menu.Priority,
		}
		if r.Menu.Pin {
			pinned = append(pinned, item)
		} else {
			unpinned++
		}
	}
	if len(pinned) > 0 || unpinned > 0 {
		sortMenuItems(pinned)
		children := pinned
		if unpinned > 0 {
			children = append(children, AdminSiteMenuItem{Label: "Other Resources", Page: "resources", Priority: 999})
		}
		menu = append(menu, AdminSiteMenuItem{Label: "Resources", Priority: 80, Children: children})
	}

	if graph.Site.Features.AppFeatures {
		menu = append(menu, buildAppFeaturesMenuSection(reg, prefs))
	}

	settings := []AdminSiteMenuItem{
		{Label: "App Settings", Page: "app-settings", Priority: 10},
	}
	if graph.Site.Features.ApiDocs {
		docs := []AdminSiteMenuItem{
			{Label: "API Reference", Page: "api-docs", Priority: 0},
			{Label: "BFFX Guide", Page: "bffx-docs", Priority: 10},
		}
		sortMenuItems(docs)
		settings = append(settings, AdminSiteMenuItem{Label: "Docs", Priority: 20, Children: docs})
	}
	sortMenuItems(settings)
	menu = append(menu, AdminSiteMenuItem{Label: "Settings", Priority: 90, Children: settings})

	return menu
}

func buildAppFeaturesMenuSection(reg *Registry, prefs NavPreferences) AdminSiteMenuItem {
	var pinned []AdminSiteMenuItem
	unpinned := 0

	for _, scr := range reg.Screens {
		name := scr.Metadata.Name
		page := "screen:" + name
		var scrSpec struct {
			DisplayName string `yaml:"displayName"`
		}
		_ = scr.UnmarshalSpec(&scrSpec)
		label := scrSpec.DisplayName
		if label == "" {
			label = name
		}
		item := AdminSiteMenuItem{Label: label, Page: page, Priority: 100}
		if featureNavPinned(prefs, page) {
			pinned = append(pinned, item)
		} else {
			unpinned++
		}
	}

	sortMenuItems(pinned)
	children := pinned
	if unpinned > 0 || len(children) == 0 {
		children = append(children, AdminSiteMenuItem{
			Label:    "Other Features",
			Page:     "app-features",
			Priority: 999,
		})
	}
	return AdminSiteMenuItem{Label: "App Features", Priority: 70, Children: children}
}

func featureNavPinned(prefs NavPreferences, page string) bool {
	if prefs.Features != nil {
		if v, ok := prefs.Features[page]; ok {
			return v
		}
	}
	return false
}

func injectPinnedFeatureItems(graph *AdminGraph, reg *Registry, prefs NavPreferences) {
	for _, scr := range reg.Screens {
		name := scr.Metadata.Name
		page := "screen:" + name
		if !featureNavPinned(prefs, page) {
			continue
		}
		child := AdminSiteMenuItem{
			Label:    name,
			Page:     page,
			Priority: 100,
		}
		if menuContainsPage(graph.Site.Menu, page) {
			continue
		}
		if !injectMenuChild(&graph.Site.Menu, "App Features", child) {
			graph.Site.Menu = append(graph.Site.Menu, child)
		}
	}
}

func injectPinnedNavItems(graph *AdminGraph) {
	for _, r := range graph.Resources {
		if !adminResourceNavVisible(r) || !r.Menu.Pin {
			continue
		}
		if strings.EqualFold(r.Resource, "User") {
			continue
		}
		child := AdminSiteMenuItem{
			Label:    r.Menu.Label,
			Page:     "resource:" + r.Resource,
			Priority: r.Menu.Priority,
		}
		if menuContainsPage(graph.Site.Menu, child.Page) {
			continue
		}
		if injectMenuChild(&graph.Site.Menu, "Resources", child) {
			continue
		}
		graph.Site.Menu = append(graph.Site.Menu, child)
	}
}

func ensureDocsMenuSection(graph *AdminGraph) {
	if !graph.Site.Features.ApiDocs {
		removeMenuPage(&graph.Site.Menu, "api-docs")
		removeMenuPage(&graph.Site.Menu, "bffx-docs")
		return
	}
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label != "Settings" {
			continue
		}
		// Drop legacy top-level API Reference under Settings.
		removeMenuPage(&graph.Site.Menu[i].Children, "api-docs")
		removeMenuPage(&graph.Site.Menu[i].Children, "bffx-docs")
		for j := range graph.Site.Menu[i].Children {
			if graph.Site.Menu[i].Children[j].Label == "Docs" {
				graph.Site.Menu[i].Children[j] = buildDocsMenuSection()
				return
			}
		}
		graph.Site.Menu[i].Children = append(graph.Site.Menu[i].Children, buildDocsMenuSection())
		return
	}
}

func buildDocsMenuSection() AdminSiteMenuItem {
	docs := []AdminSiteMenuItem{
		{Label: "API Reference", Page: "api-docs", Priority: 0},
		{Label: "BFFX Guide", Page: "bffx-docs", Priority: 10},
	}
	sortMenuItems(docs)
	return AdminSiteMenuItem{Label: "Docs", Priority: 20, Children: docs}
}

func ensureAppFeaturesMenuSection(graph *AdminGraph, reg *Registry, prefs NavPreferences) {
	if !graph.Site.Features.AppFeatures {
		removeMenuPage(&graph.Site.Menu, "app-features")
		return
	}
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label == "App Features" {
			graph.Site.Menu[i] = buildAppFeaturesMenuSection(reg, prefs)
			return
		}
	}
	removeMenuPage(&graph.Site.Menu, "app-features")
	graph.Site.Menu = append(graph.Site.Menu, buildAppFeaturesMenuSection(reg, prefs))
}

func injectMenuChild(items *[]AdminSiteMenuItem, groupLabel string, child AdminSiteMenuItem) bool {
	for i := range *items {
		if strings.EqualFold((*items)[i].Label, groupLabel) {
			if !menuContainsPage((*items)[i].Children, child.Page) {
				(*items)[i].Children = append((*items)[i].Children, child)
				sortMenuItems((*items)[i].Children)
			}
			return true
		}
		if injectMenuChild(&(*items)[i].Children, groupLabel, child) {
			return true
		}
	}
	return false
}

func removeMenuPage(items *[]AdminSiteMenuItem, page string) {
	out := (*items)[:0]
	for _, it := range *items {
		if it.Page == page {
			continue
		}
		if len(it.Children) > 0 {
			removeMenuPage(&it.Children, page)
		}
		out = append(out, it)
	}
	*items = out
}

func attachResourcesToMenuParents(graph *AdminGraph) {
	for i := range graph.Site.Menu {
		attachResourcesToMenuItem(graph, &graph.Site.Menu[i])
	}
}

func attachResourcesToMenuItem(graph *AdminGraph, item *AdminSiteMenuItem) {
	for i := range item.Children {
		attachResourcesToMenuItem(graph, &item.Children[i])
	}
	if item.Page != "" && !strings.EqualFold(item.Label, "Operations") {
		return
	}
	parentLabel := item.Label
	for _, r := range graph.Resources {
		if !adminResourceNavVisible(r) {
			continue
		}
		if !menuParentMatches(r.Menu.Parent, parentLabel) {
			continue
		}
		child := AdminSiteMenuItem{
			Label:    r.Menu.Label,
			Page:     "resource:" + r.Resource,
			Priority: r.Menu.Priority,
		}
		if !menuContainsPage(item.Children, child.Page) {
			item.Children = append(item.Children, child)
		}
	}
	sortMenuItems(item.Children)
}

func adminResourceNavVisible(r AdminResourceSpec) bool {
	if r.Enabled != nil && !*r.Enabled {
		return false
	}
	return !IsReservedAdminResource(r.Resource)
}

func menuContainsPage(items []AdminSiteMenuItem, page string) bool {
	for _, it := range items {
		if it.Page == page {
			return true
		}
		if menuContainsPage(it.Children, page) {
			return true
		}
	}
	return false
}

func sortMenuItems(items []AdminSiteMenuItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].Label < items[j].Label
	})
	for i := range items {
		if len(items[i].Children) > 0 {
			sortMenuItems(items[i].Children)
		}
	}
}
