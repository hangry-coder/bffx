package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectWorkspaceRoot_BFFX_ROOT(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BFFX_ROOT", tmp)
	if got := DetectWorkspaceRoot(""); got != filepath.Clean(tmp) {
		t.Fatalf("DetectWorkspaceRoot: want %q got %q", tmp, got)
	}
}

func TestDetectWorkspaceRoot_invalidBFFX_ROOT(t *testing.T) {
	tmp := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BFFX_ROOT", filepath.Join(tmp, "not-a-workspace"))
	if got := DetectWorkspaceRoot(""); got != "" {
		t.Fatalf("expected empty when cwd has no pkg/ and BFFX_ROOT invalid, got %q", got)
	}
}

func TestCloud_unknownProvider(t *testing.T) {
	tmp := t.TempDir()
	err := Cloud(tmp, "unknown-provider-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHasPkgDir(t *testing.T) {
	tmp := t.TempDir()
	if hasPkgDir(tmp) {
		t.Fatal("expected false before mkdir pkg")
	}
	_ = os.MkdirAll(filepath.Join(tmp, "pkg"), 0o755)
	if !hasPkgDir(tmp) {
		t.Fatal("expected true after mkdir pkg")
	}
}

func TestDeployHistoryAndRollback(t *testing.T) {
	tmp := t.TempDir()

	// 1. Initial load from missing file should return empty history
	history, err := loadDeployHistory(tmp)
	if err != nil {
		t.Fatalf("loadDeployHistory failed: %v", err)
	}
	if len(history.Deploys) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(history.Deploys))
	}

	// 2. Resolve rollback target on empty history should fail
	_, err = resolveRollbackTarget(tmp)
	if err == nil {
		t.Fatal("expected error resolving rollback target with empty history")
	}

	// 3. Record one deployment
	err = recordDeploy(tmp, "sha-1", "image:sha-1")
	if err != nil {
		t.Fatalf("recordDeploy failed: %v", err)
	}

	// 4. Still only 1 entry, rollback should fail
	_, err = resolveRollbackTarget(tmp)
	if err == nil {
		t.Fatal("expected error resolving rollback target with only 1 entry")
	}

	// 5. Record second deployment
	err = recordDeploy(tmp, "sha-2", "image:sha-2")
	if err != nil {
		t.Fatalf("recordDeploy failed: %v", err)
	}

	// 6. Now rollback should succeed and target the first deploy (sha-1)
	target, err := resolveRollbackTarget(tmp)
	if err != nil {
		t.Fatalf("resolveRollbackTarget failed: %v", err)
	}
	if target.SHA != "sha-1" {
		t.Fatalf("expected rollback target sha-1, got %s", target.SHA)
	}

	// 7. Verify history got shrunk back to 1 entry (sha-1)
	history, err = loadDeployHistory(tmp)
	if err != nil {
		t.Fatalf("loadDeployHistory failed: %v", err)
	}
	if len(history.Deploys) != 1 || history.Deploys[0].SHA != "sha-1" {
		t.Fatalf("expected history to contain only sha-1, got: %v", history.Deploys)
	}

	// 8. Test ring buffer limit (keep last 10)
	for i := 1; i <= 12; i++ {
		tag := fmt.Sprintf("sha-%d", i)
		_ = recordDeploy(tmp, tag, "image:"+tag)
	}

	history, err = loadDeployHistory(tmp)
	if err != nil {
		t.Fatalf("loadDeployHistory failed: %v", err)
	}
	if len(history.Deploys) != 10 {
		t.Fatalf("expected history to be capped at 10, got %d", len(history.Deploys))
	}
}

