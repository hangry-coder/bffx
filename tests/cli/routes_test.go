package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// routesFixtureRoot is always committed (unlike pilot_project/, which is gitignored).
func routesFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "examples", "notes")
	p := filepath.Join(root, "bffx", "project.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("routes fixture missing %s (required for CI): %v", p, err)
	}
	return root
}

func TestRoutesListJSON(t *testing.T) {
	tmpDir := t.TempDir()
	bffxBin := buildCLIBinary(t, tmpDir)

	cmd := exec.Command(bffxBin, "routes", "list", "--root", routesFixtureRoot(t), "--json", "--filter", "action")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("routes list: %v\n%s", err, out)
	}
	var routes []map[string]any
	if err := json.Unmarshal(out, &routes); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(routes) < 1 {
		t.Fatalf("expected at least one route, got %d", len(routes))
	}
	found := false
	for _, r := range routes {
		if r["kind"] == "Action" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one Action route")
	}
}

func TestRoutesDefaultList(t *testing.T) {
	tmpDir := t.TempDir()
	bffxBin := buildCLIBinary(t, tmpDir)
	cmd := exec.Command(bffxBin, "routes", "--root", routesFixtureRoot(t))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("routes: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "METHOD") {
		t.Fatalf("expected table header, got: %s", out)
	}
}

func buildCLIBinary(t *testing.T, tmpDir string) string {
	t.Helper()
	bffxBin := filepath.Join(tmpDir, "bffx")
	cmdBuild := exec.Command("go", "build", "-o", bffxBin, "../../cmd/bffx")
	if output, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("build bffx: %v\n%s", err, output)
	}
	return bffxBin
}
