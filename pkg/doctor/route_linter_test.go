package doctor

import (
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestLintRoutePrefixes(t *testing.T) {
	reg := &manifest.Registry{}

	// 1. Mock Project with custom prefix
	var projectNode yaml.Node
	yaml.Unmarshal([]byte("app:\n  apiPrefix: /api/v2/"), &projectNode)
	// yaml.Unmarshal unmarshals into the Document node, we want the first content node
	reg.Project = &manifest.Manifest{
		Kind: "Project",
		Spec: *projectNode.Content[0],
	}
	reg.ApiPrefix = "/api/v2"

	// 2. Action with bad prefix
	var actionBadSpec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: POST\n  path: /api/v1/bad"), &actionBadSpec)
	actionBad := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "BadAction"},
		Spec:     *actionBadSpec.Content[0],
	}
	reg.Actions = append(reg.Actions, actionBad)

	// 3. Action with good prefix
	var actionGoodSpec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: POST\n  path: /api/v2/good"), &actionGoodSpec)
	actionGood := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "GoodAction"},
		Spec:     *actionGoodSpec.Content[0],
	}
	reg.Actions = append(reg.Actions, actionGood)

	// 4. Duplicate routes
	var dup1Spec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: GET\n  path: /api/v2/dup"), &dup1Spec)
	dup1 := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "Dup1"},
		Spec:     *dup1Spec.Content[0],
	}
	reg.Actions = append(reg.Actions, dup1)

	var dup2Spec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: GET\n  path: /api/v2/dup"), &dup2Spec)
	dup2 := &manifest.Manifest{
		Kind:     "Builder",
		Metadata: manifest.Metadata{Name: "Dup2"},
		Spec:     *dup2Spec.Content[0],
	}
	reg.Builders = append(reg.Builders, dup2)

	// 5. Implicit Screen Path
	screenImplicit := &manifest.Manifest{
		Kind:     "Screen",
		Metadata: manifest.Metadata{Name: "ImplicitScreen"},
		Path:     "pkg/features/foo/screens/bar.yaml",
	}
	// spec.route.path is empty
	reg.Screens = append(reg.Screens, screenImplicit)

	results := lintRoutePrefixes(".", reg)

	var sawPrefixWarn, sawDuplicateFail, sawImplicitHint bool

	for _, res := range results {
		switch res.Name {
		case "Route Prefix Lint":
			if res.Status == "warn" {
				sawPrefixWarn = true
			}
		case "Route Duplicate Lint":
			if res.Status == "fail" {
				sawDuplicateFail = true
			}
		case "Screen Path Hint":
			if res.Status == "warn" {
				sawImplicitHint = true
			}
		}
	}

	if !sawPrefixWarn {
		t.Error("Expected prefix warning for /api/v1/bad when prefix is /api/v2/")
	}
	if !sawDuplicateFail {
		t.Error("Expected duplicate failure for /api/v2/dup")
	}
	if !sawImplicitHint {
		t.Error("Expected implicit path hint for screen")
	}
}

func TestLintCacheTTLOnActions(t *testing.T) {
	reg := &manifest.Registry{}

	// 1. GET action with cache_ttl -> should WARN
	var getActionSpec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: GET\n  path: /api/v2/cached-get\n  cache_ttl: 60"), &getActionSpec)
	getAction := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "CachedGetAction"},
		Spec:     *getActionSpec.Content[0],
	}
	reg.Actions = append(reg.Actions, getAction)

	// 2. POST action with cache_ttl -> should FAIL
	var postActionSpec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: POST\n  path: /api/v2/cached-post\n  cache_ttl: 120"), &postActionSpec)
	postAction := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "CachedPostAction"},
		Spec:     *postActionSpec.Content[0],
	}
	reg.Actions = append(reg.Actions, postAction)

	// 3. Normal action with no cache_ttl -> should pass cleanly
	var normalActionSpec yaml.Node
	yaml.Unmarshal([]byte("route:\n  method: GET\n  path: /api/v2/normal"), &normalActionSpec)
	normalAction := &manifest.Manifest{
		Kind:     "Action",
		Metadata: manifest.Metadata{Name: "NormalAction"},
		Spec:     *normalActionSpec.Content[0],
	}
	reg.Actions = append(reg.Actions, normalAction)

	results := lintCacheTTLOnActions(reg)

	var sawGetWarn, sawPostFail bool
	for _, res := range results {
		if res.Name == "Action Cache Safety" {
			if res.Status == "warn" && strings.Contains(res.Message, "CachedGetAction") {
				sawGetWarn = true
			}
			if res.Status == "fail" && strings.Contains(res.Message, "CachedPostAction") {
				sawPostFail = true
			}
		}
	}

	if !sawGetWarn {
		t.Error("expected a warn diagnostic for GET action with cache_ttl")
	}
	if !sawPostFail {
		t.Error("expected a fail diagnostic for POST action with cache_ttl")
	}
}
