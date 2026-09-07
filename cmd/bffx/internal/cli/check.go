package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func HandleCheck(args []string) {
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
		log.Fatalf("failed to load manifests: %v", err)
	}

	fmt.Println("--- Manifest Validation ---")
	valErrors := reg.Validate()
	if len(valErrors) == 0 {
		fmt.Println("✅ All manifests are valid.")
	} else {
		for _, e := range valErrors {
			fmt.Printf("❌ %s\n", e)
		}
	}

	fmt.Println("\n--- Security Linting ---")
	var lintErrors []manifest.LinterError
	if strict {
		lintErrors = manifest.LintRegistryWithOptions(reg, true)
	} else {
		lintErrors = manifest.LintRegistry(reg)
	}
	lintHasErrors := false
	if len(lintErrors) == 0 {
		fmt.Println("✅ No security issues found.")
	} else {
		for _, e := range lintErrors {
			fmt.Println(e.Error())
			if e.Severity == "error" {
				lintHasErrors = true
			}
		}
	}

	if len(valErrors) > 0 {
		os.Exit(1)
	}
	if strict && lintHasErrors {
		os.Exit(1)
	}
}
