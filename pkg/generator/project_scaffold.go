package generator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/version"
)

func scaffoldDirectories(projectDir string, opts ProjectOptions) error {
	var dirs []string
	if opts.Layout == LayoutV2 {
		dirs = []string{
			"internal/features",
			"internal/platform",
			"db/migrations",
			"assets/i18n",
			"assets/emails",
			"config",
			"cmd/api",
			"cmd/worker",
			"tests",
			"gen/go",
			"gen/python",
			".bffx",
		}
	} else {
		dirs = []string{
			"bffx/resources",
			"bffx/builders",
			"bffx/actions",
			"bffx/addons",
			"hooks",
			"worker/skills",
			"worker/functions",
			"tests",
			"gen/go",
			"gen/python",
			"cmd/orchestrator",
			"i18n",
			"bffx/screens",
			".bffx",
		}
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(projectDir, d), 0o755); err != nil {
			return err
		}
	}
	verPath := filepath.Join(projectDir, ".bffx", "framework_version")
	if err := os.WriteFile(verPath, []byte(strings.TrimSpace(version.FrameworkVersion)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}
