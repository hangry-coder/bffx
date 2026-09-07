package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	bffxCLIOnce     sync.Once
	bffxCLIResolved string
	bffxCLIErr      error
)

// bffxModuleRoot finds the repo root (directory containing go.mod with `module bffx`) by walking up from cwd.
func bffxModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	start := dir
	for range 24 {
		mod := filepath.Join(dir, "go.mod")
		if b, err := os.ReadFile(mod); err == nil && strings.Contains(string(b), "module github.com/hangry-coder/bffx") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("bffx module root not found (started from %s)", start)
}

// bffxCLIPath returns a path to a `bffx` binary built with `go build ./cmd/bffx`, cached for the process.
// E2E must not assume a preinstalled `./bffx` at the repo root.
func bffxCLIPath() (string, error) {
	bffxCLIOnce.Do(func() {
		root, err := bffxModuleRoot()
		if err != nil {
			bffxCLIErr = err
			return
		}
		out := filepath.Join(os.TempDir(), fmt.Sprintf("bffx-e2e-%d", os.Getpid()))
		if runtime.GOOS == "windows" {
			out += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", out, "./cmd/bffx")
		cmd.Dir = root
		combined, err := cmd.CombinedOutput()
		if err != nil {
			bffxCLIErr = fmt.Errorf("go build -o %s ./cmd/bffx (dir=%s): %w\n%s", out, root, err, combined)
			return
		}
		bffxCLIResolved = out
	})
	return bffxCLIResolved, bffxCLIErr
}
