package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func HandleEnv(args []string) {
	if len(args) < 1 || args[0] != "check" {
		fmt.Println("usage: bffx env check")
		os.Exit(1)
	}

	root := "."
	for i := 1; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	app.LoadEnv(root)

	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("failed to load manifests: %v", err)
	}

	var projectSpec manifest.ProjectSpec
	reg.Project.UnmarshalSpec(&projectSpec)

	fmt.Println("--- bffx Environment Check ---")

	vars := []struct {
		name     string
		required bool
		hint     string
	}{
		{"BFFX_JWT_SECRET", projectSpec.Store.Mode != "memory", "Required for persistent auth in " + projectSpec.Store.Mode + " mode"},
		{"BFFX_WORKER_SECRET", false, "Optional: protects job result write-backs from worker"},
		{"REDIS_URL", projectSpec.Runtime.Redis.Enabled, "Required for async jobs since Redis is enabled"},
		{"BFFX_API_URL", projectSpec.Runtime.Worker.Enabled, "Required for worker write-backs"},
	}

	failed := false
	for _, v := range vars {
		val := os.Getenv(v.name)
		status := "OK"
		if val == "" {
			if v.required {
				status = "MISSING (REQUIRED)"
				failed = true
			} else {
				status = "MISSING (OPTIONAL)"
			}
		} else {
			// Mask secret values
			if len(val) > 8 {
				val = val[:4] + "****" + val[len(val)-4:]
			} else {
				val = "****"
			}
			status = "SET (" + val + ")"
		}
		fmt.Printf("%-20s %s\n", v.name+":", status)
		if val == "" && v.hint != "" {
			fmt.Printf("  -> %s\n", v.hint)
		}
	}

	if failed {
		fmt.Println("\nResult: FAIL - Missing required environment variables.")
		os.Exit(1)
	} else {
		fmt.Println("\nResult: PASS")
	}
}
