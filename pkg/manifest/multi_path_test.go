package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_LoadAll_MultiPath(t *testing.T) {
	tmp := t.TempDir()

	// 1. Setup legacy: bffx/auth/login.yaml
	legacyDir := filepath.Join(tmp, "bffx", "auth")
	require.NoError(t, os.MkdirAll(legacyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "login.yaml"), []byte("kind: Action"), 0644))

	// 2. Setup v2: internal/features/billing/manifests/invoice.yaml
	v2Dir := filepath.Join(tmp, "internal", "features", "billing", "manifests")
	require.NoError(t, os.MkdirAll(v2Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2Dir, "invoice.yaml"), []byte("kind: Action"), 0644))

	// 3. Setup v2 nested: internal/features/billing/manifests/stripe/webhook.yaml
	v2NestedDir := filepath.Join(tmp, "internal", "features", "billing", "manifests", "stripe")
	require.NoError(t, os.MkdirAll(v2NestedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2NestedDir, "webhook.yaml"), []byte("kind: Action"), 0644))

	// 4. Setup v2 actions: internal/features/auth/actions/verify.yaml
	v2ActionDir := filepath.Join(tmp, "internal", "features", "auth", "actions")
	require.NoError(t, os.MkdirAll(v2ActionDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2ActionDir, "verify.yaml"), []byte("kind: Action"), 0644))

	// 5. Setup v2 screens: internal/features/auth/screens/login.yaml
	v2ScreenDir := filepath.Join(tmp, "internal", "features", "auth", "screens")
	require.NoError(t, os.MkdirAll(v2ScreenDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2ScreenDir, "login.yaml"), []byte("kind: Screen"), 0644))

	// 6. Setup v2 seeds: internal/features/movement/seeds/protocols.yaml
	v2SeedsDir := filepath.Join(tmp, "internal", "features", "movement", "seeds")
	require.NoError(t, os.MkdirAll(v2SeedsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2SeedsDir, "protocols.yaml"), []byte(`kind: Seed
metadata:
  name: MovementProtocolsSeed
spec:
  table: Routine
  rows: []
`), 0644))

	// 7. Setup v2 invalid (not in allowed folder): internal/features/billing/other.yaml
	v2InvalidDir := filepath.Join(tmp, "internal", "features", "billing")
	require.NoError(t, os.WriteFile(filepath.Join(v2InvalidDir, "other.yaml"), []byte("kind: Action"), 0644))

	reg, err := LoadAll(tmp)
	require.NoError(t, err)

	// Verify implicit names
	actionNames := make(map[string]bool)
	for _, a := range reg.Actions {
		actionNames[a.Metadata.Name] = true
	}
	screenNames := make(map[string]bool)
	for _, s := range reg.Screens {
		screenNames[s.Metadata.Name] = true
	}

	assert.True(t, actionNames["auth_login"], "Should have auth_login from legacy")
	assert.True(t, actionNames["billing_invoice"], "Should have billing_invoice from v2 (stripped manifests/)")
	assert.True(t, actionNames["billing_stripe_webhook"], "Should have billing_stripe_webhook from v2 nested")
	assert.True(t, actionNames["auth_actions_verify"], "Should have auth_actions_verify from v2 actions")
	assert.True(t, screenNames["auth_screens_login"], "Should have auth_screens_login from v2 screens")
	
	assert.False(t, actionNames["billing_other"], "Should NOT have billing_other (not in allowed folder)")

	seedNames := make(map[string]bool)
	for _, s := range reg.Seeds {
		seedNames[s.Metadata.Name] = true
	}
	assert.True(t, seedNames["MovementProtocolsSeed"], "Should load Seed from internal/features/*/seeds/")
	assert.False(t, actionNames["billing_manifests_invoice"], "Should have stripped 'manifests' from name")
}

func TestRegistry_LoadAll_Collision(t *testing.T) {
	tmp := t.TempDir()

	// Legacy login
	legacyDir := filepath.Join(tmp, "bffx", "auth")
	require.NoError(t, os.MkdirAll(legacyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "login.yaml"), []byte("kind: Action\nmetadata:\n  name: auth_login"), 0644))

	// V2 login (collision on name)
	v2Dir := filepath.Join(tmp, "internal", "features", "auth", "manifests")
	require.NoError(t, os.MkdirAll(v2Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(v2Dir, "login_v2.yaml"), []byte("kind: Action\nmetadata:\n  name: auth_login"), 0644))

	_, err := LoadAll(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate manifest found for Action:auth_login")
}
