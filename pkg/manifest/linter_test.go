package manifest

import (
	"strings"
	"testing"
)

func TestLintResource_TreeUnknownStrict(t *testing.T) {
	spec := &ResourceSpec{
		Tree: "not-a-valid-tree-keyword",
		Fields: []ResourceField{
			{Name: "x", Type: "string"},
		},
	}
	errs := lintResource("Note", spec, true)
	var found bool
	for _, e := range errs {
		if strings.Contains(strings.ToLower(e.Message), "unknown tree") {
			found = true
			if e.Severity != "error" {
				t.Errorf("expected error severity, got %q", e.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected unknown tree lint, got %#v", errs)
	}
}

func TestLintResource_TreeUnknownNonStrict(t *testing.T) {
	spec := &ResourceSpec{
		Tree: "weird",
		Fields: []ResourceField{
			{Name: "x", Type: "string"},
		},
	}
	errs := lintResource("Note", spec, false)
	var found bool
	for _, e := range errs {
		if strings.Contains(strings.ToLower(e.Message), "unknown tree") {
			found = true
			if e.Severity != "warning" {
				t.Errorf("expected warning severity, got %q", e.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected unknown tree lint, got %#v", errs)
	}
}

func TestLintAction_SensitiveNamePublicAuth(t *testing.T) {
	spec := &ActionSpec{}
	spec.Route.Auth = "public"

	// Sensitive names should trigger warnings
	for _, name := range []string{"delete_user", "UpdateProfile", "admin_panel"} {
		errs := lintAction(name, spec, false)
		if len(errs) != 1 || errs[0].Severity != "warning" {
			t.Errorf("expected 1 warning for action name %q, got %#v", name, errs)
		}
	}

	// Safe names should NOT trigger warnings
	for _, name := range []string{"get_weather", "display_banner", "ping"} {
		errs := lintAction(name, spec, false)
		if len(errs) != 0 {
			t.Errorf("expected no warning for safe action name %q, got %#v", name, errs)
		}
	}
}

func TestLintAction_SensitiveNamePublicAuthStrict(t *testing.T) {
	spec := &ActionSpec{}
	spec.Route.Auth = "public"

	errs := lintAction("delete_user", spec, true)
	if len(errs) != 1 || errs[0].Severity != "error" {
		t.Errorf("expected 1 error in strict mode, got %#v", errs)
	}
}

