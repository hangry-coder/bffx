package golden

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/handlers"
	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// Regenerate with: go test ./tests/golden -run TestGoldenAdminGraph -update-admin-golden
var updateAdminGolden = flag.Bool("update-admin-golden", false, "rewrite tests/golden/testdata/notes/admin-graph.gen.golden from examples/notes")

func TestGoldenAdminGraph(t *testing.T) {
	repo := bffxModuleRoot(t)
	root, err := os.MkdirTemp("", "bffx-admin-golden-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	relReplace, err := filepath.Rel(root, repo)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	goMod := fmt.Sprintf("module bffx/examples/notes\n\ngo 1.26\n\nrequire github.com/hangry-coder/bffx v0.0.0\n\nreplace github.com/hangry-coder/bffx => %s\n", relReplace)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	copyDir(t, filepath.Join(repo, "tests", "golden", "fixtures", "notes", "bffx"), filepath.Join(root, "bffx"))
	copyDir(t, filepath.Join(repo, "tests", "golden", "fixtures", "notes", "hooks"), filepath.Join(root, "hooks"))

	// Run Sync to compile admin manifests and write .bffx/admin-graph.json
	if _, err := compiler.Sync(root, false); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Load registry for handler
	reg, err := manifest.LoadAll(root)
	if err != nil {
		t.Fatalf("load registry failed: %v", err)
	}

	gotPath := filepath.Join(root, ".bffx", "admin-graph.json")
	gotBytes, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated admin-graph.json: %v", err)
	}

	expPath := filepath.Join(repo, "tests", "golden", "testdata", "notes", "admin-graph.gen.golden")
	if *updateAdminGolden {
		if err := os.WriteFile(expPath, gotBytes, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", expPath)
		return
	}

	wantBytes, err := os.ReadFile(expPath)
	if err != nil {
		t.Fatalf("read expected admin-graph: %v", err)
	}

	// Compare structured JSON to avoid formatting drift
	var gotGraph, wantGraph manifest.AdminGraph
	if err := json.Unmarshal(gotBytes, &gotGraph); err != nil {
		t.Fatalf("unmarshal got bytes: %v", err)
	}
	if err := json.Unmarshal(wantBytes, &wantGraph); err != nil {
		t.Fatalf("unmarshal want bytes: %v", err)
	}

	gotJSON, _ := json.MarshalIndent(gotGraph, "", "  ")
	wantJSON, _ := json.MarshalIndent(wantGraph, "", "  ")

	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("admin-graph.json drift (re-run with -update-admin-golden after intentional manifest compiler changes):\n--- want\n+++ got\n%s", diffLines(string(wantJSON), string(gotJSON)))
	}

	// 2. Validate API config route GetConfig
	h := handlers.NewResourceHandler(storage.NewMemoryStore(), storage.NewMemoryStore(), reg, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	h.GetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected GetConfig status 200, got %d", w.Code)
	}

	var apiGraph manifest.AdminGraph
	if err := json.Unmarshal(w.Body.Bytes(), &apiGraph); err != nil {
		t.Fatalf("unmarshal API GetConfig response: %v", err)
	}

	apiJSON, _ := json.MarshalIndent(apiGraph, "", "  ")
	if string(apiJSON) != string(gotJSON) {
		t.Errorf("API config endpoint returned different graph compared to compiled file:\n--- compiled\n+++ API\n%s", diffLines(string(gotJSON), string(apiJSON)))
	}
}
