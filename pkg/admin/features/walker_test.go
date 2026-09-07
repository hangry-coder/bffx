package features

import (
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func makeManifest(t *testing.T, kind, name, feature, specYAML string) *manifest.Manifest {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(specYAML), &node); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	// yaml.Unmarshal into a Node yields a document node; we want the content.
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	return &manifest.Manifest{
		ApiVersion: "bffx.io/v1alpha1",
		Kind:       kind,
		Metadata:   manifest.Metadata{Name: name},
		Feature:    feature,
		Spec:       node,
	}
}

func newReg() *manifest.Registry {
	return &manifest.Registry{ApiPrefix: "/api/v1"}
}

func TestWalk_EmptyRegistry(t *testing.T) {
	tree := Walk(nil, "")
	if len(tree.Groups) != 0 || len(tree.OrphanActions) != 0 {
		t.Fatalf("expected empty tree, got %+v", tree)
	}

	tree = Walk(newReg(), "")
	if len(tree.Groups) != 0 || len(tree.OrphanActions) != 0 {
		t.Fatalf("expected empty tree from empty reg, got %+v", tree)
	}
}

func TestWalk_BuiltinAndActionAndResourceSources(t *testing.T) {
	reg := newReg()

	// Resource manifest used by Home screen.
	reg.Resources = append(reg.Resources, makeManifest(t, "Resource", "user", "mobile", `
group: mobile
fields:
  - { name: email, type: string }
`))

	// Action referenced by UserProfile screen via the shorthand
	// `{ name: subscription, action: get_subscription }`.
	reg.Actions = append(reg.Actions, makeManifest(t, "Action", "get_subscription_action", "mobile", `
group: mobile
route:
  method: GET
  path: /api/v1/app/subscription
  auth: required
`))

	// Orphan Action — no Screen references this.
	reg.Actions = append(reg.Actions, makeManifest(t, "Action", "log_water_action", "mobile", `
group: mobile
route:
  method: POST
  path: /api/v1/app/water-log
  auth: required
`))

	// Home screen: built-in currentUser + Resource ref.
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
icon: home
order: 1
requires_auth: required
route:
  method: GET
  path: /api/v1/screens/home
sources:
  - currentUser
  - { kind: Resource, name: user }
sections:
  - { key: weekly_stats, default_visible: true }
`))

	// UserProfile screen: shorthand source.
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "userprofile", "mobile", `
group: mobile
name: UserProfile
nav_type: folded
order: 2
sources:
  - { name: subscription, action: get_subscription }
sections:
  - { key: subscription_info, default_visible: true }
`))

	tree := Walk(reg, "")

	if len(tree.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(tree.Groups))
	}
	g := tree.Groups[0]
	if g.Name != "mobile" {
		t.Fatalf("expected group=mobile, got %q", g.Name)
	}
	if g.DisplayName != "Mobile" {
		t.Fatalf("expected DisplayName=Mobile, got %q", g.DisplayName)
	}
	if len(g.Screens) != 2 {
		t.Fatalf("expected 2 screens, got %d", len(g.Screens))
	}

	// Screens are sorted by Order ascending, so home (1) comes before userprofile (2).
	home := g.Screens[0]
	if home.Name != "home" || home.NavType != "bottom" || home.Order != 1 {
		t.Fatalf("home screen mismatch: %+v", home)
	}
	if home.RoutePath != "/api/v1/screens/home" || home.RouteMethod != "GET" {
		t.Fatalf("home route mismatch: %+v", home)
	}
	if len(home.Sources) != 2 {
		t.Fatalf("home expected 2 sources, got %d", len(home.Sources))
	}
	if home.Sources[0].Kind != "builtin" || home.Sources[0].Name != "currentUser" {
		t.Fatalf("home first source: %+v", home.Sources[0])
	}
	if home.Sources[1].Kind != "resource" || home.Sources[1].Resource != "user" {
		t.Fatalf("home resource source: %+v", home.Sources[1])
	}
	if len(home.TouchesResources) != 1 || home.TouchesResources[0] != "user" {
		t.Fatalf("home touches mismatch: %+v", home.TouchesResources)
	}
	if len(home.Sections) != 1 || home.Sections[0].Key != "weekly_stats" {
		t.Fatalf("home sections mismatch: %+v", home.Sections)
	}

	prof := g.Screens[1]
	if prof.Name != "userprofile" {
		t.Fatalf("expected userprofile second: %+v", prof)
	}
	if len(prof.Sources) != 1 || prof.Sources[0].Kind != "action" {
		t.Fatalf("userprofile source kind: %+v", prof.Sources)
	}
	if prof.Sources[0].Action != "get_subscription_action" {
		t.Fatalf("expected action resolved to get_subscription_action, got %q", prof.Sources[0].Action)
	}
	if prof.Sources[0].RoutePath != "/api/v1/app/subscription" {
		t.Fatalf("expected route path resolved, got %q", prof.Sources[0].RoutePath)
	}

	// Orphan detection: log_water_action is unreferenced; subscription Action is referenced.
	if len(tree.OrphanActions) != 1 {
		t.Fatalf("expected 1 orphan, got %d (%+v)", len(tree.OrphanActions), tree.OrphanActions)
	}
	if tree.OrphanActions[0].Name != "log_water_action" {
		t.Fatalf("unexpected orphan: %+v", tree.OrphanActions[0])
	}
	if tree.OrphanActions[0].RouteMethod != "POST" {
		t.Fatalf("orphan method: %+v", tree.OrphanActions[0])
	}
}

func TestWalk_GroupFilter(t *testing.T) {
	reg := newReg()
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources: []
sections: []
`))
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "dashboard", "admin", `
group: admin
name: Dashboard
nav_type: folded
order: 1
route: { method: GET, path: /api/v1/screens/dashboard }
sources: []
sections: []
`))

	tree := Walk(reg, "mobile")
	if len(tree.Groups) != 1 || tree.Groups[0].Name != "mobile" {
		t.Fatalf("group filter mobile failed: %+v", tree)
	}

	tree = Walk(reg, "ADMIN") // case-insensitive
	if len(tree.Groups) != 1 || tree.Groups[0].Name != "admin" {
		t.Fatalf("group filter ADMIN failed: %+v", tree)
	}
}

func TestWalk_ActionDefaultRouteFallback(t *testing.T) {
	reg := newReg()
	// No explicit path on screen — walker fills with apiPrefix + screens/<lower(name)>.
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "Home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 0
sources: []
sections: []
`))

	tree := Walk(reg, "mobile")
	if len(tree.Groups) != 1 || len(tree.Groups[0].Screens) != 1 {
		t.Fatalf("walk fallback shape: %+v", tree)
	}
	if got := tree.Groups[0].Screens[0].RoutePath; !strings.HasSuffix(got, "/screens/home") {
		t.Fatalf("expected fallback route path, got %q", got)
	}
}

func TestWalk_UnknownSourceKindStillRendered(t *testing.T) {
	reg := newReg()
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 0
sources:
  - { name: weird, mystery: true }
sections: []
`))
	tree := Walk(reg, "mobile")
	if got := tree.Groups[0].Screens[0].Sources[0].Kind; got != "unknown" {
		t.Fatalf("expected unknown kind, got %q", got)
	}
}

func TestWalk_ActionToActionChainAvoidsFalseOrphan(t *testing.T) {
	reg := newReg()

	// Screen references parent_action; parent_action references child_action.
	// child_action must NOT be flagged as orphan.
	reg.Actions = append(reg.Actions, makeManifest(t, "Action", "parent_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/parent, auth: required }
sources:
  - { kind: Action, name: child_action }
`))
	reg.Actions = append(reg.Actions, makeManifest(t, "Action", "child_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/child, auth: required }
`))
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "home", "mobile", `
group: mobile
name: Home
nav_type: bottom
order: 1
route: { method: GET, path: /api/v1/screens/home }
sources:
  - { kind: Action, name: parent_action }
sections: []
`))

	tree := Walk(reg, "")
	if len(tree.OrphanActions) != 0 {
		t.Fatalf("expected zero orphans (action chain), got %+v", tree.OrphanActions)
	}
}

// TestWalk_FeatureClustersGroupActionsByDerivedName guards the
// real-world case: 15 mobile Actions like `log_meal_action`,
// `fasting_overview_action`, `activity_overview_action` should produce
// human-friendly feature clusters (meal, fasting, activity) so operators
// see "what the mobile app does" even when an Action is not yet wired to
// a Screen.
func TestWalk_FeatureClustersGroupActionsByDerivedName(t *testing.T) {
	reg := newReg()
	reg.Actions = append(reg.Actions,
		makeManifest(t, "Action", "log_meal_action", "mobile", `
group: mobile
route: { method: POST, path: /api/v1/app/log-meal, auth: required }
`),
		makeManifest(t, "Action", "log_water_action", "mobile", `
group: mobile
route: { method: POST, path: /api/v1/app/log-water, auth: required }
`),
		makeManifest(t, "Action", "fasting_overview_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/app/fasting-state, auth: required }
`),
		makeManifest(t, "Action", "fasting_weekly_stats_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/app/fasting/weekly-stats, auth: required }
`),
		makeManifest(t, "Action", "activity_overview_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/app/activity-state, auth: required }
`),
		makeManifest(t, "Action", "get_subscription_action", "mobile", `
group: mobile
route: { method: GET, path: /api/v1/app/subscription, auth: required }
`),
	)
	// One screen wires subscription, the rest are orphans.
	reg.Screens = append(reg.Screens, makeManifest(t, "Screen", "userprofile_screen", "mobile", `
group: mobile
name: UserProfile
nav_type: folded
order: 0
sources:
  - { name: subscription, action: get_subscription }
sections: []
`))

	tree := Walk(reg, "")

	clusterNames := make(map[string]int)
	for _, c := range tree.FeatureClusters {
		clusterNames[c.Name] = len(c.Actions)
	}

	// `log_meal_action`     -> "meal"
	// `log_water_action`    -> "water"
	// `fasting_*`           -> "fasting" (2 actions)
	// `activity_overview`   -> "activity"
	// `get_subscription`    -> "subscription"
	for _, want := range []string{"meal", "water", "fasting", "activity", "subscription"} {
		if _, ok := clusterNames[want]; !ok {
			t.Errorf("expected feature cluster %q, got %v", want, clusterNames)
		}
	}
	if clusterNames["fasting"] != 2 {
		t.Errorf("expected fasting cluster to contain 2 actions, got %d", clusterNames["fasting"])
	}

	// Subscription cluster should NOT contain orphan actions and SHOULD
	// reference the userprofile_screen that uses it.
	for _, c := range tree.FeatureClusters {
		if c.Name != "subscription" {
			continue
		}
		if len(c.ScreensUsing) == 0 || c.ScreensUsing[0] != "userprofile_screen" {
			t.Errorf("subscription cluster should reference userprofile_screen, got %v", c.ScreensUsing)
		}
		if c.Actions[0].IsOrphan {
			t.Errorf("subscription action should not be marked orphan, got %+v", c.Actions[0])
		}
	}

	// Meal cluster MUST flag its action as orphan since no screen sources it.
	for _, c := range tree.FeatureClusters {
		if c.Name != "meal" {
			continue
		}
		if !c.Actions[0].IsOrphan {
			t.Errorf("meal action should be marked orphan, got %+v", c.Actions[0])
		}
	}
}

func TestDeriveFeatureName_StripsVerbAndNamespacePrefixes(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"log_meal_action", "meal"},
		{"log_water_action", "water"},
		{"get_subscription_action", "subscription"},
		{"fasting_overview_action", "fasting"},
		{"activity_overview_action", "activity"},
		{"mobile_actions_log_meal_action", "meal"},
		{"admin_actions_list_ai_config_action", "ai"},
		{"convert_guest_to_user_action", "guest"},
	}
	for _, tc := range cases {
		m := &manifest.Manifest{Metadata: manifest.Metadata{Name: tc.name}, Feature: "mobile"}
		if got := deriveFeatureName(m); got != tc.want {
			t.Errorf("deriveFeatureName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
