package providers

import (
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoffProvider(t *testing.T) {
	// Setup test data path
	wd, _ := os.Getwd()
	flagsPath := filepath.Join(wd, "testdata", "goff", "flags.yaml")

	p := NewGoffProvider()
	config := featureflags.ProviderConfig{
		Name: "goff",
		CustomConfig: map[string]interface{}{
			"flagsFile":       flagsPath,
			"pollingInterval": "5s",
		},
	}

	err := p.Init(config)
	require.NoError(t, err)
	defer p.Close()

	// Give it a moment to load the file
	time.Sleep(100 * time.Millisecond)

	t.Run("Evaluate Default Rule", func(t *testing.T) {
		ctx := featureflags.EvalContext{UserID: "user-1"}
		val, err := p.Evaluate("test-flag", ctx)
		require.NoError(t, err)
		assert.Equal(t, true, val)
	})

	t.Run("Evaluate Targeting Rule", func(t *testing.T) {
		ctx := featureflags.EvalContext{UserID: "admin-user"}
		val, err := p.Evaluate("test-flag", ctx)
		require.NoError(t, err)
		assert.Equal(t, true, val)
	})

	t.Run("EvaluateAll", func(t *testing.T) {
		ctx := featureflags.EvalContext{UserID: "user-1"}
		flags, err := p.EvaluateAll(ctx)
		require.NoError(t, err)
		assert.Contains(t, flags, "test-flag")
		assert.Equal(t, true, flags["test-flag"].Value)
	})

	t.Run("Non-existent Flag", func(t *testing.T) {
		ctx := featureflags.EvalContext{UserID: "user-1"}
		val, err := p.Evaluate("missing-flag", ctx)
		assert.Error(t, err)
		assert.Nil(t, val)
	})
}
