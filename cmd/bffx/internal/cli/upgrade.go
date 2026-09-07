package cli

import (
	"fmt"
	"log"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/compiler"
)

func HandleUpgrade(args []string) {
	root := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}
	app.LoadEnv(root)
	fmt.Println("Checking for schema drift...")
	changes, err := compiler.Sync(root, false)
	if err != nil {
		log.Fatalf("upgrade failed: %v", err)
	}

	if len(changes) == 0 {
		fmt.Println("No schema changes detected. Everything is up to date.")
		return
	}

	fmt.Printf("Detected %d changes:\n", len(changes))
	for _, c := range changes {
		action := "[+]"
		if c.Action == "removed" {
			action = "[-] (CRITICAL: potential data loss)"
		}
		fmt.Printf("  %s resource %q field %q %s\n", action, c.Resource, c.Field, c.Action)
	}
	fmt.Println("\nSync complete. Generated artifacts updated.")
}
