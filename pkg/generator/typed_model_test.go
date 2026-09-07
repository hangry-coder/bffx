package generator

import (
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

func TestRenderTypedResource_UserShape(t *testing.T) {
	var spec manifest.ResourceSpec
	raw := `fields:
  - {name: email, type: string, required: true}
  - {name: age, type: int}
`
	if err := yaml.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatal(err)
	}
	out, err := renderTypedResource("User", &spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "type User struct") {
		t.Fatalf("missing struct: %s", out)
	}
	if !strings.Contains(out, "Email string") || !strings.Contains(out, "Age int") {
		t.Fatalf("missing fields: %s", out)
	}
	if !strings.Contains(out, "func (m *User) ToMap()") {
		t.Fatal("missing ToMap")
	}
	if !strings.Contains(out, "func (m *User) FromMap(") {
		t.Fatal("missing FromMap")
	}
}
