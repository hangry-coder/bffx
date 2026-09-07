package validation

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"testing"
)

func TestValidator(t *testing.T) {
	v := NewValidator()
	spec := &manifest.ResourceSpec{
		Fields: []manifest.ResourceField{
			{Name: "title", Type: "string", Required: true},
			{Name: "count", Type: "int"},
		},
	}

	// 1. Valid
	err := v.Validate(spec, map[string]any{"title": "hello", "count": 10}, false)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// 2. Missing required
	err = v.Validate(spec, map[string]any{"count": 10}, false)
	if err == nil {
		t.Errorf("expected field required error")
	}

	// 3. Wrong type
	err = v.Validate(spec, map[string]any{"title": "hello", "count": "not-int"}, false)
	if err == nil {
		t.Errorf("expected type mismatch error")
	}
}

func TestAllowedPayload(t *testing.T) {
	spec := &manifest.ResourceSpec{
		Fields: []manifest.ResourceField{
			{Name: "title", Type: "string"},
			{Name: "count", Type: "int"},
		},
	}
	in := map[string]any{"title": "x", "count": float64(1), "evil": "y"}
	out := AllowedPayload(spec, in)
	if _, ok := out["evil"]; ok {
		t.Fatalf("unknown field should be stripped")
	}
	if out["title"] != "x" {
		t.Fatalf("expected title preserved")
	}
}
