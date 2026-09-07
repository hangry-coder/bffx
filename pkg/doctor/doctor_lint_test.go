package doctor

import (
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestLintObservabilityContractCanonicalProvider(t *testing.T) {
	got := lintObservabilityContract(&manifest.ProjectSpec{
		Batteries: manifest.BatteriesConfig{Observability: "slog"},
	})
	if len(got) == 0 || got[0].Status != "ok" {
		t.Fatalf("expected ok for slog provider, got %#v", got)
	}
}

func TestLintObservabilityContractRejectsUnknownProvider(t *testing.T) {
	got := lintObservabilityContract(&manifest.ProjectSpec{
		Batteries: manifest.BatteriesConfig{Observability: "datadog"},
	})
	if len(got) == 0 || got[0].Status != "fail" {
		t.Fatalf("expected fail for datadog provider, got %#v", got)
	}
}

func TestLintExtensionTaxonomyRejectsUnknownFlagProvider(t *testing.T) {
	var m manifest.Manifest
	if err := yaml.Unmarshal([]byte(`apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: test
spec:
  featureFlags:
    provider: unknown-vendor
`), &m); err != nil {
		t.Fatal(err)
	}
	reg := &manifest.Registry{Project: &m}
	got := lintExtensionTaxonomy(reg)
	var sawFail bool
	for _, r := range got {
		if r.Name == "Taxonomy: featureflags provider" && r.Status == "fail" {
			sawFail = true
		}
	}
	if !sawFail {
		t.Fatalf("expected fail for unknown flag provider, got %#v", got)
	}
}

func TestLintAdminManifests(t *testing.T) {
	// 1. Create a backing resource
	var backingNode yaml.Node
	yaml.Unmarshal([]byte(`
fields:
  - { name: email, type: string }
  - { name: active, type: bool }
`), &backingNode)
	if backingNode.Kind == yaml.DocumentNode && len(backingNode.Content) > 0 {
		backingNode = *backingNode.Content[0]
	}
	userRes := &manifest.Manifest{
		Kind:     "Resource",
		Metadata: manifest.Metadata{Name: "user"},
		Spec:     backingNode,
	}

	// 2. Create a valid admin resource manifest
	var validNode yaml.Node
	yaml.Unmarshal([]byte(`
resource: user
index:
  columns: [email, active]
  filters:
    - field: email
      as: string
  scopes:
    - name: active_users
      where:
        active: true
`), &validNode)
	if validNode.Kind == yaml.DocumentNode && len(validNode.Content) > 0 {
		validNode = *validNode.Content[0]
	}
	validAdmin := &manifest.Manifest{
		Kind:     "AdminResource",
		Metadata: manifest.Metadata{Name: "valid_admin"},
		Spec:     validNode,
	}

	// 3. Create an invalid admin resource manifest
	var invalidNode yaml.Node
	yaml.Unmarshal([]byte(`
resource: user
index:
  columns: [email, non_existent_col]
  filters:
    - field: non_existent_filter
      as: string
  scopes:
    - name: active_users
      where:
        non_existent_scope_field: true
`), &invalidNode)
	if invalidNode.Kind == yaml.DocumentNode && len(invalidNode.Content) > 0 {
		invalidNode = *invalidNode.Content[0]
	}
	invalidAdmin := &manifest.Manifest{
		Kind:     "AdminResource",
		Metadata: manifest.Metadata{Name: "invalid_admin"},
		Spec:     invalidNode,
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{userRes},
		AdminResources: []*manifest.Manifest{
			validAdmin,
			invalidAdmin,
		},
	}

	got := lintAdminManifests(reg)

	// Validate validAdmin has no failures, invalidAdmin has failures
	var invalidColFails, invalidFilterFails, invalidScopeFails int
	for _, r := range got {
		if r.Status == "fail" {
			if strings.Contains(r.Message, "index column \"non_existent_col\" does not exist") {
				invalidColFails++
			}
			if strings.Contains(r.Message, "filter field \"non_existent_filter\" does not exist") {
				invalidFilterFails++
			}
			if strings.Contains(r.Message, "where-clause field \"non_existent_scope_field\" does not exist") {
				invalidScopeFails++
			}
			// Check that valid resource field references did not fail
			if strings.Contains(r.Message, "email") || strings.Contains(r.Message, "active") {
				if !strings.Contains(r.Message, "non_existent_") {
					t.Errorf("unexpected failure for valid field: %s", r.Message)
				}
			}
		}
	}

	if invalidColFails != 1 {
		t.Errorf("expected 1 column failure, got %d", invalidColFails)
	}
	if invalidFilterFails != 1 {
		t.Errorf("expected 1 filter failure, got %d", invalidFilterFails)
	}
	if invalidScopeFails != 1 {
		t.Errorf("expected 1 scope failure, got %d", invalidScopeFails)
	}
}
