package cli

import (
	"fmt"
	"log"
	"strings"

	"github.com/hangry-coder/bffx/pkg/generator"
)

func HandleUpdate(args []string) {
	if len(args) > 0 && args[0] == "framework" {
		root := "."
		vendorOnly := false
		dryRun := false
		force := false
		rest := args[1:]
		for i := 0; i < len(rest); i++ {
			switch {
			case rest[i] == "--root" && i+1 < len(rest):
				root = rest[i+1]
				i++
			case rest[i] == "--vendor-only":
				vendorOnly = true
			case rest[i] == "--dry-run":
				dryRun = true
			case rest[i] == "--force":
				force = true
			case rest[i] != "" && !strings.HasPrefix(rest[i], "-"):
				root = rest[i]
			}
		}
		opts := &generator.FrameworkUpdateOptions{DryRun: dryRun, Force: force}
		if vendorOnly {
			if err := generator.UpdateVendorCore(root, opts); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			return
		}
		if err := generator.UpdateFramework(root, opts); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		return
	}

	fmt.Println("To update the BFFX CLI, please use your package manager:")
	fmt.Println("  macOS / Linux: brew update && brew upgrade bffx")
	fmt.Println("  If installed manually, download the latest binary from GitHub Releases.")
	fmt.Println("\nTo update an existing project to the latest framework structure, run:")
	fmt.Println("  bffx update framework [--root DIR] [--dry-run] [--force]")
	fmt.Println("To refresh only the vendored framework core (.bffx/core):")
	fmt.Println("  bffx update framework --vendor-only [--root DIR] [--dry-run] [--force]")
	fmt.Println("From a generated project root, .bffx/framework_root or a walk-up to pkg/ is used; set BFFX_ROOT to override.")
}
