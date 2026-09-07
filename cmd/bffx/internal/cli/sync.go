package cli

import (
	"fmt"
	"log"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/compiler"
)

func HandleSync(args []string) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println("Usage: bffx sync [--dry-run] [--root DIR]")
			return
		}
	}
	root := "."
	dryRun := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--dry-run" {
			dryRun = true
		} else if arg == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}
	if err := app.BootstrapEnv(root); err != nil {
		log.Printf("[bffx] Warning: failed to bootstrap env: %v", err)
	}
	if _, err := compiler.Sync(root, dryRun); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
	if !dryRun {
		fmt.Println("sync complete")
	}
}
