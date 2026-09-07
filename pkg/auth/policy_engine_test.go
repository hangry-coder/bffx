package auth

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestPolicyEngine_Evaluate(t *testing.T) {
	e := NewPolicyEngine()
	
	userClaims := map[string]any{"sub": "user-1", "role": "user"}
	adminClaims := map[string]any{"sub": "admin-1", "role": "admin"}
	resource := map[string]any{"created_by": "user-1"}
	
	// 1. Basic String Rules
	assert.True(t, e.Evaluate("public", nil, nil), "public should allow nil claims")
	assert.True(t, e.Evaluate("authenticated", userClaims, nil), "authenticated should allow user claims")
	assert.False(t, e.Evaluate("authenticated", nil, nil), "authenticated should reject nil claims")
	
	// 2. Owner Rule
	assert.True(t, e.Evaluate("owner", userClaims, resource), "owner should match sub and created_by")
	assert.False(t, e.Evaluate("owner", map[string]any{"sub": "user-2"}, resource), "owner should reject different sub")
	assert.False(t, e.Evaluate("owner", userClaims, nil), "owner should reject nil resource")
	
	// 3. Admin Bypass
	assert.True(t, e.Evaluate("owner", adminClaims, map[string]any{"created_by": "other"}), "admin should bypass owner check")
	assert.True(t, e.Evaluate("pro", adminClaims, nil), "admin should bypass arbitrary role check")
	
	// 4. Entitlements
	proClaims := map[string]any{"sub": "u1", "entitlements": []string{"pro", "beta"}}
	assert.True(t, e.Evaluate("entitled:pro", proClaims, nil))
	assert.True(t, e.Evaluate("entitled:BETA", proClaims, nil), "entitlement check should be case-insensitive")
	assert.False(t, e.Evaluate("entitled:premium", proClaims, nil))

	// 5. Logical Operators (Map Rules)
	andRule := map[string]any{
		"and": []any{"authenticated", "owner"},
	}
	assert.True(t, e.Evaluate(andRule, userClaims, resource))
	assert.False(t, e.Evaluate(andRule, nil, resource))
	
	orRule := map[string]any{
		"or": []any{"admin", "owner"},
	}
	assert.True(t, e.Evaluate(orRule, userClaims, resource))
	assert.True(t, e.Evaluate(orRule, adminClaims, nil))
	
	unlessRule := map[string]any{
		"unless": "public",
	}
	assert.False(t, e.Evaluate(unlessRule, nil, nil), "unless public should fail when public passes")
	
	// 6. List Rules (Default OR)
	listRule := []any{"admin", "owner"}
	assert.True(t, e.Evaluate(listRule, userClaims, resource))
	assert.False(t, e.Evaluate(listRule, map[string]any{"sub": "other"}, resource))
}
