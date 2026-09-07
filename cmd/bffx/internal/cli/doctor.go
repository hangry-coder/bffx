package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/doctor"
)

func HandleDoctor(args []string) {
	root := "."
	strict := false
	offline := false
	envName := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		} else if args[i] == "--strict" {
			strict = true
		} else if args[i] == "--offline" || args[i] == "--skip-connectivity" {
			offline = true
		} else if args[i] == "--env" && i+1 < len(args) {
			envName = args[i+1]
			i++
		}
	}
	if envName != "" {
		os.Setenv("BFFX_ENV", envName)
	}
	if offline {
		os.Setenv("BFFX_OFFLINE", "true")
	}
	app.LoadEnv(root)
	results, err := doctor.Check(root)
	if err != nil {
		log.Fatalf("doctor check failed: %v", err)
	}

	fmt.Println("--- bffx doctor ---")
	failed := false
	for _, res := range results {
		icon := "✅"
		status := res.Status
		if strict && status == "warn" {
			status = "fail"
		}
		
		if status == "warn" {
			icon = "⚠️"
		} else if status == "fail" {
			icon = "❌"
			failed = true
		}
		fmt.Printf("%s %-25s: %s\n", icon, res.Name, res.Message)
	}

	if failed {
		os.Exit(1)
	}
}
