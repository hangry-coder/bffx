package docsbundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// EnsureBundled copies framework docs into .bffx/docs/ when the index is missing or empty.
// frameworkRoot is resolved from BFFX_ROOT or project .bffx/framework_root.
func EnsureBundled(projectRoot string) error {
	idxPath := filepath.Join(ProjectDocsDir(projectRoot), "index.json")
	if data, err := os.ReadFile(idxPath); err == nil {
		var idx Index
		if json.Unmarshal(data, &idx) == nil && len(idx.Entries) > 0 {
			return nil
		}
	}

	fw := detectFrameworkRoot(projectRoot)
	if fw == "" {
		return nil
	}
	return Bundle(projectRoot, fw)
}

func detectFrameworkRoot(projectRoot string) string {
	if w := strings.TrimSpace(os.Getenv("BFFX_ROOT")); w != "" && hasPkgDir(w) {
		return filepath.Clean(w)
	}
	if projectRoot != "" {
		if b, err := os.ReadFile(filepath.Join(projectRoot, ".bffx", "framework_root")); err == nil {
			line := strings.TrimSpace(strings.Split(string(b), "\n")[0])
			if line != "" && hasPkgDir(line) {
				return filepath.Clean(line)
			}
		}
	}
	return ""
}

func hasPkgDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "pkg"))
	return err == nil && st.IsDir()
}
