package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func HandleAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: bffx add [admin]")
		os.Exit(1)
	}

	root := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	reg, _ := manifest.LoadAll(root)
	layout := generator.LayoutLegacy
	if reg != nil && reg.Project != nil {
		if reg.ProjectSpec().Layout == "v2" {
			layout = generator.LayoutV2
		}
	}

	target := args[0]
	switch target {
	case "admin":
		if err := generator.EnableAdmin(root); err != nil {
			log.Fatalf("failed to enable admin: %v", err)
		}
		fmt.Println("Admin dashboard enabled in project.yaml.")
		fmt.Println("Note: You must set BFFX_ADMIN_SECRET environment variable to access the dashboard.")
	default:
		if err := generator.GenerateAddon(root, target, layout); err != nil {
			log.Fatalf("failed to add %s: %v", target, err)
		}
	}
}
