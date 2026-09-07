package api_test

import (
	"testing"

	"github.com/hangry-coder/bffx/pkg/auth"
)

// TestPolicyEngine_Evaluate covers the core PolicyEngine surface used by
// router.withPolicy: string keywords (public, authenticated, owner, admin),
// dynamic role/entitlement checks, and AND/OR/UNLESS composition.
func TestPolicyEngine_Evaluate(t *testing.T) {
	engine := auth.NewPolicyEngine()

	cases := []struct {
		name     string
		rule     any
		claims   map[string]any
		resource map[string]any
		want     bool
	}{
		{
			name: "public always allowed",
			rule: "public",
			want: true,
		},
		{
			name:   "authenticated requires claims",
			rule:   "authenticated",
			claims: map[string]any{"sub": "u1"},
			want:   true,
		},
		{
			name: "authenticated rejects nil claims",
			rule: "authenticated",
			want: false,
		},
		{
			name:     "owner allows when sub matches created_by",
			rule:     "owner",
			claims:   map[string]any{"sub": "u1"},
			resource: map[string]any{"created_by": "u1"},
			want:     true,
		},
		{
			name:     "owner denies when sub differs",
			rule:     "owner",
			claims:   map[string]any{"sub": "u1"},
			resource: map[string]any{"created_by": "u2"},
			want:     false,
		},
		{
			name:     "owner denies when claims missing",
			rule:     "owner",
			resource: map[string]any{"created_by": "u1"},
			want:     false,
		},
		{
			name:   "admin allows admin role",
			rule:   "admin",
			claims: map[string]any{"role": "admin"},
			want:   true,
		},
		{
			name:   "admin denies user role",
			rule:   "admin",
			claims: map[string]any{"role": "user"},
			want:   false,
		},
		{
			name:   "entitled:pro allows when slug present",
			rule:   "entitled:pro",
			claims: map[string]any{"entitlements": []string{"pro", "beta"}},
			want:   true,
		},
		{
			name:   "entitled:pro denies when slug missing",
			rule:   "entitled:pro",
			claims: map[string]any{"entitlements": []string{"free"}},
			want:   false,
		},
		{
			name:     "OR list: any match returns true",
			rule:     []any{"admin", "owner"},
			claims:   map[string]any{"sub": "u1", "role": "user"},
			resource: map[string]any{"created_by": "u1"},
			want:     true,
		},
		{
			name:   "OR list: no match returns false",
			rule:   []any{"admin", "entitled:pro"},
			claims: map[string]any{"role": "user"},
			want:   false,
		},
		{
			name: "AND map: all must pass",
			rule: map[string]any{
				"and": []any{"authenticated", "admin"},
			},
			claims: map[string]any{"role": "admin"},
			want:   true,
		},
		{
			name: "AND map: one failure rejects",
			rule: map[string]any{
				"and": []any{"authenticated", "admin"},
			},
			claims: map[string]any{"role": "user"},
			want:   false,
		},
		{
			name: "UNLESS map: passes when sub-rule fails",
			rule: map[string]any{
				"unless": "admin",
			},
			claims: map[string]any{"role": "user"},
			want:   true,
		},
		{
			name: "UNLESS map: rejects when sub-rule passes",
			rule: map[string]any{
				"unless": "admin",
			},
			claims: map[string]any{"role": "admin"},
			want:   false,
		},
		{
			name: "nil rule denies",
			rule: nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.Evaluate(tc.rule, tc.claims, tc.resource)
			if got != tc.want {
				t.Errorf("Evaluate(%v, claims=%v, resource=%v) = %v, want %v",
					tc.rule, tc.claims, tc.resource, got, tc.want)
			}
		})
	}
}
