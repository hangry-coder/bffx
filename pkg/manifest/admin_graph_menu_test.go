package manifest

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildAdminGraph_DefaultMenu(t *testing.T) {
	reg := manifestRegistryForMenuTest(t)
	graph, err := BuildAdminGraph(reg)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Site.Menu) == 0 {
		t.Fatal("expected default menu")
	}
	if graph.Site.Menu[0].Page != "dashboard" {
		t.Fatalf("first item = %q", graph.Site.Menu[0].Page)
	}
	foundFlags := false
	foundAPI := false
	foundGuide := false
	for _, top := range graph.Site.Menu {
		if top.Page == "feature-flags" {
			foundFlags = true
		}
		walkMenu(top, func(it AdminSiteMenuItem) {
			if it.Page == "api-docs" {
				foundAPI = true
			}
			if it.Page == "bffx-docs" {
				foundGuide = true
			}
		})
	}
	if !foundFlags {
		t.Error("expected feature-flags in menu")
	}
	if !foundAPI || !foundGuide {
		t.Errorf("expected api-docs and bffx-docs under Settings → Docs, api=%v guide=%v", foundAPI, foundGuide)
	}
	var appFeatures *AdminSiteMenuItem
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label == "App Features" {
			appFeatures = &graph.Site.Menu[i]
			break
		}
	}
	if appFeatures == nil {
		t.Fatal("App Features section missing")
	}
	if !menuContainsPage(appFeatures.Children, "app-features") {
		t.Fatal("expected Other Features link under App Features")
	}
	for _, child := range appFeatures.Children {
		if child.Label == "Catalog" {
			t.Fatal("legacy Catalog label should be renamed to Other Features")
		}
	}
	var monitoring *AdminSiteMenuItem
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label == "Monitoring" {
			monitoring = &graph.Site.Menu[i]
			break
		}
	}
	if monitoring == nil {
		t.Fatal("Monitoring section missing")
	}
	if monitoring.Page != "" {
		t.Fatalf("Monitoring should be a group header, not page %q", monitoring.Page)
	}
	if !menuContainsPage(monitoring.Children, "performance") {
		t.Fatalf("expected Performance under Monitoring, children=%v", monitoring.Children)
	}
	if menuContainsPage(monitoring.Children, "alerts") {
		t.Fatal("alerts must not appear in sidebar menu")
	}
	for _, top := range graph.Site.Menu {
		if top.Label != "Settings" {
			continue
		}
		walkMenu(top, func(it AdminSiteMenuItem) {
			if it.Page == "system-settings" {
				t.Fatal("system-settings must not appear in sidebar menu")
			}
		})
	}
}

func TestBuildAdminGraph_AppParentMatchesOperationsLabel(t *testing.T) {
	reg := manifestRegistryForMenuTest(t)
	reg.AdminSites = []*Manifest{{
		Kind:     "AdminSite",
		Metadata: Metadata{Name: "default"},
		Spec: mustYAMLNode(t, `menu:
  - label: Dashboard
    page: dashboard
  - label: Operations
    priority: 100
`),
	}}
	var noteSpec yaml.Node
	_ = yaml.Unmarshal([]byte(`fields:
  - {name: title, type: string}
`), &noteSpec)
	reg.Resources = append(reg.Resources, &Manifest{
		Metadata: Metadata{Name: "Note"},
		Spec:     noteSpec,
	})
	reg.AdminResources = []*Manifest{{
		Kind:     "AdminResource",
		Metadata: Metadata{Name: "NoteAdmin"},
		Spec: mustYAMLNode(t, `resource: Note
menu:
  label: Notes
  parent: app
  pin: true
`),
	}}

	graph, err := BuildAdminGraph(reg)
	if err != nil {
		t.Fatal(err)
	}
	var ops *AdminSiteMenuItem
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label == "Operations" {
			ops = &graph.Site.Menu[i]
			break
		}
	}
	if ops == nil {
		t.Fatal("Operations menu group missing")
	}
	if !menuContainsPage(ops.Children, "resource:Note") {
		t.Fatalf("expected Note under Operations via app parent alias, children=%v", ops.Children)
	}
}

func TestBuildAdminGraph_AttachesResourcesUnderOperations(t *testing.T) {
	reg := manifestRegistryForMenuTest(t)
	reg.AdminSites = []*Manifest{{
		Kind:     "AdminSite",
		Metadata: Metadata{Name: "default"},
		Spec: mustYAMLNode(t, `title: Test
features:
  feature_flags: false
  api_docs: false
menu:
  - label: Dashboard
    page: dashboard
  - label: Operations
    priority: 100
`),
	}}
	var noteSpec yaml.Node
	_ = yaml.Unmarshal([]byte(`fields:
  - {name: title, type: string}
`), &noteSpec)
	reg.Resources = append(reg.Resources, &Manifest{
		Metadata: Metadata{Name: "Note"},
		Spec:     noteSpec,
	})
	reg.AdminResources = []*Manifest{{
		Kind:     "AdminResource",
		Metadata: Metadata{Name: "NoteAdmin"},
		Spec: mustYAMLNode(t, `resource: Note
menu:
  label: Notes
  parent: Operations
  pin: true
`),
	}}

	graph, err := BuildAdminGraph(reg)
	if err != nil {
		t.Fatal(err)
	}
	var ops *AdminSiteMenuItem
	for i := range graph.Site.Menu {
		if graph.Site.Menu[i].Label == "Operations" {
			ops = &graph.Site.Menu[i]
			break
		}
	}
	if ops == nil {
		t.Fatal("Operations menu group missing")
	}
	if !menuContainsPage(ops.Children, "resource:Note") {
		t.Fatalf("expected Note under Operations, children=%v", ops.Children)
	}
}

func manifestRegistryForMenuTest(t *testing.T) *Registry {
	t.Helper()
	var userSpec yaml.Node
	_ = yaml.Unmarshal([]byte(`fields:
  - {name: email, type: string}
  - {name: name, type: string}
`), &userSpec)
	return &Registry{
		Project: &Manifest{
			Metadata: Metadata{Name: "test"},
			Spec:     yaml.Node{},
		},
		Resources: []*Manifest{{
			Metadata: Metadata{Name: "User"},
			Spec:     userSpec,
		}},
	}
}

func walkMenu(item AdminSiteMenuItem, fn func(AdminSiteMenuItem)) {
	fn(item)
	for _, c := range item.Children {
		walkMenu(c, fn)
	}
}

func mustYAMLNode(t *testing.T, content string) yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return *doc.Content[0]
	}
	return doc
}
