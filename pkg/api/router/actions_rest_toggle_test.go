package router

import (
	"testing"
)

func TestActionRESTEnabled_ManifestOverrideWins(t *testing.T) {
	t.Setenv("BFFX_ACTIONS_REST_ENABLED", "false")
	tt := []struct {
		name string
		rest *bool
		want bool
	}{
		{"manifest=nil + env=false → false", nil, false},
		{"manifest=true overrides env=false", boolPtr(true), true},
		{"manifest=false matches env", boolPtr(false), false},
	}
	for _, c := range tt {
		t.Run(c.name, func(t *testing.T) {
			if got := actionRESTEnabled(c.rest); got != c.want {
				t.Fatalf("actionRESTEnabled(%v) = %v, want %v", c.rest, got, c.want)
			}
		})
	}
}

func TestActionRESTEnabled_DefaultTrue(t *testing.T) {
	t.Setenv("BFFX_ACTIONS_REST_ENABLED", "")
	if !actionRESTEnabled(nil) {
		t.Fatal("default should be true when env unset and no manifest override")
	}
}

func TestActionRESTEnabled_EnvVariants(t *testing.T) {
	for _, v := range []string{"false", "FALSE", "0", "no", "off"} {
		t.Setenv("BFFX_ACTIONS_REST_ENABLED", v)
		if actionRESTEnabled(nil) {
			t.Fatalf("env=%q should disable REST", v)
		}
	}
	for _, v := range []string{"true", "1", "yes"} {
		t.Setenv("BFFX_ACTIONS_REST_ENABLED", v)
		if !actionRESTEnabled(nil) {
			t.Fatalf("env=%q should NOT disable REST", v)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
