package manifest

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProjectSpec_PackagingMode(t *testing.T) {
	data := []byte(`apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: app
spec:
  packaging:
    mode: minimal
`)
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	var spec ProjectSpec
	if err := m.UnmarshalSpec(&spec); err != nil {
		t.Fatal(err)
	}
	if spec.Packaging.Mode != "minimal" {
		t.Fatalf("mode=%q", spec.Packaging.Mode)
	}
}
