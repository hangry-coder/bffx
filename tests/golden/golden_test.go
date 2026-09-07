package golden

import (
	"github.com/hangry-coder/bffx/pkg/compiler"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regenerate with: go test ./tests/golden -run TestGoldenNotesRegistry -update-notes-golden
var updateNotesGolden = flag.Bool("update-notes-golden", false, "rewrite tests/golden/testdata/notes/registry.gen.golden from examples/notes")

func bffxModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(b), "module github.com/hangry-coder/bffx") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate bffx module root (module github.com/hangry-coder/bffx) from cwd")
	return ""
}

func TestGoldenMinimal(t *testing.T) {
	root, err := os.MkdirTemp("", "bffx-golden-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	repo := bffxModuleRoot(t)
	relReplace, err := filepath.Rel(root, repo)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	goMod := fmt.Sprintf("module goldenminimal\n\ngo 1.26\n\nrequire github.com/hangry-coder/bffx v0.0.0\n\nreplace github.com/hangry-coder/bffx => %s\n", relReplace)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	bffxDir := filepath.Join(root, "bffx")
	os.MkdirAll(filepath.Join(bffxDir, "resources"), 0o755)

	projectYaml, _ := os.ReadFile("fixtures/minimal/project.yaml")
	os.WriteFile(filepath.Join(bffxDir, "project.yaml"), projectYaml, 0o644)

	taskYaml, _ := os.ReadFile("fixtures/minimal/task.yaml")
	os.WriteFile(filepath.Join(bffxDir, "resources/task.yaml"), taskYaml, 0o644)

	_, err = compiler.Sync(root, false)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	for _, f := range []string{
		filepath.Join(".bffx", "graph.json"),
		filepath.Join("cmd", "orchestrator", "registry.gen.go"),
	} {
		p := filepath.Join(root, f)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected artifact missing %s: %v", f, err)
		}
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

func TestGoldenNotesRegistry(t *testing.T) {
	repo := bffxModuleRoot(t)
	root, err := os.MkdirTemp("", "bffx-notes-golden-*")
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

	if _, err := compiler.Sync(root, false); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	gotPath := filepath.Join(root, "cmd", "orchestrator", "registry.gen.go")
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated registry: %v", err)
	}
	expPath := filepath.Join(repo, "tests", "golden", "testdata", "notes", "registry.gen.golden")
	if *updateNotesGolden {
		if err := os.WriteFile(expPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", expPath)
		return
	}
	want, err := os.ReadFile(expPath)
	if err != nil {
		t.Fatalf("read expected registry: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("registry.gen.go drift (re-run with -update-notes-golden after intentional compiler changes):\n--- want\n+++ got\n%s", diffLines(string(want), string(got)))
	}
}

func diffLines(a, b string) string {
	aLines := strings.Split(strings.TrimSpace(a), "\n")
	bLines := strings.Split(strings.TrimSpace(b), "\n")
	max := len(aLines)
	if len(bLines) > max {
		max = len(bLines)
	}
	var sb strings.Builder
	for i := 0; i < max; i++ {
		al, bl := "", ""
		if i < len(aLines) {
			al = aLines[i]
		}
		if i < len(bLines) {
			bl = bLines[i]
		}
		if al != bl {
			fmt.Fprintf(&sb, "%4d - %q\n%4d + %q\n", i+1, al, i+1, bl)
		}
	}
	return sb.String()
}
