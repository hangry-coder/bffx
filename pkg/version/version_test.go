package version

import (
	"os"
	"testing"
)

func TestFrameworkModulePath(t *testing.T) {
	// Default canonical path
	_ = os.Unsetenv("BFFX_FRAMEWORK_MODULE")
	if got := FrameworkModulePath(); got != "github.com/hangry-coder/bffx" {
		t.Fatalf("expected github.com/hangry-coder/bffx, got %q", got)
	}

	// Overridden via env var
	t.Setenv("BFFX_FRAMEWORK_MODULE", "example.com/custom/bffx")
	if got := FrameworkModulePath(); got != "example.com/custom/bffx" {
		t.Fatalf("expected example.com/custom/bffx, got %q", got)
	}
}
