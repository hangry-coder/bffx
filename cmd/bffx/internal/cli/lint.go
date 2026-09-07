package cli

import (
	"fmt"
	"os"

	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func HandleLint(args []string) {
	root := "."
	strict := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--strict" {
			strict = true
			continue
		}
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		logger.Fatal("Failed to load manifests: %v", err)
	}

	errs := manifest.LintRegistryWithOptions(reg, strict)
	if len(errs) == 0 {
		fmt.Println("✅ Manifests are clean.")
		return
	}

	hasErrors := false
	for _, e := range errs {
		fmt.Println(e.Error())
		if e.Severity == "error" {
			hasErrors = true
		}
	}
	if hasErrors {
		os.Exit(1)
	}
}
